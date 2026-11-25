package capture

import (
	"encoding/json"
	"fmt"
	"io/fs"
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

func TestTakeDocsCapturesOnlyApprovedMarkdownWithIndependentCoverage(t *testing.T) {
	root := t.TempDir()
	write(t, root, "README.md", "# Root\nSee [design](docs/design.md).\n")
	write(t, root, "docs/design.md", "# Design\nA link to [another file](../src/linked.md) stays text.\n")
	write(t, root, "docs/.metadata.md", "hidden Markdown file in an allowed directory\n")
	write(t, root, "docs/nested/notes.md", "nested documentation\n")
	write(t, root, "src/README.md", "out of scope\n")
	write(t, root, "src/linked.md", "link targets are not followed\n")
	write(t, root, "docs/.hidden/private.md", "hidden directory\n")
	write(t, root, "docs/private.md", "policy excluded\n")
	write(t, root, ".intentignore", "docs/private.md\n")

	// Entries outside scope and denied by policy must be rejected before Info,
	// the metadata operation immediately preceding the capture read.
	walk := func(root string, visit fs.WalkDirFunc) error {
		return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d == nil {
				return visit(path, d, err)
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			if rel == "src/README.md" || rel == "docs/private.md" {
				d = captureDirEntry{DirEntry: d, info: func() (fs.FileInfo, error) {
					t.Fatalf("filtered path was inspected before selection: %s", rel)
					return nil, nil
				}}
			}
			return visit(path, d, nil)
		})
	}
	s, c, err := takeDocs(root, walk)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"README.md", "docs/design.md", "docs/.metadata.md", "docs/nested/notes.md"} {
		if _, ok := s.Files[name]; !ok {
			t.Errorf("approved Markdown was not captured: %s", name)
		}
	}
	for _, name := range []string{"src/README.md", "docs/private.md"} {
		if _, ok := s.Files[name]; ok {
			t.Errorf("out-of-scope or excluded path was captured: %s", name)
		}
	}
	if s.Files["docs/design.md"].Content != "# Design\nA link to [another file](../src/linked.md) stays text.\n" {
		t.Fatal("Markdown text or link was transformed")
	}
	if _, ok := s.Files["src/linked.md"]; ok {
		t.Fatal("Markdown link target outside the approved scope was followed")
	}
	if _, ok := s.Files["docs/.hidden/private.md"]; ok {
		t.Fatal("Markdown under a hidden directory was captured")
	}
	if c.Scope != "docs-only" || c.BudgetBytes != MaxSnapshotBytes || c.CapturedBytes == 0 {
		t.Fatalf("unexpected docs coverage: %+v", c)
	}
	if c.Complete || c.ExcludedCount != 1 || len(c.Excluded) != 1 || c.Excluded[0] != (CoverageOmission{File: "docs/private.md", Reason: "policy_excluded"}) {
		t.Fatalf("policy exclusion was not reported independently: %+v", c)
	}
}

func TestTakeDocsReportsOversizeInvalidUTF8AndChangedFiles(t *testing.T) {
	root := t.TempDir()
	write(t, root, "docs/large.md", strings.Repeat("x", MaxFileBytes+1))
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs/invalid.md"), []byte{0xff, 0xfe}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs/nul.md"), []byte("before\x00after"), 0600); err != nil {
		t.Fatal(err)
	}
	write(t, root, "docs/change.md", "old content\n")
	walk := func(root string, visit fs.WalkDirFunc) error {
		return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d == nil || filepath.Base(path) != "change.md" {
				return visit(path, d, err)
			}
			entry := captureDirEntry{DirEntry: d, info: func() (fs.FileInfo, error) {
				info, infoErr := d.Info()
				if infoErr == nil {
					if removeErr := os.Remove(path); removeErr != nil {
						t.Fatal(removeErr)
					}
				}
				return info, infoErr
			}}
			return visit(path, entry, nil)
		})
	}
	s, c, err := takeDocs(root, walk)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Files) != 0 || len(s.Omitted) != 4 || c.Complete || c.OmittedCount != 4 {
		t.Fatalf("expected all four invalid docs omitted: files=%v snapshot_omitted=%v coverage=%+v", s.Files, s.Omitted, c)
	}
	wantReasons := map[string]string{
		"docs/change.md":  "file_changed",
		"docs/invalid.md": "binary_or_invalid_utf8",
		"docs/large.md":   "file_too_large",
		"docs/nul.md":     "binary_or_invalid_utf8",
	}
	for _, omission := range c.Omitted {
		if wantReasons[omission.File] != omission.Reason {
			t.Errorf("unexpected omission: %+v", omission)
		}
		delete(wantReasons, omission.File)
	}
	if len(wantReasons) != 0 {
		t.Errorf("missing omission reports: %v", wantReasons)
	}
}

func TestTakeGoAndDocsKeepBudgetsAndOmissionsIndependent(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 16; i++ {
		write(t, root, fmt.Sprintf("%02d.go", i), strings.Repeat("g", MaxFileBytes))
	}
	for i := 0; i < 17; i++ {
		write(t, root, fmt.Sprintf("docs/%02d.md", i), strings.Repeat("d", MaxFileBytes))
	}
	s, c, err := TakeGoAndDocs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Files) != 32 || c.Scope != "go+docs" || c.Components == nil {
		t.Fatalf("unexpected combined capture: files=%d coverage=%+v", len(s.Files), c)
	}
	goCoverage, docsCoverage := c.Components.Go, c.Components.Docs
	if !goCoverage.Complete || goCoverage.CapturedBytes != MaxSnapshotBytes || goCoverage.OmittedCount != 0 {
		t.Fatalf("docs budget affected Go coverage: %+v", goCoverage)
	}
	if docsCoverage.Complete || docsCoverage.CapturedBytes != MaxSnapshotBytes || docsCoverage.OmittedCount != 1 || len(docsCoverage.Omitted) != 1 || docsCoverage.Omitted[0].Reason != "snapshot_budget" {
		t.Fatalf("docs overflow was not isolated to docs coverage: %+v", docsCoverage)
	}
	encoded, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var report map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &report); err != nil {
		t.Fatal(err)
	}
	if _, exists := report["captured_bytes"]; exists {
		t.Fatalf("combined report exposed a misleading shared byte total: %s", encoded)
	}
	if _, exists := report["omitted_count"]; exists {
		t.Fatalf("combined report exposed a misleading shared omission total: %s", encoded)
	}
}

func TestTakeGoAndDocsKeepPolicyExclusionsInTheirOwnReports(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".intentignore", "docs/private.md\npkg/private.go\n")
	write(t, root, "docs/private.md", "private docs\n")
	write(t, root, "pkg/private.go", "package pkg\n")
	_, c, err := TakeGoAndDocs(root)
	if err != nil {
		t.Fatal(err)
	}
	if c.Components == nil {
		t.Fatal("combined component coverage missing")
	}
	goCoverage, docsCoverage := c.Components.Go, c.Components.Docs
	if goCoverage.ExcludedCount != 1 || len(goCoverage.Excluded) != 1 || goCoverage.Excluded[0] != (CoverageOmission{File: "pkg/private.go", Reason: "policy_excluded"}) {
		t.Fatalf("Go exclusions mixed with docs policy exclusions: %+v", goCoverage)
	}
	if docsCoverage.ExcludedCount != 1 || len(docsCoverage.Excluded) != 1 || docsCoverage.Excluded[0] != (CoverageOmission{File: "docs/private.md", Reason: "policy_excluded"}) {
		t.Fatalf("docs exclusion missing or mixed with Go: %+v", docsCoverage)
	}
}

func TestTakeDocsDoesNotFollowLinkedDocsDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	write(t, outside, "design.md", "outside secret\n")
	if err := os.Symlink(outside, filepath.Join(root, "docs")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	s, c, err := TakeDocs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Files) != 0 || len(s.Omitted) != 1 || s.Omitted[0] != "docs" {
		t.Fatalf("linked docs directory was followed: %+v", s)
	}
	if c.Complete || c.OmittedCount != 1 || len(c.Omitted) != 1 || c.Omitted[0] != (CoverageOmission{File: "docs", Reason: "unsafe_path"}) {
		t.Fatalf("linked docs directory omission missing: %+v", c)
	}
}

func TestTakeDocsEnforcesApprovedFileCountLimit(t *testing.T) {
	root := t.TempDir()
	for i := 0; i <= MaxDocsFiles; i++ {
		write(t, root, fmt.Sprintf("docs/%04d.md", i), "")
	}
	s, c, err := TakeDocs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Files) != MaxDocsFiles || c.Complete || c.OmittedCount != 1 || len(c.Omitted) != 1 {
		t.Fatalf("docs file-count cap was not reported: files=%d coverage=%+v", len(s.Files), c)
	}
	if c.Omitted[0] != (CoverageOmission{File: "docs/2000.md", Reason: "file_count_limit"}) {
		t.Fatalf("unexpected docs file-count omission: %+v", c.Omitted)
	}
}

func TestCombinedGoCaptureEnforcesFileCountLimit(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		write(t, root, name, "package p\n")
	}
	s, c, err := takeGoInternal(root, filepath.WalkDir, true, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Files) != 2 || c.Complete || c.OmittedCount != 1 || c.Omitted[0] != (CoverageOmission{File: "c.go", Reason: "file_count_limit"}) {
		t.Fatalf("Go file-count cap was not reported: files=%v coverage=%+v", s.Files, c)
	}
}
