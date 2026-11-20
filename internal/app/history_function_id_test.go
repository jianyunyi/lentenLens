package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"intentlens/internal/codeindex"
)

func TestInspectReadsUniqueLegacyHistory(t *testing.T) {
	a, root := fixture(t)
	old := seedExplanation(t, a, root)

	info, err := a.Inspect(old.File, old.Evidence[0].StartLine)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.History) != 1 || info.History[0].ID != old.ID {
		t.Fatalf("expected only legacy explanation %q, got %+v", old.ID, info.History)
	}
}

func TestInspectAndReviseWithHistoricalFunctionID(t *testing.T) {
	a, root := fixture(t)
	old := seedExplanation(t, a, root)
	name := "explanation-" + old.ID + ".json"
	source, err := os.ReadFile(filepath.Join(root, old.File))
	if err != nil {
		t.Fatal(err)
	}
	// A second declaration gives F a position-qualified ID; removing it makes F unique again.
	put(t, root, old.File, string(source)+"func F() int { return 2 }\n")
	historical, err := a.Inspect(old.File, old.Evidence[0].StartLine)
	if err != nil {
		t.Fatal(err)
	}
	old.FunctionID = historical.Function.ID
	if err := a.write(name, old); err != nil {
		t.Fatal(err)
	}
	put(t, root, old.File, string(source))

	path := filepath.Join(a.State, name)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	assertLegacyUnchanged := func(operation string) {
		t.Helper()
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read legacy record after %s: %v", operation, err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("%s changed the legacy record on disk", operation)
		}
	}

	info, err := a.Inspect(old.File, old.Evidence[0].StartLine)
	if err != nil {
		t.Fatal(err)
	}
	if info.Function.ID == "" || info.Function.ID == old.FunctionID {
		t.Fatalf("expected a current ID distinct from %q, got %q", old.FunctionID, info.Function.ID)
	}
	if len(info.History) != 1 || info.History[0].ID != old.ID {
		t.Fatalf("expected legacy explanation %q in history for %q, got %+v", old.ID, info.Function.ID, info.History)
	}
	assertLegacyUnchanged("Inspect")

	revision, err := a.Revise(context.Background(), old.ID, "key")
	if err != nil {
		t.Fatal(err)
	}
	var stored Explanation
	if err := a.read("explanation-"+revision.ID+".json", &stored); err != nil {
		t.Fatal(err)
	}
	if stored.FunctionID != info.Function.ID || stored.Supersedes != old.ID {
		t.Fatalf("stored revision has function %q and parent %q, want %q and %q", stored.FunctionID, stored.Supersedes, info.Function.ID, old.ID)
	}
	assertLegacyUnchanged("Revise")
}

func TestFindFunctionByHistoricalIDFallsBackOnlyForUniqueLegacyName(t *testing.T) {
	first := codeindex.Function{ID: "main.go:F@2:1", File: "main.go", Name: "F"}
	second := codeindex.Function{ID: "main.go:F@3:1", File: "main.go", Name: "F"}
	otherFile := codeindex.Function{ID: "other.go:F@2:1", File: "other.go", Name: "F"}

	old := Explanation{File: "main.go", FunctionID: "main.go:F"}
	got, ok := findFunctionByHistoricalID([]codeindex.Function{first}, old)
	if !ok || got.ID != first.ID {
		t.Fatalf("unique legacy ID did not resolve: got=%+v matched=%t", got, ok)
	}

	if _, ok := findFunctionByHistoricalID([]codeindex.Function{first, second}, old); ok {
		t.Fatal("ambiguous legacy ID was assigned to one of multiple same-file functions")
	}

	exact := Explanation{File: "main.go", FunctionID: second.ID}
	got, ok = findFunctionByHistoricalID([]codeindex.Function{first, second}, exact)
	if !ok || got.ID != second.ID {
		t.Fatalf("exact function ID did not take priority: got=%+v matched=%t", got, ok)
	}

	if _, ok := findFunctionByHistoricalID([]codeindex.Function{otherFile}, old); ok {
		t.Fatal("legacy ID matched a function from another file")
	}
}

func TestInspectDoesNotGuessAmbiguousLegacyHistory(t *testing.T) {
	a, root := fixture(t)
	put(t, root, "main.go", "package p\nfunc F() {}\nfunc F() {}\n")
	idx, err := a.Index()
	if err != nil {
		t.Fatal(err)
	}
	var first, second codeindex.Function
	for _, function := range idx.Functions {
		if function.File == "main.go" && function.Name == "F" {
			if first.ID == "" {
				first = function
			} else {
				second = function
			}
		}
	}
	if first.ID == "" || second.ID == "" || first.ID == second.ID {
		t.Fatalf("fixture did not produce distinct same-name IDs: %+v", idx.Functions)
	}
	old := Explanation{ID: newID(), File: "main.go", FunctionID: "main.go:F", Created: "2026-11-20T00:00:00Z"}
	if err := a.write("explanation-"+old.ID+".json", old); err != nil {
		t.Fatal(err)
	}
	exact := Explanation{ID: newID(), File: "main.go", FunctionID: first.ID, Created: "2026-11-21T00:00:00Z"}
	if err := a.write("explanation-"+exact.ID+".json", exact); err != nil {
		t.Fatal(err)
	}

	info, err := a.Inspect("main.go", first.StartLine)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.History) != 1 || info.History[0].ID != exact.ID {
		t.Fatalf("ambiguous legacy record should be hidden while exact ID remains visible: %+v", info.History)
	}
}
