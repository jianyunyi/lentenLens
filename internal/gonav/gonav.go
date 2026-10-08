// Package gonav provides conservative, source-only Go navigation. It never
// downloads modules, executes code, or treats a dynamic call as a unique target.
package gonav

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path"
	"sort"
	"strings"
)

type Target struct {
	File      string `json:"file"`
	Name      string `json:"name"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type Call struct {
	Name    string   `json:"name"`
	Line    int      `json:"line"`
	Status  string   `json:"status"` // resolved, candidates, or unknown
	Targets []Target `json:"targets,omitempty"`
}

type Function struct {
	Target
	Calls []Call `json:"calls"`
}

type Diagnostic struct {
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type Result struct {
	Functions   []Function   `json:"functions"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Analyze accepts already-filtered source files; callers must enforce exclusion
// rules before supplying content. Packages are grouped by directory AND name.
// All supplied files participate: build tags are not evaluated. Selector calls,
// imported calls, function variables and unresolved calls remain unknown.
func Analyze(files map[string]string) Result {
	r := Result{Functions: []Function{}, Diagnostics: []Diagnostic{}}
	fset := token.NewFileSet()
	groups := map[string][]*ast.File{}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		f, err := parser.ParseFile(fset, name, files[name], parser.AllErrors)
		if err != nil {
			r.Diagnostics = append(r.Diagnostics, Diagnostic{File: name, Kind: "syntax", Message: err.Error()})
			continue // Partial syntax trees cannot support reliable navigation.
		}
		key := path.Dir(strings.ReplaceAll(name, "\\", "/")) + "/" + f.Name.Name
		groups[key] = append(groups[key], f)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		group := groups[key]
		info := &types.Info{Uses: map[*ast.Ident]types.Object{}}
		cfg := types.Config{Error: func(err error) {
			d := Diagnostic{Kind: "type", Message: err.Error()}
			if e, ok := err.(types.Error); ok {
				p := fset.Position(e.Pos)
				d.File, d.Line = p.Filename, p.Line
			}
			r.Diagnostics = append(r.Diagnostics, d)
		}}
		// No importer: local facts remain useful even when imports are unavailable.
		_, _ = cfg.Check(key, fset, group, info)
		top := map[string][]Target{}
		positions := map[token.Pos]Target{}
		for _, f := range group {
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				t := target(fset, fn)
				if fn.Recv == nil {
					top[fn.Name.Name] = append(top[fn.Name.Name], t)
					positions[fn.Name.Pos()] = t
				}
			}
		}
		for _, f := range group {
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				entry := Function{Target: target(fset, fn), Calls: []Call{}}
				if fn.Body != nil {
					ast.Inspect(fn.Body, func(n ast.Node) bool {
						call, ok := n.(*ast.CallExpr)
						if !ok {
							return true
						}
						c := Call{Name: callName(call.Fun), Line: fset.Position(call.Pos()).Line, Status: "unknown"}
						if id, ok := unwrap(call.Fun).(*ast.Ident); ok {
							if obj, ok := info.Uses[id].(*types.Func); ok {
								if t, exists := positions[obj.Pos()]; exists {
									c.Targets = []Target{t}
									c.Status = "resolved"
									if len(top[id.Name]) > 1 {
										c.Targets = top[id.Name]
										c.Status = "candidates"
									}
								}
							}
						}
						entry.Calls = append(entry.Calls, c)
						return true
					})
				}
				r.Functions = append(r.Functions, entry)
			}
		}
	}
	sort.Slice(r.Functions, func(i, j int) bool {
		a, b := r.Functions[i], r.Functions[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.StartLine < b.StartLine
	})
	return r
}

func target(fs *token.FileSet, fn *ast.FuncDecl) Target {
	p := fs.Position(fn.Pos())
	return Target{File: p.Filename, Name: fn.Name.Name, StartLine: p.Line, EndLine: fs.Position(fn.End()).Line}
}

func unwrap(e ast.Expr) ast.Expr {
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		default:
			return e
		}
	}
}

func callName(e ast.Expr) string {
	switch x := unwrap(e).(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return callName(x.X) + "." + x.Sel.Name
	default:
		return "<expression>"
	}
}
