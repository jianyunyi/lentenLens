package app

import (
	"context"
	"encoding/json"
	"errors"
	"intentlens/internal/capture"
	"intentlens/internal/exercise"
	"io"
	"os"
	"strings"
)

type SharedExercise struct {
	Schema           int           `json:"schema"`
	ID               string        `json:"id"`
	Version          string        `json:"version"`
	Supersedes       string        `json:"supersedes,omitempty"`
	PackageDir       string        `json:"package_dir"`
	Spec             exercise.Spec `json:"spec"`
	AuthorValidation string        `json:"author_validation"`
}
type ExerciseShareDraft struct {
	Record     SharedExercise `json:"record"`
	Digest     string         `json:"digest"`
	PolicyHash string         `json:"policy_hash"`
}

func (a *App) PrepareExerciseShare(id string) (ExerciseShareDraft, error) {
	e, err := a.loadExercise(id)
	if err != nil {
		return ExerciseShareDraft{}, err
	}
	v, err := a.ExerciseView(id)
	if err != nil {
		return ExerciseShareDraft{}, err
	}
	if v.NeedsReview || v.Validation.Status != "validated" {
		return ExerciseShareDraft{}, errors.New("题目未完成三项验证或依据需复核，不能共享为可执行题目")
	}
	spec := e.Spec
	spec.Reference = nil
	record := SharedExercise{Schema: 1, ID: e.ID, Version: e.Version, PackageDir: e.PackageDir, Spec: spec, AuthorValidation: "作者环境通过：原测试、基线预期失败、参考修改通过；接收者本地尚未验证"}
	if e.Supersedes != "" {
		records, _ := a.SharedExercises()
		for _, r := range records {
			if r.ID == e.Supersedes {
				record.Supersedes = e.Supersedes
			}
		}
	}
	policy, err := capture.LoadPolicy(a.Root)
	if err != nil {
		return ExerciseShareDraft{}, err
	}
	d := ExerciseShareDraft{Record: record, Digest: digest(record), PolicyHash: policy.Hash()}
	return d, a.write("exercise-share-"+id+".json", d)
}
func (a *App) ConfirmExerciseShare(id, previewDigest string, confirm bool) error {
	if !confirm || !validID(id) {
		return errors.New("需要确认题目共享预览")
	}
	unlock, err := a.lock("exercise")
	if err != nil {
		return err
	}
	defer unlock()
	var draft ExerciseShareDraft
	if err = a.read("exercise-share-"+id+".json", &draft); err != nil {
		return err
	}
	if draft.Digest != previewDigest || digest(draft.Record) != previewDigest {
		return errors.New("题目预览摘要变化")
	}
	latest, err := a.PrepareExerciseShare(id)
	if err != nil {
		return err
	}
	if latest.Digest != draft.Digest || latest.PolicyHash != draft.PolicyHash {
		return errors.New("依据、题目或排除范围变化，请重新预览")
	}
	root, err := a.exerciseSharedDir(true)
	if err != nil {
		return err
	}
	defer root.Close()
	b, _ := json.MarshalIndent(draft.Record, "", "  ")
	if len(b) > maxShareBytes {
		return errors.New("题目共享记录过大")
	}
	f, err := root.OpenFile(id+".json", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("题目已共享或目标不可写，未覆盖已有版本")
	}
	_, err = f.Write(b)
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		_ = root.Remove(id + ".json")
	}
	return err
}
func (a *App) exerciseSharedDir(create bool) (*os.Root, error) {
	root, err := os.OpenRoot(a.Root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	for _, p := range []string{".intent", ".intent/exercises"} {
		info, e := root.Lstat(p)
		if errors.Is(e, os.ErrNotExist) && create {
			if e = root.Mkdir(p, 0700); e != nil && !errors.Is(e, os.ErrExist) {
				return nil, e
			}
			info, e = root.Lstat(p)
		}
		if e != nil {
			return nil, e
		}
		if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return nil, errors.New("共享题目目录不能是链接")
		}
	}
	return root.OpenRoot(".intent/exercises")
}
func (a *App) SharedExercises() ([]SharedExercise, error) {
	out := []SharedExercise{}
	root, err := a.exerciseSharedDir(false)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	entries, err := dir.ReadDir(-1)
	dir.Close()
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := root.Lstat(entry.Name())
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxShareBytes {
			return nil, errors.New("共享题目文件不安全或过大")
		}
		f, err := root.Open(entry.Name())
		if err != nil {
			return nil, err
		}
		decoder := json.NewDecoder(io.LimitReader(f, maxShareBytes+1))
		decoder.DisallowUnknownFields()
		var r SharedExercise
		err = decoder.Decode(&r)
		var extra any
		end := decoder.Decode(&extra)
		f.Close()
		if err != nil || end != io.EOF || r.Schema != 1 || !validID(r.ID) || entry.Name() != r.ID+".json" || r.Version != exercise.Version(r.Spec) || len(r.Spec.Reference) > 0 || (r.PackageDir != "." && !safeRelative(r.PackageDir)) {
			return nil, errors.New("共享题目版本、字段或路径无效")
		}
		if err = exercise.Check(r.Spec); err != nil {
			return nil, err
		}
		allowed := true
		for name := range exercise.Files(r.Spec, false) {
			ok, e := capture.Allowed(a.Root, exercise.JoinSource(r.PackageDir, name))
			if e != nil {
				return nil, e
			}
			allowed = allowed && ok
		}
		if allowed {
			out = append(out, r)
		}
	}
	return out, nil
}
func (a *App) ImportExercise(id string) (ExerciseDisplay, error) {
	if !validID(id) {
		return ExerciseDisplay{}, errors.New("无效题目 ID")
	}
	records, err := a.SharedExercises()
	if err != nil {
		return ExerciseDisplay{}, err
	}
	for _, r := range records {
		if r.ID != id {
			continue
		}
		if _, err = a.loadExercise(id); err == nil {
			return a.ExerciseView(id)
		}
		e := Exercise{ID: r.ID, Version: r.Version, Supersedes: r.Supersedes, PackageDir: r.PackageDir, Spec: r.Spec, Imported: true}
		if err = a.write("exercise-"+id+".json", e); err != nil {
			return ExerciseDisplay{}, err
		}
		return a.ExerciseView(id)
	}
	return ExerciseDisplay{}, errors.New("共享题目不存在或已被排除")
}
func (a *App) ExerciseFeedback(ctx context.Context, id, attemptID, reason, key string) (ExerciseDisplay, error) {
	if strings.TrimSpace(reason) == "" || len(reason) > 4000 {
		return ExerciseDisplay{}, errors.New("请输入不超过 4000 字节的理由说明")
	}
	unlock, err := a.lock("exercise")
	if err != nil {
		return ExerciseDisplay{}, err
	}
	defer unlock()
	view, err := a.ExerciseView(id)
	if err != nil {
		return view, err
	}
	if view.NeedsReview {
		return view, errors.New("题目依据需复核")
	}
	for i := range view.Attempts {
		attempt := &view.Attempts[i]
		if attempt.ID != attemptID {
			continue
		}
		payload, _ := json.Marshal(map[string]any{"requirements": view.Spec.Requirements, "answer": attempt.Files, "result": attempt.Result, "reason": reason})
		raw, err := a.exerciseModel(ctx, key, `仅评价用户对修改理由的解释质量，返回 JSON {"feedback":"中文补充反馈"}。不要给分，不改变测试结果，不宣称完全理解或测试未覆盖的正确性。测试失败必须保留，未验证题目反馈只能暂定。材料：`+string(payload))
		if err != nil {
			return view, err
		}
		var response struct {
			Feedback string `json:"feedback"`
		}
		if json.Unmarshal([]byte(raw), &response) != nil || response.Feedback == "" || len(response.Feedback) > 12000 {
			return view, errors.New("理由反馈 JSON 无效")
		}
		if _, err = a.loadExercise(id); err != nil {
			return view, err
		}
		attempt.Reason = reason
		attempt.Feedback = response.Feedback
		if err = a.write("exercise-attempts-"+id+".json", view.Attempts); err != nil {
			return view, err
		}
		return a.ExerciseView(id)
	}
	return view, errors.New("对应版本的作答不存在")
}
