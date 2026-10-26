package app

import (
	"fmt"
	"intentlens/internal/capture"
	"intentlens/internal/explain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fillNonGoBudget(t *testing.T, root string) {
	t.Helper()
	for i := 0; i < capture.MaxSnapshotBytes/capture.MaxFileBytes; i++ {
		put(t, root, fmt.Sprintf("a%02d.txt", i), strings.Repeat("x", capture.MaxFileBytes))
	}
}

func TestApplyIgnoresUnrelatedTextBudget(t *testing.T) {
	a, root := fixture(t)
	e := seedExercise(t, a, root)
	execution, err := a.PrepareExecution(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	put(t, execution.Workspace, "main.go", e.Spec.Reference["main.go"])
	fillNonGoBudget(t, root)
	general, err := capture.Take(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := general.Files["main.go"]; ok {
		t.Fatal("fixture did not omit source from general snapshot")
	}
	preview, err := a.PrepareExerciseApply(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.ConfirmExerciseApply(e.ID, preview.Digest, true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "main.go"))
	if string(b) != e.Spec.Reference["main.go"] {
		t.Fatal("source not applied")
	}
}

func TestRoundAndExplanationFreshnessAgreeUnderTextBudget(t *testing.T) {
	a, root := fixture(t)
	if _, err := a.Start("s"); err != nil {
		t.Fatal(err)
	}
	source := "package p\nfunc Z() {}\n"
	put(t, root, "z.go", source)
	round, err := a.Stop("s")
	if err != nil {
		t.Fatal(err)
	}
	e := Explanation{ID: newID(), FunctionID: "round:" + round.ID, Created: "2026-10-26", Evidence: []explain.Evidence{{File: "z.go", Role: "after", Source: source, Hash: hash(source)}}}
	if err = a.write("explanation-"+e.ID+".json", e); err != nil {
		t.Fatal(err)
	}
	fillNonGoBudget(t, root)
	check := func(fresh bool) {
		t.Helper()
		rounds, err := a.List()
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := a.ExplanationByID(e.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(rounds) != 1 || rounds[0].Analysis == nil {
			t.Fatal("missing round analysis")
		}
		if rounds[0].Analysis.Freshness != loaded.Freshness || (loaded.Freshness == "已引用证据未变") != fresh {
			t.Fatalf("inconsistent freshness: %s / %s", rounds[0].Analysis.Freshness, loaded.Freshness)
		}
	}
	check(true)
	put(t, root, "z.go", source+"// changed\n")
	check(false)
	if err = os.Remove(filepath.Join(root, "z.go")); err != nil {
		t.Fatal(err)
	}
	check(false)
}

func TestInspectAlsoChecksEvidenceDigest(t *testing.T) {
	a, root := fixture(t)
	e := seedExplanation(t, a, root)
	e.Evidence[0].Source = "tampered evidence"
	if err := a.write("explanation-"+e.ID+".json", e); err != nil {
		t.Fatal(err)
	}
	info, err := a.Inspect("main.go", 2)
	if err != nil || len(info.History) != 1 {
		t.Fatalf("history: %+v %v", info, err)
	}
	if !strings.Contains(info.History[0].Freshness, "摘要不匹配") {
		t.Fatal("Inspect accepted mismatched source/hash")
	}
	if got := a.freshness(nil); !strings.Contains(got, "过期") {
		t.Fatal("missing evidence shown current")
	}
}
