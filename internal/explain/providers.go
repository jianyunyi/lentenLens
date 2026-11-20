package explain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Resolved preserves old Anthropic settings while validating new destinations.
func (c Config) Resolved() (Config, error) {
	if c.VerificationMode == "" {
		c.VerificationMode = ModeLight
	}
	switch c.VerificationMode {
	case ModeLight, ModeFast, ModeStrict:
	default:
		return c, errors.New("不支持的验证模式；可选 light、fast 或 strict")
	}
	if c.Provider == "" {
		c.Provider = "anthropic"
	}
	var base, key string
	switch c.Provider {
	case "anthropic":
		base = "https://api.anthropic.com/v1"
		key = "ANTHROPIC_API_KEY"
	case "openai":
		base = "https://api.openai.com/v1"
		key = "OPENAI_API_KEY"
	case "openai-compatible":
		key = "INTENTLENS_API_KEY"
	case "gemini":
		base = "https://generativelanguage.googleapis.com/v1beta"
		key = "GEMINI_API_KEY"
	default:
		return c, errors.New("不支持的模型协议")
	}
	if c.BaseURL == "" {
		c.BaseURL = base
	}
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(c.BaseURL, "#") {
		return c, errors.New("API 基础地址无效；不得含凭据、查询参数或片段")
	}
	ip := net.ParseIP(u.Hostname())
	loopback := strings.EqualFold(u.Hostname(), "localhost") || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && (u.Scheme != "http" || !loopback) {
		return c, errors.New("API 必须使用 HTTPS；仅本机 loopback 可使用 HTTP")
	}
	if c.APIKeyEnv == "" {
		c.APIKeyEnv = key
	}
	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(c.APIKeyEnv) {
		return c, errors.New("密钥环境变量名无效")
	}
	if c.Provider == "gemini" && c.Model != "" && !regexp.MustCompile(`^(models/)?[A-Za-z0-9][A-Za-z0-9._-]*$`).MatchString(c.Model) {
		return c, errors.New("模型 Gemini 的 ID 应为模型名称或 API 返回的 models/<name> 资源名")
	}
	return c, nil
}

func callLimit(ctx context.Context, cfg Config, key, system, prompt string, outputLimit int) (string, int, error) {
	return callLimitMode(ctx, cfg, key, system, prompt, outputLimit, "")
}

func callLimitMode(ctx context.Context, cfg Config, key, system, prompt string, outputLimit int, mode string) (string, int, error) {
	cfg, err := cfg.Resolved()
	if err != nil {
		return "", -1, err
	}
	endpoint := cfg.BaseURL
	var payload map[string]any
	switch cfg.Provider {
	case "anthropic":
		endpoint += "/messages"
		payload = map[string]any{"model": cfg.Model, "max_tokens": outputLimit, "system": system, "messages": []any{map[string]string{"role": "user", "content": prompt}}}
	case "openai", "openai-compatible":
		endpoint += "/chat/completions"
		payload = map[string]any{"model": cfg.Model, "stream": false, "messages": []any{map[string]string{"role": "system", "content": system}, map[string]string{"role": "user", "content": prompt}}}
		field := "max_tokens"
		if cfg.Provider == "openai" {
			field = "max_completion_tokens"
		}
		payload[field] = outputLimit
	case "gemini":
		modelResource := cfg.Model
		if !strings.HasPrefix(modelResource, "models/") {
			modelResource = "models/" + modelResource
		}
		endpoint += "/" + modelResource + ":generateContent"
		payload = map[string]any{"systemInstruction": map[string]any{"parts": []any{map[string]string{"text": system}}}, "contents": []any{map[string]any{"role": "user", "parts": []any{map[string]string{"text": prompt}}}}, "generationConfig": map[string]any{"maxOutputTokens": outputLimit, "responseMimeType": "application/json"}}
	}
	if cfg.Provider == "openai-compatible" && (mode == ModeLight || mode == ModeFast) && isDeepSeekBaseURL(cfg.BaseURL) {
		payload["thinking"] = map[string]string{"type": "disabled"}
	}
	if cfg.Endpoint != "" {
		endpoint = cfg.Endpoint
	} // Non-serializable test seam.
	data, err := json.Marshal(payload)
	if err != nil {
		return "", -1, errors.New("模型请求无法编码为 JSON")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return "", -1, errors.New("模型请求地址无效")
	}
	req.Header.Set("content-type", "application/json")
	switch cfg.Provider {
	case "anthropic":
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", "2023-06-01")
	case "gemini":
		req.Header.Set("x-goog-api-key", key)
	default:
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client := &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("模型端点重定向被拒绝") }}
	if cfg.HTTPTransport != nil {
		client.Transport = cfg.HTTPTransport
	}
	res, err := client.Do(req)
	if err != nil {
		return "", -1, errors.New("模型请求失败，结果及计费未确认")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", -1, fmt.Errorf("模型请求失败 HTTP %d；未记录响应正文", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1024*1024+1))
	if err != nil || len(raw) > 1024*1024 {
		return "", -1, errors.New("模型响应读取失败或超过 1 MiB")
	}
	return decodeResponse(cfg.Provider, raw)
}

func isDeepSeekBaseURL(base string) bool {
	u, err := url.Parse(base)
	return err == nil && strings.EqualFold(strings.TrimSuffix(u.Hostname(), "."), "api.deepseek.com")
}

func decodeResponse(provider string, raw []byte) (string, int, error) {
	var result struct {
		Content []struct {
			Type string  `json:"type"`
			Text *string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      *struct {
			Input       *int `json:"input_tokens"`
			Output      *int `json:"output_tokens"`
			CacheRead   int  `json:"cache_read_input_tokens"`
			CacheCreate int  `json:"cache_creation_input_tokens"`
			Total       *int `json:"total_tokens"`
		} `json:"usage"`
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
				Refusal string          `json:"refusal"`
			} `json:"message"`
			Finish string `json:"finish_reason"`
		} `json:"choices"`
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text    *string `json:"text"`
					Thought bool    `json:"thought"`
				} `json:"parts"`
			} `json:"content"`
			Finish string `json:"finishReason"`
		} `json:"candidates"`
		Metadata *struct {
			Total *int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return "", -1, errors.New("模型响应无法解析")
	}
	// A negative token count means the response did not provide valid, exact usage.
	// Keep valid usage available even when a later completeness or content check fails.
	tokens := -1
	switch provider {
	case "anthropic":
		if u := result.Usage; u != nil && u.Input != nil && u.Output != nil {
			total, valid := 0, true
			for _, n := range []int{*u.Input, *u.Output, u.CacheRead, u.CacheCreate} {
				if n < 0 || n > math.MaxInt-total {
					valid = false
					break
				}
				total += n
			}
			if valid {
				tokens = total
			}
		}
	case "openai", "openai-compatible":
		if result.Usage != nil && result.Usage.Total != nil && *result.Usage.Total >= 0 {
			tokens = *result.Usage.Total
		}
	case "gemini":
		if result.Metadata != nil && result.Metadata.Total != nil && *result.Metadata.Total >= 0 {
			tokens = *result.Metadata.Total
		}
	}
	fail := func(err error) (string, int, error) { return "", tokens, err }
	var out strings.Builder
	switch provider {
	case "anthropic":
		if result.StopReason != "end_turn" && result.StopReason != "stop_sequence" {
			return fail(incompleteFinishError(result.StopReason))
		}
		for _, c := range result.Content {
			if c.Type == "text" || (c.Type == "" && c.Text != nil) {
				if c.Text == nil {
					return fail(errors.New("提供方 Anthropic 返回的文本块缺少文本内容"))
				}
				out.WriteString(*c.Text)
			} else if c.Text != nil && c.Type != "thinking" && c.Type != "redacted_thinking" {
				return fail(errors.New("提供方 Anthropic 响应包含无法完整显示的文本块"))
			}
		}
	case "openai", "openai-compatible":
		if len(result.Choices) != 1 {
			return fail(errors.New("模型返回的候选数不是恰好一条，无法验证单条完整答案"))
		}
		if result.Choices[0].Finish != "stop" {
			return fail(incompleteFinishError(result.Choices[0].Finish))
		}
		if result.Choices[0].Message.Refusal != "" {
			return fail(errors.New("模型拒绝返回固定探针文本"))
		}
		text, err := decodeOpenAITextContent(result.Choices[0].Message.Content)
		if err != nil {
			return fail(err)
		}
		out.WriteString(text)
	case "gemini":
		if len(result.Candidates) != 1 || result.Candidates[0].Finish != "STOP" {
			if len(result.Candidates) != 1 {
				return fail(errors.New("提供方 Gemini 返回的候选数不是恰好一条，无法验证单条完整答案"))
			}
			return fail(incompleteFinishError(result.Candidates[0].Finish))
		}
		for _, p := range result.Candidates[0].Content.Parts {
			if p.Thought {
				continue
			}
			if p.Text == nil {
				return fail(errors.New("提供方 Gemini 响应包含无法完整显示的非文本部分"))
			}
			out.WriteString(*p.Text)
		}
	}
	if tokens < 0 {
		return fail(errors.New("模型未返回有效用量；保留未决预算占用"))
	}
	text := strings.TrimSpace(out.String())
	if text == "" {
		return fail(errors.New("模型未返回文本答案"))
	}
	// Some compatible services wrap otherwise valid JSON in a fenced block.
	if strings.HasPrefix(text, "```json\n") && strings.HasSuffix(text, "\n```") {
		text = strings.TrimSuffix(strings.TrimPrefix(text, "```json\n"), "\n```")
	}
	return text, tokens, nil
}

func incompleteFinishError(reason string) error {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "", "null":
		return errors.New("模型未返回完整结束标记，无法确认输出完整")
	case "length", "max_tokens":
		return errors.New("模型输出达到 token 上限，属于截断结果")
	case "content_filter", "safety", "recitation", "blocked", "prohibited_content", "spii":
		return errors.New("模型输出被 provider 内容策略拦截，不能视为完整答案")
	default:
		return errors.New("模型输出未正常完成，不能视为完整答案")
	}
}

func decodeOpenAITextContent(raw json.RawMessage) (string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", errors.New("OpenAI 响应未返回文本内容")
	}
	var text string
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &text); err != nil {
			return "", errors.New("OpenAI 响应文本格式无效")
		}
		return text, nil
	}
	var blocks []struct {
		Type    string  `json:"type"`
		Text    *string `json:"text"`
		Refusal *string `json:"refusal"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil || len(blocks) == 0 {
		return "", errors.New("OpenAI 响应的文本块格式不受支持")
	}
	var out strings.Builder
	for _, block := range blocks {
		if block.Type == "refusal" || block.Refusal != nil {
			return "", errors.New("模型拒绝返回固定探针文本")
		}
		if block.Type != "" && block.Type != "text" && block.Type != "output_text" {
			return "", errors.New("OpenAI 响应包含无法完整显示的非文本块")
		}
		if block.Text == nil {
			return "", errors.New("OpenAI 文本块缺少文本内容")
		}
		out.WriteString(*block.Text)
	}
	return out.String(), nil
}
