package explain

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestExplainRequiresConsentAndReservesBudget(t *testing.T) {
	calls := 0
	server := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	cfg := Config{Endpoint: "https://model.invalid", HTTPTransport: server, Model: "test", TokenLimit: 1}
	_, _, err := Run(context.Background(), cfg, Usage{}, "key", "explain", []Evidence{{ID: "e1", Source: "func F() {}"}})
	if err == nil || calls != 0 {
		t.Fatalf("disabled analysis made requests: %d, %v", calls, err)
	}
	cfg.Enabled = true
	_, _, err = Run(context.Background(), cfg, Usage{}, "key", "explain", []Evidence{{ID: "e1", Source: "func F() {}"}})
	if err == nil || calls != 0 {
		t.Fatalf("budget bypassed: %d, %v", calls, err)
	}
}

func TestRunChecksEveryClaimAndRejectsInventedReferences(t *testing.T) {
	calls := 0
	server := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("x-api-key") != "test-key" {
			t.Error("missing API key")
		}
		text := `{"claims":[{"id":"c1","text":"returns zero","kind":"behavior","evidence_ids":["e1"]},{"id":"c2","text":"invented","kind":"motive","evidence_ids":["fake"]}],"unknowns":["choice not recorded"]}`
		if calls == 2 {
			text = `{"checks":[{"claim_id":"c1","status":"supported"}]}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"stop_reason": "end_turn", "content": []any{map[string]any{"type": "text", "text": text}}, "usage": map[string]int{"input_tokens": 10, "output_tokens": 20}})
	}))
	cfg := Config{Enabled: true, Endpoint: "https://model.invalid", HTTPTransport: server, Model: "test", TokenLimit: 100000}
	doc, usage, err := Run(context.Background(), cfg, Usage{}, "test-key", "explain", []Evidence{{ID: "e1", Source: "func F() int { return 0 }"}})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(doc.Claims) != 1 || doc.Claims[0].Status != "supported" {
		t.Fatalf("bad checked document: %+v calls=%d", doc, calls)
	}
	if usage.ActualTokens != 60 {
		t.Fatalf("usage=%+v", usage)
	}
}

func TestReviewFailureAccountsForUsage(t *testing.T) {
	const answerTokens = 2 // modeFixtureResponse reports one input and one output token.
	for _, tc := range []struct {
		name     string
		httpCode int
		stop     string
		tokens   int
	}{
		{name: "http error", httpCode: http.StatusInternalServerError},
		{name: "missing usage", stop: "end_turn"},
		{name: "truncated with exact usage", stop: "max_tokens", tokens: 17},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, reviewEstimate := 0, 0
			server := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls == 1 {
					_ = json.NewEncoder(w).Encode(modeFixtureResponse("anthropic", `{"claims":[{"id":"c1","text":"zero","kind":"behavior","evidence_ids":["e1"]}]}`, "end_turn"))
					return
				}
				var request struct {
					System    string `json:"system"`
					MaxTokens int    `json:"max_tokens"`
					Messages  []struct{ Content string }
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Messages) != 1 {
					t.Fatalf("invalid review request: %+v, %v", request, err)
				}
				reviewEstimate = estimateTokens(len(request.System)+len(request.Messages[0].Content), request.MaxTokens)
				if tc.httpCode != 0 {
					w.WriteHeader(tc.httpCode)
					return
				}
				response := modeFixtureResponse("anthropic", `{"checks":[{"claim_id":"c1","status":"supported"}]}`, tc.stop)
				if tc.tokens == 0 {
					delete(response, "usage")
				} else {
					response["usage"] = map[string]int{"input_tokens": tc.tokens - 5, "output_tokens": 5}
				}
				_ = json.NewEncoder(w).Encode(response)
			}))
			cfg := Config{Enabled: true, Endpoint: "https://model.invalid", HTTPTransport: server, Model: "test", TokenLimit: 100000, VerificationMode: ModeLight}
			evidence := []Evidence{{ID: "e1", Source: "return 0"}}
			doc, usage, err := Run(context.Background(), cfg, Usage{}, "key", "explain", evidence)
			if err != nil || calls != 2 || len(doc.Claims) != 1 || doc.Claims[0].Status != "uncertain" {
				t.Fatalf("review failure must leave the claim unverified: calls=%d doc=%+v err=%v", calls, doc, err)
			}
			want := Usage{ActualTokens: answerTokens + tc.tokens}
			if tc.tokens == 0 {
				want.UncertainTokens = reviewEstimate
			}
			if usage != want {
				t.Fatalf("usage=%+v, want %+v", usage, want)
			}
		})
	}
}

func TestRunAccountsKnownUsageOnIncompleteResponse(t *testing.T) {
	server := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"finish_reason": "length", "message": map[string]string{"content": `{"claims":[]}`}}},
			"usage":   map[string]int{"total_tokens": 17},
		})
	}))
	_, usage, err := Run(context.Background(), Config{Enabled: true, Provider: "openai-compatible", BaseURL: "https://model.invalid", HTTPTransport: server, Model: "fixture", TokenLimit: 1000000}, Usage{}, "key", "explain", []Evidence{{ID: "e1", Source: "return 0"}})
	if err == nil || usage.ActualTokens != 17 || usage.UncertainTokens != 0 {
		t.Fatalf("known failed-response usage was not charged exactly: usage=%+v err=%v", usage, err)
	}
}

func TestReviewRequestEstimateCanSkipSecondCall(t *testing.T) {
	question := "explain"
	evidence := []Evidence{{ID: "e1", File: "main.go", StartLine: 1, EndLine: 2, Source: "return 1"}}
	reserve := ReservationForMode(ModeLight, question, evidence)
	calls := 0
	server := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": `{"claims":[{"id":"c1","text":"returns one","kind":"behavior","evidence_ids":["e1"]}]}`}}},
			"usage":   map[string]int{"total_tokens": reserve - 1},
		})
	}))
	tokenLimit := reserve
	doc, usage, err := Run(context.Background(), Config{Enabled: true, Provider: "openai-compatible", BaseURL: "https://model.invalid", HTTPTransport: server, Model: "fixture", TokenLimit: tokenLimit, VerificationMode: ModeLight}, Usage{}, "key", question, evidence)
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := profileForRun(ModeLight, question, evidence, false)
	reviewPayload, _ := json.Marshal(map[string]any{"claims": doc.Claims, "evidence": citedEvidence(doc.Claims, evidence)})
	reviewEstimate := estimateTokens(len(reviewPayload)+len(profile.reviewPrompt), profile.reviewLimit)
	if reviewEstimate <= usage.Remaining(tokenLimit) {
		t.Fatalf("fixture did not exceed remaining budget: estimate=%d remaining=%d", reviewEstimate, usage.Remaining(tokenLimit))
	}
	if calls != 1 || len(doc.Claims) != 1 || doc.Claims[0].Status != "uncertain" || usage.ActualTokens != reserve-1 || usage.UncertainTokens != 0 {
		t.Fatalf("review was not skipped at actual-request admission: calls=%d doc=%+v usage=%+v", calls, doc, usage)
	}
	if !strings.Contains(strings.Join(doc.Unknowns, " "), "剩余预算不足以检查证据") {
		t.Fatalf("skip reason missing: %+v", doc.Unknowns)
	}
}

func TestStructuredGenerationUsesBudgetAndAccountsUsage(t *testing.T) {
	calls := 0
	server := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload map[string]any
		if json.NewDecoder(r.Body).Decode(&payload) != nil {
			t.Error("bad request")
		}
		if payload["max_tokens"] != float64(8192) {
			t.Error("output allowance wrong")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"stop_reason": "end_turn", "content": []any{map[string]string{"type": "text", "text": `{"feedback":"fixture"}`}}, "usage": map[string]int{"input_tokens": 10, "output_tokens": 5}})
	}))
	cfg := Config{Enabled: true, Model: "test", Endpoint: "https://model.invalid", HTTPTransport: server, TokenLimit: 1}
	if _, _, err := Structured(context.Background(), cfg, Usage{}, "key", "prompt"); err == nil || calls != 0 {
		t.Fatal("budget bypass")
	}
	cfg.TokenLimit = 200000
	raw, usage, err := Structured(context.Background(), cfg, Usage{}, "key", "prompt")
	if err != nil || raw != `{"feedback":"fixture"}` || usage.ActualTokens != 15 {
		t.Fatalf("%s %+v %v", raw, usage, err)
	}
}
