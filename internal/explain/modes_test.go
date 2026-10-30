package explain

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func modeFixtureResponse(provider, text, finish string) map[string]any {
	switch provider {
	case "anthropic":
		return map[string]any{"stop_reason": finish, "content": []any{map[string]string{"type": "text", "text": text}}, "usage": map[string]int{"input_tokens": 1, "output_tokens": 1}}
	case "gemini":
		return map[string]any{"candidates": []any{map[string]any{"finishReason": finish, "content": map[string]any{"parts": []any{map[string]string{"text": text}}}}}, "usageMetadata": map[string]int{"totalTokenCount": 2}}
	default:
		return map[string]any{"choices": []any{map[string]any{"finish_reason": finish, "message": map[string]string{"content": text}}}, "usage": map[string]int{"total_tokens": 2}}
	}
}

func modeFixtureFinish(provider string) string {
	if provider == "anthropic" {
		return "end_turn"
	}
	if provider == "gemini" {
		return "STOP"
	}
	return "stop"
}

func modeRequestLimit(provider string, body map[string]any) int {
	switch provider {
	case "anthropic":
		return int(body["max_tokens"].(float64))
	case "openai":
		return int(body["max_completion_tokens"].(float64))
	case "gemini":
		return int(body["generationConfig"].(map[string]any)["maxOutputTokens"].(float64))
	default:
		return int(body["max_tokens"].(float64))
	}
}

func TestVerificationModesUseExpectedRequestCountsAndCapsAcrossProviders(t *testing.T) {
	for _, provider := range []string{"anthropic", "openai", "openai-compatible", "gemini"} {
		for _, mode := range []string{ModeLight, ModeFast, ModeStrict} {
			t.Run(provider+"/"+mode, func(t *testing.T) {
				calls := 0
				var requests []map[string]any
				server := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					requests = append(requests, body)
					text := `{"claims":[{"id":"c1","text":"returns one","kind":"behavior","section":"contract","evidence_ids":["e1"]}]}`
					if calls == 2 {
						text = `{"checks":[{"claim_id":"c1","status":"supported"}]}`
					}
					_ = json.NewEncoder(w).Encode(modeFixtureResponse(provider, text, modeFixtureFinish(provider)))
				}))

				evidence := []Evidence{
					{ID: "e1", File: "main.go", StartLine: 7, EndLine: 9, Source: "func F() int { return 1 }"},
					{ID: "unused", File: "unused.go", StartLine: 20, EndLine: 30, Source: "func Unused() {}"},
				}
				doc, usage, err := Run(context.Background(), Config{Enabled: true, Provider: provider, BaseURL: "https://model.invalid", HTTPTransport: server, Model: "fixture", TokenLimit: 1000000, VerificationMode: mode}, Usage{}, "local-test-key", "explain", evidence)
				if err != nil {
					t.Fatal(err)
				}
				wantCalls, wantFirst, wantReview := 2, lightAnswerLimit, lightReviewLimit
				if mode == ModeFast {
					wantCalls, wantFirst, wantReview = 1, fastAnswerLimit, 0
				} else if mode == ModeStrict {
					wantFirst, wantReview = strictAnswerLimit, strictReviewLimit
				}
				if calls != wantCalls || modeRequestLimit(provider, requests[0]) != wantFirst {
					t.Fatalf("calls=%d first-limit=%v, want %d calls and %d tokens", calls, modeRequestLimit(provider, requests[0]), wantCalls, wantFirst)
				}
				if mode != ModeFast && modeRequestLimit(provider, requests[1]) != wantReview {
					t.Fatalf("review limit=%v, want %d", modeRequestLimit(provider, requests[1]), wantReview)
				}
				if len(doc.Claims) != 1 {
					t.Fatalf("claims=%+v", doc.Claims)
				}
				if mode == ModeFast {
					if doc.Claims[0].Status != "uncertain" || !strings.Contains(strings.Join(doc.Unknowns, " "), "未进行独立证据复核") {
						t.Fatalf("fast mode overstated verification: %+v", doc)
					}
				} else if doc.Claims[0].Status != "supported" {
					t.Fatalf("two-stage status=%q", doc.Claims[0].Status)
				}
				userPrompt := requestUserPrompt(provider, requests[0])
				if modeRequestLimit(provider, requests[0]) == 0 || !strings.Contains(userPrompt, "main.go") || !strings.Contains(userPrompt, `"start_line":7`) {
					t.Fatalf("request omitted evidence location: %s", requestJSON(requests[0]))
				}
				if mode != ModeFast && strings.Contains(requestUserPrompt(provider, requests[1]), "unused.go") {
					t.Fatalf("review context was not reduced to cited evidence: %s", requestJSON(requests[1]))
				}
				if usage.ActualTokens != 2*wantCalls {
					t.Fatalf("usage=%+v", usage)
				}
			})
		}
	}
}

func requestJSON(body map[string]any) string {
	encoded, _ := json.Marshal(body)
	return string(encoded)
}

func requestUserPrompt(provider string, body map[string]any) string {
	switch provider {
	case "gemini":
		contents := body["contents"].([]any)
		parts := contents[0].(map[string]any)["parts"].([]any)
		return parts[0].(map[string]any)["text"].(string)
	default:
		messages := body["messages"].([]any)
		index := 1
		if provider == "anthropic" {
			index = 0
		}
		return messages[index].(map[string]any)["content"].(string)
	}
}

func TestFastModeAutomaticallyUpgradesLargeContextToStrict(t *testing.T) {
	calls := 0
	limits := []int{}
	server := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		limits = append(limits, modeRequestLimit("openai-compatible", body))
		text := `{"claims":[{"id":"c1","text":"bounded claim","kind":"behavior","evidence_ids":["e1"]}]}`
		if calls == 2 {
			text = `{"checks":[{"claim_id":"c1","status":"supported"}]}`
		}
		_ = json.NewEncoder(w).Encode(modeFixtureResponse("openai-compatible", text, "stop"))
	}))

	doc, _, err := Run(context.Background(), Config{Enabled: true, Provider: "openai-compatible", BaseURL: "https://model.invalid", HTTPTransport: server, Model: "fixture", TokenLimit: 1000000, VerificationMode: ModeFast}, Usage{}, "key", "small question", []Evidence{{ID: "e1", File: "large.go", StartLine: 1, EndLine: 500, Source: strings.Repeat("x", fastMaxEvidenceBytes+1)}})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(limits) != 2 || limits[0] != strictAnswerLimit || limits[1] != strictReviewLimit {
		t.Fatalf("large fast request was not upgraded: calls=%d limits=%v", calls, limits)
	}
	if len(doc.Claims) != 1 || doc.Claims[0].Status != "supported" || !strings.Contains(strings.Join(doc.Unknowns, " "), "自动升级为严格") {
		t.Fatalf("upgrade path not disclosed: %+v", doc)
	}
}

func TestFastReservationIsSmallerAndDefaultIsLight(t *testing.T) {
	evidence := []Evidence{{ID: "e1", File: "main.go", StartLine: 1, EndLine: 2, Source: "func F() {}"}}
	light := ReservationForMode(ModeLight, "explain", evidence)
	fast := ReservationForMode(ModeFast, "explain", evidence)
	if fast >= light {
		t.Fatalf("fast=%d should reserve less than light=%d", fast, light)
	}
	resolved, err := (Config{}).Resolved()
	if err != nil || resolved.VerificationMode != ModeLight {
		t.Fatalf("default config=%+v err=%v", resolved, err)
	}
	if _, err = (Config{VerificationMode: "unknown"}).Resolved(); err == nil {
		t.Fatal("unknown verification mode accepted")
	}
}

func TestDeepSeekThinkingDisabledIsProviderAndModeSpecific(t *testing.T) {
	for _, tc := range []struct {
		name     string
		provider string
		baseURL  string
		mode     string
		want     bool
	}{
		{name: "light deepseek", provider: "openai-compatible", baseURL: "https://api.deepseek.com/v1", mode: ModeLight, want: true},
		{name: "fast deepseek", provider: "openai-compatible", baseURL: "https://api.deepseek.com/v1", mode: ModeFast, want: true},
		{name: "strict deepseek", provider: "openai-compatible", baseURL: "https://api.deepseek.com/v1", mode: ModeStrict},
		{name: "other compatible provider", provider: "openai-compatible", baseURL: "https://api.example.test/v1", mode: ModeLight},
		{name: "openai provider", provider: "openai", baseURL: "https://api.deepseek.com/v1", mode: ModeLight},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				thinking, ok := body["thinking"].(map[string]any)
				if ok != tc.want || (tc.want && thinking["type"] != "disabled") {
					t.Errorf("thinking field=%v, want disabled=%v", body["thinking"], tc.want)
				}
				_ = json.NewEncoder(w).Encode(modeFixtureResponse("openai-compatible", `{}`, "stop"))
			}))
			_, _, err := callLimitMode(context.Background(), Config{Provider: tc.provider, BaseURL: tc.baseURL, Endpoint: "https://model.invalid", HTTPTransport: server, Model: "fixture"}, "test-key", "system", "prompt", 128, tc.mode)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLightReviewTruncationNeverMarksClaimsSupported(t *testing.T) {
	calls := 0
	server := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		text, finish := `{"claims":[{"id":"c1","text":"returns one","kind":"behavior","evidence_ids":["e1"]}]}`, "stop"
		if calls == 2 {
			text, finish = `{"checks":[{"claim_id":"c1","status":"supported"}]}`, "length"
		}
		_ = json.NewEncoder(w).Encode(modeFixtureResponse("openai-compatible", text, finish))
	}))

	doc, usage, err := Run(context.Background(), Config{Enabled: true, Provider: "openai-compatible", BaseURL: "https://model.invalid", HTTPTransport: server, Model: "fixture", TokenLimit: 1000000, VerificationMode: ModeLight}, Usage{}, "key", "explain", []Evidence{{ID: "e1", File: "main.go", StartLine: 1, EndLine: 2, Source: "return 1"}})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(doc.Claims) != 1 || doc.Claims[0].Status != "uncertain" || usage.UncertainTokens == 0 || !strings.Contains(strings.Join(doc.Unknowns, " "), "证据支持检查失败") {
		t.Fatalf("truncated review was overstated: calls=%d doc=%+v usage=%+v", calls, doc, usage)
	}
}

func TestLightReviewRejectsProviderSpecificTruncationReasons(t *testing.T) {
	for _, provider := range []string{"anthropic", "openai", "openai-compatible", "gemini"} {
		t.Run(provider, func(t *testing.T) {
			valid, truncated := modeFixtureFinish(provider), "length"
			if provider == "anthropic" {
				truncated = "max_tokens"
			} else if provider == "gemini" {
				truncated = "MAX_TOKENS"
			}
			if _, _, err := decodeResponse(provider, mustJSON(modeFixtureResponse(provider, `{}`, truncated))); err == nil {
				t.Fatalf("provider %s accepted truncation reason %s", provider, truncated)
			}
			if _, _, err := decodeResponse(provider, mustJSON(modeFixtureResponse(provider, `{}`, valid))); err != nil {
				t.Fatalf("provider %s rejected normal finish marker %s: %v", provider, valid, err)
			}
		})
	}
}

func mustJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func TestProviderMissingFinishSignalIsIncomplete(t *testing.T) {
	for _, provider := range []string{"anthropic", "openai", "openai-compatible", "gemini"} {
		t.Run(provider, func(t *testing.T) {
			text, _, err := decodeResponse(provider, mustJSON(modeFixtureResponse(provider, `{}`, "")))
			if err == nil || text != "" || !strings.Contains(err.Error(), "结束标记") {
				t.Fatalf("missing finish signal accepted: text=%q err=%v", text, err)
			}
		})
	}
}

func TestProviderTruncationSignalsAreReportedWithoutPartialText(t *testing.T) {
	for _, tc := range []struct {
		provider string
		reason   string
	}{
		{"anthropic", "max_tokens"},
		{"openai", "length"},
		{"openai-compatible", "length"},
		{"gemini", "MAX_TOKENS"},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			text, _, err := decodeResponse(tc.provider, mustJSON(modeFixtureResponse(tc.provider, `{"partial":`, tc.reason)))
			if err == nil || text != "" || !strings.Contains(err.Error(), "截断") {
				t.Fatalf("truncated partial text accepted or misreported: text=%q err=%v", text, err)
			}
		})
	}
}
