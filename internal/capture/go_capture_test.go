package capture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTakeGoSkipsNestedWorktreesAndNonGoBudget(t *testing.T) {
	root := t.TempDir()
	write(t, root, "internal/runner/grpc/auth.go", "package grpc\nfunc Auth() {}\n")
	write(t, root, ".worktrees/compose-production/internal/runner/grpc/auth.go", "package grpc\nfunc Wrong() {}\n")
	write(t, root, ".cache/huge.txt", strings.Repeat("x", MaxFileBytes))
	s, c, err := TakeGo(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Files["internal/runner/grpc/auth.go"]; !ok {
		t.Fatal("real Go source was not captured")
	}
	if _, ok := s.Files[".worktrees/compose-production/internal/runner/grpc/auth.go"]; ok {
		t.Fatal("nested worktree was captured")
	}
	if c.CapturedBytes != len(s.Files["internal/runner/grpc/auth.go"].Content) {
		t.Fatalf("non-Go file consumed budget: %+v", c)
	}
	if c.ExcludedCount == 0 {
		t.Fatal("expected excluded directory count")
	}
	worktree, _, err := TakeGo(filepath.Join(root, ".worktrees/compose-production"))
	if err != nil || len(worktree.Files) != 1 {
		t.Fatalf("worktree root should index normally: files=%d err=%v", len(worktree.Files), err)
	}
}

func TestTakeGoReportsGoBudgetAndOversize(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 17; i++ {
		write(t, root, string(rune('a'+i))+".go", strings.Repeat("x", MaxFileBytes))
	}
	write(t, root, "too-large.go", strings.Repeat("x", MaxFileBytes+1))
	_, c, err := TakeGo(root)
	if err != nil {
		t.Fatal(err)
	}
	if c.CapturedBytes != MaxSnapshotBytes || c.OmittedCount == 0 || c.Truncated || c.Complete {
		t.Fatalf("bad Go coverage: %+v", c)
	}
	seenLarge := false
	for _, o := range c.Omitted {
		if o.File == "too-large.go" && o.Reason == "file_too_large" {
			seenLarge = true
		}
	}
	if !seenLarge {
		t.Fatalf("missing oversize reason: %+v", c.Omitted)
	}
}

func TestReadCaptureFileRejectsReplacement(t *testing.T) {
	dir := t.TempDir()
	const name = "main.go"
	write(t, dir, name, "package old\n")
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	// Keep the original inode alive and give the replacement the same size and mtime.
	if err := os.Rename(filepath.Join(dir, name), filepath.Join(dir, "original.go")); err != nil {
		t.Fatal(err)
	}
	write(t, dir, name, "package new\n")
	if err := os.Chtimes(filepath.Join(dir, name), info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if data, err := readCaptureFile(root, name, info); err != errFileChanged || data != nil {
		t.Fatalf("readCaptureFile = %q, %v, want no data and errFileChanged", data, err)
	}
}
