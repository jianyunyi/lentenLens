package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"intentlens/internal/capture"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestReliabilityProcessHelper(t *testing.T) {
	mode := os.Getenv("INTENTLENS_TEST_HELPER")
	if mode == "" {
		return
	}
	if mode == "hold" || mode == "try" {
		a := &App{State: os.Getenv("INTENTLENS_TEST_STATE")}
		release, err := a.lock("capture")
		if err != nil {
			os.Exit(23)
		}
		defer release()
		if mode == "try" {
			return
		}
		fmt.Println("LOCKED")
		time.Sleep(time.Hour)
		return
	}
	if mode == "apply-crash" || mode == "apply-stage-crash" {
		a, err := Open(os.Getenv("INTENTLENS_TEST_ROOT"), os.Getenv("INTENTLENS_TEST_STATE"))
		if err != nil {
			t.Fatal(err)
		}
		policy, err := capture.LoadPolicy(a.Root)
		if err != nil {
			t.Fatal(err)
		}
		a.applyFault = func(int) error { os.Exit(93); return nil }
		if mode == "apply-stage-crash" {
			a.applyStageFault = func() error { os.Exit(94); return nil }
		}
		if err = a.applyChanges(ExerciseApply{PolicyHash: policy.Hash(), Changes: []ExerciseChange{{"a.go", "old-a", "new-a"}, {"b.go", "old-b", "new-b"}}}); err != nil {
			t.Fatal(err)
		}
		t.Fatal("crash was not reached")
	}
}

func helperCommand(t *testing.T, mode, root, state string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestReliabilityProcessHelper$")
	cmd.Env = append(os.Environ(), "INTENTLENS_TEST_HELPER="+mode, "INTENTLENS_TEST_ROOT="+root, "INTENTLENS_TEST_STATE="+state)
	return cmd
}

func TestLockProcessDeathRecoveryAndExclusion(t *testing.T) {
	state := t.TempDir()
	// Old exclusive-create lock files must not require manual deletion.
	if err := os.WriteFile(filepath.Join(state, "capture.lock"), []byte("pid=99999999\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := helperCommand(t, "hold", "", state)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	scanner := bufio.NewScanner(out)
	if !scanner.Scan() || scanner.Text() != "LOCKED" {
		t.Fatal("helper did not acquire lock")
	}
	contender := helperCommand(t, "try", "", state)
	err = contender.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 23 {
		t.Fatalf("second process acquired held lock: %v", err)
	}
	if err = cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	a := &App{State: state}
	release, err := a.lock("capture")
	if err != nil {
		t.Fatalf("dead owner did not release lock: %v", err)
	}
	if _, err = a.lock("capture"); err == nil {
		t.Fatal("same-process contender entered")
	}
	release()
	release()
	if _, err = os.Stat(filepath.Join(state, "capture.lock")); err != nil {
		t.Fatal("lock inode was removed")
	}
	release, err = a.lock("capture")
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestLockRejectsSymlink(t *testing.T) {
	a := &App{State: t.TempDir()}
	outside := filepath.Join(t.TempDir(), "lock")
	if err := os.WriteFile(outside, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(a.State, "capture.lock")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := a.lock("capture"); err == nil {
		t.Fatal("linked lock accepted")
	}
	b, _ := os.ReadFile(outside)
	if string(b) != "keep" {
		t.Fatal("outside lock modified")
	}
}

func TestOpenQuarantinesBadHistoryAndRevokesIt(t *testing.T) {
	a, root := fixture(t)
	for _, name := range []string{"active.json", "round-bad.json", "explanation-bad.json", "correction-bad.json", "share-draft-bad.json", "exercise-bad.json", "exercise-attempts-bad.json"} {
		if err := os.WriteFile(filepath.Join(a.State, name), []byte("{PRIVATE_SENTINEL"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	reopened, err := Open(root, filepath.Dir(a.State))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = reopened.Start("new-baseline"); err != nil {
		t.Fatal(err)
	}
	if err = reopened.Abort(); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := reopened.Diagnostics()
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || diagnostics[0]["quarantined_count"] != "7" || strings.Contains(fmt.Sprint(diagnostics), "PRIVATE_SENTINEL") {
		t.Fatalf("bad recovery diagnostics: %+v", diagnostics)
	}
	q, err := os.ReadDir(filepath.Join(a.State, "quarantine"))
	if err != nil || len(q) != 8 {
		t.Fatalf("missing private quarantine: %d %v", len(q), err)
	}
	put(t, root, ".intentignore", "private/\n")
	if _, err = Open(root, filepath.Dir(a.State)); err != nil {
		t.Fatal(err)
	}
	q, err = os.ReadDir(filepath.Join(a.State, "quarantine"))
	if err != nil || len(q) != 1 {
		t.Fatal("revoked unreadable source was retained")
	}
}

func TestCorruptBudgetAndConsentStayFailClosed(t *testing.T) {
	for _, name := range []string{"usage.json", "settings.json", "apply-journal.json"} {
		t.Run(name, func(t *testing.T) {
			a, root := fixture(t)
			path := filepath.Join(a.State, name)
			if err := os.WriteFile(path, []byte(" null\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(root, filepath.Dir(a.State)); err == nil {
				t.Fatal("critical record silently reset")
			}
			b, _ := os.ReadFile(path)
			if string(b) != " null\n" {
				t.Fatal("critical recovery evidence changed")
			}
		})
	}
}

func TestApplyCrashRecoveryAndExternalConflict(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(fmt.Sprint(conflict), func(t *testing.T) {
			a, root := fixture(t)
			put(t, root, "a.go", "old-a")
			put(t, root, "b.go", "old-b")
			cmd := helperCommand(t, "apply-crash", root, filepath.Dir(a.State))
			out, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 93 {
				t.Fatalf("crash helper failed: %v %s", err, out)
			}
			b, _ := os.ReadFile(filepath.Join(root, "a.go"))
			if string(b) != "new-a" {
				t.Fatal("fixture did not crash after first replacement")
			}
			if conflict {
				put(t, root, "a.go", "external-edit")
			}
			_, err = Open(root, filepath.Dir(a.State))
			if conflict {
				if err == nil {
					t.Fatal("external conflict silently overwritten")
				}
				b, _ = os.ReadFile(filepath.Join(root, "a.go"))
				if string(b) != "external-edit" {
					t.Fatal("external edit overwritten")
				}
				if _, err = os.Stat(filepath.Join(a.State, applyJournalName)); err != nil {
					t.Fatal("journal lost")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"a", "b"} {
					b, _ = os.ReadFile(filepath.Join(root, name+".go"))
					if string(b) != "old-"+name {
						t.Fatal("partial application not restored")
					}
				}
				if _, err = os.Stat(filepath.Join(a.State, applyJournalName)); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("completed recovery retained journal")
				}
			}
		})
	}
}

func TestApplyFailureRollsBackAndPreservesPermissions(t *testing.T) {
	a, root := fixture(t)
	put(t, root, "a.go", "old-a")
	put(t, root, "b.go", "old-b")
	if err := os.Chmod(filepath.Join(root, "a.go"), 0640); err != nil {
		t.Fatal(err)
	}
	policy, _ := capture.LoadPolicy(root)
	plan := ExerciseApply{PolicyHash: policy.Hash(), Changes: []ExerciseChange{{"a.go", "old-a", "new-a"}, {"b.go", "old-b", "new-b"}}}
	a.applyFault = func(int) error { return errors.New("injected failure") }
	if err := a.applyChanges(plan); err == nil {
		t.Fatal("injected failure ignored")
	}
	for _, name := range []string{"a", "b"} {
		b, _ := os.ReadFile(filepath.Join(root, name+".go"))
		if string(b) != "old-"+name {
			t.Fatal("failed write changed source")
		}
	}
	a.applyFault = nil
	if err := a.applyChanges(plan); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, "a.go"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0640 {
		t.Fatalf("permissions changed: %v", info.Mode())
	}
}

func TestAtomicReplaceRejectsLinksAndConflict(t *testing.T) {
	a, root := fixture(t)
	_ = a
	put(t, root, "a.go", "external")
	confined, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer confined.Close()
	before := "old"
	if err = atomicReplace(confined, "a.go", []byte("new"), 0600, &before); err == nil {
		t.Fatal("stale source overwritten")
	}
	b, _ := os.ReadFile(filepath.Join(root, "a.go"))
	if string(b) != "external" {
		t.Fatal("conflict destroyed source")
	}
	outside := t.TempDir()
	put(t, outside, "secret.go", "secret")
	if err = os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err = atomicReplace(confined, "link/secret.go", []byte("overwrite"), 0600, nil); err == nil {
		t.Fatal("escape accepted")
	}
	b, _ = os.ReadFile(filepath.Join(outside, "secret.go"))
	if string(b) != "secret" {
		t.Fatal("outside data modified")
	}
}

func TestStagedSourceCleanedAfterCrash(t *testing.T) {
	for _, revoke := range []bool{false, true} {
		t.Run(fmt.Sprint(revoke), func(t *testing.T) {
			a, root := fixture(t)
			put(t, root, "a.go", "old-a")
			put(t, root, "b.go", "old-b")
			cmd := helperCommand(t, "apply-stage-crash", root, filepath.Dir(a.State))
			out, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 94 {
				t.Fatalf("staging crash not reached: %v %s", err, out)
			}
			names, _ := filepath.Glob(filepath.Join(root, ".intentlens-write-*"))
			if len(names) != 1 {
				t.Fatal("missing staging fixture")
			}
			snap, err := capture.Take(root)
			if err != nil {
				t.Fatal(err)
			}
			for p := range snap.Files {
				if strings.HasPrefix(p, ".intentlens-write-") {
					t.Fatal("staged source captured")
				}
			}
			if revoke {
				put(t, root, ".intentignore", "a.go\n")
			}
			_, err = Open(root, filepath.Dir(a.State))
			if !revoke && err != nil {
				t.Fatal(err)
			}
			names, _ = filepath.Glob(filepath.Join(root, ".intentlens-write-*"))
			if len(names) != 0 {
				t.Fatal("staged source survived recovery")
			}
			b, _ := os.ReadFile(filepath.Join(root, "a.go"))
			if string(b) != "old-a" {
				t.Fatal("pre-rename crash changed source")
			}
		})
	}
}

func TestPrivateAbandonedTemporarySourceIsRemoved(t *testing.T) {
	a, root := fixture(t)
	path := filepath.Join(a.State, ".intentlens-write-abandoned")
	if err := os.WriteFile(path, []byte("PRIVATE_SOURCE"), 0600); err != nil {
		t.Fatal(err)
	}
	release, err := a.lock("state")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Open(root, filepath.Dir(a.State)); err == nil {
		t.Fatal("active state writer was ignored")
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatal("active writer temporary deleted")
	}
	release()
	if _, err = Open(root, filepath.Dir(a.State)); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("abandoned source retained")
	}
}

func TestCriticalRecordShapesFailClosed(t *testing.T) {
	cases := map[string][]string{
		"usage.json":    {"{}", "{\"actual_tokens\":null,\"uncertain_tokens\":null}", "{\"actual_tokens\":-1,\"uncertain_tokens\":0}", "{\"actual_tokens\":1}"},
		"settings.json": {"{}", "{\"model\":null}", "{\"model\":{\"enabled\":null,\"model\":\"x\",\"token_limit\":100}}"},
	}
	for name, records := range cases {
		for i, raw := range records {
			t.Run(fmt.Sprintf("%s/%d", name, i), func(t *testing.T) {
				a, root := fixture(t)
				if err := os.WriteFile(filepath.Join(a.State, name), []byte(raw), 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := Open(root, filepath.Dir(a.State)); err == nil {
					t.Fatal("invalid critical shape accepted")
				}
			})
		}
	}
}
