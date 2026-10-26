package app

import (
	"context"
	"errors"
	"fmt"
	"intentlens/internal/capture"
	"intentlens/internal/codeindex"
	"intentlens/internal/explain"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Inspection struct {
	Function     codeindex.Function   `json:"function"`
	Dependencies []codeindex.Function `json:"dependencies"`
	Evidence     []explain.Evidence   `json:"evidence"`
	Issues       []string             `json:"issues"`
	Coverage     codeindex.Coverage   `json:"coverage"`
	History      []Explanation        `json:"history"`
}
type Explanation struct {
	Supersedes  string             `json:"supersedes,omitempty"`
	Corrections []Correction       `json:"corrections,omitempty"`
	ID          string             `json:"id"`
	File        string             `json:"file"`
	FunctionID  string             `json:"function_id"`
	Created     string             `json:"created"`
	Document    explain.Document   `json:"document"`
	Evidence    []explain.Evidence `json:"evidence"`
	Freshness   string             `json:"freshness"`
}

func (a *App) Index() (codeindex.Index, error) {
	idx, _, _, e := a.buildIndex()
	return idx, e
}

func (a *App) buildIndex() (codeindex.Index, capture.Snapshot, map[string]string, error) {
	snap, coverage, e := capture.TakeGo(a.Root)
	if e != nil {
		return codeindex.Index{}, capture.Snapshot{}, nil, e
	}
	files := map[string]string{}
	for p, f := range snap.Files {
		files[p] = f.Content
	}
	idx, parseOmissions := codeindex.BuildDetailed(files)
	idx.Coverage = codeindex.Coverage{Complete: coverage.Complete, Scope: coverage.Scope, BudgetBytes: coverage.BudgetBytes, CapturedBytes: coverage.CapturedBytes, OmittedCount: coverage.OmittedCount, ExcludedCount: coverage.ExcludedCount, Truncated: coverage.Truncated, OmissionReasons: coverage.OmissionReasons}
	idx.Coverage.Omitted = []codeindex.CoverageOmission{}
	for _, omission := range coverage.Omitted {
		idx.Coverage.Omitted = append(idx.Coverage.Omitted, codeindex.CoverageOmission{File: omission.File, Reason: omission.Reason})
		idx.Issues = append(idx.Issues, "未捕获："+omission.File+"（"+omission.Reason+"）")
	}
	parseFiles := make([]string, 0, len(parseOmissions))
	for file := range parseOmissions {
		parseFiles = append(parseFiles, file)
	}
	sort.Strings(parseFiles)
	for _, file := range parseFiles {
		reason := parseOmissions[file]
		idx.Coverage.Complete = false
		idx.Coverage.OmittedCount++
		if len(idx.Coverage.Omitted) < 20 {
			idx.Coverage.Omitted = append(idx.Coverage.Omitted, codeindex.CoverageOmission{File: file, Reason: reason})
		} else {
			idx.Coverage.Truncated = true
		}
	}
	for _, p := range snap.Omitted {
		idx.Issues = append(idx.Issues, "未捕获："+p)
	}
	return idx, snap, parseOmissions, nil
}

func indexOmissionError(reason string) error {
	labels := map[string]string{
		"snapshot_budget":        "Go 源码总预算已耗尽，当前文件未纳入",
		"file_too_large":         "当前文件超过 1 MiB 单文件上限",
		"parse_error":            "当前文件的 Go 语法无法完整解析",
		"binary_or_invalid_utf8": "当前文件不是可索引的 UTF-8 文本",
		"unsafe_path":            "当前文件路径包含链接或不安全路径",
	}
	label := labels[reason]
	if label == "" {
		label = "当前文件未纳入索引"
	}
	return fmt.Errorf("索引不完整：%s（%s）；请查看索引范围与遗漏原因", label, reason)
}

func (a *App) Inspect(file string, line int) (Inspection, error) {
	result := Inspection{Dependencies: []codeindex.Function{}, Evidence: []explain.Evidence{}, Issues: []string{}, History: []Explanation{}}
	if e := a.scrubExcluded(); e != nil {
		return result, e
	}
	file = filepath.ToSlash(file)
	allowed, e := capture.Allowed(a.Root, file)
	if e != nil {
		return result, e
	}
	if !allowed {
		return result, errors.New("相关上下文已排除：默认排除规则或 .intentignore 限制了索引范围；请查看索引范围与遗漏原因")
	}
	idx, snap, parseOmissions, e := a.buildIndex()
	if e != nil {
		return result, e
	}
	result.Issues = idx.Issues
	result.Coverage = idx.Coverage
	found := false
	byID := map[string]codeindex.Function{}
	for _, f := range idx.Functions {
		byID[f.ID] = f
		if f.File == file && line >= f.StartLine && line <= f.EndLine {
			result.Function = f
			found = true
		}
	}
	if !found {
		if reason, ok := parseOmissions[file]; ok {
			return result, indexOmissionError(reason)
		}
		if reason, ok := idx.Coverage.OmissionReasons[file]; ok {
			return result, indexOmissionError(reason)
		}
		if _, captured := snap.Files[file]; captured {
			return result, errors.New("当前位置没有可解析的 Go 函数")
		}
		if !idx.Coverage.Complete {
			return result, errors.New("索引不完整：预算或捕获规则遗漏了部分 Go 源码")
		}
		return result, errors.New("当前位置没有可解析的 Go 函数")
	}
	functions := []codeindex.Function{result.Function}
	seen := map[string]bool{result.Function.ID: true}
	for _, call := range result.Function.Calls {
		if call.Status != "resolved" {
			result.Issues = append(result.Issues, "调用目标未唯一确定："+call.Name)
			continue
		}
		for _, id := range call.Targets {
			if f, ok := byID[id]; ok && !seen[id] {
				seen[id] = true
				result.Dependencies = append(result.Dependencies, f)
				functions = append(functions, f)
			}
		}
	}
	for _, f := range functions {
		result.Evidence = append(result.Evidence, explain.Evidence{ID: hash(f.ID + f.Source)[:20], File: f.File, StartLine: f.StartLine, EndLine: f.EndLine, Hash: hash(f.Source), Source: f.Source})
	}
	entries, e := os.ReadDir(a.State)
	if e != nil {
		return result, e
	}
	for _, ent := range entries {
		if !strings.HasPrefix(ent.Name(), "explanation-") {
			continue
		}
		var saved Explanation
		if e = a.read(ent.Name(), &saved); e != nil {
			return result, e
		}
		if saved.FunctionID != result.Function.ID {
			continue
		}
		visible := true
		saved.Freshness = "已引用证据未变"
		for _, ev := range saved.Evidence {
			ok, err := a.evidenceAllowed(ev)
			if err != nil {
				return result, err
			}
			if !ok {
				visible = false
				break
			}
		}
		saved.Freshness = a.freshnessFromSnapshot(saved.Evidence, snap)
		if visible {
			if e = a.attachCorrections(&saved); e != nil {
				return result, e
			}
			result.History = append(result.History, saved)
		} else {
			result.Issues = append(result.Issues, "一条历史解释因证据已被排除而隐藏")
		}
	}
	return result, nil
}

func (a *App) Explain(ctx context.Context, file string, line int, question, key string) (Explanation, error) {
	info, e := a.Inspect(file, line)
	if e != nil {
		return Explanation{}, e
	}
	if question == "" {
		question = "解释当前函数的执行流程、必要依赖和有依据的实现取舍。选择动机未记录；测试没有执行。"
	}
	return a.generate(ctx, file, info.Function.ID, question, key, info.Evidence)
}

func (a *App) generate(ctx context.Context, file, functionID, question, key string, evidence []explain.Evidence) (Explanation, error) {
	doc, e := a.analyze(ctx, question, key, evidence)
	if e != nil {
		return Explanation{}, e
	}
	saved := Explanation{ID: newID(), File: file, FunctionID: functionID, Created: time.Now().UTC().Format(time.RFC3339Nano), Document: doc, Evidence: evidence, Freshness: "生成时证据；读取时重新检查"}
	return saved, a.write("explanation-"+saved.ID+".json", saved)
}

func (a *App) analyze(ctx context.Context, question, key string, evidence []explain.Evidence) (explain.Document, error) {
	unlock, e := a.lock("analysis")
	if e != nil {
		return explain.Document{}, e
	}
	defer unlock()
	settings, e := a.Config()
	if e != nil {
		return explain.Document{}, e
	}
	usage, e := a.Usage()
	if e != nil {
		return explain.Document{}, e
	}
	if !settings.Model.Enabled {
		return explain.Document{}, errors.New("模型分析未启用")
	}
	if key == "" {
		key = os.Getenv(settings.Model.APIKeyEnv)
	}
	if strings.TrimSpace(key) == "" {
		return explain.Document{}, fmt.Errorf("需要密钥环境变量 %s；请在启动编辑器或 hook 的环境中配置", settings.Model.APIKeyEnv)
	}
	reserve := explain.ReservationForMode(settings.Model.VerificationMode, question, evidence)
	if reserve > usage.Remaining(settings.Model.TokenLimit) {
		return explain.Document{}, fmt.Errorf("预算不足；本次保守预留 %d tokens", reserve)
	}
	reserved := usage
	reserved.UncertainTokens += reserve
	if e = a.write("usage.json", reserved); e != nil {
		return explain.Document{}, e
	}
	runner := a.modelRun
	if runner == nil {
		runner = explain.Run
	}
	doc, updated, callErr := runner(ctx, settings.Model, usage, key, question, evidence)
	if e = a.write("usage.json", updated); e != nil {
		return explain.Document{}, e
	}
	if callErr != nil {
		return explain.Document{}, callErr
	}
	// Policy may change while the remote request is running: do not persist newly excluded sources.
	for _, ev := range evidence {
		ok, err := a.evidenceAllowed(ev)
		if err != nil {
			return explain.Document{}, err
		}
		if !ok {
			return explain.Document{}, errors.New("分析期间排除规则变化；结果未保存")
		}
	}
	return doc, nil
}

func (a *App) ExplainRound(ctx context.Context, r Round, idx codeindex.Index, key string) (Explanation, error) {
	evidence := []explain.Evidence{}
	size := 0
	for _, change := range r.Changes {
		for i, f := range []*capture.File{change.Before, change.After} {
			if f == nil {
				continue
			}
			size += len(f.Content)
			if size > 40000 {
				return Explanation{}, errors.New("本轮内容超出自动解释范围；请在代码位置按需展开")
			}
			role := "before"
			if i == 1 {
				role = "after"
			}
			evidence = append(evidence, explain.Evidence{ID: hash(change.Path + role + f.Content)[:20], Role: role, File: change.Path, StartLine: 1, EndLine: strings.Count(f.Content, "\n") + 1, Hash: f.Hash, Source: f.Content})
		}
	}
	if len(evidence) == 0 {
		return Explanation{}, errors.New("没有可分析的净变化")
	}
	return a.generate(ctx, "", "round:"+r.ID, "按行为变化汇总本轮 before 到 after 的净变化，关联函数与测试。仅说明证据可支持的变化；选择动机未记录，测试未执行。", key, evidence)
}
