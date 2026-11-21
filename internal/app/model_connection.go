package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"intentlens/internal/capture"
	"intentlens/internal/explain"
)

// EditorAPIKeyEnv is the per-call SecretStorage credential channel. Keep it
// separate from INTENTLENS_API_KEY, which remains a CLI / hook default.
const EditorAPIKeyEnv = "INTENTLENS_EDITOR_API_KEY"

// EditorModelTarget binds an editor-injected key to the actual destination.
// The CLI reads settings independently after the editor's preflight check.
type EditorModelTarget struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	Model    string `json:"model,omitempty"`
}

func (a *App) modelAPIKey(cfg explain.Config, key string) (string, error) {
	return a.modelAPIKeyForTarget(cfg, key, false)
}

func (a *App) modelAPIKeyForTarget(cfg explain.Config, key string, requireModel bool) (string, error) {
	editorKey, editorCredential := os.LookupEnv(EditorAPIKeyEnv)
	if editorCredential && a.EditorModelTarget == nil {
		return "", errors.New("VS Code 模型密钥绑定无效，请重新保存 API Key")
	}
	if target := a.EditorModelTarget; target != nil {
		modelMismatch := (requireModel || target.Model != "") && cfg.Model != target.Model
		if cfg.Provider != target.Provider || cfg.BaseURL != target.BaseURL || modelMismatch || cfg.APIKeyEnv != "INTENTLENS_API_KEY" {
			return "", errors.New("模型服务配置已变化，请先完成配置保存并重新确认 API Key")
		}
	}
	if editorCredential {
		// An empty editor key must never fall back to an ambient CLI credential.
		return editorKey, nil
	}
	if key == "" {
		key = os.Getenv(cfg.APIKeyEnv)
	}
	return key, nil
}

func (a *App) modelRequestConfig(cfg explain.Config, paths []string) explain.Config {
	cfg.HTTPTransport = a.modelTransport
	cfg.BeforeRequest = func() error { return a.checkModelSourcePaths(paths) }
	return cfg
}

func (a *App) checkModelSourcePaths(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	policy, err := capture.LoadPolicy(a.Root)
	if err != nil {
		return fmt.Errorf("无法读取当前排除策略 (.intentignore)：%w", err)
	}
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = filepath.ToSlash(path)
		if seen[path] {
			continue
		}
		seen[path] = true
		allowed, err := policy.Allowed(a.Root, path)
		if err != nil {
			return fmt.Errorf("无法按当前排除策略检查路径 %q：%w", path, err)
		}
		if !allowed {
			return fmt.Errorf("路径 %q 已被排除或不安全，请检查当前排除策略 (.intentignore)", path)
		}
	}
	return nil
}

func (a *App) reserveModelUsage(usage explain.Usage, reserve int) error {
	usage.UncertainTokens += reserve
	if err := a.write("usage.json", usage); err != nil {
		return err
	}
	if a.afterModelReserve != nil {
		a.afterModelReserve()
	}
	return nil
}

func (a *App) settleModelUsage(usage explain.Usage, callErr error) error {
	if err := a.write("usage.json", usage); err != nil {
		return errors.Join(callErr, fmt.Errorf("无法写入模型用量结算：%w", err))
	}
	return callErr
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
	key, err := a.modelAPIKeyForTarget(cfg, "", true)
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
	tokens, err := explain.TestConnection(ctx, a.modelRequestConfig(cfg, nil), key)
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
