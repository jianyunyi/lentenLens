package app

import (
	"context"
	"errors"
	"os"
	"strings"

	"intentlens/internal/explain"
)

// EditorModelTarget binds an editor-injected key to the actual destination.
// The CLI reads settings independently after the editor's preflight check.
type EditorModelTarget struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
}

func (a *App) modelAPIKey(cfg explain.Config, key string) (string, error) {
	if target := a.EditorModelTarget; target != nil {
		if cfg.Provider != target.Provider || cfg.BaseURL != target.BaseURL || cfg.APIKeyEnv != "INTENTLENS_API_KEY" {
			return "", errors.New("模型服务配置已变化，请先完成配置保存并重新确认 API Key")
		}
	}
	if key == "" {
		key = os.Getenv(cfg.APIKeyEnv)
	}
	return key, nil
}

// TestModel performs a deliberately separate, fixed, bounded provider probe.
// No workspace files, path, source, or user question are read or transmitted.
func (a *App) TestModel(ctx context.Context) (int, error) {
	unlock, err := a.lock("analysis")
	if err != nil {
		return 0, err
	}
	defer unlock()

	var raw Settings
	if readErr := a.read("settings.json", &raw); readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return 0, errors.New("请先显式配置模型 provider、Base URL 和模型 ID")
		}
		return 0, readErr
	}
	if !raw.Model.Enabled {
		return 0, errors.New("模型 API 未启用")
	}
	if strings.TrimSpace(raw.Model.Provider) == "" || strings.TrimSpace(raw.Model.BaseURL) == "" || strings.TrimSpace(raw.Model.Model) == "" {
		return 0, errors.New("请先显式配置 provider、Base URL 和模型 ID；连接测试不会使用默认或回退模型")
	}
	cfg, err := raw.Model.Resolved()
	if err != nil {
		return 0, err
	}
	key, err := a.modelAPIKey(cfg, "")
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(key) == "" {
		return 0, errors.New("未提供此工作区配置的 API Key")
	}
	usage, err := a.Usage()
	if err != nil {
		return 0, err
	}
	reserve := explain.ConnectionTestReservation()
	if reserve > usage.Remaining(cfg.TokenLimit) {
		return 0, errors.New("模型连通测试预算不足")
	}
	reserved := usage
	reserved.UncertainTokens += reserve
	if writeErr := a.write("usage.json", reserved); writeErr != nil {
		return 0, writeErr
	}
	tokens, err := explain.TestConnection(ctx, cfg, key)
	if err != nil {
		// The provider may have received the request. Keep the reservation as
		// uncertain instead of claiming the test was free or unbilled.
		return 0, err
	}
	if tokens < 0 || tokens > reserve {
		return 0, errors.New("模型测试报告的 token 超出固定探针预留；保留未决占用")
	}
	reserved.UncertainTokens -= reserve
	reserved.ActualTokens += tokens
	if writeErr := a.write("usage.json", reserved); writeErr != nil {
		return 0, writeErr
	}
	return tokens, nil
}
