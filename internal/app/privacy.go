package app

import (
	"errors"
	"intentlens/internal/capture"
	"os"
	"path/filepath"
	"strings"
)

// scrubExcluded irreversibly removes revoked source material from local records.
// It runs before data is returned and does not contact a model or read excluded content.
func (a *App) scrubExcluded() error {
	unlock, e := a.lock("capture")
	if e != nil {
		return e
	}
	defer unlock()
	policy, e := capture.LoadPolicy(a.Root)
	if e != nil {
		return e
	}
	entries, e := os.ReadDir(a.State)
	if e != nil {
		return e
	}
	for _, ent := range entries {
		name := ent.Name()
		switch {
		case name == "active.json":
			var active Active
			if e = a.read(name, &active); e != nil {
				return e
			}
			changed := false
			for p := range active.Snapshot.Files {
				if !policy.Allows(p) {
					delete(active.Snapshot.Files, p)
					changed = true
				}
			}
			kept := []string{}
			for _, p := range active.Snapshot.Omitted {
				if policy.Allows(p) {
					kept = append(kept, p)
				} else {
					changed = true
				}
			}
			active.Snapshot.Omitted = kept
			// Preserve original policy hash so Stop can report scope changes.
			if changed {
				if e = a.write(name, active); e != nil {
					return e
				}
			}
		case strings.HasPrefix(name, "round-") && strings.HasSuffix(name, ".json"):
			var r Round
			if e = a.read(name, &r); e != nil {
				return e
			}
			changed := false
			kept := []capture.Change{}
			for _, c := range r.Changes {
				if policy.Allows(c.Path) {
					kept = append(kept, c)
				} else {
					changed = true
				}
			}
			r.Changes = kept
			omissions := []string{}
			for _, p := range r.Omitted {
				if policy.Allows(p) {
					omissions = append(omissions, p)
				} else {
					changed = true
				}
			}
			r.Omitted = omissions
			if changed {
				r.Analysis = nil
				if e = a.write(name, r); e != nil {
					return e
				}
			}
		case strings.HasPrefix(name, "explanation-") && strings.HasSuffix(name, ".json"):
			var saved Explanation
			if e = a.read(name, &saved); e != nil {
				return e
			}
			for _, ev := range saved.Evidence {
				allowed, err := a.evidenceAllowed(ev)
				if err != nil {
					return err
				}
				if !allowed {
					if e = os.Remove(filepath.Join(a.State, name)); e != nil {
						return e
					}
					break
				}
			}
		}
	}
	// Dependent private records must not outlive revoked source explanations.
	for _, ent := range entries {
		name := ent.Name()
		var sourceID string
		if strings.HasPrefix(name, "share-draft-") && strings.HasSuffix(name, ".json") {
			var draft ShareDraft
			if e = a.read(name, &draft); e != nil {
				return e
			}
			sourceID = draft.SourceID
		} else if strings.HasPrefix(name, "correction-") && strings.HasSuffix(name, ".json") {
			var correction Correction
			if e = a.read(name, &correction); e != nil {
				return e
			}
			sourceID = correction.ExplanationID
		}
		if sourceID != "" && !a.revisionExists(sourceID) {
			if e = os.Remove(filepath.Join(a.State, name)); e != nil {
				return e
			}
		}
	}
	return a.scrubExercises()
}

func (a *App) attachRoundAnalysis(r *Round) error {
	var diagnostic map[string]string
	if err := a.read("analysis-diagnostic.json", &diagnostic); err == nil && diagnostic["round"] == r.ID {
		r.Notes = append(r.Notes, "最近一次轮次解释生成失败："+diagnostic["error"])
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	entries, e := os.ReadDir(a.State)
	if e != nil {
		return e
	}
	for _, ent := range entries {
		if !strings.HasPrefix(ent.Name(), "explanation-") || !strings.HasSuffix(ent.Name(), ".json") {
			continue
		}
		var saved Explanation
		if e = a.read(ent.Name(), &saved); e != nil {
			return e
		}
		if saved.FunctionID == "round:"+r.ID && (r.Analysis == nil || saved.Created > r.Analysis.Created) {
			copy := saved
			r.Analysis = &copy
		}
	}
	if r.Analysis == nil {
		return nil
	}
	if e = a.attachCorrections(r.Analysis); e != nil {
		return e
	}
	current, e := capture.Take(a.Root)
	if e != nil {
		return e
	}
	r.Analysis.Freshness = "已引用证据未变"
	for _, c := range r.Changes {
		f, exists := current.Files[c.Path]
		if (c.After == nil && !a.pathAbsent(c.Path)) || (c.After != nil && (!exists || f.Hash != c.After.Hash)) {
			r.Analysis.Freshness = "解释已过期：引用变化或无法定位"
			break
		}
	}
	return nil
}

func (a *App) Diagnostics() ([]map[string]string, error) {
	out := []map[string]string{}
	for _, name := range []string{"hook-diagnostic.json", "analysis-diagnostic.json"} {
		var item map[string]string
		if err := a.read(name, &item); err == nil {
			out = append(out, item)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return out, nil
}
