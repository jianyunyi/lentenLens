package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"intentlens/internal/capture"
	"intentlens/internal/codeindex"
	"intentlens/internal/explain"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
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

func oldFunctionID(f codeindex.Function) string { return f.File + ":" + f.Name }

func legacyFunctionID(file, id string) (string, bool) {
	prefix := file + ":"
	if file == "" || !strings.HasPrefix(id, prefix) {
		return "", false
	}
	name := strings.TrimPrefix(id, prefix)
	if at := strings.LastIndex(name, "@"); at >= 0 {
		position := strings.Split(name[at+1:], ":")
		if len(position) != 2 {
			return "", false
		}
		for _, part := range position {
			value, err := strconv.Atoi(part)
			if err != nil || value <= 0 {
				return "", false
			}
		}
		name = name[:at]
	}
	if name == "" {
		return "", false
	}
	return prefix + name, true
}

func findFunctionByHistoricalID(functions []codeindex.Function, saved Explanation) (codeindex.Function, bool) {
	for _, f := range functions {
		if f.ID == saved.FunctionID {
			return f, true
		}
	}
	legacyID, ok := legacyFunctionID(saved.File, saved.FunctionID)
	if !ok {
		return codeindex.Function{}, false
	}
	var candidate codeindex.Function
	matches := 0
	for _, f := range functions {
		if f.File == saved.File && oldFunctionID(f) == legacyID {
			candidate = f
			matches++
		}
	}
	if matches != 1 {
		return codeindex.Function{}, false
	}
	return candidate, true
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
		if a.pathAbsent(file) {
			return result, errors.New("源码文件不存在，无法定位当前函数")
		}
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
		if readErr := a.read(ent.Name(), &saved); readErr != nil {
			return result, readErr
		}
		historicalFunction, matches := findFunctionByHistoricalID(idx.Functions, saved)
		if !matches || historicalFunction.ID != result.Function.ID {
			continue
		}
		visible := true
		saved.Freshness = "已引用证据未变"
		for i := range saved.Evidence {
			ok, err := a.evidenceAllowed(saved.Evidence[i])
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
			if correctionsErr := a.attachCorrections(&saved); correctionsErr != nil {
				return result, correctionsErr
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

const maxSelectionBytes = 16 * 1024
const maxSelectionDiagnostics = 100

func (a *App) requireSelectionFileAllowed(file string) error {
	if strings.TrimSpace(file) == "" {
		return errors.New("缺少当前文件路径，无法应用排除规则；未调用模型")
	}
	allowed, err := capture.Allowed(a.Root, filepath.ToSlash(file))
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New("当前文件已排除：默认排除规则或 .intentignore 限制了选区解释；未调用模型")
	}
	return nil
}

func (a *App) ExplainSelection(ctx context.Context, file, source, language string, startLine, endLine, savedVersion int, selectionRange explain.EditorRange, diagnostics []explain.EditorDiagnostic, key string) (explain.SelectionResult, error) {
	result := explain.SelectionResult{Claims: []explain.Claim{}, Diagnostics: []explain.SelectionDiagnostic{}, DiagnosticsStatus: "no_errors", Unknowns: []string{}, Evidence: []explain.Evidence{}, SavedVersion: savedVersion}
	if err := a.requireSelectionFileAllowed(file); err != nil {
		return result, err
	}
	if err := validateSelectionSource(source, language); err != nil {
		return result, err
	}
	if err := validateSelectionCoordinates(startLine, endLine, savedVersion, selectionRange); err != nil {
		return result, err
	}
	for _, diagnostic := range diagnostics {
		mapped, ok := mapSelectionDiagnostic(diagnostic, selectionRange)
		if ok {
			result.Diagnostics = append(result.Diagnostics, mapped)
		}
	}
	if len(result.Diagnostics) > maxSelectionDiagnostics {
		return result, errors.New("选区内错误诊断超过安全上限，未发送给模型")
	}
	if len(result.Diagnostics) > 0 {
		result.DiagnosticsStatus = "errors_found"
	}
	evidence := []explain.Evidence{{Kind: "selection", Language: language, ID: "selected-code", StartLine: startLine, EndLine: endLine, Source: source}}
	if len(result.Diagnostics) > 0 {
		diagnosticJSON, err := json.Marshal(result.Diagnostics)
		if err != nil {
			return result, errors.New("选区诊断无法编码")
		}
		evidence = append(evidence, explain.Evidence{Kind: "diagnostics", Language: "json", ID: "selected-diagnostics", StartLine: startLine, EndLine: endLine, Source: string(diagnosticJSON)})
	}
	encoded, err := json.Marshal(evidence)
	if err != nil || len(encoded) > 60000 {
		return result, errors.New("选区代码与错误诊断编码后超过模型输入安全范围，请缩小选区")
	}
	question := "请解释这段选中代码的意义和可能用途、可见的输入输出/副作用、主要执行流程，以及一种或多种有依据的设计考虑；明确区分直接可见的事实与可能的工程取舍，不臆测作者真实动机。若可见新功能实现，请说明选区能支持的实现机制，并将缺失上下文列为未知。"
	doc, err := a.analyzeSelection(ctx, question, key, file, evidence)
	if err != nil {
		return explain.SelectionResult{}, err
	}
	result.Claims = doc.Claims
	result.Unknowns = doc.Unknowns
	result.Evidence = evidence
	return result, nil
}

func validateSelectionSource(source, language string) error {
	if !utf8.ValidString(source) || strings.TrimSpace(source) == "" {
		return errors.New("请先选中非空代码")
	}
	if len(source) > maxSelectionBytes {
		return errors.New("选中代码超过 16 KiB 安全上限，请缩小选区")
	}
	if language == "" || language == "plaintext" || len(language) > 64 || strings.ContainsAny(language, "\r\n\x00") {
		return errors.New("编辑器语言标识无效或不受支持")
	}
	return nil
}

func validateSelectionCoordinates(startLine, endLine, savedVersion int, selectionRange explain.EditorRange) error {
	if startLine < 1 || endLine < startLine || endLine > 100000000 || savedVersion < 1 || !validEditorRange(selectionRange) {
		return errors.New("选区坐标或已保存版本无效")
	}
	expectedEndLine := selectionRange.End.Line + 1
	if selectionRange.End.Character == 0 && selectionRange.End.Line > selectionRange.Start.Line {
		expectedEndLine--
	}
	if expectedEndLine < startLine {
		expectedEndLine = startLine
	}
	if startLine != selectionRange.Start.Line+1 || endLine != expectedEndLine {
		return errors.New("选区行号与编辑器坐标不一致")
	}
	return nil
}

func validEditorRange(r explain.EditorRange) bool {
	return validEditorPosition(r.Start) && validEditorPosition(r.End) && editorPositionLess(r.Start, r.End)
}

func validEditorPosition(p explain.EditorPosition) bool {
	return p.Line >= 0 && p.Line <= 100000000 && p.Character >= 0 && p.Character <= 100000000
}

func editorPositionLess(a, b explain.EditorPosition) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Character < b.Character)
}

func editorPositionEqual(a, b explain.EditorPosition) bool {
	return a.Line == b.Line && a.Character == b.Character
}

func maxEditorPosition(a, b explain.EditorPosition) explain.EditorPosition {
	if editorPositionLess(a, b) {
		return b
	}
	return a
}

func minEditorPosition(a, b explain.EditorPosition) explain.EditorPosition {
	if editorPositionLess(a, b) {
		return a
	}
	return b
}

func mapSelectionDiagnostic(d explain.EditorDiagnostic, selection explain.EditorRange) (explain.SelectionDiagnostic, bool) {
	if d.Severity != "error" || !utf8.ValidString(d.Message) || !validEditorPosition(d.Range.Start) || !validEditorPosition(d.Range.End) || editorPositionLess(d.Range.End, d.Range.Start) {
		return explain.SelectionDiagnostic{}, false
	}
	start, end := d.Range.Start, d.Range.End
	if editorPositionEqual(start, end) {
		if editorPositionLess(start, selection.Start) || !editorPositionLess(start, selection.End) {
			return explain.SelectionDiagnostic{}, false
		}
	} else if !editorPositionLess(start, selection.End) || !editorPositionLess(selection.Start, end) {
		return explain.SelectionDiagnostic{}, false
	}
	clippedStart := maxEditorPosition(start, selection.Start)
	clippedEnd := minEditorPosition(end, selection.End)
	if editorPositionEqual(start, end) {
		clippedStart, clippedEnd = start, end
	}
	absolute := func(p explain.EditorPosition) explain.SelectionPosition {
		return explain.SelectionPosition{Line: p.Line + 1, Column: p.Character + 1}
	}
	relative := func(p explain.EditorPosition) explain.SelectionPosition {
		column := p.Character + 1
		if p.Line == selection.Start.Line {
			column = p.Character - selection.Start.Character + 1
		}
		return explain.SelectionPosition{Line: p.Line - selection.Start.Line + 1, Column: column}
	}
	return explain.SelectionDiagnostic{
		Severity:      d.Severity,
		Message:       d.Message,
		AbsoluteRange: explain.SelectionRange{Start: absolute(start), End: absolute(end), EndExclusive: true},
		RelativeRange: explain.SelectionRange{Start: relative(clippedStart), End: relative(clippedEnd), EndExclusive: true},
	}, true
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
	return a.analyzeWith(ctx, question, key, evidence, false, "")
}

func (a *App) analyzeSelection(ctx context.Context, question, key, file string, evidence []explain.Evidence) (explain.Document, error) {
	return a.analyzeWith(ctx, question, key, evidence, true, file)
}

func (a *App) analyzeWith(ctx context.Context, question, key string, evidence []explain.Evidence, selection bool, selectionFile string) (explain.Document, error) {
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
	key, e = a.modelAPIKey(settings.Model, key)
	if e != nil {
		return explain.Document{}, e
	}
	if strings.TrimSpace(key) == "" {
		return explain.Document{}, fmt.Errorf("需要密钥环境变量 %s；请在启动编辑器或 hook 的环境中配置", settings.Model.APIKeyEnv)
	}
	reserve := analysisReservation(settings.Model.VerificationMode, question, evidence, selection)
	if reserve > usage.Remaining(settings.Model.TokenLimit) {
		return explain.Document{}, fmt.Errorf("预算不足；本次保守预留 %d tokens", reserve)
	}
	reserved := usage
	reserved.UncertainTokens += reserve
	if writeErr := a.write("usage.json", reserved); writeErr != nil {
		return explain.Document{}, writeErr
	}
	runner := a.analysisRunner(selection)
	if selection {
		if policyErr := a.checkSelectionReservation(selectionFile, usage); policyErr != nil {
			return explain.Document{}, policyErr
		}
	}
	doc, updated, callErr := runner(ctx, settings.Model, usage, key, question, evidence)
	if writeErr := a.write("usage.json", updated); writeErr != nil {
		return explain.Document{}, writeErr
	}
	if callErr != nil {
		return explain.Document{}, callErr
	}
	if err := a.validateAnalysisEvidence(evidence, selection, selectionFile); err != nil {
		return explain.Document{}, err
	}
	return doc, nil
}

func analysisReservation(mode, question string, evidence []explain.Evidence, selection bool) int {
	if selection {
		return explain.ReservationForSelectionMode(mode, question, evidence)
	}
	return explain.ReservationForMode(mode, question, evidence)
}

func (a *App) analysisRunner(selection bool) func(context.Context, explain.Config, explain.Usage, string, string, []explain.Evidence) (explain.Document, explain.Usage, error) {
	runner := a.modelRun
	if selection {
		runner = a.selectionRun
	}
	if runner != nil {
		return runner
	}
	if selection {
		return explain.RunSelection
	}
	return explain.Run
}

func (a *App) checkSelectionReservation(file string, usage explain.Usage) error {
	if policyErr := a.requireSelectionFileAllowed(file); policyErr != nil {
		if restoreErr := a.write("usage.json", usage); restoreErr != nil {
			return fmt.Errorf("%w；无法恢复预算预留：%v", policyErr, restoreErr)
		}
		return policyErr
	}
	return nil
}

func (a *App) validateAnalysisEvidence(evidence []explain.Evidence, selection bool, selectionFile string) error {
	// Policy may change while the remote request is running: do not persist newly excluded sources.
	for i := range evidence {
		ev := &evidence[i]
		if selection {
			if policyErr := a.requireSelectionFileAllowed(selectionFile); policyErr != nil {
				return fmt.Errorf("分析期间无法确认选区文件仍被允许；结果未返回：%w", policyErr)
			}
			validSelectionEvidence := (ev.Kind == "selection" && ev.ID == "selected-code") || (ev.Kind == "diagnostics" && ev.ID == "selected-diagnostics")
			if !validSelectionEvidence || ev.File != "" {
				return errors.New("选区证据超出本次请求范围；结果未返回")
			}
			continue
		}
		ok, err := a.evidenceAllowed(*ev)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("分析期间排除规则变化；结果未保存")
		}
	}
	return nil
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
