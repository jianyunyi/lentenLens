package app

import (
	"fmt"
	"strings"
	"testing"
)

func TestCallPathIncludesAncestorsDescendantsAndBoundaryCalls(t *testing.T) {
	a, root := fixture(t)
	put(t, root, "main.go", `package p
func Root(){ A() }
func A(){ B(); Missing() }
func B(){ A() }
`)
	got, err := a.CallPath("main.go", 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedID != "main.go:A" {
		t.Fatalf("selected=%q", got.SelectedID)
	}
	if len(got.Functions) != 3 || len(got.Calls) != 4 {
		t.Fatalf("functions=%d calls=%d: %+v", len(got.Functions), len(got.Calls), got)
	}
	if len(got.RootIDs) != 1 || got.RootIDs[0] != "main.go:Root" {
		t.Fatalf("roots=%v issues=%v", got.RootIDs, got.Issues)
	}
	foundBoundary := false
	for _, c := range got.Calls {
		if c.Name == "Missing" && c.Status == "unresolved" && c.Source == "Missing()" {
			foundBoundary = true
		}
	}
	if !foundBoundary {
		t.Fatalf("missing unresolved boundary evidence: %+v", got.Calls)
	}
}

func TestCallPathCycleRepresentativeIsExplicitAndStable(t *testing.T) {
	a, root := fixture(t)
	put(t, root, "cycle.go", `package p
func A(){ B() }
func B(){ A() }
`)
	one, err := a.CallPath("cycle.go", 2)
	if err != nil {
		t.Fatal(err)
	}
	two, err := a.CallPath("cycle.go", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(one.RootIDs) != 1 || one.RootIDs[0] != "cycle.go:A" {
		t.Fatalf("roots=%v", one.RootIDs)
	}
	if !strings.Contains(strings.Join(one.Issues, "\n"), "不是实际入口源") {
		t.Fatalf("cycle warning missing: %v", one.Issues)
	}
	if one.SelectedID != two.SelectedID || strings.Join(one.RootIDs, ",") != strings.Join(two.RootIDs, ",") {
		t.Fatal("non-deterministic result")
	}
}

func TestCallPathDoesNotSwitchTraversalDirection(t *testing.T) {
	a, root := fixture(t)
	put(t, root, "focus.go", `package p
func Root(){ Selected(); Sibling() }
func Selected(){ Leaf() }
func Leaf(){}
func Sibling(){ SiblingLeaf() }
func SiblingLeaf(){}
func OtherCaller(){ Leaf() }
`)
	got, err := a.CallPath("focus.go", 3)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"focus.go:Root": true, "focus.go:Selected": true, "focus.go:Leaf": true}
	for _, fn := range got.Functions {
		if !want[fn.ID] {
			t.Fatalf("unrelated function leaked into focus: %s", fn.ID)
		}
		delete(want, fn.ID)
	}
	if len(want) != 0 {
		t.Fatalf("missing nodes: %v", want)
	}
	// Root's out-of-focus call evidence is preserved without pretending Sibling
	// is a dependency of Selected or an unresolved target.
	found := false
	for _, c := range got.Calls {
		if c.Name == "Sibling" && c.Status == "resolved" && len(c.Targets) == 1 && c.Targets[0] == "focus.go:Sibling" {
			found = true
		}
	}
	if !found {
		t.Fatal("resolved out-of-focus callsite was lost")
	}
}

func TestCallPathUsesSourceSCCsNotLexicalUncoveredNodes(t *testing.T) {
	a, root := fixture(t)
	put(t, root, "scc.go", `package p
func ALeaf(){}
func RealRoot(){ Selected() }
func Selected(){ ALeaf() }
func ZCycleA(){ ZCycleB(); Selected() }
func ZCycleB(){ ZCycleA() }
`)
	got, err := a.CallPath("scc.go", 4)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.RootIDs, ",") != "scc.go:RealRoot,scc.go:ZCycleA" {
		t.Fatalf("wrong source components: %v", got.RootIDs)
	}
	warnings := strings.Join(got.Issues, "\n")
	if !strings.Contains(warnings, "ZCycleA") || strings.Contains(warnings, "ALeaf 是静态源循环") {
		t.Fatalf("wrong cycle warning: %s", warnings)
	}
}

func TestCallPathNodeLimitIsExplicitAndCutoffIsNotOrigin(t *testing.T) {
	a, root := fixture(t)
	var source strings.Builder
	source.WriteString("package p\n")
	for i := 0; i < 205; i++ {
		if i == 204 {
			fmt.Fprintf(&source, "func N%03d(){}\n", i)
		} else {
			fmt.Fprintf(&source, "func N%03d(){ N%03d() }\n", i, i+1)
		}
	}
	put(t, root, "limit.go", source.String())
	got, err := a.CallPath("limit.go", 206)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Truncated || len(got.Functions) != maxCallPathFunctions {
		t.Fatalf("limit not enforced: %d / %v", len(got.Functions), got.Truncated)
	}
	if strings.Join(got.RootIDs, ",") != "limit.go:N005" {
		t.Fatalf("cutoff root=%v", got.RootIDs)
	}
	if !strings.Contains(strings.Join(got.Issues, "\n"), "截断边界") {
		t.Fatalf("cutoff incorrectly implied origin: %v", got.Issues)
	}
}

func TestCallPathCallsiteLimitIsExplicit(t *testing.T) {
	a, root := fixture(t)
	put(t, root, "calls.go", "package p\nfunc Root(){"+strings.Repeat("Leaf();", maxCallPathCalls+1)+"}\nfunc Leaf(){}\n")
	got, err := a.CallPath("calls.go", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Truncated || len(got.Calls) != maxCallPathCalls {
		t.Fatalf("callsite limit not enforced: %d / %v", len(got.Calls), got.Truncated)
	}
}

func TestCallPathPrivacyAndSelectionFailures(t *testing.T) {
	a, root := fixture(t)
	put(t, root, "valid.go", "package p\nfunc Allowed(){}\n")
	for _, input := range []struct {
		file string
		line int
	}{{"../outside.go", 2}, {"missing.go", 1}, {"valid.go", 1}, {"valid.go", -1}} {
		if _, err := a.CallPath(input.file, input.line); err == nil {
			t.Fatalf("unexpected success: %+v", input)
		}
	}
	put(t, root, "bad.go", "package p\nfunc Broken(\n")
	if _, err := a.CallPath("bad.go", 2); err == nil || !strings.Contains(err.Error(), "索引不完整") {
		t.Fatalf("parse failure hidden: %v", err)
	}
	put(t, root, ".intentignore", "valid.go\n")
	if _, err := a.CallPath("valid.go", 2); err == nil || !strings.Contains(err.Error(), "排除") {
		t.Fatalf("policy exclusion ignored: %v", err)
	}
}
