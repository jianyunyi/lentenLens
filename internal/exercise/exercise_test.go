package exercise

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture() Spec {
	return Spec{Title: "负数归零", Requirements: []Requirement{{ID: "r1", Text: "负数输入返回零；正数保持不变", Test: "TestIntentLensNegative"}}, Baseline: map[string]string{"value.go": "package demo\nfunc Value(n int) int { return n }\n", "value_test.go": "package demo\nimport \"testing\"\nfunc TestPositive(t *testing.T) { if Value(2)!=2 {t.Fatal(\"positive\")} }\n"}, Acceptance: "package demo\nimport \"testing\"\nfunc TestIntentLensNegative(t *testing.T) { if Value(-1)!=0 {t.Fatal(\"INTENTLENS_EXPECTED_MISSING: negative\")} }\n", Reference: map[string]string{"value.go": "package demo\nfunc Value(n int) int { if n<0 {return 0}; return n }\n"}}
}

func TestSpecRejectsExternalImportsAndUnsafeChanges(t *testing.T) {
	s := fixture()
	if err := Check(s); err != nil {
		t.Fatal(err)
	}
	for _, src := range []string{"package demo\nimport \"net/http\"", "package demo\nimport \"os\"", "package demo\n//go:embed x\nvar x string", "package demo\nfunc TestMain() {}"} {
		s = fixture()
		s.Baseline["value.go"] = src
		if Check(s) == nil {
			t.Fatal("unsafe source accepted", src)
		}
	}
	s = fixture()
	s.Reference["../outside.go"] = "package demo"
	if Check(s) == nil {
		t.Fatal("traversal accepted")
	}
	s = fixture()
	s.Requirements[0].Test = "TestNotPresent"
	if Check(s) == nil {
		t.Fatal("missing test accepted")
	}
}

func TestRealValidationAndScore(t *testing.T) {
	s := fixture()
	base := t.TempDir()
	tool, err := Tool()
	if err != nil {
		t.Fatal(err)
	}
	run := func(name string, files map[string]string) Result {
		dir := filepath.Join(base, name)
		if err := Materialize(dir, files); err != nil {
			t.Fatal(err)
		}
		return Run(context.Background(), tool, dir, filepath.Join(base, "cache"))
	}
	original := run("original", s.Baseline)
	red := run("red", Files(s, false))
	green := run("green", Files(s, true))
	if !Validated(s, original, red, green) {
		t.Fatalf("validation failed: %+v %+v %+v", original, red, green)
	}
	if len(Behavior(s, green)) != 1 || Behavior(s, green)[0].Status != "pass" {
		t.Fatal("score wrong")
	}
	s.Acceptance = strings.ReplaceAll(s.Acceptance, "Value(-1)", "Undefined(-1)")
	broken := run("broken", Files(s, false))
	if Validated(s, original, broken, green) {
		t.Fatal("compile failure accepted as expected red")
	}
	if _, err := os.Stat(filepath.Join(base, "original", "go.mod")); err != nil {
		t.Fatal(err)
	}
}

func TestVersionBindsRequirementsBaselineAndTests(t *testing.T) {
	s := fixture()
	first := Version(s)
	s.Title = "new"
	if Version(s) == first {
		t.Fatal("title not bound")
	}
	s = fixture()
	s.Acceptance += "\n"
	if Version(s) == first {
		t.Fatal("test not bound")
	}
	s = fixture()
	s.Baseline["value.go"] += "\n"
	if Version(s) == first {
		t.Fatal("baseline not bound")
	}
}

func TestRunnerRejectsChangedToolBeforeExecution(t *testing.T) {
	tool, err := Tool()
	if err != nil {
		t.Fatal(err)
	}
	if tool.SHA256[0] == '0' {
		tool.SHA256 = "1" + tool.SHA256[1:]
	} else {
		tool.SHA256 = "0" + tool.SHA256[1:]
	}
	base := t.TempDir()
	cache := filepath.Join(base, "cache")
	result := Run(context.Background(), tool, filepath.Join(base, "missing-workspace"), cache)
	if result.Passed || result.Failure != "tool_changed" {
		t.Fatalf("changed tool was not rejected: %+v", result)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatal("tool identity failure created an execution cache")
	}
}

func TestRunnerCancellationNeverReportsPassing(t *testing.T) {
	tool, err := Tool()
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	dir := filepath.Join(base, "run")
	if err = Materialize(dir, fixture().Baseline); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	result := Run(ctx, tool, dir, filepath.Join(base, "cache"))
	if result.Passed || result.Failure != "timeout" {
		t.Fatalf("cancelled run reported %+v", result)
	}
}
