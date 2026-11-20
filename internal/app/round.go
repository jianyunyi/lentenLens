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
	return active, a.write("active.json", active)
}

func (a *App) Stop(session string) (Round, error) {
	unlock, e := a.lock("capture")
	if e != nil {
		return Round{}, e
	}
	defer unlock()
	var active Active
	if e = a.read("active.json", &active); e != nil {
		return Round{}, errors.New("缺少有效的轮次开始基线")
	}
	if session == "" || active.Session != session {
		return Round{}, errors.New("会话与开始基线不匹配")
	}
	committed, e := a.committedRound(active)
	if e == nil {
		// A prior Stop published the round but did not remove the baseline.
		return committed, a.removeActive()
	}
	if !errors.Is(e, os.ErrNotExist) {
		return Round{}, e
	}
	after, e := capture.Take(a.Root)
	if e != nil {
		return Round{}, e
	}
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
		ok, err := capture.Allowed(a.Root, p)
		if err != nil {
			return Round{}, err
		}
		if !ok {
			delete(active.Snapshot.Files, p)
		}
	}
	round := Round{ID: active.ID, Session: session, Started: active.Started, Ended: time.Now().UTC().Format(time.RFC3339Nano), Changes: capture.Diff(active.Snapshot, after), Omitted: append(active.Snapshot.Omitted, after.Omitted...), Notes: notes, Source: "unconfirmed"}
	if writeErr := a.write("round-"+round.ID+".json", round); writeErr != nil {
		return Round{}, writeErr
	}
	if removeErr := a.removeActive(); removeErr != nil {
		return Round{}, removeErr
	}
	return round, nil
}

func (a *App) committedRound(active Active) (Round, error) {
	var round Round
	if !validID(active.ID) {
		return round, errors.New("无效轮次 ID")
	}
	if err := a.read("round-"+active.ID+".json", &round); err != nil {
		return Round{}, err
	}
	if round.ID != active.ID || round.Session != active.Session {
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
