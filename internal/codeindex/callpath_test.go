package codeindex

import (
	"reflect"
	"testing"
)

func TestMultipleInitFunctionsHaveUniqueStableIDs(t *testing.T) {
	files := map[string]string{
		"main.go": "package p\n//line spoofed.go:10\nfunc init(){ First() }; func init(){ Second() }\nfunc init(){ Third() }\nfunc First(){}\nfunc Second(){}\nfunc Third(){}\n",
	}
	one, two := Build(files), Build(files)
	if len(one.Issues) != 0 {
		t.Fatalf("invalid multiple-init fixture: %v", one.Issues)
	}
	if !reflect.DeepEqual(one.Functions, two.Functions) {
		t.Fatal("repeated indexing changed function IDs or call targets")
	}
	if len(one.Functions) != 6 {
		t.Fatalf("lost declarations: %+v", one.Functions)
	}
	seen := map[string]bool{}
	for i, fn := range one.Functions {
		if seen[fn.ID] {
			t.Fatalf("duplicate function ID: %s", fn.ID)
		}
		seen[fn.ID] = true
		if i < 3 {
			wantName := []string{"First", "Second", "Third"}[i]
			if fn.Name != "init" || len(fn.Calls) != 1 || fn.Calls[0].Name != wantName || fn.Calls[0].Status != "resolved" || !reflect.DeepEqual(fn.Calls[0].Targets, []string{"main.go:" + wantName}) {
				t.Fatalf("init declaration lost its own call evidence: %+v", fn)
			}
		}
	}
}

func TestLiteralBodyCallsDoNotLeakIntoOuterFunction(t *testing.T) {
	for _, body := range []string{
		"_ = func(){ Hidden() }; Visible()",
		"callback := func(){ Hidden() }; callback(); Visible()",
		"func(){ Hidden() }(); Visible()",
		"Use(func(){ Hidden() }, Visible())",
	} {
		t.Run(body, func(t *testing.T) {
			idx := Build(map[string]string{"main.go": "package p\nfunc Outer(){" + body + "}\nfunc Hidden(){}\nfunc Visible() int {return 0}\nfunc Use(func(), int){}\n"})
			if len(idx.Issues) != 0 {
				t.Fatalf("invalid literal fixture: %v", idx.Issues)
			}
			outer := idx.Functions[0]
			foundVisible := false
			for _, call := range outer.Calls {
				if call.Name == "Hidden" || reflect.DeepEqual(call.Targets, []string{"main.go:Hidden"}) {
					t.Fatalf("literal body leaked into outer calls: %+v", outer.Calls)
				}
				if call.Name == "Visible" && call.Status == "resolved" {
					foundVisible = true
				}
			}
			if !foundVisible {
				t.Fatalf("outer call or call argument was skipped: %+v", outer.Calls)
			}
		})
	}
}

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
