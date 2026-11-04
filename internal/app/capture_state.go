package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// CaptureState is the only metadata exposed for an active capture.
type CaptureState struct {
	ID      string `json:"id"`
	Session string `json:"session"`
	Started string `json:"started"`
}

// CaptureState returns the active capture metadata, or nil when no capture is active.
// It never exposes the filesystem snapshot held in the private active record.
func (a *App) CaptureState() (*CaptureState, error) {
	unlock, err := a.lock("capture")
	if err != nil {
		return nil, err
	}
	defer unlock()

	var active Active
	if err = a.read("active.json", &active); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, errors.New("活动轮次基线损坏：" + err.Error())
	}
	if strings.TrimSpace(active.ID) == "" || strings.TrimSpace(active.Session) == "" || strings.TrimSpace(active.Started) == "" {
		return nil, errors.New("活动轮次基线缺少有效元数据")
	}
	return &CaptureState{ID: active.ID, Session: active.Session, Started: active.Started}, nil
}

// AbortCapture removes only the active baseline after an exact, confirmed match.
func (a *App) AbortCapture(session, roundID string, confirm bool) error {
	unlock, err := a.lock("capture")
	if err != nil {
		return err
	}
	defer unlock()

	var active Active
	if err = a.read("active.json", &active); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("没有活动轮次可取消")
		}
		return errors.New("活动轮次基线损坏：" + err.Error())
	}
	if strings.TrimSpace(active.ID) == "" || strings.TrimSpace(active.Session) == "" || strings.TrimSpace(active.Started) == "" {
		return errors.New("活动轮次基线缺少有效元数据")
	}
	if !confirm {
		return errors.New("取消活动轮次需要 confirm=true")
	}
	if session == "" || roundID == "" {
		return errors.New("取消活动轮次需要非空 session 和 round_id")
	}
	if active.Session != session || active.ID != roundID {
		return errors.New("session 与 round_id 与活动轮次基线不匹配")
	}
	if removeErr := os.Remove(filepath.Join(a.State, "active.json")); removeErr != nil {
		return removeErr
	}
	return nil
}
