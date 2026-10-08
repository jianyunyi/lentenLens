package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (*Store, string) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	os.MkdirAll(root, 0700)
	s, err := Open(root, filepath.Join(base, "state"))
	if err != nil {
		t.Fatal(err)
	}
	return s, root
}
func put(t *testing.T, root, name, content string) {
	t.Helper()
	p := filepath.Join(root, name)
	os.MkdirAll(filepath.Dir(p), 0700)
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestRoundLifecycleAndHistoricalEvidence(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "a.go", "package a\nfunc A() {}\n")
	if _, err := s.End(); err == nil {
		t.Fatal("end without start accepted")
	}
	if _, err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(); err == nil {
		t.Fatal("overlapping round accepted")
	}
	put(t, root, "a.go", "package a\nfunc A() { println(1) }\n")
	r, err := s.End()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Changes) != 1 {
		t.Fatal(r)
	}
	e, err := s.Evidence(r.ID, "a.go", "after", 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if e.Status != "referenced_evidence_unchanged" || e.Text != "func A() { println(1) }\n" {
		t.Fatal(e)
	}
	put(t, root, "a.go", "package a\nfunc A() { println(2) }\n")
	e, err = s.Evidence(r.ID, "a.go", "after", 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if e.Status != "stale" || !strings.Contains(e.Text, "println(1)") {
		t.Fatal(e)
	}
	reopened, err := Open(root, filepath.Dir(s.Dir))
	if err != nil {
		t.Fatal(err)
	}
	rs, err := reopened.History()
	if err != nil || len(rs) != 1 {
		t.Fatalf("history: %v %v", rs, err)
	}
}
func TestStoreMustBeOutsideProject(t *testing.T) {
	root := t.TempDir()
	if _, err := Open(root, filepath.Join(root, ".state")); err == nil {
		t.Fatal("private state inside project")
	}
}
func TestPolicyChangePurgesHistoricalContent(t *testing.T) {
	s, root := fixture(t)
	s.Start()
	put(t, root, "secret.go", "UNIQUE_PRIVATE_TEXT")
	r, err := s.End()
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, ".intentignore", "secret.go\n")
	if _, err := s.History(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(s.Dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "UNIQUE_PRIVATE_TEXT") {
		t.Fatal("excluded history remained on disk")
	}
	if _, err := s.Evidence(r.ID, "secret.go", "after", 1, 1); err == nil {
		t.Fatal("excluded evidence accessible")
	}
}
func TestCurrentFunctionUnchangedDespiteOtherEdits(t *testing.T) {
	s, root := fixture(t)
	s.Start()
	put(t, root, "a.go", "package a\nfunc A() {}\nfunc B() {}\n")
	r, err := s.End()
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "a.go", "package a\n// shifted\nfunc A() {}\nfunc B() { println(2) }\n")
	e, err := s.Evidence(r.ID, "a.go", "after", 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if e.Status != "referenced_evidence_unchanged" || e.CurrentStartLine != 3 {
		t.Fatal(e)
	}
}
func TestExcludedBeforeFileDoesNotAppearAsDeletion(t *testing.T) {
	s, root := fixture(t)
	put(t, root, "hidden.go", "PRIVATE")
	s.Start()
	put(t, root, ".intentignore", "hidden.go\n")
	r, err := s.End()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(r)
	if len(r.Changes) != 0 || strings.Contains(string(b), "PRIVATE") {
		t.Fatal(string(b))
	}
}
func TestEvidenceRejectsBadRangesAndMissingVersions(t *testing.T) {
	s, root := fixture(t)
	s.Start()
	put(t, root, "a.go", "package a\n")
	r, err := s.End()
	if err != nil {
		t.Fatal(err)
	}
	for _, side := range []string{"before", "invalid"} {
		if _, err := s.Evidence(r.ID, "a.go", side, 1, 1); err == nil {
			t.Fatal("missing side accepted")
		}
	}
	if _, err := s.Evidence(r.ID, "a.go", "after", 0, 9); err == nil {
		t.Fatal("invalid range accepted")
	}
}
