package app

import (
	"context"
	"intentlens/internal/explain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderSwitchPreservesUsageAndResolvesConfiguredKey(t *testing.T) {
	a, _ := fixture(t)
	t.Setenv("INTENTLENS_TEST_PROVIDER_KEY", "secret-fixture-value")
	t.Setenv("ANTHROPIC_API_KEY", "wrong-vendor-key")
	if err := a.write("usage.json", explain.Usage{ActualTokens: 100, UncertainTokens: 20}); err != nil {
		t.Fatal(err)
	}
	cfg := Settings{Model: explain.Config{Enabled: true, Provider: "openai-compatible", BaseURL: "https://example.com/v1", APIKeyEnv: "INTENTLENS_TEST_PROVIDER_KEY", Model: "fixture", TokenLimit: 1000000}}
	if err := a.Configure(cfg, false); err == nil {
		t.Fatal("configuration bypassed confirmation")
	}
	if err := a.Configure(cfg, true); err != nil {
		t.Fatal(err)
	}
	check := func(c explain.Config, u explain.Usage, key string) {
		if key != "secret-fixture-value" || c.Provider != "openai-compatible" || u.ActualTokens != 100 || u.UncertainTokens != 20 {
			t.Fatal("wrong provider/key or reset usage")
		}
	}
	a.modelRun = func(ctx context.Context, c explain.Config, u explain.Usage, key, q string, ev []explain.Evidence) (explain.Document, explain.Usage, error) {
		check(c, u, key)
		return explain.Document{}, u, nil
	}
	if _, err := a.analyze(context.Background(), "fixture", "", nil); err != nil {
		t.Fatal(err)
	}
	a.exerciseRun = func(ctx context.Context, c explain.Config, u explain.Usage, key, q string) (string, explain.Usage, error) {
		check(c, u, key)
		return "{}", u, nil
	}
	if _, err := a.exerciseModel(context.Background(), "", "fixture"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(a.State, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret-fixture-value") {
		t.Fatal("key persisted")
	}
}
