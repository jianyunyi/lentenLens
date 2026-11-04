// Package codeindex creates a conservative static Go call index without builds.
package codeindex

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"path"
	"sort"
	"strings"
)

type Function struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Source    string `json:"source"`
	Calls     []Call `json:"calls"`
}
type Call struct {
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Targets []string `json:"targets"`
	File    string   `json:"file"`
	Line    int      `json:"line"`
	Column  int      `json:"column"`
	Source  string   `json:"source"`
}
type Index struct {
	Functions []Function `json:"functions"`
	Issues    []string   `json:"issues"`
	Coverage  Coverage   `json:"coverage"`
}
type CoverageOmission struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}
type Coverage struct {
	Complete        bool               `json:"complete"`
	Scope           string             `json:"scope"`
	BudgetBytes     int                `json:"budget_bytes"`
	CapturedBytes   int                `json:"captured_bytes"`
	OmittedCount    int                `json:"omitted_count"`
	ExcludedCount   int                `json:"excluded_count"`
	Omitted         []CoverageOmission `json:"omitted"`
	Truncated       bool               `json:"truncated"`
	OmissionReasons map[string]string  `json:"-"`
}
type unit struct {
	tree   *ast.File
	name   string
	source string
}

func Build(files map[string]string) Index {
	out, _ := BuildDetailed(files)
	return out
}

// BuildDetailed is the indexing form used by the app coverage layer. The
// returned maps are deliberately not part of Index's JSON representation.
func BuildDetailed(files map[string]string) (Index, map[string]string) {
	out := Index{Functions: []Function{}, Issues: []string{}}
	omitted := map[string]string{}
	set := token.NewFileSet()
	names := make([]string, 0, len(files))
	for name := range files {
		if strings.HasSuffix(name, ".go") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	groups := map[string][]unit{}
	for _, name := range names {
		tree, e := parser.ParseFile(set, name, files[name], parser.AllErrors)
		if e != nil {
			out.Issues = append(out.Issues, fmt.Sprintf("%s: parse failed: %v", name, e))
			omitted[name] = "parse_error"
			continue
		}
		key := path.Dir(strings.ReplaceAll(name, "\\", "/")) + ":" + tree.Name.Name
		groups[key] = append(groups[key], unit{tree: tree, name: name, source: files[name]})
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		units := groups[key]
		trees := make([]*ast.File, 0, len(units))
		for _, u := range units {
			trees = append(trees, u.tree)
		}
		info := &types.Info{Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}, Instances: map[*ast.Ident]types.Instance{}}
		// No importer is supplied: dependencies are neither fetched nor executed.
		cfg := types.Config{Error: func(err error) { out.Issues = append(out.Issues, "type check: "+err.Error()) }}
		if _, err := cfg.Check(key, set, trees, info); err != nil {
			out.Issues = append(out.Issues, "type check failed: "+err.Error())
		}
		targets := map[types.Object]string{}
		declarations := map[string][]string{}
		for _, u := range units {
			for _, d := range u.tree.Decls {
				if f, ok := d.(*ast.FuncDecl); ok && f.Recv == nil {
					declarations[f.Name.Name] = append(declarations[f.Name.Name], u.name+":"+f.Name.Name)
					if obj := info.Defs[f.Name]; obj != nil {
						targets[obj] = u.name + ":" + f.Name.Name
					}
				}
			}
		}
		for _, u := range units {
			for _, d := range u.tree.Decls {
				f, ok := d.(*ast.FuncDecl)
				if !ok {
					continue
				}
				name := f.Name.Name
				if f.Recv != nil && len(f.Recv.List) > 0 {
					var b bytes.Buffer
					if err := format.Node(&b, set, f.Recv.List[0].Type); err != nil {
						out.Issues = append(out.Issues, fmt.Sprintf("%s: receiver formatting failed: %v", u.name, err))
						b.Reset()
						b.WriteString("<unformatted receiver>")
					}
					name = "(" + b.String() + ")." + name
				}
				start, end := set.PositionFor(f.Pos(), false), set.PositionFor(f.End(), false)
				fn := Function{ID: u.name + ":" + name, Name: name, File: u.name, StartLine: start.Line, EndLine: end.Line, Source: u.source[start.Offset:end.Offset], Calls: []Call{}}
				if f.Body != nil {
					ast.Inspect(f.Body, func(n ast.Node) bool {
						c, ok := n.(*ast.CallExpr)
						if !ok {
							return true
						}
						var b bytes.Buffer
						if err := format.Node(&b, set, c.Fun); err != nil {
							out.Issues = append(out.Issues, fmt.Sprintf("%s: call formatting failed: %v", u.name, err))
							b.Reset()
							b.WriteString("<unformatted call>")
						}
						pos, end := set.PositionFor(c.Pos(), false), set.PositionFor(c.End(), false)
						source := ""
						if pos.Offset >= 0 && end.Offset <= len(u.source) && pos.Offset <= end.Offset {
							source = u.source[pos.Offset:end.Offset]
						}
						call := Call{Name: b.String(), Status: "unresolved", Targets: []string{}, File: u.name, Line: pos.Line, Column: pos.Column, Source: source}
						if id := directCallIdent(c.Fun, info); id != nil {
							obj := info.Uses[id]
							target, exists := targets[obj]
							// Type checking may pick one redeclaration arbitrarily, for example
							// when platform variants are indexed together. Retain all candidates.
							if candidates := declarations[id.Name]; len(candidates) > 1 && (exists || obj == nil) {
								call.Status = "ambiguous"
								call.Targets = append([]string{}, candidates...)
							} else if exists {
								call.Status = "resolved"
								call.Targets = []string{target}
							}
						}
						fn.Calls = append(fn.Calls, call)
						return true
					})
				}
				out.Functions = append(out.Functions, fn)
			}
		}
	}
	sort.Slice(out.Functions, func(i, j int) bool {
		a, b := out.Functions[i], out.Functions[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.StartLine < b.StartLine
	})
	sort.Strings(out.Issues)
	return out, omitted
}

// directCallIdent unwraps syntax only when it still denotes a direct call.
// IndexExpr can also mean a slice/map lookup returning a function value, so
// explicit type arguments require a confirmed go/types instantiation.
func directCallIdent(expr ast.Expr, info *types.Info) *ast.Ident {
	expr = ast.Unparen(expr)
	var base ast.Expr
	switch x := expr.(type) {
	case *ast.Ident:
		return x
	case *ast.IndexExpr:
		base = x.X
	case *ast.IndexListExpr:
		base = x.X
	default:
		return nil
	}
	id, ok := ast.Unparen(base).(*ast.Ident)
	if !ok {
		return nil
	}
	if _, ok := info.Instances[id]; !ok {
		return nil
	}
	if _, ok := info.Uses[id].(*types.Func); !ok {
		return nil
	}
	return id
}
