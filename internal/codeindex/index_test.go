package codeindex

import (
	"reflect"
	"testing"
)

func TestPlatformVariantsRemainAmbiguous(t *testing.T) {
	i := Build(map[string]string{
		"main.go":           "package p\nfunc F(){helper()}\nfunc Shadow(){helper := func(){}; helper()}",
		"helper_linux.go":   "package p\nfunc helper(){}",
		"helper_windows.go": "package p\nfunc helper(){}",
	})
	for _, f := range i.Functions {
		if f.Name == "F" {
			if len(f.Calls) != 1 || f.Calls[0].Status != "ambiguous" || !reflect.DeepEqual(f.Calls[0].Targets, []string{"helper_linux.go:helper", "helper_windows.go:helper"}) {
				t.Fatalf("platform variant arbitrarily resolved: %+v", f.Calls)
			}
		}
		if f.Name == "Shadow" && (len(f.Calls) != 1 || f.Calls[0].Status != "unresolved" || len(f.Calls[0].Targets) != 0) {
			t.Fatalf("local variable incorrectly resolved: %+v", f.Calls)
		}
	}
}

func TestResolutionAndBounds(t *testing.T) {
	i := Build(map[string]string{"a.go": "package p\nfunc A() { B(); var B = func() {}; B() }\n", "b.go": "package p\nfunc B() {}\ntype I interface { M() }\nfunc C(x I) { x.M() }\ntype T struct{}\nfunc (T) M() {}\ntype U struct{}\nfunc (*U) M() {}\n"})
	if len(i.Functions) != 5 {
		t.Fatalf("%+v", i)
	}
	a := i.Functions[0]
	if a.ID != "a.go:A" || a.StartLine != 2 || a.EndLine != 2 || len(a.Calls) != 2 || a.Calls[0].Status != "resolved" || a.Calls[0].Targets[0] != "b.go:B" || a.Calls[1].Status != "unresolved" {
		t.Fatalf("%+v", a)
	}
	if i.Functions[2].Calls[0].Status != "unresolved" {
		t.Fatal(i.Functions[2])
	}
	if i.Functions[3].ID == i.Functions[4].ID {
		t.Fatal("method IDs collide")
	}
}
func TestInvalidVisibleAndPackagesSeparate(t *testing.T) {
	i := Build(map[string]string{"broken.go": "package ?", "one/a.go": "package p\nfunc A(){B()}", "two/b.go": "package p\nfunc B(){}"})
	if len(i.Issues) == 0 {
		t.Fatal("parse issue hidden")
	}
	if i.Functions[0].Calls[0].Status != "unresolved" {
		t.Fatal("cross-directory resolution")
	}
}
