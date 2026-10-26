package app

import (
	"fmt"
	"intentlens/internal/capture"
	"os"
	"path/filepath"
	"testing"
)

func benchmarkProject(b *testing.B, n int) (*App, string) {
	b.Helper()
	root := b.TempDir()
	for i := 0; i < n; i++ {
		source := fmt.Sprintf("package p\nfunc F%d(x int) int { return x + %d }\n", i, i)
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("f%04d.go", i)), []byte(source), 0600); err != nil {
			b.Fatal(err)
		}
	}
	a, err := Open(root, b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	return a, root
}

func BenchmarkIndexWorkspace(b *testing.B) {
	for _, n := range []int{100, 1000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			a, _ := benchmarkProject(b, n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				idx, err := a.Index()
				if err != nil || len(idx.Functions) != n {
					b.Fatalf("index: %d %v", len(idx.Functions), err)
				}
			}
		})
	}
}

func BenchmarkInspectWorkspace(b *testing.B) {
	for _, n := range []int{100, 1000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			a, _ := benchmarkProject(b, n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				info, err := a.Inspect("f0000.go", 2)
				if err != nil || info.Function.Name != "F0" {
					b.Fatalf("inspect: %v", err)
				}
			}
		})
	}
}

func BenchmarkRoundHistory(b *testing.B) {
	for _, n := range []int{20, 200} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			a, _ := benchmarkProject(b, 1)
			for i := 0; i < n; i++ {
				id := newID()
				after := capture.NewFile("package p\nfunc F0(x int) int { return x }\n")
				if err := a.write("round-"+id+".json", Round{ID: id, Started: fmt.Sprint(i), Changes: []capture.Change{{Path: "f0000.go", After: &after}}}); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				rounds, err := a.List()
				if err != nil || len(rounds) != n {
					b.Fatalf("round history: %v", err)
				}
			}
		})
	}
}
