package app

import "testing"

func TestNewlyAllowedFileIsUnknownRatherThanAdded(t *testing.T) {
	a, root := fixture(t)
	put(t, root, "existing.go", "package p\nfunc Existing() {}\n")
	put(t, root, ".intentignore", "existing.go\n")
	if _, e := a.Start("s"); e != nil {
		t.Fatal(e)
	}
	put(t, root, ".intentignore", "")
	r, e := a.Stop("s")
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Changes) != 0 {
		t.Fatalf("previously unobserved source reported added: %+v", r.Changes)
	}
	if len(r.Omitted) == 0 {
		t.Fatal("missing baseline not reported")
	}
}
