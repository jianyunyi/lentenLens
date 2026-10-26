package app

import (
	"context"
	"encoding/json"
	"errors"
	"intentlens/internal/capture"
	"intentlens/internal/explain"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Correction struct {
	ID            string `json:"id"`
	ExplanationID string `json:"explanation_id"`
	ClaimID       string `json:"claim_id"`
	Reason        string `json:"reason"`
	Created       string `json:"created"`
}

func validID(id string) bool {
	if len(id) != 24 {
		return false
	}
	for _, r := range id {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
func codeEvidence(e explain.Evidence) bool { return e.Kind == "" || e.Kind == "code" }
func (a *App) evidenceAllowed(e explain.Evidence) (bool, error) {
	if e.File != "" || codeEvidence(e) {
		return capture.Allowed(a.Root, e.File)
	}
	return e.Kind == "prompt" || e.Kind == "self_report", nil
}
func digest(v any) string { b, _ := json.Marshal(v); return hash(string(b)) }

func (a *App) ExplanationByID(id string) (Explanation, error) {
	if !validID(id) {
		return Explanation{}, errors.New("无效解释 ID")
	}
	if err := a.scrubExcluded(); err != nil {
		return Explanation{}, err
	}
	var e Explanation
	if err := a.read("explanation-"+id+".json", &e); err != nil {
		return e, err
	}
	if e.ID != id {
		return e, errors.New("解释 ID 不匹配")
	}
	if err := a.attachCorrections(&e); err != nil {
		return e, err
	}
	e.Freshness = a.freshness(e.Evidence)
	return e, nil
}

func (a *App) attachCorrections(e *Explanation) error {
	e.Corrections = nil
	entries, err := os.ReadDir(a.State)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "correction-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var c Correction
		if err = a.read(entry.Name(), &c); err != nil {
			return err
		}
		if c.ExplanationID == e.ID {
			e.Corrections = append(e.Corrections, c)
		}
	}
	sort.Slice(e.Corrections, func(i, j int) bool { return e.Corrections[i].ID < e.Corrections[j].ID })
	for i := range e.Document.Claims {
		for _, c := range e.Corrections {
			if e.Document.Claims[i].ID == c.ClaimID {
				e.Document.Claims[i].Status = "flagged"
			}
		}
	}
	return nil
}

func (a *App) FlagClaim(id, claimID, reason string) (Correction, error) {
	if strings.TrimSpace(reason) == "" || len(reason) > 4000 {
		return Correction{}, errors.New("请填写不超过 4000 字节的纠错理由")
	}
	unlock, err := a.lock("revision")
	if err != nil {
		return Correction{}, err
	}
	defer unlock()
	e, err := a.ExplanationByID(id)
	if err != nil {
		return Correction{}, err
	}
	found := false
	for _, c := range e.Document.Claims {
		if c.ID == claimID {
			found = true
			break
		}
	}
	if !found {
		return Correction{}, errors.New("该版本没有对应说法")
	}
	c := Correction{ID: newID(), ExplanationID: id, ClaimID: claimID, Reason: reason, Created: time.Now().UTC().Format(time.RFC3339Nano)}
	return c, a.write("correction-"+c.ID+".json", c)
}

func (a *App) Revise(ctx context.Context, id, key string) (Explanation, error) {
	parent, err := a.ExplanationByID(id)
	if err != nil {
		return Explanation{}, err
	}
	evidence := parent.Evidence
	if strings.HasPrefix(parent.FunctionID, "round:") {
		if a.freshness(evidence) != "已引用证据未变" {
			return Explanation{}, errors.New("轮次依据已变化；请从当前代码生成解释")
		}
	} else {
		idx, err := a.Index()
		if err != nil {
			return Explanation{}, err
		}
		found := false
		for _, f := range idx.Functions {
			if f.ID == parent.FunctionID {
				info, e := a.Inspect(f.File, f.StartLine)
				if e != nil {
					return Explanation{}, e
				}
				evidence = info.Evidence
				found = true
				break
			}
		}
		if !found {
			return Explanation{}, errors.New("原函数无法定位；请从当前代码重新生成")
		}
	}
	// Do not feed the flagged text/reason back as evidence; rederive from current sources.
	doc, err := a.analyze(ctx, "重新核对源代码，生成执行流程与有依据的实现取舍；旧版本存在待纠正说法，不继承其结论。", key, evidence)
	if err != nil {
		return Explanation{}, err
	}
	revision := Explanation{ID: newID(), File: parent.File, FunctionID: parent.FunctionID, Created: time.Now().UTC().Format(time.RFC3339Nano), Supersedes: parent.ID, Document: doc, Evidence: evidence, Freshness: a.freshness(evidence)}
	return revision, a.write("explanation-"+revision.ID+".json", revision)
}

func (a *App) freshness(evidence []explain.Evidence) string {
	// Use the same Go-only budget as indexing, including for mixed round evidence.
	snap, _, err := capture.TakeGo(a.Root)
	if err != nil {
		return "解释已过期：无法检查当前依据"
	}
	return a.freshnessFromSnapshot(evidence, snap)
}

func (a *App) freshnessFromSnapshot(evidence []explain.Evidence, snap capture.Snapshot) string {
	if len(evidence) == 0 {
		return "解释已过期：缺少可检查依据"
	}
	var textSnapshot *capture.Snapshot
	after := map[string]bool{}
	for _, e := range evidence {
		if codeEvidence(e) && e.Role != "before" {
			after[e.File] = true
		}
	}
	for _, e := range evidence {
		ok, err := a.evidenceAllowed(e)
		if err != nil || !ok {
			return "解释已过期：依据已排除"
		}
		if hash(e.Source) != e.Hash {
			return "解释已过期：证据摘要不匹配"
		}
		if !codeEvidence(e) {
			continue
		}
		f, exists := snap.Files[e.File]
		if e.Role != "before" && !strings.HasSuffix(e.File, ".go") {
			// Round explanations can also reference ordinary non-Go text.
			if textSnapshot == nil {
				text, err := capture.Take(a.Root)
				if err != nil {
					return "解释已过期：无法检查当前依据"
				}
				textSnapshot = &text
			}
			f, exists = textSnapshot.Files[e.File]
		}
		if e.Role == "before" {
			if !after[e.File] && !a.pathAbsent(e.File) {
				return "解释已过期：已删除文件再次出现"
			}
			continue
		}
		if e.Role == "after" {
			if !exists || f.Hash != e.Hash {
				return "解释已过期：文件版本变化或无法定位"
			}
			continue
		}
		if !exists || e.Source == "" || strings.Count(f.Content, e.Source) != 1 {
			return "解释已过期：引用变化或无法唯一定位"
		}
	}
	return "已引用证据未变"
}

func (a *App) pathAbsent(file string) bool {
	if !safeRelative(file) {
		return false
	}
	root, err := os.OpenRoot(a.Root)
	if err != nil {
		return false
	}
	defer root.Close()
	_, err = root.Lstat(file)
	return errors.Is(err, os.ErrNotExist)
}

func (a *App) sourceDigest(e Explanation) string { e.Freshness = ""; return digest(e) }

// RevisionExists is used during preview confirmation, not as a filesystem path from user input.
func (a *App) revisionExists(id string) bool {
	if !validID(id) {
		return false
	}
	_, e := os.Stat(filepath.Join(a.State, "explanation-"+id+".json"))
	return e == nil
}
