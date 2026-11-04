package app

import (
	"context"
	"errors"
	"intentlens/internal/capture"
	"intentlens/internal/exercise"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type ExecutionPlan struct {
	ExerciseID   string              `json:"exercise_id"`
	Version      string              `json:"version"`
	Tool         exercise.Executable `json:"tool"`
	Args         []string            `json:"args"`
	Directories  []string            `json:"directories"`
	Workspace    string              `json:"workspace"`
	Scope        []string            `json:"scope"`
	SourceDigest string              `json:"source_digest"`
	PolicyHash   string              `json:"policy_hash"`
	Platform     string              `json:"platform"`
	Limits       string              `json:"limits"`
	Digest       string              `json:"digest"`
}

const exerciseExecutionLimits = "执行已关闭：当前没有经过验证的 OS 沙箱；任何确认或旧授权都不会启动本机 Go 测试。"

func (a *App) PrepareExecution(id string) (ExecutionPlan, error) {
	e, err := a.loadExercise(id)
	if err != nil {
		return ExecutionPlan{}, err
	}
	if a.exerciseNeedsReview(e) {
		return ExecutionPlan{}, errors.New("题目依据已标错或无法定位，需要复核；旧成绩保留")
	}
	dir, err := a.exerciseDirectory(id)
	if err != nil {
		return ExecutionPlan{}, err
	}
	answer := filepath.Join(dir, "answer")
	if _, err = os.Lstat(answer); errors.Is(err, os.ErrNotExist) {
		if materializeErr := exercise.Materialize(answer, exercise.Files(e.Spec, false)); materializeErr != nil {
			return ExecutionPlan{}, materializeErr
		}
	} else if err != nil {
		return ExecutionPlan{}, err
	}
	files, err := a.readExerciseAnswer(e, answer)
	if err != nil {
		return ExecutionPlan{}, err
	}
	tool, err := exercise.Tool()
	if err != nil {
		return ExecutionPlan{}, err
	}
	policy, err := capture.LoadPolicy(a.Root)
	if err != nil {
		return ExecutionPlan{}, err
	}
	plan := ExecutionPlan{ExerciseID: id, Version: e.Version, Tool: tool, Args: exercise.Args(), Directories: []string{}, Workspace: answer, Scope: append(exercise.Scope(files), exercise.Scope(exercise.Files(e.Spec, true))...), SourceDigest: exercise.Hash(files), PolicyHash: policy.Hash(), Platform: runtime.GOOS + "/" + runtime.GOARCH, Limits: exerciseExecutionLimits}
	for _, stage := range []string{"baseline", "missing", "reference", "answer-check"} {
		plan.Directories = append(plan.Directories, filepath.Join(dir, stage))
	}
	plan.Digest = digest(plan)
	return plan, nil
}

func (a *App) RunExercise(_ context.Context, _, _ string, _ bool, _ string) (ExerciseDisplay, error) {
	return ExerciseDisplay{}, exercise.ErrSandboxUnavailable
}

type ExerciseChange struct {
	File   string `json:"file"`
	Before string `json:"before"`
	After  string `json:"after"`
}
type ExerciseApply struct {
	ExerciseID string           `json:"exercise_id"`
	Version    string           `json:"version"`
	Changes    []ExerciseChange `json:"changes"`
	PolicyHash string           `json:"policy_hash"`
	Digest     string           `json:"digest"`
}

func (a *App) PrepareExerciseApply(id string) (ExerciseApply, error) {
	e, err := a.loadExercise(id)
	if err != nil {
		return ExerciseApply{}, err
	}
	if a.exerciseNeedsReview(e) {
		return ExerciseApply{}, errors.New("题目依据需复核，不能直接应用")
	}
	dir, err := a.exerciseDirectory(id)
	if err != nil {
		return ExerciseApply{}, err
	}
	files, err := a.readExerciseAnswer(e, filepath.Join(dir, "answer"))
	if err != nil {
		return ExerciseApply{}, err
	}
	snap, _, err := capture.TakeGo(a.Root)
	if err != nil {
		return ExerciseApply{}, err
	}
	plan := ExerciseApply{ExerciseID: id, Version: e.Version, Changes: []ExerciseChange{}, PolicyHash: snap.PolicyHash}
	for name, src := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		baseline, ok := e.Spec.Baseline[name]
		if !ok {
			return plan, errors.New("新增源码不能直接应用；请手动处理")
		}
		if src == baseline {
			continue
		}
		path := exercise.JoinSource(e.PackageDir, name)
		current, ok := snap.Files[path]
		if !ok || current.Content != baseline {
			return plan, errors.New("目标文件与题目基线冲突；保留练习结果，请先处理冲突")
		}
		plan.Changes = append(plan.Changes, ExerciseChange{path, baseline, src})
	}
	sort.Slice(plan.Changes, func(i, j int) bool { return plan.Changes[i].File < plan.Changes[j].File })
	if len(plan.Changes) > 1 {
		return ExerciseApply{}, errors.New("多文件应用已关闭；一次仅允许单个源码文件，未改动任何文件")
	}
	if len(plan.Changes) == 0 {
		return plan, errors.New("没有可应用的源码修改")
	}
	plan.Digest = digest(plan)
	return plan, nil
}
func (a *App) ConfirmExerciseApply(id, previewDigest string, confirmed bool) error {
	if !confirmed {
		return errors.New("需要确认应用差异")
	}
	unlock, err := a.lock("exercise")
	if err != nil {
		return err
	}
	defer unlock()
	plan, err := a.PrepareExerciseApply(id)
	if err != nil {
		return err
	}
	if previewDigest != plan.Digest {
		return errors.New("答案或目标已变化，请重新预览")
	}
	return a.applyChanges(plan)
}
