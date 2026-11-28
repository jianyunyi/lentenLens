package app

import (
	"errors"
	"intentlens/internal/capture"
	"intentlens/internal/codeindex"
	"intentlens/internal/explain"
	"os"
	"strings"
)

// CachedIndex is exclusively for background consumers. Missing/stale returns
// empty functions and a refresh hint, never starts a source scan.
type CachedIndex struct {
	codeindex.Index
	Prescan PrescanState `json:"prescan"`
}

func (a *App) PrescanIndex() (CachedIndex, error) {
	r, err := a.CachedPrescan()
	if err != nil {
		return CachedIndex{}, err
	}
	return CachedIndex{r.Go, r.Prescan}, nil
}

type CachedInspection struct {
	Inspection
	Prescan PrescanState `json:"prescan"`
}

// PrescanInspect is a snapshot lookup, not a replacement for Inspect, CallPath
// or history freshness. It never labels saved explanations as current.
func (a *App) PrescanInspect(file string, line int) (CachedInspection, error) {
	out := CachedInspection{Inspection: Inspection{Dependencies: []codeindex.Function{}, Evidence: []explain.Evidence{}, Issues: []string{}, History: []Explanation{}}}
	if !validPrescanPath(file) || line < 1 {
		return out, errors.New("无效快照位置")
	}
	ok, err := capture.Allowed(a.Root, file)
	if err != nil {
		return out, err
	}
	if !ok {
		return out, errors.New("相关上下文已排除")
	}
	r, err := a.CachedPrescan()
	if err != nil {
		return out, err
	}
	out.Prescan = r.Prescan
	if r.Prescan.State != "unknown" && r.Prescan.State != "hit" {
		return out, nil
	}
	out.Coverage = r.Go.Coverage
	out.Issues = append(out.Issues, r.Prescan.Message)
	for _, f := range r.Go.Functions {
		if f.File == file && line >= f.StartLine && line <= f.EndLine {
			out.Function = f
			entries, err := os.ReadDir(a.State)
			if err != nil {
				return out, err
			}
			for _, entry := range entries {
				if !strings.HasPrefix(entry.Name(), "explanation-") {
					continue
				}
				var saved Explanation
				if err := a.read(entry.Name(), &saved); err != nil {
					return out, err
				}
				historical, matches := findFunctionByHistoricalID(r.Go.Functions, saved)
				if !matches || historical.ID != f.ID {
					continue
				}
				allowed := true
				for _, e := range saved.Evidence {
					ok, err := a.evidenceAllowed(e)
					if err != nil {
						return out, err
					}
					if !ok {
						allowed = false
						break
					}
				}
				if allowed {
					saved.Freshness = "基于预扫快照；历史证据当前性未验证"
					if err := a.attachCorrections(&saved); err != nil {
						return out, err
					}
					out.History = append(out.History, saved)
				}
			}
			return out, nil
		}
	}
	out.Issues = append(out.Issues, "预扫快照中当前位置没有可解析函数；请显式刷新")
	return out, nil
}
