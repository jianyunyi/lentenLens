package capture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTakeGoDirectorySymlinkPreservesLaterSiblings(t *testing.T) {
	for _, nested := range []bool{false, true} {
		name := "root"
		if nested {
			name = "nested"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			parent := root
			if nested {
				parent = filepath.Join(root, "package")
				if err := os.Mkdir(parent, 0700); err != nil {
					t.Fatal(err)
				}
			}
			target := t.TempDir()
			write(t, target, "hidden.go", "package p\nfunc Hidden() {}\n")
			if err := os.Symlink(target, filepath.Join(parent, "a-link")); err != nil {
				t.Skipf("directory symlinks unavailable: %v", err)
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
				t.Fatalf("link must be excluded without losing siblings: files=%d coverage=%+v", len(snap.Files), coverage)
			}
		})
	}
}
