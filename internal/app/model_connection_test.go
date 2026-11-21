package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"intentlens/internal/explain"
)

type modelConnectionTransport func(*http.Request) (*http.Response, error)

func (f modelConnectionTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func stubModelEndpoint(a *App, handle http.HandlerFunc) string {
	a.modelTransport = modelConnectionTransport(func(req *http.Request) (*http.Response, error) {
		response := httptest.NewRecorder()
		handle.ServeHTTP(response, req)
		return response.Result(), nil
	})
	return "http://127.0.0.1:8080/v1"
}

func TestEditorCredentialBindingBeforeModelRequests(t *testing.T) {
	const base = "https://selected.fixture/v1"
	for _, tc := range []struct {
		name, keyEnv, editorKey, wantError string
		target                             *EditorModelTarget
		editor                             bool
	}{
		{name: "editor missing target", editor: true, editorKey: "editor-key", wantError: "模型密钥绑定无效"},
		{name: "empty editor channel missing target", editor: true, wantError: "模型密钥绑定无效"},
		{name: "empty target", editor: true, editorKey: "editor-key", target: &EditorModelTarget{}, wantError: "模型服务配置已变化"},
		{name: "invalid target", editor: true, editorKey: "editor-key", target: &EditorModelTarget{Provider: "openai-compatible", BaseURL: "not-a-url", Model: "fixture"}, wantError: "模型服务配置已变化"},
		{name: "provider mismatch", editor: true, editorKey: "editor-key", target: &EditorModelTarget{Provider: "openai", BaseURL: base, Model: "fixture"}, wantError: "模型服务配置已变化"},
		{name: "base mismatch", editor: true, editorKey: "editor-key", target: &EditorModelTarget{Provider: "openai-compatible", BaseURL: "https://other.fixture/v1", Model: "fixture"}, wantError: "模型服务配置已变化"},
		{name: "model mismatch", editor: true, editorKey: "editor-key", target: &EditorModelTarget{Provider: "openai-compatible", BaseURL: base, Model: "changed-model"}, wantError: "模型服务配置已变化"},
		{name: "key env mismatch", editor: true, editorKey: "editor-key", keyEnv: "INTENTLENS_APP_CLI_KEY", target: &EditorModelTarget{Provider: "openai-compatible", BaseURL: base, Model: "fixture"}, wantError: "模型服务配置已变化"},
		{name: "matching target", editor: true, editorKey: "editor-key", target: &EditorModelTarget{Provider: "openai-compatible", BaseURL: base, Model: "fixture"}},
		{name: "ordinary CLI default key"},
		{name: "ordinary CLI custom key", keyEnv: "INTENTLENS_APP_CLI_KEY"},
	} {
		for _, caller := range []string{"probe", "analysis", "exercise"} {
			t.Run(tc.name+"/"+caller, func(t *testing.T) {
				t.Setenv(EditorAPIKeyEnv, tc.editorKey)
				if !tc.editor {
					if err := os.Unsetenv(EditorAPIKeyEnv); err != nil {
						t.Fatal(err)
					}
				}
				t.Setenv("INTENTLENS_API_KEY", "cli-key")
				t.Setenv("INTENTLENS_APP_CLI_KEY", "cli-key")
				a, _ := fixture(t)
				a.EditorModelTarget = tc.target
				if err := a.Configure(Settings{Model: explain.Config{Enabled: true, Provider: "openai-compatible", BaseURL: " " + base + "/// ", APIKeyEnv: tc.keyEnv, Model: "fixture", TokenLimit: 1000000}}, true); err != nil {
					t.Fatal(err)
				}
				calls := 0
				checkKey := func(key string) {
					t.Helper()
					calls++
					want := "cli-key"
					if tc.editor {
						want = "editor-key"
					}
					if key != want {
						t.Fatal("model used the wrong credential channel")
					}
				}
				previous := http.DefaultTransport
				http.DefaultTransport = modelConnectionTransport(func(r *http.Request) (*http.Response, error) {
					if r.URL.String() != base+"/chat/completions" {
						t.Error("model used the wrong destination")
					}
					checkKey(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
					recorder := httptest.NewRecorder()
					_, _ = io.WriteString(recorder, `{"choices":[{"finish_reason":"stop","message":{"content":"{\"intentlens_connection_test\":\"ok\"}"}}],"usage":{"total_tokens":9}}`)
					return recorder.Result(), nil
				})
				t.Cleanup(func() { http.DefaultTransport = previous })
				a.modelRun = func(_ context.Context, _ explain.Config, u explain.Usage, key, _ string, _ []explain.Evidence) (explain.Document, explain.Usage, error) {
					checkKey(key)
					return explain.Document{}, u, nil
				}
				a.exerciseRun = func(_ context.Context, _ explain.Config, u explain.Usage, key, _ string) (string, explain.Usage, error) {
					checkKey(key)
					return "{}", u, nil
				}
				var err error
				switch caller {
				case "probe":
					_, err = a.TestModel(context.Background())
				case "analysis":
					_, err = a.analyze(context.Background(), "fixture", "", nil)
				case "exercise":
					_, err = a.exerciseModel(context.Background(), "", "fixture", nil)
				}
				if tc.wantError != "" {
					if err == nil || !strings.Contains(err.Error(), tc.wantError) || calls != 0 {
						t.Fatalf("requests=%d error=%v", calls, err)
					}
					usage, usageErr := a.Usage()
					if usageErr != nil || usage.ActualTokens != 0 || usage.UncertainTokens != 0 {
						t.Fatalf("rejected binding changed usage: %+v error=%v", usage, usageErr)
					}
				} else if err != nil || calls != 1 {
					t.Fatalf("requests=%d error=%v", calls, err)
				}
			})
		}
	}
}

func TestEditorCredentialNeverFallsBackToOtherKeys(t *testing.T) {
	t.Setenv(EditorAPIKeyEnv, "")
	t.Setenv("INTENTLENS_API_KEY", "ambient-cli-key")
	cfg := explain.Config{Provider: "openai-compatible", BaseURL: "https://selected.fixture/v1", APIKeyEnv: "INTENTLENS_API_KEY"}
	a := &App{}
	if _, err := a.modelAPIKey(cfg, "explicit-key"); err == nil {
		t.Fatal("explicit key bypassed the missing editor binding")
	}
	a.EditorModelTarget = &EditorModelTarget{Provider: cfg.Provider, BaseURL: cfg.BaseURL, Model: cfg.Model}
	for _, explicit := range []string{"", "explicit-key"} {
		if key, err := a.modelAPIKey(cfg, explicit); err != nil || key != "" {
			t.Fatalf("empty editor channel fell back to another key: error=%v", err)
		}
	}
	t.Setenv(EditorAPIKeyEnv, "bound-editor-key")
	if key, err := a.modelAPIKey(cfg, "explicit-key"); err != nil || key != "bound-editor-key" {
		t.Fatalf("editor credential was overridden: error=%v", err)
	}
}

func TestTestModelProbesExactConfiguredAPIAndSettlesBudget(t *testing.T) {
	a, _ := fixture(t)
	var hits atomic.Int32
	base := stubModelEndpoint(a, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected endpoint %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer fixture-api-key" {
			t.Error("wrong key sent")
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		wire, _ := json.Marshal(request)
		if request["model"] != "explicit-fixture-model" {
			t.Errorf("probe used unexpected model ID: %v", request["model"])
		}
		for _, forbidden := range []string{"auth.go", "private source", a.Root, "user question"} {
			if strings.Contains(string(wire), forbidden) {
				t.Errorf("workspace data leaked into fixed test request: %q", forbidden)
			}
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"intentlens_connection_test\":\"ok\"}"}}],"usage":{"total_tokens":9}}`))
	})
	t.Setenv("INTENTLENS_FIXTURE_KEY", "fixture-api-key")
	cfg := Settings{Model: explain.Config{Enabled: true, Provider: "openai-compatible", BaseURL: base, APIKeyEnv: "INTENTLENS_FIXTURE_KEY", Model: "explicit-fixture-model", TokenLimit: 4096}}
	if err := a.Configure(cfg, true); err != nil {
		t.Fatal(err)
	}
	tokens, err := a.TestModel(context.Background())
	if err != nil || tokens != 9 || hits.Load() != 1 {
		t.Fatalf("model test failed: tokens=%d requests=%d err=%v", tokens, hits.Load(), err)
	}
	usage, err := a.Usage()
	if err != nil || usage.ActualTokens != 9 || usage.UncertainTokens != 0 {
		t.Fatalf("test usage not settled: %+v %v", usage, err)
	}
}

func TestTestModelKeepsFailedRequestUncertainAndDoesNotRetry(t *testing.T) {
	a, _ := fixture(t)
	var hits atomic.Int32
	base := stubModelEndpoint(a, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("fixture-secret-must-not-appear"))
	})
	t.Setenv("INTENTLENS_FIXTURE_KEY", "fixture-secret")
	cfg := Settings{Model: explain.Config{Enabled: true, Provider: "openai-compatible", BaseURL: base, APIKeyEnv: "INTENTLENS_FIXTURE_KEY", Model: "chosen-only", TokenLimit: 4096}}
	if err := a.Configure(cfg, true); err != nil {
		t.Fatal(err)
	}
	_, err := a.TestModel(context.Background())
	if err == nil || strings.Contains(err.Error(), "fixture-secret") || strings.Contains(err.Error(), "fixture-secret-must-not-appear") {
		t.Fatalf("unsafe or successful error: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("request retried/fell back: %d calls", hits.Load())
	}
	usage, usageErr := a.Usage()
	if usageErr != nil || usage.ActualTokens != 0 || usage.UncertainTokens != explain.ConnectionTestReservation() {
		t.Fatalf("failed request not conservatively reserved: %+v %v", usage, usageErr)
	}
}

func TestTestModelFailsClosedWithoutExplicitProviderOrKey(t *testing.T) {
	a, _ := fixture(t)
	t.Setenv("OPENAI_API_KEY", "unrelated-secret-must-not-be-used")
	for name, settings := range map[string]Settings{
		"no-provider": {Model: explain.Config{Enabled: true, BaseURL: "https://selected.example/v1", Model: "chosen", APIKeyEnv: "OPENAI_API_KEY", TokenLimit: 10000}},
		"no-key":      {Model: explain.Config{Enabled: true, Provider: "openai-compatible", BaseURL: "https://selected.example/v1", Model: "chosen", APIKeyEnv: "INTENTLENS_MISSING_KEY", TokenLimit: 10000}},
		"disabled":    {Model: explain.Config{Provider: "openai-compatible", BaseURL: "https://selected.example/v1", Model: "chosen", APIKeyEnv: "OPENAI_API_KEY", TokenLimit: 10000}},
	} {
		if err := a.write("settings.json", settings); err != nil {
			t.Fatal(err)
		}
		if _, err := a.TestModel(context.Background()); err == nil {
			t.Fatalf("%s unexpectedly attempted a request", name)
		}
	}
}

func TestTestModelChecksBudgetBeforeRequest(t *testing.T) {
	a, _ := fixture(t)
	var hits atomic.Int32
	base := stubModelEndpoint(a, func(w http.ResponseWriter, r *http.Request) { hits.Add(1) })
	t.Setenv("INTENTLENS_FIXTURE_KEY", "key")
	cfg := Settings{Model: explain.Config{Enabled: true, Provider: "openai-compatible", BaseURL: base, APIKeyEnv: "INTENTLENS_FIXTURE_KEY", Model: "chosen", TokenLimit: 500}}
	if err := a.Configure(cfg, true); err != nil {
		t.Fatal(err)
	}
	if _, err := a.TestModel(context.Background()); err == nil || !strings.Contains(err.Error(), "预算") {
		t.Fatalf("expected bounded refusal, got %v", err)
	}
	if hits.Load() != 0 {
		t.Fatal("request sent despite insufficient token reserve")
	}
}

func TestTestModelAcceptsCombinedTextPartsAcrossProviders(t *testing.T) {
	for _, tc := range []struct {
		provider string
		response string
	}{
		{"anthropic", `{"stop_reason":"end_turn","content":[{"type":"text","text":"{\"intentlens_connection_test\":\""},{"type":"text","text":"ok\"}"}],"usage":{"input_tokens":1,"output_tokens":1}}`},
		{"openai", `{"choices":[{"finish_reason":"stop","message":{"content":[{"type":"text","text":"{\"intentlens_connection_test\":\""},{"type":"text","text":"ok\"}"}]}}],"usage":{"total_tokens":2}}`},
		{"openai-compatible", `{"choices":[{"finish_reason":"stop","message":{"content":[{"type":"text","text":"{\"intentlens_connection_test\":\""},{"type":"text","text":"ok\"}"}]}}],"usage":{"total_tokens":2}}`},
		{"gemini", `{"candidates":[{"finishReason":"STOP","content":{"parts":[{"thought":true,"text":"not answer"},{"text":"{\"intentlens_connection_test\":\""},{"text":"ok\"}"}]}}],"usageMetadata":{"totalTokenCount":2}}`},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			a, _ := fixture(t)
			hits := 0
			base := stubModelEndpoint(a, func(w http.ResponseWriter, r *http.Request) {
				hits++
				_, _ = w.Write([]byte(tc.response))
			})
			t.Setenv("INTENTLENS_FIXTURE_KEY", "fixture-api-key")
			cfg := Settings{Model: explain.Config{Enabled: true, Provider: tc.provider, BaseURL: base, APIKeyEnv: "INTENTLENS_FIXTURE_KEY", Model: "fixture", TokenLimit: 4096}}
			if err := a.Configure(cfg, true); err != nil {
				t.Fatal(err)
			}
			tokens, err := a.TestModel(context.Background())
			if err != nil || tokens != 2 || hits != 1 {
				t.Fatalf("tokens=%d requests=%d err=%v", tokens, hits, err)
			}
			usage, err := a.Usage()
			if err != nil || usage.ActualTokens != 2 || usage.UncertainTokens != 0 {
				t.Fatalf("usage not settled after complete response: %+v %v", usage, err)
			}
		})
	}
}

func TestTestModelDoesNotAcceptExtraTextOrObjectFieldsAsComplete(t *testing.T) {
	a, _ := fixture(t)
	hits := 0
	base := stubModelEndpoint(a, func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"intentlens_connection_test\":\"ok\",\"extra\":\"partial\"}"}}],"usage":{"total_tokens":2}}`))
	})
	t.Setenv("INTENTLENS_FIXTURE_KEY", "fixture-api-key")
	cfg := Settings{Model: explain.Config{Enabled: true, Provider: "openai-compatible", BaseURL: base, APIKeyEnv: "INTENTLENS_FIXTURE_KEY", Model: "fixture", TokenLimit: 4096}}
	if err := a.Configure(cfg, true); err != nil {
		t.Fatal(err)
	}
	if _, err := a.TestModel(context.Background()); err == nil || !strings.Contains(err.Error(), "未完整匹配固定探针文本") {
		t.Fatalf("unexpected success or unclear partial-response error: %v", err)
	}
	usage, err := a.Usage()
	if err != nil || hits != 1 || usage.ActualTokens != 0 || usage.UncertainTokens != explain.ConnectionTestReservation() {
		t.Fatalf("failed probe not conservatively accounted: requests=%d usage=%+v err=%v", hits, usage, err)
	}
}
