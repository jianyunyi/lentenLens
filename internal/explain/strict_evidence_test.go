package explain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

type strictFixtureTransport struct {
	t     *testing.T
	texts []string
	calls int
}

func (f *strictFixtureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.t.Helper()
	if req.URL.Hostname() != "offline.invalid" {
		f.t.Errorf("unexpected non-fixture request target: %s", req.URL.Host)
		return nil, errors.New("strict test transport rejected non-fixture target")
	}
	if f.calls >= len(f.texts) {
		return nil, errors.New("unexpected extra request in strict test")
	}
	text := f.texts[f.calls]
	f.calls++
	body, err := json.Marshal(map[string]any{
		"choices": []any{map[string]any{
			"finish_reason": "stop",
			"message":       map[string]string{"content": text},
		}},
		"usage": map[string]int{"total_tokens": 2},
	})
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}, nil
}

func runStrictEvidenceFixture(t *testing.T, claim Claim, evidence []Evidence) Document {
	t.Helper()
	answer, err := json.Marshal(Document{Claims: []Claim{claim}})
	if err != nil {
		t.Fatalf("marshal answer fixture: %v", err)
	}
	transport := &strictFixtureTransport{
		t:     t,
		texts: []string{string(answer), `{"checks":[{"claim_id":"c1","status":"supported"}]}`},
	}
	doc, _, err := Run(context.Background(), Config{
		Enabled:          true,
		Provider:         "openai-compatible",
		BaseURL:          "https://offline.invalid/v1",
		Model:            "offline-fixture",
		TokenLimit:       1_000_000,
		VerificationMode: ModeStrict,
		HTTPTransport:    transport,
	}, Usage{}, "offline-fixture-key", "explain Value", evidence)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if transport.calls != 2 {
		t.Fatalf("calls=%d, want answer and strict review", transport.calls)
	}
	return doc
}

func TestStrictDoesNotInferPurposeWithoutCitedCallSite(t *testing.T) {
	doc := runStrictEvidenceFixture(t, Claim{
		ID:          "c1",
		Text:        "Value 用于向调用方返回固定的整数结果",
		Kind:        "behavior",
		Section:     "purpose",
		EvidenceIDs: []string{"e1"},
	}, []Evidence{{
		ID:        "e1",
		File:      "value.go",
		StartLine: 1,
		EndLine:   3,
		Source:    "func Value() int {\n\treturn 5\n}",
	}})

	if len(doc.Claims) != 1 || doc.Claims[0].Status != "uncertain" {
		t.Fatalf("inferred purpose was presented as supported: %+v", doc)
	}
	if len(doc.Unknowns) == 0 {
		t.Fatalf("missing purpose-evidence warning: %+v", doc)
	}
}

func TestStrictKeepsDirectlySupportedReturnValueChange(t *testing.T) {
	doc := runStrictEvidenceFixture(t, Claim{
		ID:          "c1",
		Text:        "Value changed from returning 4 to returning 5.",
		Kind:        "behavior",
		Section:     "contract",
		EvidenceIDs: []string{"before", "after"},
	}, []Evidence{
		{ID: "before", Role: "before", File: "value.go", StartLine: 1, EndLine: 3, Source: "func Value() int {\n\treturn 4\n}"},
		{ID: "after", Role: "after", File: "value.go", StartLine: 1, EndLine: 3, Source: "func Value() int {\n\treturn 5\n}"},
	})

	if len(doc.Claims) != 1 || doc.Claims[0].Status != "supported" {
		t.Fatalf("directly evidenced return value was not preserved: %+v", doc)
	}
}

func TestStrictPurposeCanUseCitedCallSiteEvidence(t *testing.T) {
	doc := runStrictEvidenceFixture(t, Claim{
		ID:          "c1",
		Text:        "Value 用于向调用方返回固定的整数结果",
		Kind:        "behavior",
		Section:     "purpose",
		EvidenceIDs: []string{"e1", "e2"},
	}, []Evidence{
		{ID: "e1", File: "value.go", StartLine: 1, EndLine: 3, Source: "func Value() int {\n\treturn 5\n}"},
		{ID: "e2", Kind: "call_site", File: "caller.go", StartLine: 2, EndLine: 4, Source: "func Caller() int {\n\treturn Value()\n}"},
	})

	if len(doc.Claims) != 1 || doc.Claims[0].Status != "supported" {
		t.Fatalf("cited call-site evidence was ignored: %+v", doc)
	}
}
