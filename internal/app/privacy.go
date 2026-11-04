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
			if readErr := a.read(name, &active); readErr != nil {
				return readErr
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
				if writeErr := a.write(name, active); writeErr != nil {
					return writeErr
				}
			}
		case strings.HasPrefix(name, "round-") && strings.HasSuffix(name, ".json"):
			var r Round
			if readErr := a.read(name, &r); readErr != nil {
				return readErr
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
				if writeErr := a.write(name, r); writeErr != nil {
					return writeErr
				}
			}
		case strings.HasPrefix(name, "explanation-") && strings.HasSuffix(name, ".json"):
			var saved Explanation
			if readErr := a.read(name, &saved); readErr != nil {
				return readErr
			}
			for i := range saved.Evidence {
				allowed, err := a.evidenceAllowed(saved.Evidence[i])
				if err != nil {
					return err
				}
				if !allowed {
					if removeErr := os.Remove(filepath.Join(a.State, name)); removeErr != nil {
						return removeErr
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
			if readErr := a.read(name, &draft); readErr != nil {
				return readErr
			}
			sourceID = draft.SourceID
		} else if strings.HasPrefix(name, "correction-") && strings.HasSuffix(name, ".json") {
			var correction Correction
			if readErr := a.read(name, &correction); readErr != nil {
				return readErr
			}
			sourceID = correction.ExplanationID
		}
		if sourceID != "" && !a.revisionExists(sourceID) {
			if removeErr := os.Remove(filepath.Join(a.State, name)); removeErr != nil {
				return removeErr
			}
		}
	}
	return a.scrubExercises()
}

func (a *App) Diagnostics() ([]map[string]string, error) {
	out := []map[string]string{}
	for _, name := range []string{"hook-diagnostic.json", "analysis-diagnostic.json", "recovery-diagnostic.json"} {
		var item map[string]string
		if err := a.read(name, &item); err == nil {
			out = append(out, item)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return out, nil
}
