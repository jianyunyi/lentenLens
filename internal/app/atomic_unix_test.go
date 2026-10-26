//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package app

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestAtomicReplacePreservesModeWithRestrictiveUmask(t *testing.T) {
	// No parallel test: umask is process-global, and this test restores it.
	root := t.TempDir()
	put(t, root, "source.go", "before")
	if err := os.Chmod(filepath.Join(root, "source.go"), 0644); err != nil {
		t.Fatal(err)
	}
	old := syscall.Umask(0077)
	defer syscall.Umask(old)
	confined, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer confined.Close()
	before := "before"
	if err = atomicReplace(confined, "source.go", []byte("after"), 0644, &before); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, "source.go"))
	if err != nil || info.Mode().Perm() != 0644 {
		t.Fatalf("mode changed: %v %v", info, err)
	}
}
