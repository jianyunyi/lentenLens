package explain

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

const connectionTestReservation = 1024

// TestConnection probes only the explicitly configured provider/base/model. It
// never selects a fallback and sends no workspace evidence, path, or user text.
func TestConnection(ctx context.Context, cfg Config, key string) (int, error) {
	if !cfg.Enabled || strings.TrimSpace(cfg.Provider) == "" || strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.Model) == "" {
		return 0, errors.New("连接测试需要显式配置 provider、Base URL 和模型 ID")
	}
	if strings.TrimSpace(key) == "" {
		return 0, errors.New("未提供已配置的 API Key")
	}
	cfg, err := cfg.Resolved()
	if err != nil {
		return 0, err
	}
	text, tokens, err := callLimit(ctx, cfg, key,
		"Return exactly this JSON object and no other text: {\"intentlens_connection_test\":\"ok\"}.",
		"Return the required fixed JSON object.", 64)
	if err != nil {
		return 0, err
	}
	var result struct {
		Status string `json:"intentlens_connection_test"`
	}
	if err := json.Unmarshal([]byte(text), &result); err != nil || result.Status != "ok" {
		return 0, errors.New("模型响应不符合固定连通性测试格式；未回退到其他模型")
	}
	return tokens, nil
}

func ConnectionTestReservation() int { return connectionTestReservation }
