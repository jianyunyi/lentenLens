package app

// Prescan is deliberately separate from buildIndex: only explicit scans may
// enumerate source trees. Cache queries inspect private state and policy only.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"intentlens/internal/capture"
	"intentlens/internal/codeindex"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"
)

const prescanSchema = 1
const maxPrescanBytes = 64 << 20
const prescanLifetime = 7 * 24 * time.Hour
const prescanBuildLimit = 120 * time.Second
const prescanPermissionName = "prescan-permission.json"
const prescanStatusName = "prescan-status.json"

// Version changes invalidate rather than migrate retained source material.
type prescanConfig struct {
	Generator        string `json:"generator"`
	GoParser         string `json:"go_parser"`
	Markdown         string `json:"markdown_segmenter"`
	Tokenizer        string `json:"tokenizer"`
	Scope            string `json:"scope"`
	FileBytes        int    `json:"file_bytes"`
	ComponentBytes   int    `json:"component_bytes"`
	GoFiles          int    `json:"go_files"`
	DocsFiles        int    `json:"docs_files"`
	Paragraphs       int    `json:"paragraphs"`
	ParagraphBytes   int    `json:"paragraph_bytes"`
	CacheBytes       int    `json:"cache_bytes"`
	BuildSeconds     int    `json:"build_seconds"`
	RetentionSeconds int    `json:"retention_seconds"`
}

func currentPrescanConfig() prescanConfig {
	return prescanConfig{"prescan-v1", runtime.Version() + "/go-parser-types-v1", "markdown-v1", "doc-terms-v1", "go:**/*.go;docs:root/*.md+docs/**/*.md", capture.MaxFileBytes, capture.MaxSnapshotBytes, capture.MaxGoFiles, capture.MaxDocsFiles, codeindex.MaxDocParagraphs, codeindex.MaxDocParagraphBytes, maxPrescanBytes, 120, int(prescanLifetime / time.Second)}
}
func prescanPolicyHash(policy string) string {
	b, _ := json.Marshal(currentPrescanConfig())
	return hash(policy + "\n" + string(b))
}
func prescanName(policy string) string { return "prescan-" + policy + ".json" }
func isPrescanManifest(name string) bool {
	if !strings.HasPrefix(name, "prescan-") || !strings.HasSuffix(name, ".json") {
		return false
	}
	h := strings.TrimSuffix(strings.TrimPrefix(name, "prescan-"), ".json")
	return len(h) == 64 && strings.Trim(h, "0123456789abcdef") == ""
}

type prescanPermission struct {
	Version   int           `json:"version"`
	Epoch     string        `json:"epoch"`
	Workspace string        `json:"workspace"`
	Config    prescanConfig `json:"config"`
	Granted   bool          `json:"granted"`
}
type prescanFile struct {
	Hash  string `json:"hash"`
	Bytes int    `json:"captured_bytes"`
	Lines int    `json:"lines"`
}
type prescanSource struct {
	File      string `json:"file"`
	Hash      string `json:"hash"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}
type prescanCoverage struct {
	GoCapture      capture.Coverage   `json:"go_capture"`
	DocsCapture    capture.Coverage   `json:"docs_capture"`
	Go             codeindex.Coverage `json:"go"`
	Docs           capture.Coverage   `json:"docs"`
	GoReasons      map[string]string  `json:"go_omission_reasons"`
	DocsReasons    map[string]string  `json:"docs_omission_reasons"`
	ParseOmissions map[string]string  `json:"parse_omissions"`
}
type prescanBody struct {
	Go      codeindex.Index            `json:"go"`
	Docs    codeindex.DocIndex         `json:"docs"`
	Sources map[string][]prescanSource `json:"sources"`
}
type prescanManifest struct {
	Schema               int                    `json:"schema"`
	Config               prescanConfig          `json:"config"`
	Generation           string                 `json:"generation"`
	Started              time.Time              `json:"started"`
	Finished             time.Time              `json:"finished"`
	Workspace            string                 `json:"workspace"`
	PolicyHash           string                 `json:"policy_hash"`
	CapturePolicyHash    string                 `json:"capture_policy_hash"`
	AuthorizationVersion int                    `json:"authorization_version"`
	AuthorizationEpoch   string                 `json:"authorization_epoch"`
	Files                map[string]prescanFile `json:"files"`
	Coverage             prescanCoverage        `json:"coverage"`
	Integrity            string                 `json:"integrity"`
	Body                 prescanBody            `json:"body"`
}

// PrescanState is transport metadata, not a claim of realtime repository state.
type PrescanState struct {
	Verified      time.Time `json:"verified,omitempty"`
	State         string    `json:"state"`
	Message       string    `json:"message"`
	Generation    string    `json:"generation,omitempty"`
	Generated     time.Time `json:"generated,omitempty"`
	Reused        bool      `json:"reused"`
	Rebuilt       bool      `json:"rebuilt"`
	PreviousState string    `json:"previous_state,omitempty"`
}
type PrescanResult struct {
	Prescan  PrescanState       `json:"prescan"`
	Coverage prescanCoverage    `json:"coverage"`
	Go       codeindex.Index    `json:"go"`
	Docs     codeindex.DocIndex `json:"docs"`
}

func prescanState(state, reason string) PrescanState {
	messages := map[string]string{"hit": "本次显式预扫/核验已完成；仅代表预扫时点", "unknown": "基于某次预扫、当前性未验证；新鲜度未验证", "stale": "索引过期；请显式刷新预扫", "missing": "关联需先预扫；缓存不可用"}
	m := messages[state]
	if reason != "" {
		m += "（" + reason + "）"
	}
	return PrescanState{State: state, Message: m}
}
func manifestResult(m *prescanManifest, s PrescanState) PrescanResult {
	r := PrescanResult{Prescan: s, Go: codeindex.Index{Functions: []codeindex.Function{}, Issues: []string{}, Coverage: codeindex.Coverage{Scope: "go-only"}}, Docs: codeindex.DocIndex{Paragraphs: []codeindex.DocParagraph{}, Omitted: []codeindex.DocOmission{}}}
	if m != nil {
		r.Prescan.Generation = m.Generation
		r.Prescan.Generated = m.Finished
		r.Coverage = m.Coverage
		r.Coverage.Go.OmissionReasons = m.Coverage.GoReasons
		r.Coverage.GoCapture.OmissionReasons = m.Coverage.GoReasons
		r.Coverage.Docs.OmissionReasons = m.Coverage.DocsReasons
		r.Coverage.DocsCapture.OmissionReasons = m.Coverage.DocsReasons
		if r.Coverage.GoCapture.Excluded == nil {
			r.Coverage.GoCapture.Excluded = []capture.CoverageOmission{}
		}
		if r.Coverage.DocsCapture.Excluded == nil {
			r.Coverage.DocsCapture.Excluded = []capture.CoverageOmission{}
		}
		if r.Coverage.Docs.Excluded == nil {
			r.Coverage.Docs.Excluded = []capture.CoverageOmission{}
		}
		r.Go = m.Body.Go
		r.Go.Coverage.OmissionReasons = m.Coverage.GoReasons
		r.Docs = codeindex.RestoreDocPostings(m.Body.Docs)
	}
	return r
}
func manifestDigest(m *prescanManifest) string {
	copy := *m
	copy.Integrity = ""
	b, _ := json.Marshal(copy)
	var normalized any
	if json.Unmarshal(b, &normalized) != nil {
		return ""
	}
	b, _ = json.Marshal(normalized)
	return hash(string(b))
}
func validPrescanPath(p string) bool {
	return p != "" && p != "." && !strings.Contains(p, "\\") && !filepath.IsAbs(p) && filepath.ToSlash(filepath.Clean(p)) == p && p != ".." && !strings.HasPrefix(p, "../")
}
func validatePrescan(m *prescanManifest) error {
	if m.Schema != prescanSchema || m.Config != currentPrescanConfig() {
		return errors.New("schema/parser/scope version changed")
	}
	if m.Generation == "" || m.Workspace == "" || m.AuthorizationVersion != 1 || m.AuthorizationEpoch == "" || m.Started.IsZero() || m.Finished.Before(m.Started) || m.Finished.Sub(m.Started) > prescanBuildLimit || m.Finished.After(time.Now().Add(time.Minute)) || m.Files == nil || m.Body.Sources == nil || m.Coverage.GoReasons == nil || m.Coverage.DocsReasons == nil || m.Coverage.ParseOmissions == nil {
		return errors.New("invalid manifest metadata")
	}
	if m.PolicyHash != prescanPolicyHash(m.CapturePolicyHash) || m.Integrity != manifestDigest(m) {
		return errors.New("manifest integrity mismatch")
	}
	for p, f := range m.Files {
		if !validPrescanPath(p) || len(f.Hash) != 64 || f.Bytes < 0 || f.Bytes > capture.MaxFileBytes || f.Lines < 1 {
			return errors.New("invalid captured file")
		}
	}
	goBytes, docsBytes, goCount, docsCount := 0, 0, 0, 0
	for p, f := range m.Files {
		if strings.HasSuffix(p, ".go") {
			goBytes += f.Bytes
			goCount++
		} else if strings.HasSuffix(p, ".md") && (filepath.Dir(p) == "." || strings.HasPrefix(p, "docs/")) {
			docsBytes += f.Bytes
			docsCount++
		} else {
			return errors.New("file outside prescan scope")
		}
	}
	if goBytes > capture.MaxSnapshotBytes || docsBytes > capture.MaxSnapshotBytes || goCount > capture.MaxGoFiles || docsCount > capture.MaxDocsFiles || len(m.Body.Docs.Paragraphs) > codeindex.MaxDocParagraphs {
		return errors.New("manifest exceeds capture budgets")
	}
	for _, p := range m.Body.Docs.Paragraphs {
		if len(p.Source) > codeindex.MaxDocParagraphBytes || p.Hash != hash(p.Source) {
			return errors.New("invalid retained paragraph")
		}
	}
	// Check lineage closure for every record, including call target declarations.
	expected := prescanSources(m.Body.Go, m.Body.Docs, m.Files)
	if !reflect.DeepEqual(expected, m.Body.Sources) {
		return errors.New("invalid source lineage")
	}
	for _, ss := range expected {
		for _, s := range ss {
			f, ok := m.Files[s.File]
			if !ok || s.Hash != f.Hash || s.StartLine < 1 || s.EndLine < s.StartLine {
				return errors.New("source outside manifest")
			}
		}
	}
	return nil
}
func prescanSources(goidx codeindex.Index, docs codeindex.DocIndex, files map[string]prescanFile) map[string][]prescanSource {
	out := map[string][]prescanSource{}
	packages := map[string][]prescanSource{}
	names := []string{}
	for p := range files {
		if strings.HasSuffix(p, ".go") {
			names = append(names, p)
		}
	}
	sort.Strings(names)
	all := []prescanSource{}
	for _, p := range names {
		f := files[p]
		source := prescanSource{p, f.Hash, 1, f.Lines}
		packages[filepath.Dir(p)] = append(packages[filepath.Dir(p)], source)
		all = append(all, source)
	}
	// Resolution can depend on any captured file in the same package directory,
	// including files with only type declarations. The normalized on-disk source
	// sets avoid repeating that closure once for every function/call.
	for _, f := range goidx.Functions {
		ss := packages[filepath.Dir(f.File)]
		out["function:"+f.ID] = ss
		for i := range f.Calls {
			out[fmt.Sprintf("call:%s:%d", f.ID, i)] = ss
		}
	}
	for _, p := range docs.Paragraphs {
		out[p.ID] = []prescanSource{{p.File, files[p.File].Hash, p.StartLine, p.EndLine}}
	}
	for i, p := range docs.Omitted {
		if f, ok := files[p.File]; ok {
			out[fmt.Sprintf("doc-omission:%d", i)] = []prescanSource{{p.File, f.Hash, p.StartLine, p.EndLine}}
		}
	}
	if len(goidx.Issues) > 0 || goidx.Aggregated != nil {
		out["go-diagnostics"] = all
	}
	return out
}

type prescanStoredBody struct {
	Go         codeindex.Index            `json:"go"`
	Docs       codeindex.DocIndex         `json:"docs"`
	SourceSets map[string][]prescanSource `json:"source_sets"`
	Sources    map[string][]string        `json:"sources"`
}

func (b prescanBody) MarshalJSON() ([]byte, error) {
	stored := prescanStoredBody{Go: b.Go, Docs: b.Docs, SourceSets: map[string][]prescanSource{}, Sources: map[string][]string{}}
	type sliceKey struct {
		first  *prescanSource
		length int
	}
	seen := map[sliceKey]string{}
	for record, sources := range b.Sources {
		key := sliceKey{length: len(sources)}
		if len(sources) > 0 {
			key.first = &sources[0]
		}
		if id, ok := seen[key]; ok {
			stored.Sources[record] = []string{id}
			continue
		}
		raw, err := json.Marshal(sources)
		if err != nil {
			return nil, err
		}
		id := "set:" + hash(string(raw))
		seen[key] = id
		stored.SourceSets[id] = sources
		stored.Sources[record] = []string{id}
	}
	return json.Marshal(stored)
}
func (b *prescanBody) UnmarshalJSON(raw []byte) error {
	var stored prescanStoredBody
	if err := json.Unmarshal(raw, &stored); err != nil {
		return err
	}
	if stored.SourceSets == nil || stored.Sources == nil {
		return errors.New("source sets missing")
	}
	for id, sources := range stored.SourceSets {
		encoded, _ := json.Marshal(sources)
		if id != "set:"+hash(string(encoded)) {
			return errors.New("source set integrity mismatch")
		}
	}
	b.Go = stored.Go
	b.Docs = stored.Docs
	b.Sources = map[string][]prescanSource{}
	for record, sets := range stored.Sources {
		if len(sets) != 1 {
			return errors.New("invalid source set reference")
		}
		sources, ok := stored.SourceSets[sets[0]]
		if !ok {
			return errors.New("source lineage missing")
		}
		b.Sources[record] = sources
	}
	return nil
}

// No generic full-file Snapshot is retained: only file identity/size and the
// bounded derived Go declarations/Markdown sections are persisted.
func prescanFiles(snaps ...capture.Snapshot) map[string]prescanFile {
	out := map[string]prescanFile{}
	for _, s := range snaps {
		for p, f := range s.Files {
			out[p] = prescanFile{f.Hash, len(f.Content), strings.Count(f.Content, "\n") + 1}
		}
	}
	return out
}
func (a *App) prescanPermission() (prescanPermission, error) {
	var p prescanPermission
	err := a.read(prescanPermissionName, &p)
	if err == nil && (p.Version != 1 || !p.Granted || p.Epoch == "" || p.Workspace != hash(a.Root) || p.Config != currentPrescanConfig()) {
		err = errors.New("预扫保留许可无效，需要重新确认")
	}
	return p, err
}

// invalidatePrescan runs under capture lock. It deletes, never quarantines raw
// prescan source material; status is content-free and does not cite filenames.
func (a *App) invalidatePrescan(state, reason string) error {
	entries, err := os.ReadDir(a.State)
	if err != nil {
		return err
	}
	release, err := a.lock("state")
	if err != nil {
		return err
	}
	defer release()
	root, err := os.OpenRoot(a.State)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, e := range entries {
		if isPrescanManifest(e.Name()) {
			if err = root.Remove(e.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	b, _ := json.Marshal(prescanState(state, reason))
	return atomicReplace(root, prescanStatusName, b, 0o600, nil)
}

// Called from Open/scrub, including when the old policy filename no longer hits.
func (a *App) scrubPrescans(policy capture.Policy, entries []os.DirEntry) error {
	permission, permitErr := a.prescanPermission()
	for _, e := range entries {
		if !isPrescanManifest(e.Name()) {
			continue
		}
		var m prescanManifest
		if err := a.read(e.Name(), &m); err != nil {
			return a.invalidatePrescan("missing", "缓存损坏")
		}
		if m.Schema != prescanSchema || m.Config != currentPrescanConfig() {
			return a.invalidatePrescan("stale", "索引版本变化")
		}
		if err := validatePrescan(&m); err != nil {
			return a.invalidatePrescan("missing", "缓存完整性校验失败")
		}
		if permitErr != nil || m.AuthorizationEpoch != permission.Epoch || m.Workspace != hash(a.Root) {
			return a.invalidatePrescan("missing", "没有有效本地保留许可")
		}
		if m.PolicyHash != prescanPolicyHash(policy.Hash()) || e.Name() != prescanName(m.PolicyHash) {
			return a.invalidatePrescan("stale", "排除策略变化")
		}
		if time.Since(m.Started) >= prescanLifetime {
			return a.invalidatePrescan("stale", "超过七天保留期限")
		}
		a.schedulePrescanExpiry(&m)
		for p := range m.Files {
			ok, err := capture.Allowed(a.Root, p)
			if err != nil {
				return err
			}
			if !ok {
				return a.invalidatePrescan("stale", "来源路径已失效")
			}
			a.scrubbedEvidence[p] = struct{}{}
		}
	}
	return nil
}

// CachedPrescan performs no source capture/hash/enumeration. Every independent
// request is unknown, even immediately after another request published a scan.
func (a *App) CachedPrescan() (PrescanResult, error) {
	if err := a.scrubExcluded(); err != nil {
		return PrescanResult{}, err
	}
	p, err := capture.LoadPolicy(a.Root)
	if err != nil {
		return PrescanResult{}, err
	}
	var m prescanManifest
	if err = a.read(prescanName(prescanPolicyHash(p.Hash())), &m); err != nil {
		s := prescanState("missing", "")
		_ = a.read(prescanStatusName, &s)
		if s.State != "stale" {
			s = prescanState("missing", "")
		}
		return manifestResult(nil, s), nil
	}
	// Revalidate even when the scrub signature was unchanged, including retention.
	permission, permitErr := a.prescanPermission()
	if permitErr != nil || permission.Epoch != m.AuthorizationEpoch || m.Workspace != hash(a.Root) || validatePrescan(&m) != nil {
		return manifestResult(nil, prescanState("missing", "许可或完整性无效")), nil
	}
	if m.PolicyHash != prescanPolicyHash(p.Hash()) || time.Since(m.Started) >= prescanLifetime {
		return manifestResult(nil, prescanState("stale", "策略或保留期限变化")), nil
	}
	var known PrescanState
	if a.read(prescanStatusName, &known) == nil && known.State == "stale" && known.Generation == m.Generation {
		return manifestResult(nil, known), nil
	}
	return manifestResult(&m, prescanState("unknown", "")), nil
}

// Prescan requires explicit local retention consent on first use. refresh=true
// replaces even an unchanged generation; otherwise explicit full verification
// reuses an unchanged index body without parsing again.
func (a *App) Prescan(ctx context.Context, consent, refresh bool) (PrescanResult, error) {
	limit := prescanBuildLimit
	if a.prescanTimeout > 0 && a.prescanTimeout < limit {
		limit = a.prescanTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	// A dedicated retrying lock merges overlapping refreshes across processes.
	requested := time.Now()
	release, err := a.waitPrescanLock(ctx)
	if err != nil {
		return PrescanResult{}, err
	}
	defer release()
	if err = a.scrubExcluded(); err != nil {
		return PrescanResult{}, err
	}
	permission, err := a.prescanPermission()
	if err != nil {
		if !consent {
			return PrescanResult{}, errors.New("预扫会读取获准 Go/Markdown 并在仓库外保留原文索引七天；请显式确认本地预扫保留许可")
		}
		permission = prescanPermission{1, newID(), hash(a.Root), currentPrescanConfig(), true}
		if err = a.write(prescanPermissionName, permission); err != nil {
			return PrescanResult{}, err
		}
	}
	oldResult, err := a.CachedPrescan()
	if err != nil {
		return PrescanResult{}, err
	}
	// A waiter may share the generation just published by the overlapping scan.
	var published PrescanState
	_ = a.read(prescanStatusName, &published)
	if oldResult.Prescan.State == "unknown" && published.Generation == oldResult.Prescan.Generation && published.Verified.After(requested) {
		oldResult.Prescan.State = "hit"
		oldResult.Prescan.Message = prescanState("hit", "").Message
		oldResult.Prescan.Reused = true
		oldResult.Prescan.Verified = published.Verified
		return oldResult, nil
	}
	type built struct {
		m   *prescanManifest
		err error
	}
	changed := make(chan struct{}, 1)
	done := make(chan built, 1)
	go func() { m, e := a.buildPrescan(ctx, permission, refresh, changed); done <- built{m, e} }()
	var m *prescanManifest
waitBuild:
	for {
		select {
		case <-changed:
			if e := a.MarkPrescanStale(); e != nil {
				return PrescanResult{}, e
			}
			oldResult.Prescan.State = "stale"
		case <-ctx.Done():
			select {
			case <-changed:
				if err := a.MarkPrescanStale(); err != nil {
					return PrescanResult{}, err
				}
			default:
			}
			return PrescanResult{}, fmt.Errorf("预扫构建失败（上限120秒）: %w", ctx.Err())
		case b := <-done:
			m, err = b.m, b.err
			break waitBuild
		}
	}
	select {
	case <-changed:
		if e := a.MarkPrescanStale(); e != nil {
			return PrescanResult{}, e
		}
		oldResult.Prescan.State = "stale"
	default:
	}
	if err != nil {
		return PrescanResult{}, err
	}
	if err = ctx.Err(); err != nil {
		return PrescanResult{}, err
	}
	// Publication shares capture -> state ordering with scrub/recovery. Recheck
	// latest policy/permission before atomically replacing the one published slot.
	unlock, err := a.lock("capture")
	if err != nil {
		return PrescanResult{}, err
	}
	defer unlock()
	policy, err := capture.LoadPolicy(a.Root)
	if err != nil {
		return PrescanResult{}, err
	}
	latest, err := a.prescanPermission()
	if err != nil {
		return PrescanResult{}, err
	}
	if policy.Hash() != m.CapturePolicyHash || latest.Epoch != m.AuthorizationEpoch {
		if err := a.invalidatePrescan("stale", "预扫期间策略或许可变化"); err != nil {
			return PrescanResult{}, err
		}
		return PrescanResult{}, errors.New("预扫期间策略或许可变化；未发布")
	}
	reused := false
	var old prescanManifest
	if !refresh && a.read(prescanName(m.PolicyHash), &old) == nil && validatePrescan(&old) == nil && reflect.DeepEqual(old.Files, m.Files) && reflect.DeepEqual(old.Coverage, m.Coverage) && time.Since(old.Started) < prescanLifetime {
		m = &old
		reused = true
	}
	if !reused {
		if err = a.writeContext(ctx, prescanName(m.PolicyHash), m); err != nil {
			return PrescanResult{}, err
		}
	}
	// Same policy always replaces the same filename; different policies were
	// purged before building. No source-bearing generation suffix files exist.
	s := prescanState("hit", "")
	s.Verified = time.Now()
	s.Generation = m.Generation
	s.Reused = reused
	s.Rebuilt = !reused
	s.PreviousState = oldResult.Prescan.State
	if err = a.write(prescanStatusName, s); err != nil {
		return PrescanResult{}, err
	}
	unlock()
	a.scrubMu.Lock()
	a.schedulePrescanExpiry(m)
	a.scrubMu.Unlock()
	return manifestResult(m, s), nil
}
func (a *App) waitPrescanLock(ctx context.Context) (func(), error) {
	return a.waitOperationLock(ctx, "prescan")
}
func (a *App) waitOperationLock(ctx context.Context, name string) (func(), error) {
	for {
		release, err := a.lock(name)
		if err == nil {
			return release, nil
		}
		if !strings.Contains(err.Error(), "项目操作进行中") {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}
func sameCapture(a, b capture.Snapshot, ac, bc capture.Coverage) bool {
	// Includes all captured hashes, sizes and complete omission/exclusion metadata.
	return a.PolicyHash == b.PolicyHash && reflect.DeepEqual(prescanFiles(a), prescanFiles(b)) && reflect.DeepEqual(a.Omitted, b.Omitted) && reflect.DeepEqual(ac, bc)
}
func (a *App) buildPrescan(ctx context.Context, p prescanPermission, refresh bool, changed chan<- struct{}) (*prescanManifest, error) {
	started := time.Now()
	gs, gc, err := a.takePrescan(false)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	ds, dc, err := a.takePrescan(true)
	if err != nil {
		return nil, err
	}
	if gs.PolicyHash != ds.PolicyHash {
		changed <- struct{}{}
		return nil, errors.New("Go/docs捕获策略不一致；未发布")
	}
	// Second independent capture validates file sets/hashes from both first passes.
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	gs2, gc2, err := a.takePrescan(false)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	ds2, dc2, err := a.takePrescan(true)
	if err != nil {
		return nil, err
	}
	if !sameCapture(gs, gs2, gc, gc2) || !sameCapture(ds, ds2, dc, dc2) || gs2.PolicyHash != ds2.PolicyHash {
		changed <- struct{}{}
		return nil, errors.New("预扫期间文件集合/hash/策略变化；请显式刷新")
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	files := prescanFiles(gs, ds)
	var old prescanManifest
	oldErr := a.read(prescanName(prescanPolicyHash(gs.PolicyHash)), &old)
	if oldErr == nil && (!reflect.DeepEqual(files, old.Files) || !sameCoverage(gc, old.Coverage.GoCapture, old.Coverage.GoReasons) || !sameCoverage(dc, old.Coverage.DocsCapture, old.Coverage.DocsReasons)) {
		changed <- struct{}{}
	}
	if !refresh && oldErr == nil && validatePrescan(&old) == nil && old.AuthorizationEpoch == p.Epoch && time.Since(old.Started) < prescanLifetime && reflect.DeepEqual(files, old.Files) && sameCoverage(gc, old.Coverage.GoCapture, old.Coverage.GoReasons) && sameCoverage(dc, old.Coverage.DocsCapture, old.Coverage.DocsReasons) {
		// Parsing is skipped on an explicit verification hit. Preserve original
		// generation timestamps: successful verification never renews retention.
		return &old, nil
	}
	goFiles := map[string]string{}
	docFiles := map[string]string{}
	for name, f := range gs.Files {
		goFiles[name] = f.Content
	}
	for name, f := range ds.Files {
		docFiles[name] = f.Content
	}
	idx, parse := codeindex.BuildDetailed(goFiles)
	idx.Coverage = codeindex.Coverage{Complete: gc.Complete, Scope: gc.Scope, BudgetBytes: gc.BudgetBytes, CapturedBytes: gc.CapturedBytes, OmittedCount: gc.OmittedCount, ExcludedCount: gc.ExcludedCount, Truncated: gc.Truncated, OmissionReasons: gc.OmissionReasons, Omitted: []codeindex.CoverageOmission{}}
	for _, o := range gc.Omitted {
		idx.Coverage.Omitted = append(idx.Coverage.Omitted, codeindex.CoverageOmission{File: o.File, Reason: o.Reason})
		idx.Issues = append(idx.Issues, "未捕获："+o.File+"（"+o.Reason+"）")
	}
	names := []string{}
	for name := range parse {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		idx.Coverage.Complete = false
		idx.Coverage.OmittedCount++
		if len(idx.Coverage.Omitted) < 20 {
			idx.Coverage.Omitted = append(idx.Coverage.Omitted, codeindex.CoverageOmission{File: name, Reason: parse[name]})
		} else {
			idx.Coverage.Truncated = true
		}
	}
	for _, name := range gs.Omitted {
		idx.Issues = append(idx.Issues, "未捕获："+name)
	}
	docs := codeindex.BuildDocs(docFiles)
	docsCapture := dc
	dc.Complete = dc.Complete && docs.Complete
	m := &prescanManifest{Schema: prescanSchema, Config: currentPrescanConfig(), Generation: newID(), Started: started, Finished: time.Now(), Workspace: hash(a.Root), PolicyHash: prescanPolicyHash(gs.PolicyHash), CapturePolicyHash: gs.PolicyHash, AuthorizationVersion: p.Version, AuthorizationEpoch: p.Epoch, Files: files, Coverage: prescanCoverage{GoCapture: gc, DocsCapture: docsCapture, Go: idx.Coverage, Docs: dc, GoReasons: gc.OmissionReasons, DocsReasons: dc.OmissionReasons, ParseOmissions: parse}, Body: prescanBody{Go: idx, Docs: docs, Sources: prescanSources(idx, docs, files)}}
	m.Integrity = manifestDigest(m)
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return m, nil
}

// ClearPrescan revokes local retention only. It does not change model settings,
// independent capture rounds, saved explanations, or usage accounting.
func (a *App) ClearPrescan() error {
	ctx, cancel := context.WithTimeout(context.Background(), prescanBuildLimit)
	defer cancel()
	release, err := a.waitPrescanLock(ctx)
	if err != nil {
		return err
	}
	defer release()
	unlock, err := a.lock("capture")
	if err != nil {
		return err
	}
	defer unlock()
	if err = a.write(prescanPermissionName, prescanPermission{Version: 1, Epoch: newID(), Workspace: hash(a.Root), Config: currentPrescanConfig(), Granted: false}); err != nil {
		return err
	}
	return a.invalidatePrescan("missing", "本地预扫保留许可已撤销")
}

func sameCoverage(current, saved capture.Coverage, reasons map[string]string) bool {
	a, _ := json.Marshal(current)
	b, _ := json.Marshal(saved)
	return string(a) == string(b) && reflect.DeepEqual(current.OmissionReasons, reasons)
}

func (a *App) takePrescan(docs bool) (capture.Snapshot, capture.Coverage, error) {
	if a.prescanTake != nil {
		return a.prescanTake(docs)
	}
	if docs {
		return capture.TakeDocs(a.Root)
	}
	return capture.TakeGo(a.Root)
}

// MarkPrescanStale records an observed change without any source read. Retained
// old material remains disabled until explicit verification/replacement/expiry.
func (a *App) MarkPrescanStale() error {
	unlock, err := a.lock("capture")
	if err != nil {
		return err
	}
	defer unlock()
	policy, err := capture.LoadPolicy(a.Root)
	if err != nil {
		return err
	}
	var m prescanManifest
	if err = a.read(prescanName(prescanPolicyHash(policy.Hash())), &m); errors.Is(err, os.ErrNotExist) {
		entries, e := os.ReadDir(a.State)
		if e != nil {
			return e
		}
		for _, entry := range entries {
			if isPrescanManifest(entry.Name()) {
				return a.invalidatePrescan("stale", "排除策略变化")
			}
		}
		return nil
	} else if err != nil {
		return err
	}
	s := prescanState("stale", "已观察到来源变化")
	s.Generation = m.Generation
	return a.write(prescanStatusName, s)
}

// CLI processes normally exit before this timer; embedded long-lived Apps
// remove expired source material without waiting for another query.
func (a *App) schedulePrescanExpiry(m *prescanManifest) {
	if a.prescanExpiryGeneration == m.Generation {
		return
	}
	if a.prescanExpiry != nil {
		a.prescanExpiry.Stop()
	}
	a.prescanExpiryGeneration = m.Generation
	a.prescanExpires = m.Started.Add(prescanLifetime)
	remaining := time.Until(m.Started.Add(prescanLifetime))
	if remaining < 0 {
		remaining = 0
	}
	a.prescanExpiry = time.AfterFunc(remaining, func() {
		// Capture/state writers can be briefly busy; cleanup retries within 60s.
		for i := 0; i < 60; i++ {
			if a.scrubExcluded() == nil {
				return
			}
			time.Sleep(time.Second)
		}
	})
}

func (a *App) prescanAwareCaptureLock() (func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), prescanBuildLimit)
	defer cancel()
	return a.waitOperationLock(ctx, "capture")
}
func (a *App) recoverWithPrescan() error {
	ctx, cancel := context.WithTimeout(context.Background(), prescanBuildLimit)
	defer cancel()
	for {
		err := a.recoverState()
		if err == nil || !strings.Contains(err.Error(), "项目操作进行中") {
			return err
		}
		release, e := a.lock("prescan")
		if e == nil {
			release()
			if !strings.Contains(err.Error(), "（exercise）") && !strings.Contains(err.Error(), "（capture）") {
				return err
			}
		}
		if e != nil && !strings.Contains(e.Error(), "项目操作进行中") {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
}
