package codeindex

import "testing"

func TestCallEvidenceUsesPhysicalPositionsDespiteLineDirective(t *testing.T) {
	idx := Build(map[string]string{"main.go": "package p\n//line spoofed.go:900\nfunc A() { B(1) }\nfunc B(int) {}\n"})
	var a Function
	for _, f := range idx.Functions {
		if f.Name == "A" {
			a = f
		}
	}
	if a.File != "main.go" || a.StartLine != 3 {
		t.Fatalf("physical function position lost: %+v", a)
	}
	if len(a.Calls) != 1 {
		t.Fatalf("calls=%+v", a.Calls)
	}
	c := a.Calls[0]
	if c.File != "main.go" || c.Line != 3 || c.Column == 0 || c.Source != "B(1)" {
		t.Fatalf("bad physical evidence: %+v", c)
	}
}
