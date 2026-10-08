package gonav

import "testing"

func TestCrossFileAndDirectoryIsolation(t *testing.T) {
	r := Analyze(map[string]string{
		"one/a.go": "package p\nfunc A(){ B() }",
		"one/b.go": "package p\nfunc B(){}",
		"two/b.go": "package p\nfunc B(){}",
	})
	if len(r.Functions) != 3 {
		t.Fatalf("functions: %+v", r)
	}
	c := r.Functions[0].Calls[0]
	if c.Status != "resolved" || len(c.Targets) != 1 || c.Targets[0].File != "one/b.go" {
		t.Fatalf("call: %+v", c)
	}
}

func TestShadowedCallsRemainUnknown(t *testing.T) {
	r := Analyze(map[string]string{"a.go": "package p\nfunc B(){}\nfunc A(){ B := func(){}; B() }\nfunc C(B func()){ B() }"})
	for _, f := range r.Functions {
		for _, c := range f.Calls {
			if c.Status != "unknown" {
				t.Fatalf("shadowed: %+v", c)
			}
		}
	}
}

func TestSelectorAndInterfaceCallsRemainUnknown(t *testing.T) {
	r := Analyze(map[string]string{"a.go": "package p\ntype I interface{ Run() }; type T struct{}\nfunc (T) Run(){}\nfunc A(i I, t T){ i.Run(); t.Run() }"})
	var count int
	for _, f := range r.Functions {
		for _, c := range f.Calls {
			count++
			if c.Status != "unknown" {
				t.Fatalf("selector: %+v", c)
			}
		}
	}
	if count != 2 {
		t.Fatalf("calls: %d", count)
	}
}

func TestParseErrorDiagnostic(t *testing.T) {
	r := Analyze(map[string]string{"broken.go": "package p\nfunc A( {"})
	if len(r.Diagnostics) == 0 {
		t.Fatal("expected syntax diagnostic")
	}
}

func TestDuplicateDeclarationsAreNotResolved(t *testing.T) {
	r := Analyze(map[string]string{"a.go": "package p\nfunc B(){}\nfunc B(){}\nfunc A(){ B() }"})
	c := r.Functions[2].Calls[0]
	if c.Status != "candidates" || len(c.Targets) != 2 || len(r.Diagnostics) == 0 {
		t.Fatalf("duplicate: %+v", r)
	}
}

func TestFunctionRangesAndDeterminism(t *testing.T) {
	r := Analyze(map[string]string{"a.go": "package p\nfunc A(){\n B()\n}\nfunc B(){}\n"})
	f := r.Functions[0]
	if f.StartLine != 2 || f.EndLine != 4 || f.Calls[0].Line != 3 {
		t.Fatalf("range: %+v", f)
	}
}
