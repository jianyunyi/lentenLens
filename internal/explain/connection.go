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
	var result map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &result); err != nil || len(result) != 1 {
		return 0, errors.New("模型响应未完整匹配固定探针文本；未回退到其他模型")
	}
	var status string
	if err := json.Unmarshal(result["intentlens_connection_test"], &status); err != nil || status != "ok" {
		return 0, errors.New("模型响应未完整匹配固定探针文本；未回退到其他模型")
	}
	return tokens, nil
}

func ConnectionTestReservation() int { return connectionTestReservation }
