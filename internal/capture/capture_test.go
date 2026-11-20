package capture

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestDiffOmitsUnknownPathsOnEitherSide(t *testing.T) {
	for _, omitted := range []string{"data/file.txt", "data"} {
		for _, beforeOmitted := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/before=%t", omitted, beforeOmitted), func(t *testing.T) {
				known := Snapshot{Files: map[string]File{"data/file.txt": NewFile("text"), "database/keep.txt": NewFile("old")}}
				unknown := Snapshot{Files: map[string]File{"database/keep.txt": NewFile("new")}, Omitted: []string{omitted}}
				before, after := known, unknown
				if beforeOmitted {
					before, after = unknown, known
				}
				changes := Diff(before, after)
				if len(changes) != 1 || changes[0].Path != "database/keep.txt" || changes[0].Kind != "modified" {
					t.Fatalf("omitted path reported as concrete change: %+v", changes)
				}
				if !reflect.DeepEqual(unknown.Omitted, []string{omitted}) {
					t.Fatal("omissions mutated", unknown.Omitted)
				}
			})
		}
	}
}

func TestDiffTextBecomesOmitted(t *testing.T) {
	for name, content := range map[string]string{"binary": "a\x00b", "oversized": strings.Repeat("x", MaxFileBytes+1)} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "data.txt", "text")
			before, err := Take(root)
			if err != nil {
				t.Fatal(err)
			}
			write(t, root, "data.txt", content)
			after, err := Take(root)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after.Omitted, []string{"data.txt"}) {
				t.Fatal(after.Omitted)
			}
			if changes := Diff(before, after); len(changes) != 0 {
				t.Fatalf("false deletion: %+v", changes)
			}
			if changes := Diff(after, before); len(changes) != 0 {
				t.Fatalf("false addition: %+v", changes)
			}
		})
	}
}

func write(t *testing.T, root, name, data string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

type captureDirEntry struct {
	fs.DirEntry
	info func() (fs.FileInfo, error)
}

func (d captureDirEntry) Info() (fs.FileInfo, error) { return d.info() }

func TestCaptureHandlesDisappearanceDuringWalk(t *testing.T) {
	for _, goOnly := range []bool{false, true} {
		mode := "general"
		if goOnly {
			mode = "go"
		}
		for _, scenario := range []string{"before_info", "before_open", "before_read_dir", "permission_denied"} {
			t.Run(mode+"/"+scenario, func(t *testing.T) {
				root := t.TempDir()
				gone := "a.go"
				if scenario == "before_read_dir" {
					gone = "a-dir"
					if err := os.Mkdir(filepath.Join(root, gone), 0700); err != nil {
						t.Fatal(err)
					}
				} else {
					write(t, root, gone, "package p\n")
				}
				const keep = "package p\nfunc Keep() {}\n"
				write(t, root, "z.go", keep)
				walk := func(root string, visit fs.WalkDirFunc) error {
					return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
						if err != nil || path != filepath.Join(root, gone) {
							return visit(path, d, err)
						}
						remove := func() {
							if err := os.Remove(path); err != nil {
								t.Fatal(err)
							}
						}
						if scenario == "before_read_dir" {
							remove()
							return visit(path, d, nil)
						}
						entry := captureDirEntry{d, func() (fs.FileInfo, error) {
							switch scenario {
							case "permission_denied":
								return nil, &fs.PathError{Op: "lstat", Path: path, Err: fs.ErrPermission}
							case "before_info":
								remove()
								// Some DirEntry implementations cache the metadata.
								return os.Lstat(path)
							default:
								info, err := d.Info()
								remove()
								return info, err
							}
						}}
						return visit(path, entry, nil)
					})
				}

				var snap Snapshot
				var coverage Coverage
				var err error
				if goOnly {
					snap, coverage, err = takeGo(root, walk)
				} else {
					snap, _, err = takeWithPolicy(root, walk)
				}
				if scenario == "permission_denied" {
					if err == nil || err.Error() != `cannot inspect capture path "a.go"` {
						t.Fatalf("permission error must fail the snapshot: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if len(snap.Files) != 1 || snap.Files["z.go"].Content != keep || !reflect.DeepEqual(snap.Omitted, []string{gone}) {
					t.Fatalf("want only z.go captured and %s omitted, got %+v", gone, snap)
				}
				if goOnly && (coverage.Complete || coverage.OmittedCount != 1 || !reflect.DeepEqual(coverage.Omitted, []CoverageOmission{{File: gone, Reason: "file_changed"}})) {
					t.Fatalf("disappearance must leave coverage incomplete: %+v", coverage)
				}
			})
		}
	}
}

func TestDirectoryRulesAndGenerated(t *testing.T) {
	r := t.TempDir()
	write(t, r, ".intentignore", "private/**/secret?.txt\ncache/\n")
	for _, p := range []string{"private/a/b/secret1.txt", "private/secret2.txt", "nested/cache/a.txt", "cache/a.txt"} {
		write(t, r, p, "hidden")
	}
	write(t, r, "private/a/b/keep.txt", "keep")
	s, e := Take(r)
	if e != nil || len(s.Files) != 1 {
		t.Fatal(s, e)
	}
	d := Diff(Snapshot{}, Snapshot{Files: map[string]File{"gen.go": NewFile("// Code generated by test. DO NOT EDIT.\npackage p\n")}})
	if len(d) != 1 || !d[0].Folded {
		t.Fatal(d)
	}
	for _, pattern := range []string{"!keep", "../secret", "a//b", "/absolute", "a\\b", "[broken"} {
		write(t, r, ".intentignore", pattern)
		if ok, e := Allowed(r, "private/a/b/keep.txt"); e == nil || ok {
			t.Fatalf("invalid rule accepted: %q", pattern)
		}
	}
}
func TestWindowsJunctionDenied(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows junction test")
	}
	r := t.TempDir()
	outside := t.TempDir()
	write(t, outside, "secret.txt", "hidden")
	link := filepath.Join(r, "junction")
	if out, e := exec.Command("cmd", "/c", "mklink", "/J", link, outside).CombinedOutput(); e != nil {
		t.Fatalf("junction unavailable: %v %s", e, out)
	}
	s, e := Take(r)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Files) != 0 {
		t.Fatal("junction captured", s)
	}
	if ok, _ := Allowed(r, "junction/secret.txt"); ok {
		t.Fatal("junction allowed")
	}
}
func TestCapturePolicyAndBounds(t *testing.T) {
	r := t.TempDir()
	for p, c := range map[string]string{"readme.md": "hello", ".env.local": "secret", "private/a.txt": "secret", "x.log": "secret", "nested/x.log": "secret", "nested/keep.txt": "yes", "bad.bin": "a\x00b", "large.txt": strings.Repeat("x", (1<<20)+1), ".intentignore": "private/\n*.log\n"} {
		write(t, r, p, c)
	}
	s, e := Take(r)
	if e != nil {
		t.Fatal(e)
	}
	if s.Files["readme.md"].Content != "hello" || s.Files["nested/keep.txt"].Content != "yes" {
		t.Fatal(s)
	}
	for _, p := range []string{".env.local", "private/a.txt", "x.log", "nested/x.log", "bad.bin", "large.txt"} {
		if _, ok := s.Files[p]; ok {
			t.Fatalf("captured %s", p)
		}
	}
	if len(s.Omitted) < 2 {
		t.Fatal(s.Omitted)
	}
	if ok, allowedErr := Allowed(r, "../outside"); allowedErr == nil && ok {
		t.Fatal("traversal allowed")
	}
	old := s.PolicyHash
	write(t, r, ".intentignore", "private/\n*.log\n*.md\n")
	s, e = Take(r)
	if e != nil || s.PolicyHash == old {
		t.Fatal("policy hash unchanged", e)
	}
	write(t, r, ".intentignore", "[secret-invalid")
	if _, e := Take(r); e == nil || strings.Contains(e.Error(), "secret-invalid") {
		t.Fatal("invalid policy must fail safely", e)
	}
}
func TestSymlinkDenied(t *testing.T) {
	r := t.TempDir()
	outside := t.TempDir()
	write(t, outside, "secret", "hidden")
	if e := os.Symlink(outside, filepath.Join(r, "linked")); e != nil {
		t.Skip(e)
	}
	s, e := Take(r)
	if e != nil {
		t.Fatal(e)
	}
	if _, ok := s.Files["linked/secret"]; ok {
		t.Fatal("link captured")
	}
	if ok, _ := Allowed(r, "linked/secret"); ok {
		t.Fatal("link allowed")
	}
}
func TestDiffNetAndFolding(t *testing.T) {
	a := Snapshot{Files: map[string]File{"a.go": {Content: "package p\nfunc F() {}\n"}, "gone.txt": {Content: "gone"}, "same.txt": {Content: "same"}}}
	b := Snapshot{Files: map[string]File{"a.go": {Content: "package p\nfunc F(){ }\n"}, "new.txt": {Content: "new"}, "same.txt": {Content: "same"}}}
	d := Diff(a, b)
	if len(d) != 3 || d[0].Path != "a.go" || !d[0].Folded || d[1].Kind != "deleted" || d[2].Kind != "added" {
		t.Fatalf("%+v", d)
	}
	if len(Diff(a, a)) != 0 {
		t.Fatal("revert not empty")
	}
}
func TestAggregateLimit(t *testing.T) {
	r := t.TempDir()
	for i := 0; i < 18; i++ {
		write(t, r, string(rune('a'+i))+".txt", strings.Repeat("x", 1<<20))
	}
	s, e := Take(r)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Files) != 16 || len(s.Omitted) != 2 {
		t.Fatalf("files=%d omitted=%d", len(s.Files), len(s.Omitted))
	}
}
