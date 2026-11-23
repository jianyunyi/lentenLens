package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"intentlens/internal/explain"
)

// Regression: explain.Config.Endpoint is a test-only seam tagged `json:"-"`.
// Project/user data must never be able to set it, and HTTPS/address
// validation in Resolved() must still apply to base_url.
func TestSettingsEndpointIsNeverLoadedFromDisk(t *testing.T) {
	writeSettings := func(t *testing.T, a *App, raw string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(a.State, "settings.json"), []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("endpoint field in settings.json is ignored", func(t *testing.T) {
		a, root := fixture(t)
		writeSettings(t, a, `{"model":{
			"enabled":true,
			"provider":"openai-compatible",
			"base_url":"https://api.example.com/v1",
			"model":"fixture",
			"token_limit":1000,
			"endpoint":"https://attacker.invalid/steal",
			"Endpoint":"https://attacker.invalid/steal"
		}}`)
		reopened, err := Open(root, filepath.Dir(a.State))
		if err != nil {
			t.Fatal(err)
		}
		s, err := reopened.Config()
		if err != nil {
			t.Fatal(err)
		}
		if s.Model.Endpoint != "" {
			t.Fatalf("Endpoint loaded from disk: %q", s.Model.Endpoint)
		}
		if s.Model.BaseURL != "https://api.example.com/v1" {
			t.Fatalf("unexpected base_url: %q", s.Model.BaseURL)
		}
	})

	t.Run("endpoint cannot rescue an invalid base_url", func(t *testing.T) {
		for name, base := range map[string]string{
			"plain http to remote host": "http://attacker.invalid/v1",
			"credentials in url":        "https://user:pass@api.example.com/v1",
			"query string":              "https://api.example.com/v1?x=1",
			"fragment":                  "https://api.example.com/v1#frag",
		} {
			t.Run(name, func(t *testing.T) {
				a, root := fixture(t)
				writeSettings(t, a, `{"model":{
					"enabled":true,
					"provider":"openai-compatible",
					"base_url":"`+base+`",
					"model":"fixture",
					"token_limit":1000,
					"endpoint":"https://api.example.com/v1/chat/completions"
				}}`)
				reopened, err := Open(root, filepath.Dir(a.State))
				if err != nil {
					return // rejected at load time: fail-closed, acceptable
				}
				if _, err = reopened.Config(); err == nil {
					t.Fatalf("invalid base_url %q accepted", base)
				}
			})
		}
	})

	t.Run("in-memory Endpoint is never persisted", func(t *testing.T) {
		a, _ := fixture(t)
		s := Settings{Model: explain.Config{
			Enabled:    true,
			Provider:   "openai-compatible",
			BaseURL:    "https://api.example.com/v1",
			Model:      "fixture",
			TokenLimit: 1000,
			Endpoint:   "https://attacker.invalid/steal",
		}}
		if err := a.Configure(s, true); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(a.State, "settings.json"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(string(b)), "endpoint") || strings.Contains(string(b), "attacker.invalid") {
			t.Fatalf("Endpoint persisted: %s", b)
		}
	})
}
