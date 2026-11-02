package app

import (
	"context"
	"encoding/json"
	"errors"
	"intentlens/internal/exercise"
	"intentlens/internal/explain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedExercise(t *testing.T, a *App, root string) Exercise {
	t.Helper()
	source := seedExplanation(t, a, root)
	a.exerciseRun = func(_ context.Context, _ explain.Config, u explain.Usage, _, prompt string) (string, explain.Usage, error) {
		if !strings.Contains(prompt, "target_evidence") || strings.Contains(prompt, "PRIVATE_") {
			t.Fatal("exercise targeting missing or private explanation leaked")
		}
		u.ActualTokens += 10
		b, _ := json.Marshal(exercise.Spec{Title: "返回二", Requirements: []exercise.Requirement{{ID: "r1", Text: "F 返回 2", Test: "TestIntentLensTwo"}}, Acceptance: "package p\nimport \"testing\"\nfunc TestIntentLensTwo(t *testing.T) { if F()!=2 { t.Fatal(\"INTENTLENS_EXPECTED_MISSING: two\") } }\n", Reference: map[string]string{"main.go": "package p\nfunc F() int { return 2 }\n"}})
		return string(b), u, nil
	}
	e, err := a.GenerateExercise(context.Background(), source.ID, "c1", "main.go", "", "key")
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestExecutionPlanStatesRunnerIsClosed(t *testing.T) {
	a, root := fixture(t)
	e := seedExercise(t, a, root)
	plan, err := a.PrepareExecution(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if exercise.OSSandboxAvailable() || !strings.Contains(plan.Limits, "执行已关闭") || !strings.Contains(plan.Limits, "没有经过验证的 OS 沙箱") {
		t.Fatalf("closed sandbox status missing: %q", plan.Limits)
	}
}

func TestExerciseRunFailsClosedAndSingleFileApplyWorks(t *testing.T) {
	a, root := fixture(t)
	e := seedExercise(t, a, root)
	plan, err := a.PrepareExecution(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, confirmed := range []bool{false, true} {
		if _, err = a.RunExercise(context.Background(), e.ID, plan.Digest, confirmed, "validate"); !errors.Is(err, exercise.ErrSandboxUnavailable) {
			t.Fatalf("runner was not fail-closed for confirmed=%t: %v", confirmed, err)
		}
	}
	for _, name := range []string{"exercise-auth-" + e.ID + ".json", "exercise-validation-" + e.ID + ".json", "exercise-attempts-" + e.ID + ".json"} {
		if _, err = os.Stat(filepath.Join(a.State, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("closed run created %s", name)
		}
	}
	if err = os.WriteFile(filepath.Join(plan.Workspace, "main.go"), []byte(e.Spec.Reference["main.go"]), 0600); err != nil {
		t.Fatal(err)
	}
	preview, err := a.PrepareExerciseApply(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Changes) != 1 {
		t.Fatalf("expected a single-file application preview, got %d files", len(preview.Changes))
	}
	if err = a.ConfirmExerciseApply(e.ID, preview.Digest, false); err == nil {
		t.Fatal("unconfirmed apply")
	}
	if err = a.ConfirmExerciseApply(e.ID, preview.Digest, true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "main.go"))
	if string(b) != e.Spec.Reference["main.go"] {
		t.Fatal("answer not applied")
	}
}

func TestPrepareExerciseApplyRejectsMultipleFilesWithoutChangingSources(t *testing.T) {
	a, root := fixture(t)
	e := seedExercise(t, a, root)
	e.Spec.Baseline["helper.go"] = "package p\nfunc Helper() int { return 0 }\n"
	e.Spec.Reference["helper.go"] = "package p\nfunc Helper() int { return 1 }\n"
	e.Version = exercise.Version(e.Spec)
	put(t, root, "helper.go", e.Spec.Baseline["helper.go"])
	if err := a.write("exercise-"+e.ID+".json", e); err != nil {
		t.Fatal(err)
	}
	plan, err := a.PrepareExecution(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	put(t, plan.Workspace, "main.go", e.Spec.Reference["main.go"])
	put(t, plan.Workspace, "helper.go", e.Spec.Reference["helper.go"])
	if _, err = a.PrepareExerciseApply(e.ID); err == nil || !strings.Contains(err.Error(), "多文件应用已关闭") {
		t.Fatalf("multi-file preview was not rejected: %v", err)
	}
	for name, want := range map[string]string{"main.go": e.Spec.Baseline["main.go"], "helper.go": e.Spec.Baseline["helper.go"]} {
		got, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil || string(got) != want {
			t.Fatalf("preview refusal changed %s: %q %v", name, got, readErr)
		}
	}
	if _, err = os.Stat(filepath.Join(a.State, applyJournalName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preview refusal created an apply journal: %v", err)
	}
}

func TestExerciseViewHidesReferenceAndRunFailsClosed(t *testing.T) {
	a, root := fixture(t)
	e := seedExercise(t, a, root)
	v, err := a.ExerciseView(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	var raw map[string]any
	_ = json.Unmarshal(b, &raw)
	if raw["reference"] != nil || v.Spec.Reference != nil {
		t.Fatal("reference disclosed")
	}
	if _, err = a.RunExercise(context.Background(), e.ID, "", true, "score"); !errors.Is(err, exercise.ErrSandboxUnavailable) {
		t.Fatalf("runner was not fail-closed: %v", err)
	}
}

func TestExerciseScopeConflictFlagAndVersionBoundaries(t *testing.T) {
	a, root := fixture(t)
	e := seedExercise(t, a, root)
	plan, err := a.PrepareExecution(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	put(t, plan.Workspace, "main.go", e.Spec.Reference["main.go"])
	preview, err := a.PrepareExerciseApply(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "main.go", "package p\nfunc F() int { return 999 }\n")
	if err = a.ConfirmExerciseApply(e.ID, preview.Digest, true); err == nil {
		t.Fatal("conflicting target overwritten")
	}
	current, _ := os.ReadFile(filepath.Join(root, "main.go"))
	if !strings.Contains(string(current), "999") {
		t.Fatal("conflict content lost")
	}
	put(t, root, "main.go", e.Spec.Baseline["main.go"])
	newer, err := a.GenerateExercise(context.Background(), e.SourceID, e.ClaimID, "main.go", e.ID, "key")
	if err != nil {
		t.Fatal(err)
	}
	if newer.ID == e.ID || newer.Supersedes != e.ID {
		t.Fatal("new version did not preserve ancestry")
	}
	view, err := a.ExerciseView(newer.ID)
	if err != nil || len(view.Attempts) != 0 || view.Validation.Status != "unverified" {
		t.Fatal("new version inherited scores", err)
	}
	if _, err = a.FlagClaim(e.SourceID, "c1", "wrong"); err != nil {
		t.Fatal(err)
	}
	view, err = a.ExerciseView(e.ID)
	if err != nil || !view.NeedsReview {
		t.Fatal("dependent question not flagged", err)
	}
	if _, err = a.PrepareExecution(e.ID); err == nil {
		t.Fatal("flagged question executable")
	}
}

func TestExerciseSharingAndRevocationKeepPersonalStateLocal(t *testing.T) {
	a, root := fixture(t)
	e := seedExercise(t, a, root)
	plan, err := a.PrepareExecution(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.PrepareExerciseShare(e.ID); err == nil {
		t.Fatal("unvalidated exercise shared")
	}
	validation := ExerciseValidation{Status: "validated", Version: e.Version, Results: []exercise.Result{}}
	if err = a.write("exercise-validation-"+e.ID+".json", validation); err != nil {
		t.Fatal(err)
	}
	if err = a.write("exercise-attempts-"+e.ID+".json", []ExerciseAttempt{{ID: newID(), Version: e.Version, Reason: "PRIVATE_REASON", Feedback: "PRIVATE_FEEDBACK", Files: map[string]string{"main.go": "PRIVATE_ANSWER"}}}); err != nil {
		t.Fatal(err)
	}
	draft, err := a.PrepareExerciseShare(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(draft.Record)
	if strings.Contains(string(b), "PRIVATE_") || strings.Contains(string(b), "reference") || strings.Contains(string(b), a.State) {
		t.Fatal("private data in question export")
	}
	if err = a.ConfirmExerciseShare(e.ID, draft.Digest, false); err == nil {
		t.Fatal("unconfirmed question export")
	}
	if err = a.ConfirmExerciseShare(e.ID, draft.Digest, true); err != nil {
		t.Fatal(err)
	}
	receiver, err := Open(root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	imported, err := receiver.ImportExercise(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !imported.Imported || len(imported.Attempts) != 0 || imported.Validation.Status != "unverified" || len(imported.Spec.Reference) != 0 {
		t.Fatal("recipient inherited private state")
	}
	put(t, root, ".intentignore", exercise.AcceptanceFile+"\n")
	if shared, sharedErr := receiver.SharedExercises(); sharedErr != nil || len(shared) != 0 {
		t.Fatal("excluded acceptance leaked", sharedErr)
	}
	put(t, root, ".intentignore", "main.go\n")
	if _, err = a.Exercises(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(a.State, "exercise-"+e.ID+".json")); !os.IsNotExist(err) {
		t.Fatal("excluded question persisted")
	}
	if _, err = os.Stat(filepath.Dir(plan.Workspace)); !os.IsNotExist(err) {
		t.Fatal("excluded workspace persisted")
	}
	shared, err := receiver.SharedExercises()
	if err != nil || len(shared) != 0 {
		t.Fatal("excluded shared question displayed", err)
	}
}

func TestExerciseModelFeedbackCannotChangeFailingScore(t *testing.T) {
	a, root := fixture(t)
	e := seedExercise(t, a, root)
	attempt := ExerciseAttempt{ID: newID(), Version: e.Version, Files: exercise.Files(e.Spec, false), Result: exercise.Result{Passed: false, Failure: "tests_failed", Tests: map[string]string{"TestIntentLensTwo": "fail"}}, Provisional: true, Feedback: "pending"}
	if err := a.write("exercise-attempts-"+e.ID+".json", []ExerciseAttempt{attempt}); err != nil {
		t.Fatal(err)
	}
	a.exerciseRun = func(_ context.Context, _ explain.Config, u explain.Usage, _, prompt string) (string, explain.Usage, error) {
		if strings.Contains(prompt, `"reference"`) || !strings.Contains(prompt, "tests_failed") {
			t.Fatal("feedback payload lost result or exposed reference")
		}
		u.ActualTokens += 5
		return `{"feedback":"模型补充意见不代表执行通过"}`, u, nil
	}
	view, err := a.ExerciseFeedback(context.Background(), e.ID, attempt.ID, "我的修改理由", "key")
	if err != nil {
		t.Fatal(err)
	}
	if view.Attempts[0].Result.Passed || view.Attempts[0].Result.Failure != "tests_failed" || !view.Attempts[0].Provisional || view.Attempts[0].Feedback == "pending" {
		t.Fatal("model feedback overrode execution")
	}
}
