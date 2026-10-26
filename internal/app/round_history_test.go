package app

import (
	"fmt"
	"intentlens/internal/capture"
	"intentlens/internal/explain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoundHistoryBatchPreservesLatestCorrectionsAndDiagnostics(t *testing.T) {
	a, root := fixture(t)
	source := "package p\nfunc F() {}\n"
	put(t, root, "main.go", source)
	ids := []string{fmt.Sprintf("%024x", 1), fmt.Sprintf("%024x", 2)}
	newest := map[string]string{}
	for i, id := range ids {
		r := Round{ID: id, Started: fmt.Sprint(i), Changes: []capture.Change{{Path: "main.go", After: ptrFile(source)}}}
		if err := a.write("round-"+id+".json", r); err != nil {
			t.Fatal(err)
		}
		for version := 0; version < 2; version++ {
			eid := fmt.Sprintf("%024x", 10+i*2+version)
			e := Explanation{ID: eid, FunctionID: "round:" + id, Created: fmt.Sprint(version), Evidence: []explain.Evidence{{File: "main.go", Role: "after", Source: source, Hash: hash(source)}}, Document: explain.Document{Claims: []explain.Claim{{ID: "c1", Status: "supported"}}}}
			if err := a.write("explanation-"+eid+".json", e); err != nil {
				t.Fatal(err)
			}
			if version == 1 {
				newest[id] = eid
			}
		}
	}
	for i, cid := range []string{fmt.Sprintf("%024x", 22), fmt.Sprintf("%024x", 21)} {
		if err := a.write("correction-"+cid+".json", Correction{ID: cid, ExplanationID: newest[ids[0]], ClaimID: "c1", Reason: fmt.Sprint(i)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.write("analysis-diagnostic.json", map[string]string{"round": ids[0], "error": "fixture failure"}); err != nil {
		t.Fatal(err)
	}
	check := func(fresh bool) {
		t.Helper()
		rounds, err := a.List()
		if err != nil || len(rounds) != 2 {
			t.Fatalf("history: %+v %v", rounds, err)
		}
		if rounds[0].ID != ids[1] {
			t.Fatal("history ordering changed")
		}
		for _, r := range rounds {
			if r.Analysis == nil || r.Analysis.ID != newest[r.ID] {
				t.Fatalf("latest explanation lost: %+v", r)
			}
			loaded, err := a.ExplanationByID(r.Analysis.ID)
			if err != nil || loaded.Freshness != r.Analysis.Freshness || (loaded.Freshness == "已引用证据未变") != fresh {
				t.Fatalf("freshness: %+v %v", r.Analysis, err)
			}
			if r.ID == ids[0] {
				if len(r.Analysis.Corrections) != 2 || r.Analysis.Corrections[0].ID >= r.Analysis.Corrections[1].ID || r.Analysis.Document.Claims[0].Status != "flagged" {
					t.Fatal("correction join/sort lost")
				}
				if len(r.Notes) != 1 || !strings.Contains(r.Notes[0], "fixture failure") {
					t.Fatal("diagnostic lost or duplicated")
				}
			} else if len(r.Analysis.Corrections) != 0 || len(r.Notes) != 0 || r.Analysis.Document.Claims[0].Status != "supported" {
				t.Fatal("cross-round correction/diagnostic contamination")
			}
		}
	}
	check(true)
	check(true)
	put(t, root, "main.go", source+"// changed\n")
	check(false)
	put(t, root, ".intentignore", "main.go\n")
	rounds, err := a.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rounds {
		if r.Analysis != nil || len(r.Changes) != 0 {
			t.Fatal("revoked history leaked after prior query")
		}
	}
}

func ptrFile(source string) *capture.File { f := capture.NewFile(source); return &f }

func TestRoundHistoryDoesNotCacheUnreadableExplanation(t *testing.T) {
	a, root := fixture(t)
	e := seedExplanation(t, a, root)
	id := newID()
	e.FunctionID = "round:" + id
	if err := a.write("round-"+id+".json", Round{ID: id}); err != nil {
		t.Fatal(err)
	}
	if err := a.write("explanation-"+e.ID+".json", e); err != nil {
		t.Fatal(err)
	}
	if _, err := a.List(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.State, "explanation-"+e.ID+".json"), []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.List(); err == nil {
		t.Fatal("unreadable evidence replaced by cached result")
	}
}
