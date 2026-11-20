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

// Test that aggregation integration works correctly
func TestAggregationIntegration(t *testing.T) {
	// Test case 1: No issues - Aggregated should be nil
	i := Build(map[string]string{"a.go": "package p\nfunc A() {}\n"})
	if len(i.Issues) > 0 {
		t.Errorf("Expected no issues, got %d", len(i.Issues))
	}
	if i.Aggregated != nil {
		t.Error("Expected Aggregated to be nil when there are no issues")
	}

	// Test case 2: Parse errors - Aggregated should be present
	i2 := Build(map[string]string{"broken.go": "package ?"})
	if len(i2.Issues) == 0 {
		t.Fatal("Expected parse error")
	}
	if i2.Aggregated == nil {
		t.Fatal("Expected Aggregated to be non-nil when there are issues")
	}
	if i2.Aggregated.Summary.TotalRestrictions == 0 {
		t.Error("Expected at least one restriction in aggregated result")
	}

	// Test case 3: Multiple issues - verify backward compatibility
	i3 := Build(map[string]string{
		"broken.go":  "package ?",
		"broken2.go": "package ??",
	})
	if len(i3.Issues) == 0 {
		t.Fatal("Expected multiple parse errors")
	}
	// Both Issues and Aggregated should be present
	if i3.Aggregated == nil {
		t.Fatal("Expected Aggregated to be non-nil")
	}
	// Verify backward compatibility - Issues field should still be populated
	if len(i3.Issues) != i3.Aggregated.Summary.TotalRestrictions {
		t.Errorf("Issues count (%d) doesn't match aggregated count (%d)",
			len(i3.Issues), i3.Aggregated.Summary.TotalRestrictions)
	}
}

// Test that aggregation includes category grouping
func TestAggregationCategories(t *testing.T) {
	i := Build(map[string]string{
		"parse_error.go": "package ?",
	})

	if i.Aggregated == nil {
		t.Fatal("Expected Aggregated to be non-nil")
	}

	if len(i.Aggregated.Categories) == 0 {
		t.Error("Expected at least one category")
	}

	// Verify that Parse Errors category exists
	foundParseErrors := false
	for _, cat := range i.Aggregated.Categories {
		if cat.Name == "Parse Errors" {
			foundParseErrors = true
			if cat.Count == 0 {
				t.Error("Parse Errors category has zero count")
			}
		}
	}

	if !foundParseErrors {
		t.Error("Expected Parse Errors category in aggregated result")
	}
}
