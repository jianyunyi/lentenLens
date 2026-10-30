package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"intentlens/internal/explain"
)

func TestTestModelProbesExactConfiguredAPIAndSettlesBudget(t *testing.T) {
	a, _ := fixture(t)
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		for _, forbidden := range []string{"auth.go", "private source", a.Root, "user question"} {
			if strings.Contains(string(wire), forbidden) {
				t.Errorf("workspace data leaked into fixed test request: %q", forbidden)
			}
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"intentlens_connection_test\":\"ok\"}"}}],"usage":{"total_tokens":9}}`))
	}))
	defer server.Close()
	t.Setenv("INTENTLENS_FIXTURE_KEY", "fixture-api-key")
	cfg := Settings{Model: explain.Config{Enabled: true, Provider: "openai-compatible", BaseURL: server.URL + "/v1", APIKeyEnv: "INTENTLENS_FIXTURE_KEY", Model: "explicit-fixture-model", TokenLimit: 4096}}
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("fixture-secret-must-not-appear"))
	}))
	defer server.Close()
	t.Setenv("INTENTLENS_FIXTURE_KEY", "fixture-secret")
	cfg := Settings{Model: explain.Config{Enabled: true, Provider: "openai-compatible", BaseURL: server.URL + "/v1", APIKeyEnv: "INTENTLENS_FIXTURE_KEY", Model: "chosen-only", TokenLimit: 4096}}
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer server.Close()
	t.Setenv("INTENTLENS_FIXTURE_KEY", "key")
	cfg := Settings{Model: explain.Config{Enabled: true, Provider: "openai-compatible", BaseURL: server.URL + "/v1", APIKeyEnv: "INTENTLENS_FIXTURE_KEY", Model: "chosen", TokenLimit: 500}}
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
