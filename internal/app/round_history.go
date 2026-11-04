package app

import (
	"errors"
	"intentlens/internal/capture"
	"os"
	"strings"
)

// attachRoundAnalyses joins history only within this List call. Rebuilding after
// scrubExcluded preserves revocations and source checks without a persistent cache.
func (a *App) attachRoundAnalyses(rounds []Round, entries []os.DirEntry) error {
	if len(rounds) == 0 {
		return nil
	}
	var diagnostic map[string]string
	if err := a.read("analysis-diagnostic.json", &diagnostic); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	wanted := make(map[string]bool, len(rounds))
	latest := make(map[string]Explanation, len(rounds))
	for i := range rounds {
		r := &rounds[i]
		wanted["round:"+r.ID] = true
		if diagnostic["round"] == r.ID {
			r.Notes = append(r.Notes, "最近一次轮次解释生成失败："+diagnostic["error"])
		}
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "explanation-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		var saved Explanation
		if err := a.read(name, &saved); err != nil {
			return err
		}
		if !wanted[saved.FunctionID] {
			continue
		}
		if old, ok := latest[saved.FunctionID]; !ok || saved.Created > old.Created {
			latest[saved.FunctionID] = saved
		}
	}
	corrections := map[string][]Correction{}
	for i := range rounds {
		r := &rounds[i]
		if saved, ok := latest["round:"+r.ID]; ok && (r.Analysis == nil || saved.Created > r.Analysis.Created) {
			savedCopy := saved
			r.Analysis = &savedCopy
		}
		if r.Analysis != nil {
			corrections[r.Analysis.ID] = nil
		}
	}
	if len(corrections) == 0 {
		return nil
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "correction-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		var c Correction
		if err := a.read(name, &c); err != nil {
			return err
		}
		if _, ok := corrections[c.ExplanationID]; ok {
			corrections[c.ExplanationID] = append(corrections[c.ExplanationID], c)
		}
	}
	snap, _, captureErr := capture.TakeGo(a.Root)
	for i := range rounds {
		e := rounds[i].Analysis
		if e == nil {
			continue
		}
		applyCorrections(e, corrections[e.ID])
		if captureErr != nil {
			e.Freshness = "解释已过期：无法检查当前依据"
		} else {
			e.Freshness = a.freshnessFromSnapshot(e.Evidence, snap)
		}
	}
	return nil
}
