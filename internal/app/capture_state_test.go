package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureStateIsMinimalAndAbortPreservesHistory(t *testing.T) {
	a, root := fixture(t)
	if state, err := a.CaptureState(); err != nil || state != nil {
		t.Fatalf("empty capture state = %#v, %v", state, err)
	}
	if _, err := a.Start("session"); err != nil {
		t.Fatal(err)
	}
	state, err := a.CaptureState()
	if err != nil || state == nil || state.Session != "session" || state.ID == "" || state.Started == "" {
		t.Fatalf("unexpected state: %#v, %v", state, err)
	}
	b, _ := json.Marshal(state)
	if strings.Contains(string(b), "snapshot") || strings.Contains(string(b), "source") || strings.Contains(string(b), "question") {
		t.Fatalf("capture state leaked private data: %s", b)
	}
	if err := a.AbortCapture("session", state.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.State, "active.json")); !os.IsNotExist(err) {
		t.Fatalf("active baseline remains: %v", err)
	}
	if rounds, err := a.List(); err != nil || len(rounds) != 0 {
		t.Fatalf("abort changed history: %v, %v", rounds, err)
	}
	_ = root
}

func TestAbortCaptureRejectsConfirmationSessionAndIDWithoutMutation(t *testing.T) {
	a, _ := fixture(t)
	if _, err := a.Start("session"); err != nil {
		t.Fatal(err)
	}
	state, err := a.CaptureState()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		session string
		id      string
		confirm bool
	}{
		{"confirmation", "session", state.ID, false},
		{"session", "other", state.ID, true},
		{"id", "session", "other", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := a.AbortCapture(tc.session, tc.id, tc.confirm); err == nil {
				t.Fatal("invalid abort accepted")
			}
			if current, err := a.CaptureState(); err != nil || current == nil || current.ID != state.ID {
				t.Fatalf("invalid abort changed baseline: %#v, %v", current, err)
			}
		})
	}
}

func TestCaptureStateReportsCorruptActive(t *testing.T) {
	a, _ := fixture(t)
	if err := os.WriteFile(filepath.Join(a.State, "active.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CaptureState(); err == nil {
		t.Fatal("corrupt active accepted")
	}
	if err := a.AbortCapture("s", "id", true); err == nil {
		t.Fatal("corrupt active abort accepted")
	}
}

func TestCaptureStateRejectsBlankMetadata(t *testing.T) {
	a, _ := fixture(t)
	if err := os.WriteFile(filepath.Join(a.State, "active.json"), []byte(`{"id":"round","session":"   ","started":"time"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CaptureState(); err == nil {
		t.Fatal("blank session accepted")
	}
	if err := a.AbortCapture("session", "round", true); err == nil {
		t.Fatal("blank session abort accepted")
	}
}
