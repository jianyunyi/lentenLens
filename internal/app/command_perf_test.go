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

// seedCommandHistory writes n rounds, each with one changed file and a round
// explanation plus correction, approximating a long-lived project state.
func seedCommandHistory(tb testing.TB, n int) (root, state string) {
	tb.Helper()
	root, state = tb.TempDir(), tb.TempDir()
	source := "package p\n" + strings.Repeat("// padding line for realistic round size\n", 40) + "func F0(x int) int { return x + 0 }\n"
	if err := os.WriteFile(filepath.Join(root, "f0000.go"), []byte(source), 0600); err != nil {
		tb.Fatal(err)
	}
	a, err := Open(root, state)
	if err != nil {
		tb.Fatal(err)
	}
	before, after := capture.NewFile(source), capture.NewFile(source+"// changed\n")
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("%024x", i+1)
		r := Round{ID: id, Session: id, Started: fmt.Sprintf("%06d", i), Ended: fmt.Sprintf("%06d", i), Changes: []capture.Change{{Path: "f0000.go", Kind: "modified", Before: &before, After: &after}}}
		if err := a.write("round-"+id+".json", r); err != nil {
			tb.Fatal(err)
		}
		e := Explanation{ID: id, FunctionID: "round:" + id, Created: "2026-10-26", Evidence: []explain.Evidence{{File: "f0000.go", Role: "after", Source: source, Hash: hash(source)}}}
		if err := a.write("explanation-"+id+".json", e); err != nil {
			tb.Fatal(err)
		}
		if err := a.write("correction-"+id+".json", Correction{ID: id, ExplanationID: id, ClaimID: "c1"}); err != nil {
			tb.Fatal(err)
		}
	}
	return root, state
}

// BenchmarkRoundsCommand measures what one `intentlens rounds` CLI call does:
// Open (recovery + privacy scrub) followed by the first history page.
func BenchmarkRoundsCommand(b *testing.B) {
	for _, n := range []int{200, 1000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) { benchRoundsCommand(b, n) })
	}
}

func benchRoundsCommand(b *testing.B, n int) {
	root, state := seedCommandHistory(b, n)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a, err := Open(root, state)
		if err != nil {
			b.Fatal(err)
		}
		page, err := a.ListPage("", 50)
		if err != nil || len(page.Items) != 50 {
			b.Fatalf("page: %d %v", len(page.Items), err)
		}
	}
}

func TestScrubMemoRescrubsAfterPolicyChange(t *testing.T) {
	root, state := seedCommandHistory(t, 3)
	a, err := Open(root, state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".intentignore"), []byte("f0000.go\n"), 0600); err != nil {
		t.Fatal(err)
	}
	rounds, err := a.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rounds {
		if len(r.Changes) != 0 || r.Analysis != nil {
			t.Fatalf("excluded file survived in-process scrub memo: %+v", r)
		}
	}
	if _, err := os.Stat(filepath.Join(a.State, "explanation-"+rounds[0].ID+".json")); !os.IsNotExist(err) {
		t.Fatalf("explanation citing excluded file kept: %v", err)
	}
}

func TestScrubMemoRescrubsAfterEvidenceBecomesLink(t *testing.T) {
	root, state := seedCommandHistory(t, 2)
	a, err := Open(root, state)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "f0000.go")
	target := filepath.Join(t.TempDir(), "outside.go")
	if err := os.Rename(file, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, file); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	id := fmt.Sprintf("%024x", 1)
	if _, err := a.ExplanationByID(id); err == nil {
		t.Fatal("explanation citing a linked file was still returned")
	}
	if _, err := os.Stat(filepath.Join(a.State, "explanation-"+id+".json")); !os.IsNotExist(err) {
		t.Fatalf("explanation citing a linked file kept: %v", err)
	}
}

// TestPerformanceBudget enforces docs/production/performance-slo.md. It is
// opt-in because shared CI runners are noisy; the budgets carry 3x headroom
// over the Linux reference measurement.
func TestPerformanceBudget(t *testing.T) {
	if os.Getenv("INTENTLENS_PERF_GATE") != "1" {
		t.Skip("set INTENTLENS_PERF_GATE=1 to enforce performance budgets")
	}
	budgets := []struct {
		name   string
		bench  func(*testing.B)
		budget float64 // milliseconds per operation
	}{
		{"index/1000", func(b *testing.B) {
			a := benchmarkProject(b, 1000)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := a.Index(); err != nil {
					b.Fatal(err)
				}
			}
		}, 120},
		{"inspect/1000", func(b *testing.B) {
			a := benchmarkProject(b, 1000)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := a.Inspect("f0000.go", 2); err != nil {
					b.Fatal(err)
				}
			}
		}, 120},
		{"rounds-command/1000", func(b *testing.B) { benchRoundsCommand(b, 1000) }, 750},
	}
	for _, c := range budgets {
		r := testing.Benchmark(c.bench)
		if r.N == 0 {
			t.Fatalf("%s: benchmark did not run", c.name)
		}
		got := float64(r.NsPerOp()) / 1e6
		t.Logf("%s: %.1f ms/op (budget %.0f ms)", c.name, got, c.budget)
		if got > c.budget {
			t.Errorf("%s: %.1f ms/op exceeds budget %.0f ms", c.name, got, c.budget)
		}
	}
}
