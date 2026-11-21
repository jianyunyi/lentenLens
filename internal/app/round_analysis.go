package app

import (
	"context"
	"errors"
	"fmt"
	"os"
)

type automaticRoundAnalysis struct {
	RoundID       string `json:"round_id"`
	ExplanationID string `json:"explanation_id"`
	Completed     string `json:"completed"`
}

// AnalyzeRoundAutomatically is the idempotent path used by lifecycle hooks.
// Manual analysis deliberately continues to use ExplainRound directly.
func (a *App) AnalyzeRoundAutomatically(ctx context.Context, id string) (bool, error) {
	if !validID(id) {
		return false, errors.New("无效轮次 ID")
	}
	unlock, err := a.lock("round-analysis-auto")
	if err != nil {
		return false, err
	}
	defer unlock()

	markerName := "auto-analysis-round-" + id + ".json"
	var marker automaticRoundAnalysis
	err = a.read(markerName, &marker)
	if err == nil {
		if marker.RoundID != id || !validID(marker.ExplanationID) || marker.Completed == "" {
			return false, errors.New("自动轮次分析记录无效")
		}
		return false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	rounds, err := a.List()
	if err != nil {
		return false, err
	}
	var round *Round
	for i := range rounds {
		if rounds[i].ID == id {
			round = &rounds[i]
			break
		}
	}
	if round == nil {
		return false, fmt.Errorf("轮次不存在：%s", id)
	}
	idx, err := a.Index()
	if err != nil {
		return false, err
	}
	explanation, err := a.ExplainRound(ctx, *round, idx, "")
	if err != nil {
		return false, err
	}
	marker = automaticRoundAnalysis{RoundID: id, ExplanationID: explanation.ID, Completed: explanation.Created}
	if err := a.write(markerName, marker); err != nil {
		return false, err
	}
	return true, nil
}
