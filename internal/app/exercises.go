package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"intentlens/internal/capture"
	"intentlens/internal/exercise"
	"intentlens/internal/explain"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Exercise struct {
	TargetFile string        `json:"target_file,omitempty"`
	ID         string        `json:"id"`
	Version    string        `json:"version"`
	Supersedes string        `json:"supersedes,omitempty"`
	SourceID   string        `json:"source_id,omitempty"`
	ClaimID    string        `json:"claim_id,omitempty"`
	PackageDir string        `json:"package_dir"`
	Created    string        `json:"created"`
	Spec       exercise.Spec `json:"spec"`
	Imported   bool          `json:"imported,omitempty"`
}
type ExerciseValidation struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Results   []exercise.Result `json:"results"`
	Execution *ExecutionPlan    `json:"execution,omitempty"`
}
type ExerciseAttempt struct {
	ID          string                 `json:"id"`
	Version     string                 `json:"version"`
	Created     string                 `json:"created"`
	Files       map[string]string      `json:"files"`
	Result      exercise.Result        `json:"result"`
	Behavior    []exercise.CheckResult `json:"behavior"`
	Provisional bool                   `json:"provisional"`
	Reason      string                 `json:"reason,omitempty"`
	Feedback    string                 `json:"feedback"`
	Execution   ExecutionPlan          `json:"execution"`
}
type ExerciseDisplay struct {
	Exercise
	Workspace   string             `json:"workspace"`
	NeedsReview bool               `json:"needs_review"`
	Validation  ExerciseValidation `json:"validation"`
	Attempts    []ExerciseAttempt  `json:"attempts"`
}

func (a *App) exerciseModel(ctx context.Context, key, prompt string) (string, error) {
	unlock, err := a.lock("analysis")
	if err != nil {
		return "", err
	}
	defer unlock()
	cfg, err := a.Config()
	if err != nil {
		return "", err
	}
	u, err := a.Usage()
	if err != nil {
		return "", err
	}
	if key == "" {
		key = os.Getenv(cfg.Model.APIKeyEnv)
	}
	if !cfg.Model.Enabled || strings.TrimSpace(key) == "" {
		return "", errors.New("请先确认项目模型发送范围并启用分析")
	}
	reserve := explain.StructuredReservation(prompt)
	if len(prompt) > 100000 || reserve > u.Remaining(cfg.Model.TokenLimit) {
		return "", fmt.Errorf("预算不足或上下文过大：本次保守预留 %d tokens，请主动调整预算或缩小范围", reserve)
	}
	reserved := u
	reserved.UncertainTokens += reserve
	if err = a.write("usage.json", reserved); err != nil {
		return "", err
	}
	run := a.exerciseRun
	if run == nil {
		run = explain.Structured
	}
	raw, updated, callErr := run(ctx, cfg.Model, u, key, prompt)
	if err = a.write("usage.json", updated); err != nil {
		return "", err
	}
	return raw, callErr
}

func (a *App) GenerateExercise(ctx context.Context, sourceID, claimID, file, parentID, key string) (Exercise, error) {
	source, err := a.ExplanationByID(sourceID)
	if err != nil {
		return Exercise{}, err
	}
	if source.Freshness != "已引用证据未变" || len(source.Corrections) > 0 {
		return Exercise{}, errors.New("原解释已过期或被标错，不能作为新题依据")
	}
	var selected *explain.Claim
	for i := range source.Document.Claims {
		c := &source.Document.Claims[i]
		if c.ID == claimID && c.Kind == "behavior" && c.Status == "supported" {
			selected = c
		}
	}
	if selected == nil {
		return Exercise{}, errors.New("请选择已获证据支持的行为说法")
	}
	target := false
	for _, ev := range source.Evidence {
		for _, id := range selected.EvidenceIDs {
			if ev.ID == id && ev.File == file && codeEvidence(ev) && ev.Role != "before" {
				target = true
			}
		}
	}
	if !target || !strings.HasSuffix(file, ".go") {
		return Exercise{}, errors.New("该行为没有可用于练习的当前 Go 文件依据")
	}
	if parentID != "" {
		parent, e := a.loadExercise(parentID)
		if e != nil || parent.SourceID != sourceID || parent.ClaimID != claimID {
			return Exercise{}, errors.New("新版本必须关联同一行为的原题")
		}
	}
	snap, err := capture.Take(a.Root)
	if err != nil {
		return Exercise{}, err
	}
	dir := filepath.ToSlash(filepath.Dir(file))
	if ok, e := capture.Allowed(a.Root, exercise.JoinSource(dir, exercise.AcceptanceFile)); e != nil || !ok {
		return Exercise{}, errors.New("验收测试路径已被排除，不能生成练习")
	}
	files := map[string]string{}
	entries, err := os.ReadDir(filepath.Join(a.Root, filepath.FromSlash(dir)))
	if err != nil {
		return Exercise{}, err
	}
	for _, entry := range entries {
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".s" || ext == ".c" || ext == ".h" || ext == ".syso" || name == "testdata" {
			return Exercise{}, errors.New("包依赖原生代码或外部测试资源，暂不生成可执行练习")
		}
		if ext != ".go" {
			continue
		}
		p := exercise.JoinSource(dir, name)
		f, ok := snap.Files[p]
		if !ok {
			return Exercise{}, errors.New("包包含被排除、链接或无法捕获的 Go 文件，不能构建完整练习")
		}
		files[name] = f.Content
	}
	if err = exercise.Qualify(files); err != nil {
		return Exercise{}, err
	}
	targetEvidence := []explain.Evidence{}
	for _, ev := range source.Evidence {
		for _, id := range selected.EvidenceIDs {
			if ev.ID == id && codeEvidence(ev) && ev.Visibility != "local" && filepath.ToSlash(filepath.Dir(ev.File)) == dir {
				targetEvidence = append(targetEvidence, ev)
			}
		}
	}
	if len(targetEvidence) == 0 {
		return Exercise{}, errors.New("所选行为没有获准用于练习的代码片段")
	}
	payload, _ := json.Marshal(map[string]any{"target_file": filepath.Base(file), "target_evidence": targetEvidence, "baseline": files})
	prompt := `根据给定 Go 包代码为熟悉编程的开发者生成一道小行为修改题，中文。仅使用代码，不推测作者动机。不得需要外部服务、依赖下载或凭据。只允许修改已有非测试 Go 文件；保持已有测试通过。返回 JSON {"title":"...","requirements":[{"id":"r1","text":"明确行为要求","test":"TestIntentLensName"}],"acceptance":"一个完整 Go 测试文件源码","reference":{"已有文件.go":"完整参考修改源码"}}。验收文件仅包含 TestIntentLens 前缀测试函数（不定义其他函数、init、TestMain），每个要求一项测试。所有新增测试必须在基线因预期行为缺失而失败，失败断言包含 INTENTLENS_EXPECTED_MISSING；参考修改使所有新旧测试通过。输出不包含 baseline。参考修改不会展示给作答者。材料：` + string(payload)
	prompt += "\n出题必须围绕 target_evidence 标识的行为片段；包其余代码仅用于构建和回归，不任意换成另一处练习。"
	raw, err := a.exerciseModel(ctx, key, prompt)
	if err != nil {
		return Exercise{}, err
	}
	var spec exercise.Spec
	if json.Unmarshal([]byte(raw), &spec) != nil {
		return Exercise{}, errors.New("题目模型输出不是有效 JSON")
	}
	spec.Baseline = files
	if len(spec.Reference) == 0 {
		return Exercise{}, errors.New("题目缺少内部验证参考修改")
	}
	if err = exercise.Check(spec); err != nil {
		return Exercise{}, err
	}
	latest, err := a.ExplanationByID(sourceID)
	if err != nil || latest.Freshness != "已引用证据未变" || len(latest.Corrections) > 0 {
		return Exercise{}, errors.New("生成期间依据变化，题目未保存")
	}
	current, err := capture.Take(a.Root)
	if err != nil {
		return Exercise{}, err
	}
	if current.PolicyHash != snap.PolicyHash {
		return Exercise{}, errors.New("生成期间排除规则变化，题目未保存")
	}
	for name, src := range files {
		if f, ok := current.Files[exercise.JoinSource(dir, name)]; !ok || f.Content != src {
			return Exercise{}, errors.New("生成期间包代码变化，题目未保存")
		}
	}
	e := Exercise{ID: newID(), Version: exercise.Version(spec), Supersedes: parentID, SourceID: sourceID, ClaimID: claimID, TargetFile: file, PackageDir: dir, Created: time.Now().UTC().Format(time.RFC3339Nano), Spec: spec}
	return e, a.write("exercise-"+e.ID+".json", e)
}
func (a *App) loadExercise(id string) (Exercise, error) {
	var e Exercise
	if err := a.scrubExcluded(); err != nil {
		return e, err
	}
	if !validID(id) {
		return e, errors.New("无效题目 ID")
	}
	if err := a.read("exercise-"+id+".json", &e); err != nil {
		return e, err
	}
	if e.ID != id || e.Version != exercise.Version(e.Spec) || (e.PackageDir != "." && !safeRelative(e.PackageDir)) {
		return e, errors.New("题目版本或路径无效")
	}
	if err := exercise.Check(e.Spec); err != nil {
		return e, err
	}
	for name := range e.Spec.Baseline {
		ok, err := capture.Allowed(a.Root, exercise.JoinSource(e.PackageDir, name))
		if err != nil || !ok {
			return e, errors.New("相关上下文已排除")
		}
	}
	return e, nil
}
func (a *App) exerciseNeedsReview(e Exercise) bool {
	if e.SourceID == "" {
		return false
	}
	source, err := a.ExplanationByID(e.SourceID)
	return err != nil || len(source.Corrections) > 0
}
func (a *App) exerciseDirectory(id string) (string, error) {
	if !validID(id) {
		return "", errors.New("无效题目 ID")
	}
	dir := filepath.Join(a.State, "exercise-work-"+id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil || resolved != dir || !contained(a.State, resolved) {
		return "", errors.New("练习目录不能是链接或越界")
	}
	return dir, nil
}
func (a *App) ExerciseView(id string) (ExerciseDisplay, error) {
	e, err := a.loadExercise(id)
	if err != nil {
		return ExerciseDisplay{}, err
	}
	d, err := a.exerciseDirectory(id)
	if err != nil {
		return ExerciseDisplay{}, err
	}
	view := ExerciseDisplay{Exercise: e, Workspace: filepath.Join(d, "answer"), NeedsReview: a.exerciseNeedsReview(e), Validation: ExerciseValidation{Status: "unverified", Version: e.Version}, Attempts: []ExerciseAttempt{}}
	view.Spec.Reference = nil
	if err = a.read("exercise-validation-"+id+".json", &view.Validation); err != nil && !errors.Is(err, os.ErrNotExist) {
		return view, err
	}
	if view.Validation.Version != e.Version {
		view.Validation = ExerciseValidation{Status: "unverified", Version: e.Version}
	}
	if view.NeedsReview {
		view.Validation.Status = "needs_review"
	}
	if err = a.read("exercise-attempts-"+id+".json", &view.Attempts); err != nil && !errors.Is(err, os.ErrNotExist) {
		return view, err
	}
	return view, nil
}
func (a *App) Exercises() ([]ExerciseDisplay, error) {
	if err := a.scrubExcluded(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(a.State)
	if err != nil {
		return nil, err
	}
	out := []ExerciseDisplay{}
	for _, entry := range entries {
		name := entry.Name()
		id := strings.TrimSuffix(strings.TrimPrefix(name, "exercise-"), ".json")
		if strings.HasPrefix(name, "exercise-") && strings.HasSuffix(name, ".json") && validID(id) {
			v, err := a.ExerciseView(id)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created > out[j].Created })
	return out, nil
}
