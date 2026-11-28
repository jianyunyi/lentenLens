package app

import (
	"errors"
	"fmt"
	"intentlens/internal/capture"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// scrubExcluded irreversibly removes revoked source material from local records.
// It runs before data is returned and does not contact a model or read excluded content.
func (a *App) scrubExcluded() error {
	a.scrubMu.Lock()
	defer a.scrubMu.Unlock()
	unlock, e := a.prescanAwareCaptureLock()
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
	// One CLI process scrubs in Open and again in most commands; skip the repeat
	// pass while the policy and every record's name, size and mtime are unchanged.
	before, e := scrubSignature(policy, entries)
	if e != nil {
		return e
	}
	if before == a.scrubbedState && (a.prescanExpiry == nil || time.Now().Before(a.prescanExpires)) {
		// Evidence admission also depends on project link state, which the
		// signature cannot see; re-check every cited path.
		unchanged := true
		for p := range a.scrubbedEvidence {
			ok, err := capture.Allowed(a.Root, p)
			if err != nil {
				return err
			}
			if !ok {
				unchanged = false
				break
			}
		}
		if unchanged {
			return nil
		}
	}
	a.scrubbedState, a.scrubbedEvidence = "", map[string]struct{}{}
	if e = a.scrubEntries(policy, entries); e != nil {
		return e
	}
	if entries, e = os.ReadDir(a.State); e != nil {
		return e
	}
	a.scrubbedState, e = scrubSignature(policy, entries)
	return e
}

func scrubSignature(policy capture.Policy, entries []os.DirEntry) (string, error) {
	var b strings.Builder
	b.WriteString(policy.Hash())
	for _, ent := range entries {
		info, err := ent.Info()
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "\x00%s\x00%d\x00%d", ent.Name(), info.Size(), info.ModTime().UnixNano())
	}
	return hash(b.String()), nil
}

func (a *App) scrubEntries(policy capture.Policy, entries []os.DirEntry) error {
	if err := a.scrubPrescans(policy, entries); err != nil {
		return err
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
			if err := a.scrubRound(name, &r, policy); err != nil {
				return err
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
				if saved.Evidence[i].File != "" || codeEvidence(saved.Evidence[i]) {
					a.scrubbedEvidence[saved.Evidence[i].File] = struct{}{}
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

// Stop holds the capture lock already, so only scrub the round it will return.
func (a *App) scrubCommittedRound(round *Round) error {
	policy, err := capture.LoadPolicy(a.Root)
	if err != nil {
		return err
	}
	return a.scrubRound("round-"+round.ID+".json", round, policy)
}

func (a *App) scrubRound(name string, round *Round, policy capture.Policy) error {
	changes := []capture.Change{}
	for _, c := range round.Changes {
		if policy.Allows(c.Path) {
			changes = append(changes, c)
		}
	}
	omitted := []string{}
	for _, p := range round.Omitted {
		if policy.Allows(p) {
			omitted = append(omitted, p)
		}
	}
	if len(changes) == len(round.Changes) && len(omitted) == len(round.Omitted) {
		return nil
	}
	round.Changes, round.Omitted, round.Analysis = changes, omitted, nil
	return a.write(name, round)
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
