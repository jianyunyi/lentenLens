package app

import (
	"errors"
	"intentlens/internal/capture"
	"intentlens/internal/explain"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Active struct {
	ID       string           `json:"id"`
	Session  string           `json:"session"`
	Started  string           `json:"started"`
	Snapshot capture.Snapshot `json:"snapshot"`
}
type Round struct {
	Analysis *Explanation     `json:"analysis,omitempty"`
	ID       string           `json:"id"`
	Session  string           `json:"session"`
	Started  string           `json:"started"`
	Ended    string           `json:"ended"`
	Changes  []capture.Change `json:"changes"`
	Omitted  []string         `json:"omitted"`
	Notes    []string         `json:"notes"`
	Source   string           `json:"source"`
}
type Settings struct {
	Model explain.Config `json:"model"`
}

func (a *App) Start(session string) (Active, error) {
	unlock, e := a.lock("capture")
	if e != nil {
		return Active{}, e
	}
	defer unlock()
	if session == "" {
		return Active{}, errors.New("缺少 session_id")
	}
	var old Active
	e = a.read("active.json", &old)
	if e == nil {
		return Active{}, errors.New("已有未结束轮次；请结束或显式取消，不能覆盖基线")
	}
	if !errors.Is(e, os.ErrNotExist) {
		return Active{}, e
	}
	snap, e := capture.Take(a.Root)
	if e != nil {
		return Active{}, e
	}
	active := Active{ID: newID(), Session: session, Started: time.Now().UTC().Format(time.RFC3339Nano), Snapshot: snap}
	// Bind a reused session to its new round before publishing the baseline.
	if e = a.write(captureRefName(session), active.ID); e != nil {
		return Active{}, e
	}
	return active, a.write("active.json", active)
}

func (a *App) Stop(session string, expectedRoundIDs ...string) (Round, error) {
	if len(expectedRoundIDs) > 1 {
		return Round{}, errors.New("Stop 只接受一个期望轮次 ID")
	}
	expectedID := ""
	if len(expectedRoundIDs) == 1 {
		expectedID = expectedRoundIDs[0]
	}
	unlock, e := a.lock("capture")
	if e != nil {
		return Round{}, e
	}
	defer unlock()
	var active Active
	e = a.read("active.json", &active)
	if errors.Is(e, os.ErrNotExist) && session != "" {
		if expectedID != "" {
			committed, err := a.committedRound(session, expectedID)
			if err == nil {
				if err = a.scrubCommittedRound(&committed); err != nil {
					return Round{}, err
				}
				return committed, nil
			}
			if !errors.Is(err, os.ErrNotExist) {
				return Round{}, err
			}
		} else {
			committed, err := a.retryRound(session)
			if !errors.Is(err, os.ErrNotExist) {
				return committed, err
			}
		}
	}
	if e != nil {
		return Round{}, errors.New("缺少有效的轮次开始基线")
	}
	if session == "" || active.Session != session {
		return Round{}, errors.New("会话与开始基线不匹配")
	}
	if expectedID != "" && expectedID != active.ID {
		committed, err := a.committedRound(session, expectedID)
		if err == nil {
			if err = a.scrubCommittedRound(&committed); err != nil {
				return Round{}, err
			}
			return committed, nil
		}
		return Round{}, errors.New("期望轮次 ID 与活动轮次不匹配")
	}
	committed, e := a.committedRound(active.Session, active.ID)
	if e == nil {
		// A prior Stop published the round but did not remove the baseline.
		if e = a.scrubCommittedRound(&committed); e != nil {
			return Round{}, e
		}
		return committed, a.finishRound(active)
	}
	if !errors.Is(e, os.ErrNotExist) {
		return Round{}, e
	}
	after, policy, e := capture.TakeWithPolicy(a.Root)
	if e != nil {
		return Round{}, e
	}
	round, e := a.buildRound(active, after, policy)
	if e != nil {
		return Round{}, e
	}
	if writeErr := a.write("round-"+round.ID+".json", round); writeErr != nil {
		return Round{}, writeErr
	}
	if removeErr := a.finishRound(active); removeErr != nil {
		return Round{}, removeErr
	}
	return round, nil
}

// CaptureRoundID returns the round currently associated with a session. Start
// persists this reference so independent adapters can include it in Stop.
func (a *App) CaptureRoundID(session string) (string, error) {
	if strings.TrimSpace(session) == "" {
		return "", errors.New("缺少 session_id")
	}
	var id string
	err := a.read(captureRefName(session), &id)
	if err == nil && validID(id) {
		return id, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	var active Active
	activeErr := a.read("active.json", &active)
	if activeErr == nil && active.Session == session && validID(active.ID) {
		return active.ID, nil
	}
	if activeErr != nil && !errors.Is(activeErr, os.ErrNotExist) {
		return "", activeErr
	}
	return "", os.ErrNotExist
}

func (a *App) buildRound(active Active, after capture.Snapshot, policy capture.Policy) (Round, error) {
	notes := []string{"本轮工作区变化，来源未确认", "用户原话和 transcript 未采集；选择动机未记录"}
	if after.PolicyHash != active.Snapshot.PolicyHash {
		notes = append(notes, "本轮排除规则变化；新增允许范围可能没有可靠起始基线")
		for p := range after.Files {
			if _, known := active.Snapshot.Files[p]; !known {
				after.Omitted = append(after.Omitted, p)
			}
		}
	}
	// Apply the current policy to retained baseline too; ignored contents never enter a new round.
	for p := range active.Snapshot.Files {
		ok, err := policy.Allowed(a.Root, p)
		if err != nil {
			return Round{}, err
		}
		if !ok {
			delete(active.Snapshot.Files, p)
		}
	}
	// Keep omissions as coverage information for Diff, then filter what is published.
	changes := capture.Diff(active.Snapshot, after)
	omitted := append(active.Snapshot.Omitted, after.Omitted...)
	kept := omitted[:0]
	for _, p := range omitted {
		ok, err := policy.Allowed(a.Root, p)
		if err != nil {
			return Round{}, err
		}
		if ok {
			kept = append(kept, p)
		}
	}
	return Round{ID: active.ID, Session: active.Session, Started: active.Started, Ended: time.Now().UTC().Format(time.RFC3339Nano), Changes: changes, Omitted: kept, Notes: notes, Source: "unconfirmed"}, nil
}

func captureRefName(session string) string {
	return "capture-ref-" + hash(session) + ".json"
}

func (a *App) finishRound(active Active) error {
	name := captureRefName(active.Session)
	var id string
	if err := a.read(name, &id); err != nil || id != active.ID {
		// An older version may have written the baseline without a reference.
		if err := a.write(name, active.ID); err != nil {
			return err
		}
	}
	return a.removeActive()
}

func (a *App) retryRound(session string) (Round, error) {
	var round Round
	var id string
	err := a.read(captureRefName(session), &id)
	if errors.Is(err, os.ErrNotExist) {
		round, err = a.latestLegacyRound(session)
	} else if err == nil {
		round, err = a.committedRound(session, id)
	}
	if err != nil {
		return Round{}, err
	}
	if err = a.scrubCommittedRound(&round); err != nil {
		return Round{}, err
	}
	return round, nil
}

// Older versions could remove the baseline before a reference was recorded.
func (a *App) latestLegacyRound(session string) (Round, error) {
	entries, err := os.ReadDir(a.State)
	if err != nil {
		return Round{}, err
	}
	var latest Round
	var latestEnd time.Time
	for _, entry := range entries {
		name, ok := strings.CutPrefix(entry.Name(), "round-")
		if !ok {
			continue
		}
		id, ok := strings.CutSuffix(name, ".json")
		if !ok || !validID(id) {
			continue
		}
		var round Round
		if err := a.read(entry.Name(), &round); err != nil {
			return Round{}, err
		}
		if round.ID != id || round.Session != session {
			continue
		}
		if ended, err := time.Parse(time.RFC3339Nano, round.Ended); err == nil && ended.After(latestEnd) {
			latest, latestEnd = round, ended
		}
	}
	if latest.ID == "" {
		return Round{}, os.ErrNotExist
	}
	return latest, nil
}

func (a *App) committedRound(session, id string) (Round, error) {
	var round Round
	if !validID(id) {
		return round, errors.New("无效轮次 ID")
	}
	if err := a.read("round-"+id+".json", &round); err != nil {
		return Round{}, err
	}
	if round.ID != id || round.Session != session {
		return Round{}, errors.New("已提交轮次与开始基线不匹配")
	}
	return round, nil
}

func (a *App) Abort() error {
	unlock, e := a.lock("capture")
	if e != nil {
		return e
	}
	defer unlock()
	return a.removeActive()
}

func (a *App) removeActive() error {
	e := os.Remove(filepath.Join(a.State, "active.json"))
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	return e
}

func (a *App) List() ([]Round, error) {
	if e := a.scrubExcluded(); e != nil {
		return nil, e
	}
	entries, e := os.ReadDir(a.State)
	if e != nil {
		return nil, e
	}
	rounds := []Round{}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "round-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var r Round
		if readErr := a.read(entry.Name(), &r); readErr != nil {
			return nil, readErr
		}
		filtered := []capture.Change{}
		for _, c := range r.Changes {
			ok, err := capture.Allowed(a.Root, c.Path)
			if err != nil {
				return nil, err
			}
			if ok {
				filtered = append(filtered, c)
			}
		}
		r.Changes = filtered
		rounds = append(rounds, r)
	}
	if analysesErr := a.attachRoundAnalyses(rounds, entries); analysesErr != nil {
		return nil, analysesErr
	}
	sort.Slice(rounds, func(i, j int) bool { return rounds[i].Started > rounds[j].Started })
	return rounds, nil
}

func (a *App) Config() (Settings, error) {
	var s Settings
	e := a.read("settings.json", &s)
	if errors.Is(e, os.ErrNotExist) {
		s.Model.TokenLimit = 200000
		e = nil
	}
	if e == nil {
		s.Model, e = s.Model.Resolved()
	}
	return s, e
}
func (a *App) Configure(s Settings, consent bool) error {
	var err error
	s.Model, err = s.Model.Resolved()
	if err != nil {
		return err
	}
	if s.Model.Enabled && (!consent || s.Model.Model == "" || s.Model.TokenLimit <= 0) {
		return errors.New("启用需确认代码发送范围、模型名称和正数 token 预算")
	}
	unlock, e := a.lock("settings")
	if e != nil {
		return e
	}
	defer unlock()
	return a.write("settings.json", s)
}
func (a *App) Usage() (explain.Usage, error) {
	var u explain.Usage
	e := a.read("usage.json", &u)
	if errors.Is(e, os.ErrNotExist) {
		return u, nil
	}
	return u, e
}
