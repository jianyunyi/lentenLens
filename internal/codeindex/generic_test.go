package codeindex

import (
	"reflect"
	"testing"
)

func TestGenericAndParenthesizedDirectCalls(t *testing.T) {
	tests := []struct{ name, decl, body, target string }{
		{"explicit single type", "func Identity[T any](v T) T { return v }", "Identity[int](1)", "helper.go:Identity"},
		{"explicit multiple types", "func Pair[A, B any](a A, b B) {}", "Pair[int, string](1, \"x\")", "helper.go:Pair"},
		{"inferred types", "func Identity[T any](v T) T { return v }", "Identity(1)", "helper.go:Identity"},
		{"parenthesized function", "func Helper() {}", "(Helper)()", "helper.go:Helper"},
		{"parenthesized generic", "func Identity[T any](v T) T { return v }", "((Identity[int]))(1)", "helper.go:Identity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := Build(map[string]string{"main.go": "package p\nfunc Caller() { " + tt.body + " }", "helper.go": "package p\n" + tt.decl})
			if len(idx.Issues) != 0 {
				t.Fatalf("invalid fixture: %v", idx.Issues)
			}
			for _, fn := range idx.Functions {
				if fn.Name == "Caller" {
					if len(fn.Calls) != 1 || fn.Calls[0].Status != "resolved" || !reflect.DeepEqual(fn.Calls[0].Targets, []string{tt.target}) {
						t.Fatalf("missing direct dependency: %+v", fn.Calls)
					}
					return
				}
			}
			t.Fatal("caller missing")
		})
	}
}

func TestIndirectAndConversionCallsStayUnresolved(t *testing.T) {
	tests := []struct{ name, body string }{
		{"function variable", "Identity := func(v int) int { return v }; (Identity)(1)"},
		{"generic function value", "f := Identity[int]; (f)(1)"},
		{"indexed function slice", "Identity := []func(int) int{func(v int) int { return v }}; Identity[0](1)"},
		{"indexed function map", "Identity := map[int]func(int) int{}; (Identity[0])(1)"},
		{"type conversion", "type Identity int; _ = (Identity)(1)"},
		{"generic type conversion", "type Box[T any] int; _ = Box[int](1)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := Build(map[string]string{"main.go": "package p\nfunc Identity[T any](v T) T { return v }\nfunc Caller() { " + tt.body + " }"})
			if len(idx.Issues) != 0 {
				t.Fatalf("invalid fixture: %v", idx.Issues)
			}
			for _, fn := range idx.Functions {
				if fn.Name == "Caller" {
					if len(fn.Calls) != 1 || fn.Calls[0].Status != "unresolved" || len(fn.Calls[0].Targets) != 0 {
						t.Fatalf("indirect call guessed: %+v", fn.Calls)
					}
					return
				}
			}
			t.Fatal("caller missing")
		})
	}
}

func TestParenthesizedPlatformVariantsRemainAmbiguous(t *testing.T) {
	idx := Build(map[string]string{
		"main.go":           "package p\nfunc Caller() { (Helper)() }",
		"helper_linux.go":   "package p\nfunc Helper() {}",
		"helper_windows.go": "package p\nfunc Helper() {}",
	})
	for _, fn := range idx.Functions {
		if fn.Name == "Caller" {
			if len(fn.Calls) != 1 || fn.Calls[0].Status != "ambiguous" || !reflect.DeepEqual(fn.Calls[0].Targets, []string{"helper_linux.go:Helper", "helper_windows.go:Helper"}) {
				t.Fatalf("variant guessed: %+v", fn.Calls)
			}
			return
		}
	}
	t.Fatal("caller missing")
}
