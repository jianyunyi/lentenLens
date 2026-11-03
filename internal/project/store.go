// Package project stores private round history outside the captured repository.
package project

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"intentlens/internal/capture"
)

type Store struct {
	Root string
	Dir  string
}
type Active struct {
	ID        string           `json:"id"`
	StartedAt time.Time        `json:"started_at"`
	Baseline  capture.Snapshot `json:"baseline"`
}
type Round struct {
	ID             string           `json:"id"`
	StartedAt      time.Time        `json:"started_at"`
	EndedAt        time.Time        `json:"ended_at"`
	Changes        []capture.Change `json:"changes"`
	Skipped        []string         `json:"skipped,omitempty"`
	AnalysisStatus string           `json:"analysis_status"`
}
type state struct {
	Version int     `json:"version"`
	Root    string  `json:"root"`
	Active  *Active `json:"active,omitempty"`
	Rounds  []Round `json:"rounds"`
}

func Open(root, stateRoot string) (*Store, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("project is not a directory")
	}
	if stateRoot == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		stateRoot = filepath.Join(cache, "IntentLens", "projects")
	}
	stateRoot, err = filepath.Abs(stateRoot)
	if err != nil {
		return nil, err
	}
	if within(root, stateRoot) {
		return nil, fmt.Errorf("private state directory must be outside the project")
	}
	if err = os.MkdirAll(stateRoot, 0700); err != nil {
		return nil, err
	}
	stateRoot, err = filepath.EvalSymlinks(stateRoot)
	if err != nil {
		return nil, err
	}
	if within(root, stateRoot) {
		return nil, fmt.Errorf("private state directory resolves inside the project")
	}
	key := root
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	sum := sha256.Sum256([]byte(key))
	dir := filepath.Join(stateRoot, hex.EncodeToString(sum[:]))
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	if canonical != dir {
		return nil, fmt.Errorf("project state directory must not be a symlink")
	}
	return &Store{Root: root, Dir: dir}, nil
}

func within(root, name string) bool {
	rel, err := filepath.Rel(root, name)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func (s *Store) transact(fn func(*state, capture.Policy) error) error {
	lockPath := filepath.Join(s.Dir, "state.lock")
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("cannot lock local state (another process or interrupted run): %w", err)
	}
	defer os.Remove(lockPath)
	defer lock.Close()
	policy, err := capture.LoadPolicy(s.Root)
	if err != nil {
		return err
	}
	st := state{Version: 1, Root: s.Root, Rounds: []Round{}}
	name := filepath.Join(s.Dir, "state.json")
	if info, err := os.Lstat(name); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("state must be a regular file")
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(data, &st); err != nil {
			return fmt.Errorf("invalid state: %w", err)
		}
		if st.Version != 1 || st.Root != s.Root {
			return fmt.Errorf("state version or project mismatch")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Enforce current exclusions on historical content before returning any output.
	if st.Active != nil {
		st.Active.Baseline = policy.Filter(st.Active.Baseline)
	}
	for i := range st.Rounds {
		kept := []capture.Change{}
		for _, c := range st.Rounds[i].Changes {
			if policy.Allows(c.Path) {
				kept = append(kept, c)
			}
		}
		st.Rounds[i].Changes = kept
		skips := []string{}
		for _, skip := range st.Rounds[i].Skipped {
			if policy.Allows(skip) {
				skips = append(skips, skip)
			}
		}
		st.Rounds[i].Skipped = skips
	}
	// Save the scrubbed state even if the requested operation subsequently fails.
	if err = s.save(st); err != nil {
		return err
	}
	if err = fn(&st, policy); err != nil {
		return err
	}
	return s.save(st)
}

func (s *Store) save(st state) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.Dir, ".state-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(s.Dir, "state.json"))
}

func (s *Store) Start() (Active, error) {
	var result Active
	err := s.transact(func(st *state, _ capture.Policy) error {
		if st.Active != nil {
			return fmt.Errorf("round %s is already active; end or cancel it first", st.Active.ID)
		}
		snap, err := capture.Take(s.Root)
		if err != nil {
			return err
		}
		id := make([]byte, 12)
		if _, err = rand.Read(id); err != nil {
			return err
		}
		result = Active{ID: hex.EncodeToString(id), StartedAt: time.Now().UTC(), Baseline: snap}
		st.Active = &result
		return nil
	})
	return result, err
}

func (s *Store) End() (Round, error) {
	var result Round
	err := s.transact(func(st *state, _ capture.Policy) error {
		if st.Active == nil {
			return fmt.Errorf("no active round; run start before editing")
		}
		after, err := capture.Take(s.Root)
		if err != nil {
			return err
		}
		// Read policy again in Take; filter the baseline with that same latest policy.
		latestPolicy, err := capture.LoadPolicy(s.Root)
		if err != nil {
			return err
		}
		before := latestPolicy.Filter(st.Active.Baseline)
		after = latestPolicy.Filter(after)
		result = Round{ID: st.Active.ID, StartedAt: st.Active.StartedAt, EndedAt: time.Now().UTC(), Changes: capture.Diff(before, after), Skipped: append(before.Omitted, after.Omitted...), AnalysisStatus: "not_generated_no_model_configured"}
		st.Rounds = append(st.Rounds, result)
		st.Active = nil
		return nil
	})
	return result, err
}

func (s *Store) History() ([]Round, error) {
	var result []Round
	err := s.transact(func(st *state, _ capture.Policy) error { result = st.Rounds; return nil })
	return result, err
}
func (s *Store) Cancel() error {
	return s.transact(func(st *state, _ capture.Policy) error {
		if st.Active == nil {
			return fmt.Errorf("no active round")
		}
		st.Active = nil
		return nil
	})
}
func (s *Store) Status() (map[string]any, error) {
	var result map[string]any
	err := s.transact(func(st *state, _ capture.Policy) error {
		result = map[string]any{"project": s.Root, "state_directory": s.Dir, "completed_rounds": len(st.Rounds), "active_round": nil, "analysis": "not_configured"}
		if st.Active != nil {
			result["active_round"] = map[string]any{"id": st.Active.ID, "started_at": st.Active.StartedAt, "captured_files": len(st.Active.Baseline.Files), "skipped": st.Active.Baseline.Omitted}
		}
		return nil
	})
	return result, err
}
