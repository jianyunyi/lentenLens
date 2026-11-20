package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadStateRecordRetriesAfterAtomicReplacement(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if err := os.WriteFile(filepath.Join(dir, "record.json"), []byte(`{"value":"old"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	replaced := false
	open := func(name string) (*os.File, error) {
		if !replaced {
			replaced = true
			if err := atomicReplace(root, name, []byte(`{"value":"new"}`), 0o600, nil); err != nil {
				t.Fatal(err)
			}
		}
		return root.Open(name)
	}
	var got struct {
		Value string `json:"value"`
	}
	if err := readStateRecord(root, "record.json", &got, open); err != nil {
		t.Fatalf("read after replacement: %v", err)
	}
	if got.Value != "new" {
		t.Fatalf("read value = %q, want new", got.Value)
	}
}
