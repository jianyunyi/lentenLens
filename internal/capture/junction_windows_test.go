//go:build windows

package capture

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTakeGoJunctionPreservesLaterSiblings(t *testing.T) {
	for _, name := range []string{"root", "nested"} {
		t.Run(name, func(t *testing.T) {
			root, target := t.TempDir(), t.TempDir()
			parent := root
			if name == "nested" {
				parent = filepath.Join(root, "package")
				if err := os.Mkdir(parent, 0700); err != nil {
					t.Fatal(err)
				}
			}
			write(t, target, "hidden.go", "package p\nfunc Hidden() {}\n")
			link := filepath.Join(parent, "a-link")
			// Junctions require no symlink privilege. Creation failure must fail Windows CI.
			if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
				t.Fatalf("create junction: %v: %s", err, out)
			}
			write(t, parent, "z.go", "package p\nfunc Z() {}\n")
			write(t, parent, "z-subdir/child.go", "package p\nfunc Child() {}\n")
			snap, coverage, err := TakeGo(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"z.go", "z-subdir/child.go"} {
				rel, err := filepath.Rel(root, filepath.Join(parent, filepath.FromSlash(path)))
				if err != nil {
					t.Fatal(err)
				}
				if _, ok := snap.Files[filepath.ToSlash(rel)]; !ok {
					t.Errorf("later sibling missing: %s", rel)
				}
			}
			if len(snap.Files) != 2 || coverage.ExcludedCount != 1 || coverage.OmittedCount != 0 || coverage.Complete {
				t.Fatalf("junction must be excluded without losing siblings: files=%v coverage=%+v", snap.Files, coverage)
			}
			rel, err := filepath.Rel(root, filepath.Join(link, "hidden.go"))
			if err != nil {
				t.Fatal(err)
			}
			if ok, err := Allowed(root, filepath.ToSlash(rel)); err != nil || ok {
				t.Fatalf("junction target allowed: %t %v", ok, err)
			}
		})
	}
}
