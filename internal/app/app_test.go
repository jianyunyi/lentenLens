package app

import (
	"context"
	"intentlens/internal/explain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (*App, string) {
	t.Helper()
	root := t.TempDir()
	a, err := Open(root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return a, root
}

func TestIgnoreChangePurgesDiskAndDoesNotReappear(t *testing.T) {
	a, root := fixture(t)
	put(t, root, "secret.go", "package p\nfunc Secret() { println(\"PRIVATE_SENTINEL\") }\n")
	_, _ = a.Start("s")
	put(t, root, "secret.go", "package p\nfunc Secret() { println(\"PRIVATE_SENTINEL_NEW\") }\n")
	_, _ = a.Stop("s")
	saved := Explanation{ID: "test", File: "secret.go", FunctionID: "secret.go:Secret", Evidence: []explain.Evidence{{File: "secret.go", Source: "PRIVATE_SENTINEL"}}}
	if e := a.write("explanation-test.json", saved); e != nil {
		t.Fatal(e)
	}
	put(t, root, ".intentignore", "secret.go\n")
	if _, e := a.List(); e != nil {
		t.Fatal(e)
	}
	entries, _ := os.ReadDir(a.State)
	for _, entry := range entries {
		b, _ := os.ReadFile(filepath.Join(a.State, entry.Name()))
		if strings.Contains(string(b), "PRIVATE_SENTINEL") {
			t.Fatalf("excluded data remains in %s", entry.Name())
		}
	}
	put(t, root, ".intentignore", "")
	rounds, e := a.List()
	if e != nil {
		t.Fatal(e)
	}
	if len(rounds[0].Changes) != 0 {
		t.Fatal("purged content reappeared")
	}
}

func TestRoundExplanationIsReadableAndBecomesStale(t *testing.T) {
	a, root := fixture(t)
	_, _ = a.Start("s")
	source := "package p\nfunc F() {}\n"
	put(t, root, "main.go", source)
	r, e := a.Stop("s")
	if e != nil {
		t.Fatal(e)
	}
	saved := Explanation{ID: "test", FunctionID: "round:" + r.ID, Created: "2026-10-06", Evidence: []explain.Evidence{{File: "main.go", Role: "after", Hash: hash(source), Source: source}}}
	if e = a.write("explanation-test.json", saved); e != nil {
		t.Fatal(e)
	}
	rounds, e := a.List()
	if e != nil {
		t.Fatal(e)
	}
	if rounds[0].Analysis == nil || rounds[0].Analysis.Freshness != "已引用证据未变" {
		t.Fatalf("missing round explanation %+v", rounds)
	}
	put(t, root, "main.go", "package p\nfunc F() { println(1) }\n")
	rounds, e = a.List()
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(rounds[0].Analysis.Freshness, "过期") {
		t.Fatal("stale round marked current")
	}
}
func put(t *testing.T, root, path, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRoundUsesStartBaselineAndRetainsHistoricalChanges(t *testing.T) {
	a, root := fixture(t)
	put(t, root, "main.go", "package p\nfunc F() int { return 1 }\n")
	if _, err := a.Start("session"); err != nil {
		t.Fatal(err)
	}
	put(t, root, "main.go", "package p\nfunc F() int { return 2 }\n")
	r, err := a.Stop("session")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Changes) != 1 || r.Changes[0].Before.Content != "package p\nfunc F() int { return 1 }\n" {
		t.Fatalf("bad net diff: %+v", r)
	}
	if retry, err := a.Stop("session"); err != nil || digest(retry) != digest(r) {
		t.Fatalf("duplicate Stop = %+v, %v, want the committed round", retry, err)
	}
	rounds, err := a.List()
	if err != nil || len(rounds) != 1 {
		t.Fatalf("history lost: %v %v", rounds, err)
	}
}

func TestNoBaselineAndOverlappingRoundsAreExplicit(t *testing.T) {
	a, _ := fixture(t)
	if _, err := a.Stop("missing"); err == nil {
		t.Fatal("fabricated baseline")
	}
	if _, err := a.Start("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Start("b"); err == nil {
		t.Fatal("overlapping capture accepted")
	}
	if _, err := a.Stop("b"); err == nil {
		t.Fatal("mismatched session accepted")
	}
}

func TestInspectAndStoredHistoryHonorExclusions(t *testing.T) {
	a, root := fixture(t)
	put(t, root, "main.go", "package p\nfunc F() int { return 1 }\n")
	_, _ = a.Start("s")
	put(t, root, "main.go", "package p\nfunc F() int { return 2 }\n")
	_, _ = a.Stop("s")
	put(t, root, ".intentignore", "main.go\n")
	if _, err := a.Inspect("main.go", 2); err == nil {
		t.Fatal("excluded code inspected")
	}
	rounds, err := a.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(rounds[0].Changes) != 0 {
		t.Fatal("newly excluded history returned")
	}
	if _, err := a.Inspect("../outside.go", 1); err == nil {
		t.Fatal("path escaped root")
	}
}

func TestStateMustBeOutsideProjectAndExplanationIsOptIn(t *testing.T) {
	root := t.TempDir()
	if _, err := Open(root, filepath.Join(root, "private")); err == nil {
		t.Fatal("private data permitted inside repo")
	}
	a, root := fixture(t)
	put(t, root, "main.go", "package p\nfunc F() int { return 1 }\n")
	if _, err := a.Explain(context.Background(), "main.go", 2, "", ""); err == nil {
		t.Fatal("analysis without opt-in")
	}
}

func TestBackgroundAnalysisErrorVisibleOnRound(t *testing.T) {
	a, root := fixture(t)
	_, _ = a.Start("s")
	put(t, root, "a.go", "package p\n")
	r, e := a.Stop("s")
	if e != nil {
		t.Fatal(e)
	}
	if e = a.write("analysis-diagnostic.json", map[string]string{"round": r.ID, "error": "budget exhausted"}); e != nil {
		t.Fatal(e)
	}
	rounds, e := a.List()
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(strings.Join(rounds[0].Notes, " "), "budget exhausted") {
		t.Fatal("background failure invisible")
	}
}
