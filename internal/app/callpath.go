package app

import (
	"errors"
	"fmt"
	"intentlens/internal/aggregator"
	"intentlens/internal/capture"
	"intentlens/internal/codeindex"
	"sort"
	"strings"
)

const (
	maxCallPathFunctions = 200
	maxCallPathCalls     = 1000
)

type PathCall struct {
	CallerID string   `json:"caller_id"`
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Targets  []string `json:"targets"`
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Column   int      `json:"column"`
	Source   string   `json:"source"`
}

type CallPath struct {
	SelectedID string                       `json:"selected_id"`
	RootIDs    []string                     `json:"root_ids"`
	Functions  []codeindex.Function         `json:"functions"`
	Calls      []PathCall                   `json:"calls"`
	Coverage   codeindex.Coverage           `json:"coverage"`
	Issues     []string                     `json:"issues"`
	Truncated  bool                         `json:"truncated"`
	Aggregated *aggregator.AggregatedResult `json:"aggregated,omitempty"`
}

// CallPath returns ancestors(selected) UNION descendants(selected) through
// resolved direct static edges, not the larger weakly connected component.
// Calls in returned functions are preserved even when their target is outside
// this focus graph. Neither runtime ordering nor dynamic edges are inferred.
func (a *App) CallPath(file string, line int) (CallPath, error) {
	out := CallPath{RootIDs: []string{}, Functions: []codeindex.Function{}, Calls: []PathCall{}, Issues: []string{}}
	if err := a.scrubExcluded(); err != nil {
		return out, err
	}
	file = strings.ReplaceAll(file, "\\", "/")
	allowed, err := capture.Allowed(a.Root, file)
	if err != nil {
		return out, err
	}
	if !allowed {
		return out, errors.New("相关上下文已排除：默认排除规则或 .intentignore 限制了索引范围；请查看索引范围与遗漏原因")
	}
	idx, snap, parseOmissions, err := a.buildIndex()
	if err != nil {
		return out, err
	}
	out.Coverage = idx.Coverage
	// Reuse the index's bounded aggregation of parse/type-check restrictions.
	// Call-path scope and traversal notices remain in Issues.
	out.Aggregated = idx.Aggregated
	out.Issues = append(out.Issues, idx.Issues...)
	out.Issues = append(out.Issues, "仅展示同目录同包内可唯一解析的直接静态调用；源头是索引范围内的候选入口，不代表运行时入口或执行顺序。跨包、接口和动态调用可能未知；捕获完整不等于调用解析完整。")
	byID := make(map[string]codeindex.Function, len(idx.Functions))
	for _, f := range idx.Functions {
		byID[f.ID] = f
	}
	selected := ""
	for _, f := range idx.Functions {
		if f.File == file && line >= f.StartLine && line <= f.EndLine {
			if selected == "" || f.StartLine > byID[selected].StartLine {
				selected = f.ID
			}
		}
	}
	if selected == "" {
		if a.pathAbsent(file) {
			return out, errors.New("源码文件不存在，无法定位当前函数")
		}
		if reason, ok := parseOmissions[file]; ok {
			return out, indexOmissionError(reason)
		}
		if reason, ok := idx.Coverage.OmissionReasons[file]; ok {
			return out, indexOmissionError(reason)
		}
		if _, ok := snap.Files[file]; ok {
			return out, errors.New("当前位置没有可解析的 Go 函数")
		}
		return out, errors.New("当前位置没有可解析的 Go 函数")
	}
	out.SelectedID = selected
	incoming := map[string][]string{}
	outgoing := map[string][]string{}
	for _, f := range idx.Functions {
		for _, c := range f.Calls {
			if c.Status != "resolved" || len(c.Targets) != 1 {
				continue
			}
			t := c.Targets[0]
			if _, ok := byID[t]; !ok {
				continue
			}
			outgoing[f.ID] = append(outgoing[f.ID], t)
			incoming[t] = append(incoming[t], f.ID)
		}
	}
	seen := map[string]bool{selected: true}
	// Each traversal starts at selected. Switching direction at an ancestor or a
	// descendant would pull unrelated siblings/callers into the requested path.
	walk := func(adjacency map[string][]string) {
		visited := map[string]bool{selected: true}
		queue := []string{selected}
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			neighbors := append([]string{}, adjacency[id]...)
			sort.Strings(neighbors)
			for _, next := range neighbors {
				if visited[next] {
					continue
				}
				visited[next] = true
				if !seen[next] && len(seen) >= maxCallPathFunctions {
					out.Truncated = true
					continue
				}
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	walk(incoming)
	walk(outgoing)
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		out.Functions = append(out.Functions, byID[id])
	}
	for _, id := range ids {
		for _, c := range byID[id].Calls {
			if len(out.Calls) >= maxCallPathCalls {
				out.Truncated = true
				break
			}
			out.Calls = append(out.Calls, PathCall{CallerID: id, Name: c.Name, Status: c.Status, Targets: append([]string{}, c.Targets...), File: c.File, Line: c.Line, Column: c.Column, Source: c.Source})
		}
	}
	if out.Truncated {
		out.Issues = append(out.Issues, fmt.Sprintf("调用路径达到有界上限（函数最多 %d、调用点最多 %d），结果已截断", maxCallPathFunctions, maxCallPathCalls))
	}
	var rootIssues []string
	out.RootIDs, rootIssues = callPathRoots(ids, seen, outgoing, incoming)
	out.Issues = append(out.Issues, rootIssues...)
	sort.Strings(out.Issues)
	return out, nil
}

// Source SCCs of the induced focus graph cover every branch, including cycles
// that feed a node also reachable from an ordinary root. A lexicographic scan
// of uncovered nodes is insufficient: it may pick a downstream non-cycle node.
func callPathRoots(ids []string, seen map[string]bool, outgoing, incoming map[string][]string) ([]string, []string) {
	nextIndex := 0
	index, low := map[string]int{}, map[string]int{}
	onStack := map[string]bool{}
	stack := []string{}
	components := [][]string{}
	componentOf := map[string]int{}
	var visit func(string)
	visit = func(id string) {
		nextIndex++
		index[id], low[id] = nextIndex, nextIndex
		stack = append(stack, id)
		onStack[id] = true
		neighbors := append([]string{}, outgoing[id]...)
		sort.Strings(neighbors)
		for _, next := range neighbors {
			if !seen[next] {
				continue
			}
			if index[next] == 0 {
				visit(next)
				if low[next] < low[id] {
					low[id] = low[next]
				}
			} else if onStack[next] && index[next] < low[id] {
				low[id] = index[next]
			}
		}
		if low[id] == index[id] {
			component := []string{}
			for {
				last := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[last] = false
				componentOf[last] = len(components)
				component = append(component, last)
				if last == id {
					break
				}
			}
			sort.Strings(component)
			components = append(components, component)
		}
	}
	for _, id := range ids {
		if index[id] == 0 {
			visit(id)
		}
	}
	roots, issues := []string{}, []string{}
	for componentID, component := range components {
		hasIncoming, cutIncoming := false, false
		cyclic := len(component) > 1
		for _, id := range component {
			for _, previous := range incoming[id] {
				if !seen[previous] {
					cutIncoming = true
				} else if componentOf[previous] != componentID {
					hasIncoming = true
				}
			}
			for _, next := range outgoing[id] {
				if next == id {
					cyclic = true
				}
			}
		}
		if hasIncoming {
			continue
		}
		representative := component[0]
		roots = append(roots, representative)
		if cyclic {
			issues = append(issues, "root_ids 中的 "+representative+" 是静态源循环组件的确定性代表，不是实际入口源")
		}
		if cutIncoming {
			issues = append(issues, "root_ids 中的 "+representative+" 仍有未纳入的上游调用；这是截断边界，不是已证明的源头")
		}
	}
	sort.Strings(roots)
	return roots, issues
}
