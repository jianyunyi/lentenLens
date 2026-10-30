package exercise

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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

func TestRunFailsClosedWithoutVerifiedSandbox(t *testing.T) {
	base := t.TempDir()
	cache := filepath.Join(base, "cache")
	result := Run(context.Background(), Executable{Path: filepath.Join(base, "must-not-start")}, filepath.Join(base, "missing-workspace"), cache)
	if result.Passed || result.Failure != "sandbox_unavailable" || result.ExitCode != -1 {
		t.Fatalf("runner did not fail closed: %+v", result)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("closed runner created cache or child-process state: %v", err)
	}
}

func TestClosedSandboxGateDoesNotInvokeChildProcessCallback(t *testing.T) {
	childStarted := false
	result := runWhenSandboxVerified(false, func() Result {
		childStarted = true
		return Result{Passed: true}
	})
	if childStarted {
		t.Fatal("closed sandbox gate invoked the child-process callback")
	}
	if result.Passed || result.Failure != "sandbox_unavailable" {
		t.Fatalf("closed sandbox gate returned unexpected result: %+v", result)
	}
}
