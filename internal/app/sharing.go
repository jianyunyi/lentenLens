package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"intentlens/internal/capture"
	"intentlens/internal/explain"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxShareBytes = 1 << 20
const shareTempPrefix = ".intentlens-share-"

type SharedRecord struct {
	Schema     int                `json:"schema"`
	ID         string             `json:"id"`
	File       string             `json:"file,omitempty"`
	FunctionID string             `json:"function_id"`
	Created    string             `json:"created"`
	Supersedes string             `json:"supersedes,omitempty"`
	Document   explain.Document   `json:"document"`
	Evidence   []explain.Evidence `json:"evidence"`
}
type ShareDraft struct {
	ID         string       `json:"id"`
	SourceID   string       `json:"source_id"`
	SourceHash string       `json:"source_hash"`
	PolicyHash string       `json:"policy_hash"`
	Digest     string       `json:"digest"`
	Record     SharedRecord `json:"record"`
}
type SharedView struct {
	SharedRecord
	Freshness string   `json:"freshness"`
	Issues    []string `json:"issues"`
}

func (a *App) PrepareShare(ctx context.Context, id string, approvedIDs []string, key string) (ShareDraft, error) {
	source, err := a.ExplanationByID(id)
	if err != nil {
		return ShareDraft{}, err
	}
	if len(source.Corrections) > 0 {
		return ShareDraft{}, errors.New("该版本存在标错记录；请先生成修订版本")
	}
	if source.Freshness != "已引用证据未变" {
		return ShareDraft{}, errors.New("解释依据已过期；请先生成修订版本")
	}
	policy, err := capture.LoadPolicy(a.Root)
	if err != nil {
		return ShareDraft{}, err
	}
	approved := map[string]bool{}
	known := map[string]bool{}
	for i := range source.Evidence {
		known[source.Evidence[i].ID] = true
	}
	for _, eid := range approvedIDs {
		if !known[eid] {
			return ShareDraft{}, errors.New("选中了不存在的证据")
		}
		approved[eid] = true
	}
	evidence := []explain.Evidence{}
	for i := range source.Evidence {
		e := &source.Evidence[i]
		if (!codeEvidence(*e) || e.Visibility == "local") && !approved[e.ID] {
			continue
		}
		ok, allowedErr := a.evidenceAllowed(*e)
		if allowedErr != nil {
			return ShareDraft{}, allowedErr
		}
		if !ok {
			return ShareDraft{}, errors.New("相关证据已被排除")
		}
		if e.Hash != hash(e.Source) {
			return ShareDraft{}, errors.New("证据摘要不匹配")
		}
		shared := *e
		shared.Visibility = "shared"
		evidence = append(evidence, shared)
	}
	if len(evidence) == 0 {
		return ShareDraft{}, errors.New("没有获准共享的证据")
	}
	// Only allowed sources and a fixed request enter the model: never local claims, reasons or questions.
	doc, err := a.analyze(ctx, "仅根据本次获准共享的证据，为团队成员生成执行流程、行为变化和有依据的取舍。未提供的动机与约束保留未知。", key, evidence)
	if err != nil {
		return ShareDraft{}, err
	}
	latest, err := a.ExplanationByID(id)
	if err != nil {
		return ShareDraft{}, err
	}
	if a.sourceDigest(latest) != a.sourceDigest(source) || latest.Freshness != "已引用证据未变" {
		return ShareDraft{}, errors.New("生成期间源版本变化；请重新准备共享")
	}
	current, err := capture.LoadPolicy(a.Root)
	if err != nil {
		return ShareDraft{}, err
	}
	if current.Hash() != policy.Hash() {
		return ShareDraft{}, errors.New("生成期间排除规则变化；请重新准备")
	}
	record := SharedRecord{Schema: 1, ID: newID(), File: source.File, FunctionID: source.FunctionID, Created: time.Now().UTC().Format(time.RFC3339Nano), Document: doc, Evidence: evidence}
	if source.Supersedes != "" {
		record.Supersedes = a.sharedParent(source.Supersedes)
	}
	if _, err = validateRecord(record, true); err != nil {
		return ShareDraft{}, err
	}
	draft := ShareDraft{ID: record.ID, SourceID: source.ID, SourceHash: a.sourceDigest(source), PolicyHash: policy.Hash(), Digest: digest(record), Record: record}
	return draft, a.write("share-draft-"+draft.ID+".json", draft)
}

func (a *App) ConfirmShare(id, previewDigest string, confirmed bool) (SharedRecord, error) {
	if !confirmed {
		return SharedRecord{}, errors.New("需要用户确认完整共享预览")
	}
	if !validID(id) {
		return SharedRecord{}, errors.New("无效共享草稿 ID")
	}
	unlock, err := a.lock("revision")
	if err != nil {
		return SharedRecord{}, err
	}
	defer unlock()
	var draft ShareDraft
	if readErr := a.read("share-draft-"+id+".json", &draft); readErr != nil {
		return SharedRecord{}, readErr
	}
	if draft.ID != id || draft.Record.ID != id || draft.Digest != previewDigest || digest(draft.Record) != previewDigest {
		return SharedRecord{}, errors.New("共享预览已变化，请重新查看")
	}
	policy, err := capture.LoadPolicy(a.Root)
	if err != nil {
		return SharedRecord{}, err
	}
	if policy.Hash() != draft.PolicyHash {
		return SharedRecord{}, errors.New("排除规则变化，请重新准备共享")
	}
	source, err := a.ExplanationByID(draft.SourceID)
	if err != nil {
		return SharedRecord{}, err
	}
	if len(source.Corrections) > 0 || a.sourceDigest(source) != draft.SourceHash || source.Freshness != "已引用证据未变" {
		return SharedRecord{}, errors.New("源版本或证据变化，请重新准备共享")
	}
	if _, err = validateRecord(draft.Record, true); err != nil {
		return SharedRecord{}, err
	}
	for i := range draft.Record.Evidence {
		ok, eerr := a.evidenceAllowed(draft.Record.Evidence[i])
		if eerr != nil {
			return SharedRecord{}, eerr
		}
		if !ok {
			return SharedRecord{}, errors.New("证据已被排除")
		}
	}
	data, err := json.MarshalIndent(draft.Record, "", "  ")
	if err != nil {
		return SharedRecord{}, err
	}
	if len(data) > maxShareBytes {
		return SharedRecord{}, errors.New("共享记录超过 1 MiB")
	}
	dir, err := a.sharedDir(true)
	if err != nil {
		return SharedRecord{}, err
	}
	defer dir.Close()
	if err := publishShared(dir, id+".json", data, draft.Digest); err != nil {
		return SharedRecord{}, err
	}
	return draft.Record, nil
}

func publishShared(dir *os.Root, name string, data []byte, previewDigest string) error {
	temp := shareTempPrefix + newID() + ".tmp"
	f, err := dir.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if temp != "" {
			_ = dir.Remove(temp)
		}
	}()
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if err = errors.Join(err, f.Close()); err != nil {
		return err
	}
	if err = dir.Link(temp, name); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		existing, readErr := readShared(dir, name)
		if readErr != nil {
			return fmt.Errorf("已有共享记录无法读取：%w", readErr)
		}
		if digest(existing) != previewDigest {
			return errors.New("已有共享记录的摘要与预览不匹配")
		}
	}
	removeErr := dir.Remove(temp)
	if removeErr == nil {
		temp = ""
	}
	d, err := dir.Open(".")
	if err != nil {
		return errors.Join(removeErr, err)
	}
	// Sync even on an idempotent retry: the previous publisher may have
	// stopped after linking the complete file but before syncing the directory.
	return errors.Join(removeErr, syncDir(d), d.Close())
}

func (a *App) sharedDir(create bool) (*os.Root, error) {
	root, err := os.OpenRoot(a.Root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	for _, p := range []string{".intent", ".intent/shared"} {
		info, e := root.Lstat(p)
		if errors.Is(e, os.ErrNotExist) && create {
			if e = root.Mkdir(p, 0o700); e != nil && !errors.Is(e, os.ErrExist) {
				return nil, e
			}
			info, e = root.Lstat(p)
		}
		if e != nil {
			return nil, e
		}
		if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return nil, errors.New("共享目录不能是链接或普通文件")
		}
	}
	return root.OpenRoot(".intent/shared")
}

func readShared(dir *os.Root, name string) (SharedRecord, error) {
	var record SharedRecord
	info, err := dir.Lstat(name)
	if err != nil {
		return record, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxShareBytes {
		return record, errors.New("共享记录必须是小于 1 MiB 的普通文件")
	}
	f, err := dir.Open(name)
	if err != nil {
		return record, err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, maxShareBytes+1))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&record); err != nil {
		return record, errors.New("共享记录格式无效")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return record, errors.New("共享记录包含额外内容")
	}
	if record.ID+".json" != name {
		return record, errors.New("共享记录 ID 与文件名不匹配")
	}
	return record, nil
}

func validateRecord(r SharedRecord, strict bool) ([]string, error) {
	issues := []string{}
	if r.Schema != 1 || !validID(r.ID) || (r.Supersedes != "" && !validID(r.Supersedes)) {
		return issues, errors.New("不支持的共享版本或无效 ID")
	}
	if r.File != "" && !safeRelative(r.File) {
		return issues, errors.New("共享文件路径无效")
	}
	known := map[string]bool{}
	for i := range r.Evidence {
		e := &r.Evidence[i]
		if e.ID == "" || known[e.ID] || hash(e.Source) != e.Hash {
			return issues, errors.New("共享证据摘要或 ID 无效")
		}
		known[e.ID] = true
		if codeEvidence(*e) && !safeRelative(e.File) || e.File != "" && !safeRelative(e.File) {
			return issues, errors.New("证据路径不安全")
		}
		if !codeEvidence(*e) && e.Kind != "prompt" && e.Kind != "self_report" {
			return issues, errors.New("未知证据来源")
		}
		if e.Visibility != "shared" {
			return issues, errors.New("证据未经共享标记")
		}
	}
	seen := map[string]bool{}
	for _, c := range r.Document.Claims {
		if c.ID == "" || seen[c.ID] || c.Text == "" || (c.Status != "supported" && c.Status != "uncertain") {
			return issues, errors.New("共享说法无效或已标错")
		}
		seen[c.ID] = true
		missing := len(c.EvidenceIDs) == 0
		for _, eid := range c.EvidenceIDs {
			if !known[eid] {
				missing = true
			}
		}
		if missing {
			if strict {
				return issues, errors.New("共享说法缺少获准证据")
			}
			issues = append(issues, "依据未共享："+c.ID)
		}
	}
	return issues, nil
}
func safeRelative(p string) bool {
	for _, part := range strings.Split(p, "/") {
		if part == ".." || part == "." || part == "" {
			return false
		}
	}
	return filepath.IsLocal(p) && !strings.ContainsAny(p, ":\\") && p != "." && !strings.HasPrefix(p, "/")
}

func (a *App) Shared() ([]SharedView, error) {
	if err := a.scrubExcluded(); err != nil {
		return nil, err
	}
	views := []SharedView{}
	dir, err := a.sharedDir(false)
	if errors.Is(err, os.ErrNotExist) {
		return views, nil
	}
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	f, err := dir.Open(".")
	if err != nil {
		return nil, err
	}
	entries, err := f.ReadDir(-1)
	closeErr := f.Close()
	err = errors.Join(err, closeErr)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), shareTempPrefix) || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		record, e := readShared(dir, entry.Name())
		if e != nil {
			return nil, fmt.Errorf("无法读取共享记录 %s：%w", entry.Name(), e)
		}
		issues, e := validateRecord(record, false)
		if e != nil {
			return nil, e
		}
		visible := true
		for i := range record.Evidence {
			ok, e := a.evidenceAllowed(record.Evidence[i])
			if e != nil {
				return nil, e
			}
			if !ok {
				visible = false
				break
			}
		}
		if !visible {
			continue
		}
		if a.sharedFlagged(record.ID) {
			issues = append(issues, "原本地版本已标错；该共享版本需要复核")
		}
		for i := range record.Document.Claims {
			c := &record.Document.Claims[i]
			for _, issue := range issues {
				if issue == "依据未共享："+c.ID {
					c.Status = "uncertain"
				}
			}
		}
		views = append(views, SharedView{SharedRecord: record, Freshness: a.freshness(record.Evidence), Issues: issues})
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Created > views[j].Created })
	return views, nil
}

func (a *App) sharedParent(sourceID string) string {
	seen := map[string]bool{}
	for validID(sourceID) && !seen[sourceID] {
		seen[sourceID] = true
		if id := a.sharedDirectParent(sourceID); id != "" {
			return id
		}
		var source Explanation
		if a.read("explanation-"+sourceID+".json", &source) != nil {
			break
		}
		sourceID = source.Supersedes
	}
	return ""
}

func (a *App) sharedDirectParent(sourceID string) string {
	if validID(sourceID) {
		if dir, err := a.sharedDir(false); err == nil {
			_, readErr := readShared(dir, sourceID+".json")
			closeErr := dir.Close()
			if readErr == nil && closeErr == nil {
				return sourceID
			}
		}
	}
	entries, err := os.ReadDir(a.State)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "share-draft-") {
			var d ShareDraft
			if a.read(e.Name(), &d) == nil && d.SourceID == sourceID {
				dir, err := a.sharedDir(false)
				if err == nil {
					r, readErr := readShared(dir, d.ID+".json")
					closeErr := dir.Close()
					if readErr == nil && closeErr == nil && r.ID == d.ID {
						return d.ID
					}
				}
			}
		}
	}
	return ""
}
func (a *App) sharedFlagged(id string) bool {
	if !validID(id) {
		return false
	}
	var adopted Explanation
	if a.read("explanation-"+id+".json", &adopted) == nil {
		if a.attachCorrections(&adopted) != nil || len(adopted.Corrections) > 0 {
			return true
		}
	}
	var d ShareDraft
	if a.read("share-draft-"+id+".json", &d) != nil {
		return false
	}
	var e Explanation
	if a.read("explanation-"+d.SourceID+".json", &e) != nil {
		return true
	}
	if a.attachCorrections(&e) != nil {
		return true
	}
	return len(e.Corrections) > 0
}

// AdoptShared creates local review state without modifying the shared version.
func (a *App) AdoptShared(id string) (Explanation, error) {
	if !validID(id) {
		return Explanation{}, errors.New("无效共享 ID")
	}
	views, err := a.Shared()
	if err != nil {
		return Explanation{}, err
	}
	for i := range views {
		view := &views[i]
		if view.ID != id {
			continue
		}
		if _, err = validateRecord(view.SharedRecord, true); err != nil {
			return Explanation{}, err
		}
		if a.revisionExists(id) {
			return a.ExplanationByID(id)
		}
		e := Explanation{ID: id, File: view.File, FunctionID: view.FunctionID, Created: view.Created, Document: view.Document, Evidence: view.Evidence}
		if writeErr := a.write("explanation-"+id+".json", e); writeErr != nil {
			return Explanation{}, writeErr
		}
		return a.ExplanationByID(id)
	}
	return Explanation{}, errors.New("共享记录不存在或相关上下文已排除")
}
