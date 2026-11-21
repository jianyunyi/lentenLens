package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"intentlens/internal/capture"
)

func TestCommittedRoundWithLeftoverBaseline(t *testing.T) {
	for _, action := range []string{"retry Stop", "reopen"} {
		t.Run(action, func(t *testing.T) {
			a, root := fixture(t)
			put(t, root, "main.go", "package p\nvar value = 1\n")
			active, err := a.Start("session")
			if err != nil {
				t.Fatal(err)
			}
			put(t, root, "main.go", "package p\nvar value = 2\n")
			committed, err := a.Stop(active.Session)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(a.State, "round-"+active.ID+".json")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			// Recreate an older version's disk state after publication but before cleanup.
			if err := os.Remove(filepath.Join(a.State, captureRefName(active.Session))); err != nil {
				t.Fatal(err)
			}
			if err := a.write("active.json", active); err != nil {
				t.Fatal(err)
			}
			put(t, root, "main.go", "package p\nvar value = 3\n")
			if action == "retry Stop" {
				got, err := a.Stop(active.Session)
				if err != nil || digest(got) != digest(committed) {
					t.Fatalf("Stop returned %#v, %v, want the committed round", got, err)
				}
			} else {
				a, err = Open(root, filepath.Dir(a.State))
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := os.Stat(filepath.Join(a.State, "active.json")); !os.IsNotExist(err) {
				t.Fatalf("active baseline remains: %v", err)
			}
			if got, err := a.Stop(active.Session); err != nil || digest(got) != digest(committed) {
				t.Fatalf("Stop after cleanup returned %#v, %v, want the committed round", got, err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("committed round changed: %v", err)
			}
		})
	}
}

func TestStopRetryWithReusedSession(t *testing.T) {
	a, root := fixture(t)
	const session = "reused"
	var first, latest Round
	for _, value := range []string{"1", "2"} {
		active, err := a.Start(session)
		if err != nil {
			t.Fatal(err)
		}
		put(t, root, "main.go", "package p\nvar value = "+value+"\n")
		round, err := a.Stop(session)
		if err != nil || round.ID != active.ID {
			t.Fatalf("Stop returned round %q, %v, want %q", round.ID, err, active.ID)
		}
		first, latest = latest, round
	}
	if first.ID == latest.ID {
		t.Fatal("the reused session did not get a new round ID")
	}
	if got, err := a.Stop(session); err != nil || digest(got) != digest(latest) {
		t.Fatalf("retry returned round %q, %v, want %q", got.ID, err, latest.ID)
	}
	// A previously completed session cannot stand in for a different session.
	if _, err := a.Stop("never-started"); err == nil {
		t.Fatal("Stop accepted a session with no baseline or history")
	}
	// Preexisting history can be recovered even if the older version already
	// removed the baseline and never wrote a reference.
	if err := os.Remove(filepath.Join(a.State, captureRefName(session))); err != nil {
		t.Fatal(err)
	}
	if got, err := a.Stop(session); err != nil || digest(got) != digest(latest) {
		t.Fatalf("legacy retry returned round %q, %v, want %q", got.ID, err, latest.ID)
	}
	active, err := a.Start(session)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.AbortCapture(session, active.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Stop(session); err == nil {
		t.Fatal("Stop returned an older round after the session's latest round was canceled")
	}
}

func TestStopRoundIdentityReturnsOnlyRequestedRoundAndProtectsActiveRound(t *testing.T) {
	a, root := fixture(t)
	const session = "reused-session"
	firstActive, err := a.Start(session)
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "main.go", "package p\nvar value = 1\n")
	first, err := a.Stop(session, firstActive.ID)
	if err != nil || first.ID != firstActive.ID {
		t.Fatalf("first stop = %#v, %v", first, err)
	}

	secondActive, err := a.Start(session)
	if err != nil {
		t.Fatal(err)
	}
	if secondActive.ID == first.ID {
		t.Fatal("Start reused a committed round ID")
	}
	put(t, root, "main.go", "package p\nvar value = 2\n")
	got, err := a.Stop(session, first.ID)
	if err != nil || got.ID != first.ID {
		t.Fatalf("late stop = %#v, %v; want committed round %q", got, err, first.ID)
	}
	active, err := a.CaptureState()
	if err != nil || active == nil || active.ID != secondActive.ID {
		t.Fatalf("late Stop changed active round: %#v, %v", active, err)
	}

	if _, err := a.Stop(session, "ffffffffffffffffffffffff"); err == nil || !strings.Contains(err.Error(), "期望轮次 ID 与活动轮次不匹配") {
		t.Fatalf("wrong round ID was not rejected explicitly: %v", err)
	}
	active, err = a.CaptureState()
	if err != nil || active == nil || active.ID != secondActive.ID {
		t.Fatalf("rejected Stop changed active round: %#v, %v", active, err)
	}
	second, err := a.Stop(session, secondActive.ID)
	if err != nil || second.ID != secondActive.ID {
		t.Fatalf("correct second stop = %#v, %v", second, err)
	}
}

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

func TestBuildRoundUsesPolicyFromAfterCapture(t *testing.T) {
	a, root := fixture(t)
	put(t, root, "tracked.go", "package p\nvar value = 1\n")
	put(t, root, "ignored.go", "package p\n")
	active, err := a.Start("session")
	if err != nil {
		t.Fatal(err)
	}

	put(t, root, "tracked.go", "package p\nvar value = 2\n")
	put(t, root, ".intentignore", "ignored.go\n")
	after, policy, err := capture.TakeWithPolicy(root)
	if err != nil {
		t.Fatal(err)
	}
	// Reverse the rules between Stop's capture and its baseline filtering.
	put(t, root, ".intentignore", "tracked.go\n")
	round, err := a.buildRound(active, after, policy)
	if err != nil {
		t.Fatal(err)
	}
	if len(round.Changes) != 1 || round.Changes[0].Path != "tracked.go" || round.Changes[0].Kind != "modified" {
		t.Fatalf("changes = %+v, want only tracked.go modified", round.Changes)
	}
}

func TestFirstStopExcludesRevokedPaths(t *testing.T) {
	a, root := fixture(t)
	put(t, root, "revoked.go", "package p\n")
	put(t, root, "kept.go", "package p\n")
	put(t, root, "revoked.bin", "\x00")
	put(t, root, "before.bin", "\x00")
	active, err := a.Start("session")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := active.Snapshot.Files["revoked.go"]; !ok || !slices.Contains(active.Snapshot.Omitted, "revoked.bin") {
		t.Fatalf("baseline did not capture expected paths: %+v", active.Snapshot)
	}

	put(t, root, ".intentignore", "revoked.go\nrevoked.bin\n")
	put(t, root, "kept.go", "package p\nvar kept = true\n")
	if err := os.Remove(filepath.Join(root, "before.bin")); err != nil {
		t.Fatal(err)
	}
	put(t, root, "after.bin", "\x00")
	round, err := a.Stop(active.Session)
	if err != nil {
		t.Fatal(err)
	}
	if len(round.Changes) != 1 || round.Changes[0].Path != "kept.go" {
		t.Fatalf("changes = %+v, want only kept.go", round.Changes)
	}
	if want := []string{"before.bin", "after.bin"}; !slices.Equal(round.Omitted, want) {
		t.Fatalf("omissions = %q, want %q", round.Omitted, want)
	}
	response, err := json.Marshal(round)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(filepath.Join(a.State, "round-"+round.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"revoked.go", "revoked.bin"} {
		if bytes.Contains(response, []byte(path)) || bytes.Contains(saved, []byte(path)) {
			t.Errorf("excluded path %q appears in the first Stop response or stored round", path)
		}
	}
}
