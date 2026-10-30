package explain

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestClaimSectionsPreserveEvidenceChecks(t *testing.T) {
	calls := 0
	server := newInMemoryRoundTripper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		text := `{"claims":[{"id":"a","text":"计算总额","kind":"behavior","section":"purpose","evidence_ids":["e"]},{"id":"b","text":"检查输入","kind":"behavior","section":"flow","evidence_ids":["e"]},{"id":"c","text":"未知分类","kind":"behavior","section":"invented","evidence_ids":["e"]},{"id":"d","text":"无依据","kind":"behavior","section":"design","evidence_ids":["missing"]}]}`
		if calls == 2 {
			text = `{"checks":[{"claim_id":"a","status":"supported"},{"claim_id":"b","status":"unsupported"},{"claim_id":"c","status":"uncertain"}]}`
		}
		json.NewEncoder(w).Encode(map[string]any{"stop_reason": "end_turn", "content": []any{map[string]string{"type": "text", "text": text}}, "usage": map[string]int{"input_tokens": 2, "output_tokens": 2}})
	}))
	doc, _, err := Run(context.Background(), Config{Enabled: true, Model: "fixture", Endpoint: "https://model.invalid", HTTPTransport: server, TokenLimit: 1000000}, Usage{}, "key", "fixture", []Evidence{{ID: "e", Source: "return 1"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Claims) != 2 || doc.Claims[0].Section != "purpose" || doc.Claims[0].Status != "supported" || doc.Claims[1].Section != "" || len(doc.Unknowns) < 2 {
		t.Fatalf("classification bypassed checking: %+v", doc)
	}
}
