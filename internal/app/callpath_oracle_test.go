package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type oracleCall struct {
	Caller, Name, Status string
	Targets              []string
	File                 string
	Line, Column         int
	Source               string
}
type oracleFixture struct {
	name, file string
	line       int
	files      map[string]string
	selected   string
	functions  []string
	roots      []string
	calls      []oracleCall
	facts      []bool
}
type oracleMetric struct {
	Numerator   int     `json:"numerator"`
	Denominator int     `json:"denominator"`
	Percent     float64 `json:"percent"`
}
type oracleMetrics struct {
	Version                string       `json:"version"`
	FixtureCount           int          `json:"fixture_count"`
	GraphEdgePrecision     oracleMetric `json:"graph_edge_precision"`
	GraphEdgeRecall        oracleMetric `json:"graph_edge_recall"`
	GraphEdgeTruePositive  int          `json:"graph_edge_true_positive"`
	GraphEdgeFalsePositive int          `json:"graph_edge_false_positive"`
	GraphEdgeFalseNegative int          `json:"graph_edge_false_negative"`
	CallsiteTargetAccuracy oracleMetric `json:"callsite_target_accuracy"`
	RootExact              oracleMetric `json:"root_exact"`
	PathExact              oracleMetric `json:"path_exact"`
	EvidenceExact          oracleMetric `json:"evidence_exact"`
	SourceFactAnswers      oracleMetric `json:"source_fact_answers"`
	UnknownAbstention      oracleMetric `json:"unknown_abstention"`
	ModelAnswerAccuracy    string       `json:"model_answer_accuracy"`
	Scope                  string       `json:"scope"`
}

func oc(caller, name, status string, targets []string, file string, line, column int, source string) oracleCall {
	return oracleCall{caller, name, status, targets, file, line, column, source}
}
func resolved(caller, name, target, file string, line, col int, source string) oracleCall {
	return oc(caller, name, "resolved", []string{target}, file, line, col, source)
}
func unknown(caller, name, status, file string, line, col int, source string) oracleCall {
	return oc(caller, name, status, []string{}, file, line, col, source)
}

// TestCallPathOracleBenchmark is model-free: every expected ID, edge, status,
// and physical evidence value below is authored independently of App.CallPath.
func TestCallPathOracleBenchmark(t *testing.T) {
	fixtures := oracleFixtures()
	m := oracleMetrics{Version: "callpath-oracle/v2", FixtureCount: len(fixtures), ModelAnswerAccuracy: "not_measured", Scope: "known synthetic Go fixtures only; source facts are deterministic graph queries, not model accuracy"}
	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			a, root := fixture(t)
			for name, body := range fx.files {
				put(t, root, name, body)
			}
			got, err := a.CallPath(fx.file, fx.line)
			if err != nil {
				t.Fatal(err)
			}
			checkFixture(t, fx, got, &m)
		})
	}
	if p := os.Getenv("CALLPATH_ORACLE_JSON"); p != "" {
		m.GraphEdgePrecision.Percent = pct(m.GraphEdgePrecision.Numerator, m.GraphEdgePrecision.Denominator)
		m.GraphEdgeRecall.Percent = pct(m.GraphEdgeRecall.Numerator, m.GraphEdgeRecall.Denominator)
		m.CallsiteTargetAccuracy.Percent = pct(m.CallsiteTargetAccuracy.Numerator, m.CallsiteTargetAccuracy.Denominator)
		m.RootExact.Percent = pct(m.RootExact.Numerator, m.RootExact.Denominator)
		m.PathExact.Percent = pct(m.PathExact.Numerator, m.PathExact.Denominator)
		m.EvidenceExact.Percent = pct(m.EvidenceExact.Numerator, m.EvidenceExact.Denominator)
		m.SourceFactAnswers.Percent = pct(m.SourceFactAnswers.Numerator, m.SourceFactAnswers.Denominator)
		m.UnknownAbstention.Percent = pct(m.UnknownAbstention.Numerator, m.UnknownAbstention.Denominator)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		b, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, append(b, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) * 100 / float64(d)
}
func metric(n, d int) oracleMetric { return oracleMetric{n, d, pct(n, d)} }
func checkFixture(t *testing.T, fx oracleFixture, got CallPath, m *oracleMetrics) {
	if got.SelectedID != fx.selected {
		t.Fatalf("selected=%q want %q", got.SelectedID, fx.selected)
	}
	wantFns := append([]string{}, fx.functions...)
	sort.Strings(wantFns)
	gotFns := make([]string, len(got.Functions))
	for i, f := range got.Functions {
		gotFns[i] = f.ID
	}
	sort.Strings(gotFns)
	pathOK := reflect.DeepEqual(gotFns, wantFns)
	m.PathExact.Denominator++
	if pathOK {
		m.PathExact.Numerator++
	} else {
		t.Errorf("functions=%v want %v", gotFns, wantFns)
	}
	wantRoots := append([]string{}, fx.roots...)
	sort.Strings(wantRoots)
	rootsOK := reflect.DeepEqual(got.RootIDs, wantRoots)
	m.RootExact.Denominator++
	if rootsOK {
		m.RootExact.Numerator++
	} else {
		t.Errorf("roots=%v want %v", got.RootIDs, wantRoots)
	}
	wantCalls := map[string]oracleCall{}
	for _, c := range fx.calls {
		wantCalls[c.Caller+"|"+c.Name+"|"+fmt.Sprint(c.Line)+"|"+c.Source] = c
	}
	gotCalls := map[string]PathCall{}
	for _, c := range got.Calls {
		gotCalls[c.CallerID+"|"+c.Name+"|"+fmt.Sprint(c.Line)+"|"+c.Source] = c
	}
	if len(gotCalls) != len(wantCalls) {
		t.Errorf("call count=%d want %d", len(gotCalls), len(wantCalls))
	}
	for key, w := range wantCalls {
		g, ok := gotCalls[key]
		evidenceOK := ok && g.CallerID == w.Caller && g.Name == w.Name && g.Status == w.Status && reflect.DeepEqual(g.Targets, w.Targets) && g.File == w.File && g.Line == w.Line && g.Column == w.Column && g.Source == w.Source
		m.EvidenceExact.Denominator++
		if evidenceOK {
			m.EvidenceExact.Numerator++
		} else {
			t.Errorf("call %s got=%+v want=%+v", key, g, w)
		}
		if w.Status == "resolved" {
			m.CallsiteTargetAccuracy.Denominator++
			if evidenceOK {
				m.CallsiteTargetAccuracy.Numerator++
			}
		} else if w.Status == "unresolved" {
			m.UnknownAbstention.Denominator++
			if evidenceOK && g.Status == w.Status && len(g.Targets) == 0 {
				m.UnknownAbstention.Numerator++
			}
		}
	}
	wantEdges, gotEdges := map[string]bool{}, map[string]bool{}
	for _, c := range fx.calls {
		if c.Status == "resolved" {
			for _, target := range c.Targets {
				if contains(wantFns, target) {
					wantEdges[c.Caller+"->"+target] = true
				}
			}
		}
	}
	for _, c := range got.Calls {
		if c.Status == "resolved" {
			for _, target := range c.Targets {
				if contains(gotFns, c.CallerID) && contains(gotFns, target) {
					gotEdges[c.CallerID+"->"+target] = true
				}
			}
		}
	}
	tp, fp, fn := 0, 0, 0
	for edge := range gotEdges {
		if wantEdges[edge] {
			tp++
		} else {
			fp++
			t.Errorf("false graph edge: %s", edge)
		}
	}
	for edge := range wantEdges {
		if !gotEdges[edge] {
			fn++
			t.Errorf("missing graph edge: %s", edge)
		}
	}
	m.GraphEdgeTruePositive += tp
	m.GraphEdgeFalsePositive += fp
	m.GraphEdgeFalseNegative += fn
	m.GraphEdgePrecision.Numerator += tp
	m.GraphEdgePrecision.Denominator += tp + fp
	m.GraphEdgeRecall.Numerator += tp
	m.GraphEdgeRecall.Denominator += tp + fn
	for i, want := range fx.facts {
		gotFact := sourceFact(got, i, fx)
		m.SourceFactAnswers.Denominator++
		if gotFact == want {
			m.SourceFactAnswers.Numerator++
		} else {
			t.Errorf("source fact %d=%v want %v", i, gotFact, want)
		}
	}
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func sourceFact(g CallPath, n int, fx oracleFixture) bool {
	switch n {
	case 0:
		return g.SelectedID == fx.selected
	case 1:
		return contains(g.RootIDs, fx.roots[0])
	case 2:
		for _, f := range g.Functions {
			if f.ID == fx.selected {
				return len(f.Calls) == countCaller(fx.calls, fx.selected)
			}
		}
		return false
	case 3:
		return len(g.RootIDs) == len(fx.roots)
	}
	return false
}
func countCaller(cs []oracleCall, id string) int {
	n := 0
	for _, c := range cs {
		if c.Caller == id {
			n++
		}
	}
	return n
}

func oracleFixtures() []oracleFixture {
	return []oracleFixture{
		{name: "chain-multifile", file: "leaf.go", line: 2, selected: "leaf.go:B", functions: []string{"main.go:Root", "mid.go:A", "leaf.go:B"}, roots: []string{"main.go:Root"}, files: map[string]string{"main.go": "package p\nfunc Root(){ A() }\n", "mid.go": "package p\nfunc A(){ B() }\n", "leaf.go": "package p\nfunc B(){}\n"}, calls: []oracleCall{resolved("main.go:Root", "A", "mid.go:A", "main.go", 2, 14, "A()"), resolved("mid.go:A", "B", "leaf.go:B", "mid.go", 2, 11, "B()")}, facts: []bool{true, true, true, true}},
		{name: "diamond-multiroot", file: "shared.go", line: 2, selected: "shared.go:Shared", functions: []string{"a.go:One", "b.go:Two", "shared.go:Shared"}, roots: []string{"a.go:One", "b.go:Two"}, files: map[string]string{"a.go": "package p\nfunc One(){ Shared() }\n", "b.go": "package p\nfunc Two(){ Shared() }\n", "shared.go": "package p\nfunc Shared(){}\n"}, calls: []oracleCall{resolved("a.go:One", "Shared", "shared.go:Shared", "a.go", 2, 13, "Shared()"), resolved("b.go:Two", "Shared", "shared.go:Shared", "b.go", 2, 13, "Shared()")}, facts: []bool{true, true, true, true}},
		{name: "disconnected-sibling", file: "a.go", line: 2, selected: "a.go:A", functions: []string{"a.go:A", "leaf.go:L"}, roots: []string{"a.go:A"}, files: map[string]string{"a.go": "package p\nfunc A(){ L() }\n", "leaf.go": "package p\nfunc L(){}\n", "other.go": "package p\nfunc Other(){}\n"}, calls: []oracleCall{resolved("a.go:A", "L", "leaf.go:L", "a.go", 2, 11, "L()")}, facts: []bool{true, true, true, true}},
		{name: "shared-descendant-other-caller", file: "root.go", line: 2, selected: "root.go:Root", functions: []string{"root.go:Root", "shared.go:Shared"}, roots: []string{"root.go:Root"}, files: map[string]string{"root.go": "package p\nfunc Root(){ Shared() }\n", "other.go": "package p\nfunc Other(){ Shared() }\n", "shared.go": "package p\nfunc Shared(){}\n"}, calls: []oracleCall{resolved("root.go:Root", "Shared", "shared.go:Shared", "root.go", 2, 14, "Shared()")}, facts: []bool{true, true, true, true}},
		{name: "direct-generic", file: "generic.go", line: 2, selected: "generic.go:Use", functions: []string{"generic.go:Use", "generic.go:Helper"}, roots: []string{"generic.go:Use"}, files: map[string]string{"generic.go": "package p\nfunc Use(){ Helper[int](1) }\nfunc Helper[T any](T){}\n"}, calls: []oracleCall{resolved("generic.go:Use", "Helper[int]", "generic.go:Helper", "generic.go", 2, 13, "Helper[int](1)")}, facts: []bool{true, true, true, true}},
		{name: "shadowed-function-var", file: "shadow.go", line: 3, selected: "shadow.go:Use", functions: []string{"shadow.go:Use"}, roots: []string{"shadow.go:Use"}, files: map[string]string{"shadow.go": "package p\nfunc Helper(){}\nfunc Use(){ Helper := func(){}; Helper() }\n"}, calls: []oracleCall{unknown("shadow.go:Use", "Helper", "unresolved", "shadow.go", 3, 33, "Helper()")}, facts: []bool{true, true, true, true}},
		{name: "ambiguous-duplicate", file: "dup1.go", line: 2, selected: "dup1.go:Use", functions: []string{"dup1.go:Use"}, roots: []string{"dup1.go:Use"}, files: map[string]string{"dup1.go": "package p\nfunc Use(){ Helper() }\nfunc Helper(){}\n", "dup2.go": "package p\nfunc Helper(){}\n"}, calls: []oracleCall{oc("dup1.go:Use", "Helper", "ambiguous", []string{"dup1.go:Helper", "dup2.go:Helper"}, "dup1.go", 2, 13, "Helper()")}, facts: []bool{true, true, true, true}},
		{name: "real-plus-cycle", file: "entry.go", line: 2, selected: "entry.go:Entry", functions: []string{"a.go:A", "b.go:B", "entry.go:Entry", "real.go:Real"}, roots: []string{"real.go:Real"}, files: map[string]string{"entry.go": "package p\nfunc Entry(){ A() }\n", "a.go": "package p\nfunc A(){ B() }\n", "b.go": "package p\nfunc B(){ A() }\n", "real.go": "package p\nfunc Real(){ Entry() }\n"}, calls: []oracleCall{resolved("entry.go:Entry", "A", "a.go:A", "entry.go", 2, 15, "A()"), resolved("a.go:A", "B", "b.go:B", "a.go", 2, 11, "B()"), resolved("b.go:B", "A", "a.go:A", "b.go", 2, 11, "A()"), resolved("real.go:Real", "Entry", "entry.go:Entry", "real.go", 2, 14, "Entry()")}, facts: []bool{true, true, true, true}},
		{name: "cross-package-selector", file: "main.go", line: 2, selected: "main.go:Main", functions: []string{"main.go:Main"}, roots: []string{"main.go:Main"}, files: map[string]string{"main.go": "package p\nfunc Main(){ ext.Do() }\n"}, calls: []oracleCall{unknown("main.go:Main", "ext.Do", "unresolved", "main.go", 2, 14, "ext.Do()")}, facts: []bool{true, true, true, true}},
		{name: "line-directive-physical", file: "line.go", line: 3, selected: "line.go:Physical", functions: []string{"line.go:Physical"}, roots: []string{"line.go:Physical"}, files: map[string]string{"line.go": "package p\n//line virtual.go:99\nfunc Physical(){ Missing() }\n"}, calls: []oracleCall{unknown("line.go:Physical", "Missing", "unresolved", "line.go", 3, 18, "Missing()")}, facts: []bool{true, true, true, true}},
	}
}
