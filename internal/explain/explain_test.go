package explain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExplainRequiresConsentAndReservesBudget(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer server.Close()
	cfg := Config{Endpoint: server.URL, Model: "test", TokenLimit: 1}
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("x-api-key") != "test-key" {
			t.Error("missing API key")
		}
		text := `{"claims":[{"id":"c1","text":"returns zero","kind":"behavior","evidence_ids":["e1"]},{"id":"c2","text":"invented","kind":"motive","evidence_ids":["fake"]}],"unknowns":["choice not recorded"]}`
		if calls == 2 {
			text = `{"checks":[{"claim_id":"c1","status":"supported"}]}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}, "usage": map[string]int{"input_tokens": 10, "output_tokens": 20}})
	}))
	defer server.Close()
	cfg := Config{Enabled: true, Endpoint: server.URL, Model: "test", TokenLimit: 100000}
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

func TestReviewFailureDoesNotPublishVerifiedClaims(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			w.WriteHeader(500)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"content": []any{map[string]any{"type": "text", "text": `{"claims":[{"id":"c1","text":"zero","kind":"behavior","evidence_ids":["e1"]}]}`}}, "usage": map[string]int{"input_tokens": 10, "output_tokens": 10}})
	}))
	defer server.Close()
	doc, usage, err := Run(context.Background(), Config{Enabled: true, Endpoint: server.URL, Model: "test", TokenLimit: 100000}, Usage{}, "key", "explain", []Evidence{{ID: "e1", Source: "return 0"}})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Claims[0].Status != "uncertain" || usage.UncertainTokens == 0 {
		t.Fatalf("failure presented as verified: %+v %+v", doc, usage)
	}
}
