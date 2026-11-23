package explain

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Regression: a Config decoded from JSON cannot redirect the outbound request
// via "endpoint"; the request must target the validated base_url.
func TestDecodedConfigRequestGoesToValidatedBaseURL(t *testing.T) {
	raw := []byte(`{
		"provider":"openai-compatible",
		"base_url":"https://api.example.com/v1",
		"enabled":true,
		"model":"fixture",
		"token_limit":1000,
		"endpoint":"https://attacker.invalid/steal"
	}`)
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Endpoint != "" {
		t.Fatalf("Endpoint decoded from JSON: %q", cfg.Endpoint)
	}

	var seen []string
	cfg.HTTPTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		seen = append(seen, req.URL.String())
		body := `{"choices":[{"finish_reason":"stop","message":{"content":"{}"}}],"usage":{"total_tokens":1}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})
	_, _, _ = callLimit(context.Background(), cfg, "key", "system", "prompt", 16)

	if len(seen) != 1 || seen[0] != "https://api.example.com/v1/chat/completions" {
		t.Fatalf("request sent to %v, want only the validated base_url endpoint", seen)
	}

	// Same JSON with a non-HTTPS remote base_url must be rejected before any transport call.
	bad := strings.Replace(string(raw), "https://api.example.com/v1", "http://attacker.invalid/v1", 1)
	var badCfg Config
	if err := json.Unmarshal([]byte(bad), &badCfg); err != nil {
		t.Fatal(err)
	}
	calls := 0
	badCfg.HTTPTransport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, io.EOF
	})
	if _, _, err := callLimit(context.Background(), badCfg, "key", "system", "prompt", 16); err == nil || calls != 0 {
		t.Fatalf("invalid base_url not rejected before transport: err=%v calls=%d", err, calls)
	}
}
