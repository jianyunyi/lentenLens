package app

import (
	"encoding/json"
	"fmt"
	"intentlens/internal/capture"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validGoOfSize(n int) string {
	prefix := "package p\n//"
	return prefix + strings.Repeat("x", n-len(prefix))
}

func putCoverage(t *testing.T, root, name, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestIndexCoverageKeepsAuthAheadOfValidGoBudgetPressure(t *testing.T) {
	a, root := fixture(t)
	putCoverage(t, root, "internal/runner/grpc/auth.go", "package grpc\nfunc Auth() {}\n")
	for i := 0; i < 22; i++ {
		putCoverage(t, root, "pressure"+string(rune('a'+i))+".go", validGoOfSize(capture.MaxFileBytes))
	}
	putCoverage(t, root, "pressure-target.go", "package p\nfunc Target() {}\n")
	putCoverage(t, root, "ordinary.txt", strings.Repeat("x", 16<<20))
	idx, err := a.Index()
	if err != nil {
		t.Fatal(err)
	}
	if idx.Coverage.CapturedBytes > capture.MaxSnapshotBytes || idx.Coverage.Complete {
		t.Fatalf("budget coverage invalid: %+v", idx.Coverage)
	}
	found := false
	for _, f := range idx.Functions {
		if f.File == "internal/runner/grpc/auth.go" && f.Name == "Auth" {
			found = true
		}
	}
	if !found {
		t.Fatal("auth.go was displaced by valid Go pressure")
	}
	if _, err := a.Inspect("pressureq.go", 1); err == nil || !strings.Contains(err.Error(), "snapshot_budget") {
		t.Fatalf("budget target did not get its own reason: %v", err)
	}
}

func TestIndexCoverageDistinguishesCapturedNoFunctionAndParseError(t *testing.T) {
	a, root := fixture(t)
	putCoverage(t, root, "main.go", "package p\nfunc Main() {}\n")
	putCoverage(t, root, "data.go", "package p\nvar Data = 1\n")
	putCoverage(t, root, "broken.go", "package p\nfunc Broken( {\n")
	if _, err := a.Inspect("data.go", 2); err == nil || !strings.Contains(err.Error(), "没有可解析的 Go 函数") {
		t.Fatalf("captured non-function got wrong error: %v", err)
	}
	if _, err := a.Inspect("broken.go", 2); err == nil || !strings.Contains(err.Error(), "parse_error") {
		t.Fatalf("parse omission got wrong error: %v", err)
	}
}

func TestTakeStillCapturesOrdinaryTextWhileTakeGoIgnoresIt(t *testing.T) {
	root := t.TempDir()
	putCoverage(t, root, "main.go", "package p\nfunc Main() {}\n")
	putCoverage(t, root, "notes.txt", "ordinary text\n")
	all, err := capture.Take(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := all.Files["notes.txt"]; !ok {
		t.Fatal("Take stopped capturing ordinary non-Go text")
	}
	goOnly, coverage, err := capture.TakeGo(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := goOnly.Files["notes.txt"]; ok || coverage.CapturedBytes != len(goOnly.Files["main.go"].Content) {
		t.Fatalf("TakeGo included non-Go content: %+v", coverage)
	}
}

func TestIndexCoverageNestedCopiesAndNonGoCannotStarveAuth(t *testing.T) {
	a, root := fixture(t)
	source := "package grpc\nfunc Auth() {}\n"
	putCoverage(t, root, "internal/runner/grpc/auth.go", source)
	for i := 0; i < 17; i++ {
		putCoverage(t, root, fmt.Sprintf(".worktrees/compose-production/internal/runner/grpc/%02d.go", i), validGoOfSize(capture.MaxFileBytes))
		putCoverage(t, root, fmt.Sprintf("00-generated/%02d.txt", i), strings.Repeat("x", capture.MaxFileBytes))
	}
	putCoverage(t, root, ".worktrees/compose-production/shadow.go", "package p\nfunc WrongWorkspace() {}\n")
	putCoverage(t, root, ".cache/cache.go", validGoOfSize(capture.MaxFileBytes))
	putCoverage(t, root, ".pnpm-store/store.go", validGoOfSize(capture.MaxFileBytes))
	idx, err := a.Index()
	if err != nil {
		t.Fatal(err)
	}
	if idx.Coverage.OmittedCount != 0 || idx.Coverage.ExcludedCount < 3 {
		t.Fatalf("unexpected coverage: %+v", idx.Coverage)
	}
	found := false
	for _, fn := range idx.Functions {
		if strings.HasPrefix(fn.File, ".worktrees/") || strings.HasPrefix(fn.File, ".cache/") || strings.HasPrefix(fn.File, ".pnpm-store/") {
			t.Fatalf("generated copy leaked into index: %s", fn.File)
		}
		if fn.File == "internal/runner/grpc/auth.go" {
			found = true
		}
	}
	if !found {
		t.Fatal("real auth.go displaced by nested copies or non-Go text")
	}
	info, err := a.Inspect("internal/runner/grpc/auth.go", 2)
	if err != nil || info.Function.Name != "Auth" {
		t.Fatalf("real function unusable: %+v %v", info.Function, err)
	}
	if b, err := os.ReadFile(filepath.Join(root, "internal/runner/grpc/auth.go")); err != nil || string(b) != source {
		t.Fatal("indexing altered source")
	}
}

func TestIndexCoverageTargetBeyondOmissionSamplesGetsItsOwnReason(t *testing.T) {
	a, root := fixture(t)
	for i := 0; i < 16; i++ {
		putCoverage(t, root, fmt.Sprintf("a-pressure/%02d.go", i), validGoOfSize(capture.MaxFileBytes))
	}
	putCoverage(t, root, "b-too-large.go", validGoOfSize(capture.MaxFileBytes+1))
	for i := 0; i < 24; i++ {
		putCoverage(t, root, fmt.Sprintf("z-skipped/%02d.go", i), "package p\nfunc Skipped() {}\n")
	}
	const target = "zz-budget-target.go"
	putCoverage(t, root, target, "package p\nfunc Target() {}\n")
	idx, err := a.Index()
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Coverage.Omitted) != 20 || !idx.Coverage.Truncated || idx.Coverage.OmittedCount <= 20 {
		t.Fatalf("sample cap broken: %+v", idx.Coverage)
	}
	for _, item := range idx.Coverage.Omitted {
		if item.File == target {
			t.Fatal("fixture target must be outside public sample")
		}
	}
	if _, err := a.Inspect(target, 2); err == nil || !strings.Contains(err.Error(), "snapshot_budget") || !strings.Contains(err.Error(), "预算") {
		t.Fatalf("wrong target reason after sample cap: %v", err)
	}
}

func TestIndexCoverageParseSamplesAreBoundedAndDeterministic(t *testing.T) {
	a, root := fixture(t)
	for i := 0; i < 24; i++ {
		putCoverage(t, root, fmt.Sprintf("broken/%02d.go", i), "package p\nfunc Broken( {\n")
	}
	var first string
	for run := 0; run < 5; run++ {
		idx, err := a.Index()
		if err != nil {
			t.Fatal(err)
		}
		if len(idx.Coverage.Omitted) != 20 || !idx.Coverage.Truncated || idx.Coverage.OmittedCount < 24 {
			t.Fatalf("parse sample cap broken: %+v", idx.Coverage)
		}
		b, err := json.Marshal(idx.Coverage)
		if err != nil {
			t.Fatal(err)
		}
		if run == 0 {
			first = string(b)
		} else if string(b) != first {
			t.Fatal("unchanged index diagnostics changed ordering; UI would falsely reject every action")
		}
	}
	if _, err := a.Inspect("broken/23.go", 2); err == nil || !strings.Contains(err.Error(), "parse_error") {
		t.Fatalf("missing parse target reason beyond sample: %v", err)
	}
}

func TestIndexCoveragePolicyExclusionDoesNotMislabelCapturedNoFunction(t *testing.T) {
	a, root := fixture(t)
	putCoverage(t, root, "data.go", "package p\nvar Data = 1\n")
	putCoverage(t, root, "private.go", "package p\nfunc Private() {}\n")
	putCoverage(t, root, ".intentignore", "private.go\n")
	if _, err := a.Inspect("private.go", 2); err == nil || !strings.Contains(err.Error(), "排除") || !strings.Contains(err.Error(), ".intentignore") {
		t.Fatalf("unclear exclusion: %v", err)
	}
	if _, err := a.Inspect("data.go", 2); err == nil || !strings.Contains(err.Error(), "没有可解析的 Go 函数") || strings.Contains(err.Error(), "索引不完整") {
		t.Fatalf("unrelated exclusion mislabeled current file: %v", err)
	}
}
