package app

import (
	"context"
	"encoding/json"
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
func TestExerciseAuthorizationValidationScoreAndApply(t *testing.T) {
	a, root := fixture(t)
	e := seedExercise(t, a, root)
	plan, err := a.PrepareExecution(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.RunExercise(context.Background(), e.ID, plan.Digest, false, "validate"); err == nil {
		t.Fatal("executed without consent")
	}
	view, err := a.RunExercise(context.Background(), e.ID, plan.Digest, true, "validate")
	if err != nil {
		t.Fatal(err)
	}
	if view.Validation.Status != "validated" {
		t.Fatalf("validation: %+v", view.Validation)
	}
	view, err = a.RunExercise(context.Background(), e.ID, plan.Digest, false, "score")
	if err != nil {
		t.Fatal(err)
	}
	if view.Attempts[0].Result.Passed {
		t.Fatal("unmodified answer passed")
	}
	if err = os.WriteFile(filepath.Join(view.Workspace, "main.go"), []byte(e.Spec.Reference["main.go"]), 0600); err != nil {
		t.Fatal(err)
	}
	changedPlan, err := a.PrepareExecution(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if changedPlan.Digest == plan.Digest {
		t.Fatal("source changes did not invalidate the execution plan")
	}
	if _, err = a.RunExercise(context.Background(), e.ID, plan.Digest, false, "score"); err == nil {
		t.Fatal("stale preview was accepted after source changed")
	}
	if _, err = a.RunExercise(context.Background(), e.ID, changedPlan.Digest, false, "score"); err == nil {
		t.Fatal("previous authorization was reused for changed source")
	}
	view, err = a.RunExercise(context.Background(), e.ID, changedPlan.Digest, true, "score")
	if err != nil {
		t.Fatal(err)
	}
	if !view.Attempts[len(view.Attempts)-1].Result.Passed {
		t.Fatal("reference answer failed")
	}
	preview, err := a.PrepareExerciseApply(e.ID)
	if err != nil {
		t.Fatal(err)
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
func TestExerciseViewHidesReferenceAndChangedTestsCannotPass(t *testing.T) {
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
	plan, err := a.PrepareExecution(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(plan.Workspace, exercise.AcceptanceFile), []byte("package p"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = a.RunExercise(context.Background(), e.ID, plan.Digest, true, "score"); err == nil {
		t.Fatal("tampered tests accepted")
	}
}

func TestExerciseScopeConflictFlagAndVersionBoundaries(t *testing.T) {
	a, root := fixture(t)
	e := seedExercise(t, a, root)
	plan, err := a.PrepareExecution(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	put(t, plan.Workspace, "extra_test.go", "package p\n")
	if _, err = a.RunExercise(context.Background(), e.ID, plan.Digest, true, "score"); err == nil {
		t.Fatal("expanded scope accepted")
	}
	if err = os.Remove(filepath.Join(plan.Workspace, "extra_test.go")); err != nil {
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
	if _, err = a.RunExercise(context.Background(), e.ID, plan.Digest, true, "validate"); err != nil {
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
	if shared, err := receiver.SharedExercises(); err != nil || len(shared) != 0 {
		t.Fatal("excluded acceptance leaked", err)
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
