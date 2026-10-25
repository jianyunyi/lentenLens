package app

import (
	"context"
	"encoding/json"
	"fmt"
	"intentlens/internal/capture"
	"intentlens/internal/exercise"
	"intentlens/internal/explain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func exhaustTextSnapshotBudget(t *testing.T, root string) {
	t.Helper()
	for i := 0; i < capture.MaxSnapshotBytes/capture.MaxFileBytes; i++ {
		put(t, root, fmt.Sprintf("a%02d.txt", i), strings.Repeat("x", capture.MaxFileBytes))
	}
	snap, err := capture.Take(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snap.Files["z.go"]; ok {
		t.Fatal("fixture did not exhaust general snapshot budget")
	}
}

func TestFunctionFreshnessAgreesUnderTextBudgetPressure(t *testing.T) {
	a, root := fixture(t)
	seedExplanation(t, a, root) // Configure a deterministic model stub.
	if err := os.Remove(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}
	put(t, root, "z.go", "package p\nfunc F() int { return 1 }\n")
	exhaustTextSnapshotBudget(t, root)
	saved, err := a.Explain(context.Background(), "z.go", 2, "", "key")
	if err != nil {
		t.Fatal(err)
	}
	check := func(wantFresh bool) {
		t.Helper()
		info, err := a.Inspect("z.go", 2)
		if err != nil || len(info.History) != 1 {
			t.Fatalf("inspect history: %+v %v", info.History, err)
		}
		loaded, err := a.ExplanationByID(saved.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, status := range []string{info.History[0].Freshness, loaded.Freshness} {
			if (status == "已引用证据未变") != wantFresh {
				t.Errorf("freshness disagreement: want fresh=%t, got %s", wantFresh, status)
			}
		}
	}
	check(true)
	if _, err := a.PrepareShare(context.Background(), saved.ID, nil, "key"); err != nil {
		t.Fatalf("unchanged explanation cannot be shared: %v", err)
	}
	put(t, root, "z.go", "package p\nfunc F() int { return 2 }\n")
	check(false)
	if _, err := a.PrepareShare(context.Background(), saved.ID, nil, "key"); err == nil {
		t.Fatal("changed explanation can be shared")
	}
	if err := os.Remove(filepath.Join(root, "z.go")); err != nil {
		t.Fatal(err)
	}
	loaded, err := a.ExplanationByID(saved.ID)
	if err != nil || !strings.Contains(loaded.Freshness, "过期") {
		t.Fatalf("deleted source not stale: %+v %v", loaded, err)
	}
}

func TestFreshnessKeepsNonGoRoundEvidence(t *testing.T) {
	a, root := fixture(t)
	goSource, text := "package p\nfunc F() {}\n", "documented behavior\n"
	put(t, root, "z.go", goSource)
	put(t, root, "notes.txt", text)
	evidence := []explain.Evidence{
		{File: "z.go", Role: "after", Source: goSource, Hash: hash(goSource)},
		{File: "notes.txt", Role: "after", Source: text, Hash: hash(text)},
	}
	if got := a.freshness(evidence); got != "已引用证据未变" {
		t.Fatalf("unchanged mixed evidence: %s", got)
	}
	put(t, root, "notes.txt", text+"changed\n")
	if got := a.freshness(evidence); !strings.Contains(got, "过期") {
		t.Fatalf("changed non-Go evidence accepted: %s", got)
	}
}

func TestGenerateExerciseIgnoresUnrelatedTextBudget(t *testing.T) {
	a, root := fixture(t)
	source := seedExplanation(t, a, root)
	if err := os.Rename(filepath.Join(root, "main.go"), filepath.Join(root, "z.go")); err != nil {
		t.Fatal(err)
	}
	source.File, source.FunctionID = "z.go", "z.go:F"
	source.Evidence[0].File = "z.go"
	if err := a.write("explanation-"+source.ID+".json", source); err != nil {
		t.Fatal(err)
	}
	exhaustTextSnapshotBudget(t, root)
	a.exerciseRun = func(_ context.Context, _ explain.Config, u explain.Usage, _, _ string) (string, explain.Usage, error) {
		b, err := json.Marshal(exercise.Spec{Title: "返回二", Requirements: []exercise.Requirement{{ID: "r1", Text: "F 返回 2", Test: "TestIntentLensTwo"}}, Acceptance: "package p\nimport \"testing\"\nfunc TestIntentLensTwo(t *testing.T) { if F()!=2 { t.Fatal(\"INTENTLENS_EXPECTED_MISSING: two\") } }\n", Reference: map[string]string{"z.go": "package p\nfunc F() int { return 2 }\n"}})
		return string(b), u, err
	}
	e, err := a.GenerateExercise(context.Background(), source.ID, "c1", "z.go", "", "key")
	if err != nil {
		t.Fatal(err)
	}
	if e.Spec.Baseline["z.go"] != "package p\nfunc F() int { return 1 }\n" || len(e.Spec.Baseline) != 1 {
		t.Fatalf("incorrect package baseline: %+v", e.Spec.Baseline)
	}
}
