package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSharedRejectsReplacementAfterLstat(t *testing.T) {
	dirPath := t.TempDir()
	dir, err := os.OpenRoot(dirPath)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	name := "record.json"
	if err := os.WriteFile(filepath.Join(dirPath, name), []byte(`{"id":"record"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	open := func(name string) (*os.File, error) {
		if err := atomicReplace(dir, name, []byte(`{"id":"record"}`), 0o600, nil); err != nil {
			t.Fatal(err)
		}
		return dir.Open(name)
	}
	if _, err := readSharedRecord(dir, name, open); err == nil || !strings.Contains(err.Error(), "读取期间发生变化") {
		t.Fatalf("readSharedRecord error = %v, want path-change rejection", err)
	}
}

func TestReadSharedRejectsOpenedFileOverLimit(t *testing.T) {
	dirPath := t.TempDir()
	dir, err := os.OpenRoot(dirPath)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	name := "record.json"
	if err := os.WriteFile(filepath.Join(dirPath, name), []byte(`{"id":"record"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	open := func(name string) (*os.File, error) {
		path := filepath.Join(dirPath, name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
		if err != nil {
			return nil, err
		}
		if _, err := f.Write([]byte(strings.Repeat("x", maxShareBytes+1))); err != nil {
			f.Close()
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
		return dir.Open(name)
	}
	if _, err := readSharedRecord(dir, name, open); err == nil || !strings.Contains(err.Error(), "小于 1 MiB") {
		t.Fatalf("readSharedRecord error = %v, want opened-size rejection", err)
	}
}
