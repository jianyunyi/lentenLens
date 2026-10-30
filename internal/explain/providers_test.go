package explain

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestProviderUsageCannotOverflowBudget(t *testing.T) {
	server := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": "{}"}}}, "usage": map[string]int{"total_tokens": math.MaxInt}})
	}))
	cfg := Config{Enabled: true, Provider: "openai", BaseURL: "https://model.invalid", HTTPTransport: server, Model: "fixture", TokenLimit: 1000000}
	_, u, err := Structured(context.Background(), cfg, Usage{ActualTokens: 100}, "key", "prompt")
	if err == nil || u.ActualTokens != 100 || u.UncertainTokens == 0 {
		t.Fatalf("overflow not rejected: %+v %v", u, err)
	}
}

func TestProviderFailureBoundaries(t *testing.T) {
	for _, provider := range []string{"anthropic", "openai", "openai-compatible", "gemini"} {
		for _, mode := range []string{"redirect", "oversize", "empty", "truncated", "missing-usage", "cancelled", "timeout"} {
			t.Run(provider+"/"+mode, func(t *testing.T) {
				transportCalls := 0
				timeoutObserved := false
				transport := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					transportCalls++
					if mode == "redirect" {
						http.Redirect(w, r, "https://redirect-target.invalid/", http.StatusTemporaryRedirect)
						return
					}
					if mode == "timeout" {
						<-r.Context().Done()
						timeoutObserved = r.Context().Err() == context.DeadlineExceeded
						return
					}
					if mode == "oversize" {
						_, _ = w.Write([]byte(strings.Repeat("x", 1024*1024+1)))
						return
					}
					bodyText := "{}"
					if mode == "empty" {
						bodyText = ""
					}
					var response map[string]any
					switch provider {
					case "anthropic":
						finish := "end_turn"
						if mode == "truncated" {
							finish = "max_tokens"
						}
						response = map[string]any{"stop_reason": finish, "content": []any{map[string]string{"type": "text", "text": bodyText}}, "usage": map[string]int{"input_tokens": 1, "output_tokens": 1}}
					case "gemini":
						finish := "STOP"
						if mode == "truncated" {
							finish = "MAX_TOKENS"
						}
						response = map[string]any{"candidates": []any{map[string]any{"finishReason": finish, "content": map[string]any{"parts": []any{map[string]string{"text": bodyText}}}}}, "usageMetadata": map[string]int{"totalTokenCount": 2}}
					default:
						finish := "stop"
						if mode == "truncated" {
							finish = "length"
						}
						response = map[string]any{"choices": []any{map[string]any{"finish_reason": finish, "message": map[string]string{"content": bodyText}}}, "usage": map[string]int{"total_tokens": 2}}
					}
					if mode == "missing-usage" {
						delete(response, "usage")
						delete(response, "usageMetadata")
					}
					_ = json.NewEncoder(w).Encode(response)
				}))
				ctx := context.Background()
				cancel := func() {}
				if mode == "cancelled" {
					ctx, cancel = context.WithCancel(ctx)
					cancel()
				} else if mode == "timeout" {
					ctx, cancel = context.WithTimeout(ctx, 10*time.Millisecond)
				}
				defer cancel()
				_, usage, err := Structured(ctx, Config{Enabled: true, Provider: provider, BaseURL: "https://model.invalid", HTTPTransport: transport, Model: "fixture", TokenLimit: 1000000}, Usage{}, "secret", "prompt")
				wantCalls := 1
				if mode == "cancelled" {
					wantCalls = 0
				}
				if err == nil || usage.UncertainTokens == 0 || transportCalls != wantCalls || (mode == "timeout" && !timeoutObserved) {
					t.Fatalf("boundary bypass: err=%v usage=%+v transportCalls=%d want=%d timeoutObserved=%v", err, usage, transportCalls, wantCalls, timeoutObserved)
				}
			})
		}
	}
}

func TestProviderClientAddsNinetySecondTimeout(t *testing.T) {
	seen := false
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		deadline, ok := req.Context().Deadline()
		if !ok {
			t.Error("HTTP client did not bound the request with a deadline")
		} else {
			remaining := time.Until(deadline)
			if remaining <= 80*time.Second || remaining > 90*time.Second {
				t.Errorf("unexpected remaining deadline: %v", remaining)
			}
		}
		seen = true
		return nil, context.DeadlineExceeded
	})
	_, usage, err := Structured(context.Background(), Config{Enabled: true, Provider: "openai-compatible", BaseURL: "https://model.invalid", HTTPTransport: transport, Model: "fixture", TokenLimit: 1000000}, Usage{}, "key", "prompt")
	if !seen || err == nil || usage.UncertainTokens == 0 {
		t.Fatalf("timeout was not bounded/accounted: seen=%v usage=%+v err=%v", seen, usage, err)
	}
}

func TestProviderExplanationAndEvidenceCheckAcrossProtocols(t *testing.T) {
	for _, provider := range []string{"anthropic", "openai", "openai-compatible", "gemini"} {
		t.Run(provider, func(t *testing.T) {
			calls := 0
			server := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				text := `{"claims":[{"id":"c1","text":"returns one","kind":"behavior","evidence_ids":["e1"]}]}`
				if calls == 2 {
					text = `{"checks":[{"claim_id":"c1","status":"supported"}]}`
				}
				var response map[string]any
				switch provider {
				case "anthropic":
					response = map[string]any{"content": []any{map[string]string{"type": "text", "text": text}}, "usage": map[string]int{"input_tokens": 1, "output_tokens": 1}}
				case "gemini":
					response = map[string]any{"candidates": []any{map[string]any{"finishReason": "STOP", "content": map[string]any{"parts": []any{map[string]string{"text": text}}}}}, "usageMetadata": map[string]int{"totalTokenCount": 2}}
				default:
					response = map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": text}}}, "usage": map[string]int{"total_tokens": 2}}
				}
				json.NewEncoder(w).Encode(response)
			}))
			doc, u, err := Run(context.Background(), Config{Enabled: true, Provider: provider, BaseURL: "https://model.invalid", HTTPTransport: server, Model: "fixture", TokenLimit: 1000000}, Usage{}, "key", "question", []Evidence{{ID: "e1", Source: "return 1"}})
			if err != nil || calls != 2 || len(doc.Claims) != 1 || doc.Claims[0].Status != "supported" || u.ActualTokens != 4 {
				t.Fatalf("err=%v calls=%d doc=%+v usage=%+v", err, calls, doc, u)
			}
		})
	}
}

func TestProviderWireProtocols(t *testing.T) {
	for _, provider := range []string{"anthropic", "openai", "openai-compatible", "gemini"} {
		t.Run(provider, func(t *testing.T) {
			server := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				switch provider {
				case "anthropic":
					if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "secret" || body["system"] != "system" {
						t.Errorf("wrong Anthropic request")
					}
					w.Write([]byte(`{"content":[{"type":"text","text":"{}"}],"usage":{"input_tokens":2,"output_tokens":3,"cache_read_input_tokens":4,"cache_creation_input_tokens":1}}`))
				case "gemini":
					if r.URL.Path != "/v1/models/fixture:generateContent" || r.Header.Get("x-goog-api-key") != "secret" || r.URL.RawQuery != "" || body["systemInstruction"] == nil {
						t.Errorf("wrong Gemini request")
					}
					w.Write([]byte(`{"candidates":[{"finishReason":"STOP","content":{"parts":[{"thought":true,"text":"not-answer"},{"text":"{}"}]}}],"usageMetadata":{"totalTokenCount":10}}`))
				default:
					if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer secret" || body["messages"] == nil {
						t.Errorf("wrong compatible request")
					}
					field := "max_tokens"
					if provider == "openai" {
						field = "max_completion_tokens"
					}
					if body[field] != float64(2048) {
						t.Errorf("wrong output limit: %v", body)
					}
					w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{}"}}],"usage":{"prompt_tokens":4,"completion_tokens":6,"total_tokens":10}}`))
				}
			}))
			out, n, err := call(context.Background(), Config{Provider: provider, BaseURL: "https://model.invalid/v1", HTTPTransport: server, Model: "fixture"}, "secret", "system", "prompt")
			if err != nil || out != "{}" || n != 10 {
				t.Fatalf("out=%q tokens=%d err=%v", out, n, err)
			}
		})
	}
}

func TestGeminiResourceModelIDIsUsedVerbatim(t *testing.T) {
	transport := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models/gemini-flash:generateContent" {
			t.Errorf("unexpected Gemini resource endpoint: %s", r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "secret" || r.URL.RawQuery != "" {
			t.Error("Gemini Key was not confined to the expected header")
		}
		_, _ = w.Write([]byte(`{"candidates":[{"finishReason":"STOP","content":{"parts":[{"text":"{}"}]}}],"usageMetadata":{"totalTokenCount":2}}`))
	}))
	text, tokens, err := call(context.Background(), Config{Provider: "gemini", BaseURL: "https://model.invalid/v1", HTTPTransport: transport, Model: "models/gemini-flash"}, "secret", "system", "prompt")
	if err != nil || text != "{}" || tokens != 2 {
		t.Fatalf("text=%q tokens=%d err=%v", text, tokens, err)
	}
}

func TestProviderConfiguration(t *testing.T) {
	cfg, err := (Config{}).Resolved()
	if err != nil || cfg.Provider != "anthropic" || cfg.APIKeyEnv != "ANTHROPIC_API_KEY" {
		t.Fatalf("legacy defaults: %+v %v", cfg, err)
	}
	for _, base := range []string{"http://remote.example/v1", "https://user:secret@example.com/v1", "https://example.com/v1?key=secret", "https://example.com/v1#fragment"} {
		if _, err := (Config{BaseURL: base}).Resolved(); err == nil {
			t.Errorf("accepted unsafe URL: %s", base)
		}
	}
	for _, cfg := range []Config{{Provider: "unknown"}, {APIKeyEnv: "NOT A VARIABLE"}, {Provider: "gemini", Model: "x?key=secret"}, {Provider: "openai-compatible"}} {
		if _, err := cfg.Resolved(); err == nil {
			t.Errorf("accepted bad config: %+v", cfg)
		}
	}
}

func TestProvidersFailWithoutConfirmedUsageAndOnHTTPError(t *testing.T) {
	for _, provider := range []string{"anthropic", "openai", "openai-compatible", "gemini"} {
		for _, response := range []string{`{}`, `{"usage":{"total_tokens":-1},"usageMetadata":{"totalTokenCount":-1}}`, `not-json`, `http-error`} {
			t.Run(provider+response, func(t *testing.T) {
				server := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if response == "http-error" {
						w.WriteHeader(401)
					}
					w.Write([]byte(response))
				}))
				cfg := Config{Enabled: true, Provider: provider, BaseURL: "https://model.invalid", HTTPTransport: server, Model: "fixture", TokenLimit: 1000000}
				_, usage, err := Structured(context.Background(), cfg, Usage{}, "secret", "prompt")
				if err == nil || usage.UncertainTokens == 0 || usage.ActualTokens != 0 || strings.Contains(err.Error(), "secret") {
					t.Fatalf("err=%v usage=%+v", err, usage)
				}
			})
		}
	}
}
