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
	"time"
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
		if err = exercise.Materialize(answer, exercise.Files(e.Spec, false)); err != nil {
			return ExecutionPlan{}, err
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
	plan := ExecutionPlan{ExerciseID: id, Version: e.Version, Tool: tool, Args: exercise.Args(), Directories: []string{}, Workspace: answer, Scope: append(exercise.Scope(files), exercise.Scope(exercise.Files(e.Spec, true))...), SourceDigest: exercise.Hash(files), PolicyHash: policy.Hash(), Platform: runtime.GOOS + "/" + runtime.GOARCH, Limits: "只执行本题单包 Go 测试；测试超时 10s，单进程总超时 60s，输出上限 1 MiB；模块下载关闭，不继承模型密钥。独立目录只隔离文件，不是 OS 沙箱，无法保证代码不访问网络或目录外资源。"}
	for _, stage := range []string{"baseline", "missing", "reference", "answer-check"} {
		plan.Directories = append(plan.Directories, filepath.Join(dir, stage))
	}
	plan.Digest = digest(plan)
	return plan, nil
}

// resetExerciseStage touches only a named child of the canonical private exercise directory.
func resetExerciseStage(base, dir string, files map[string]string) error {
	if filepath.Dir(dir) != base || !contained(base, dir) {
		return errors.New("执行目录越界")
	}
	if info, err := os.Lstat(dir); err == nil {
		if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return errors.New("执行目录不能是链接")
		}
		resolved, err := filepath.EvalSymlinks(dir)
		if err != nil || resolved != dir {
			return errors.New("执行路径不安全")
		}
		if err = os.RemoveAll(dir); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return exercise.Materialize(dir, files)
}
func (a *App) RunExercise(ctx context.Context, id, previewDigest string, confirmed bool, mode string) (ExerciseDisplay, error) {
	if mode != "validate" && mode != "score" {
		return ExerciseDisplay{}, errors.New("未知执行模式")
	}
	unlock, err := a.lock("exercise")
	if err != nil {
		return ExerciseDisplay{}, err
	}
	defer unlock()
	plan, err := a.PrepareExecution(id)
	if err != nil {
		return ExerciseDisplay{}, err
	}
	if plan.Digest != previewDigest {
		return ExerciseDisplay{}, errors.New("命令、目录、依赖或范围变化，请重新预览并确认执行")
	}
	var previous ExecutionPlan
	if !confirmed {
		if a.read("exercise-auth-"+id+".json", &previous) != nil || previous.Digest != plan.Digest {
			return ExerciseDisplay{}, errors.New("尚未确认本次练习的命令、目录与执行范围")
		}
	} else if err = a.write("exercise-auth-"+id+".json", plan); err != nil {
		return ExerciseDisplay{}, err
	}
	e, err := a.loadExercise(id)
	if err != nil {
		return ExerciseDisplay{}, err
	}
	dir, err := a.exerciseDirectory(id)
	if err != nil {
		return ExerciseDisplay{}, err
	}
	run := func(stage int, files map[string]string) (exercise.Result, error) {
		if _, err := a.loadExercise(id); err != nil {
			return exercise.Result{}, err
		}
		policy, err := capture.LoadPolicy(a.Root)
		if err != nil || policy.Hash() != plan.PolicyHash {
			return exercise.Result{}, errors.New("执行前排除范围变化，请重新确认")
		}
		if err := resetExerciseStage(dir, plan.Directories[stage], files); err != nil {
			return exercise.Result{}, err
		}
		return exercise.Run(ctx, plan.Tool, plan.Directories[stage], filepath.Join(dir, "cache")), nil
	}
	if mode == "validate" {
		validation := ExerciseValidation{Status: "failed", Version: e.Version, Results: []exercise.Result{}, Execution: &plan}
		base, err := run(0, e.Spec.Baseline)
		if err != nil {
			return ExerciseDisplay{}, err
		}
		validation.Results = append(validation.Results, base)
		red, err := run(1, exercise.Files(e.Spec, false))
		if err != nil {
			return ExerciseDisplay{}, err
		}
		validation.Results = append(validation.Results, red)
		if len(e.Spec.Reference) > 0 {
			green, err := run(2, exercise.Files(e.Spec, true))
			if err != nil {
				return ExerciseDisplay{}, err
			}
			validation.Results = append(validation.Results, green)
			if exercise.Validated(e.Spec, base, red, green) {
				validation.Status = "validated"
			}
		} else {
			validation.Status = "unverified_reference_unavailable"
		}
		if _, err = a.loadExercise(id); err != nil {
			return ExerciseDisplay{}, err
		}
		if err = a.write("exercise-validation-"+id+".json", validation); err != nil {
			return ExerciseDisplay{}, err
		}
	} else {
		files, err := a.readExerciseAnswer(e, plan.Workspace)
		if err != nil {
			return ExerciseDisplay{}, err
		}
		if exercise.Hash(files) != plan.SourceDigest {
			return ExerciseDisplay{}, errors.New("作答代码在确认后变化，请重新预览并确认执行")
		}
		result, err := run(3, files)
		if err != nil {
			return ExerciseDisplay{}, err
		}
		view, err := a.ExerciseView(id)
		if err != nil {
			return view, err
		}
		attempt := ExerciseAttempt{ID: newID(), Version: e.Version, Created: time.Now().UTC().Format(time.RFC3339Nano), Files: files, Result: result, Behavior: exercise.Behavior(e.Spec, result), Provisional: view.Validation.Status != "validated", Feedback: "尚未请求模型理由反馈；测试结果不代表完全理解", Execution: plan}
		view.Attempts = append(view.Attempts, attempt)
		if err = a.write("exercise-attempts-"+id+".json", view.Attempts); err != nil {
			return view, err
		}
	}
	return a.ExerciseView(id)
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
