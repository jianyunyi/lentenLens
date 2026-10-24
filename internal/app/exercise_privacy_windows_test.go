//go:build windows

package app

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestExerciseRevocationRetriesAfterWindowsFileLock(t *testing.T) {
	a, root := fixture(t)
	e := seedExercise(t, a, root)
	dir, err := a.exerciseDirectory(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "locked.txt")
	if err = os.WriteFile(file, []byte("PRIVATE_REFERENCE_REMAINDER"), 0600); err != nil {
		t.Fatal(err)
	}
	name, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)
	put(t, root, ".intentignore", "main.go\n")
	if err = a.scrubExcluded(); err == nil {
		t.Fatal("locked file deletion unexpectedly succeeded")
	}
	if _, err = os.Stat(filepath.Join(a.State, "exercise-"+e.ID+".json")); err != nil {
		t.Fatal("retry index lost", err)
	}
	if err = syscall.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	if err = a.scrubExcluded(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("revoked source remains after retry")
	}
}
