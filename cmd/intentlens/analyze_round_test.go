package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"intentlens/internal/app"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
)

type cliInMemoryTransport struct{ handler http.Handler }

func (transport cliInMemoryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	recorder := httptest.NewRecorder()
	transport.handler.ServeHTTP(recorder, req)
	return recorder.Result(), nil
}

func useModelTransport(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	previous := openApp
	openApp = func(root, state string) (*app.App, error) { return app.OpenWithHTTPTransport(root, state, transport) }
	t.Cleanup(func() { openApp = previous })
}

func analyzeRoundCLI(t *testing.T, root, state string, input string) (int, map[string]any) {
	t.Helper()
	var out bytes.Buffer
	code := run([]string{"analyze-round", "--root", root, "--state", state}, bytes.NewBufferString(input), &out)
	var result map[string]any
	if json.Unmarshal(out.Bytes(), &result) != nil {
		t.Fatalf("analyze-round output is not JSON (code=%d): %s", code, out.String())
	}
	return code, result
}

func makeRound(t *testing.T, root, state string) string {
	t.Helper()
	var out bytes.Buffer
	if code := run([]string{"start", "--root", root, "--state", state}, bytes.NewBufferString(`{"session":"e2e"}`), &out); code != 0 {
		t.Fatalf("start: %s", out.String())
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package p\nfunc F() int { return 1 }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run([]string{"stop", "--root", root, "--state", state}, bytes.NewBufferString(`{"session":"e2e"}`), &out); code != 0 {
		t.Fatalf("stop: %s", out.String())
	}
	var round map[string]any
	if err := json.Unmarshal(out.Bytes(), &round); err != nil {
		t.Fatal(err)
	}
	return round["id"].(string)
}

func TestAnalyzeRoundInMemoryTransportRegeneratesAndPreservesOldExplanation(t *testing.T) {
	var calls atomic.Int64
	transport := cliInMemoryTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if call%2 == 1 {
			body, _ := io.ReadAll(r.Body)
			match := regexp.MustCompile(`id.{0,20}([a-f0-9]{20})`).FindSubmatch(body)
			if len(match) != 2 {
				t.Errorf("model prompt omitted evidence id: %s", body)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			content, _ := json.Marshal(map[string]any{"claims": []any{map[string]any{"id": "c1", "text": "adds F", "kind": "behavior", "evidence_ids": []string{string(match[1])}, "status": "supported"}}})
			response, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": string(content)}}}, "usage": map[string]int{"total_tokens": 1}})
			_, _ = w.Write(response)
		} else {
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"checks\":[]}"}}],"usage":{"total_tokens":1}}`))
		}
	})}
	useModelTransport(t, transport)
	t.Setenv("INTENTLENS_E2E_KEY", "local-only-key")
	root, state := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".intentignore"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "settings.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	config := `{"consent":true,"settings":{"model":{"enabled":true,"provider":"openai-compatible","base_url":"https://model.invalid","api_key_env":"INTENTLENS_E2E_KEY","model":"fixture","token_limit":1000000}}}`
	var out bytes.Buffer
	if configureCode := run([]string{"configure", "--root", root, "--state", state}, bytes.NewBufferString(config), &out); configureCode != 0 {
		t.Fatalf("configure: %s", out.String())
	}
	roundID := makeRound(t, root, state)
	code, result := analyzeRoundCLI(t, root, state, `{"round_id":"`+roundID+`"}`)
	if code != 0 || result["analyzed"] != true || calls.Load() != 2 {
		t.Fatalf("first analysis: code=%d result=%v calls=%d", code, result, calls.Load())
	}
	var round app.RoundDetail
	out.Reset()
	if code = run([]string{"round-detail", "--root", root, "--state", state}, bytes.NewBufferString(`{"round_id":"`+roundID+`"}`), &out); code != 0 || json.Unmarshal(out.Bytes(), &round) != nil || round.Analysis == nil {
		t.Fatalf("round detail after first analysis: code=%d output=%s", code, out.String())
	}
	first := round.Analysis.ID
	// A failed opt-in attempt records a diagnostic; enabling the model and
	// retrying this same round must remove only that round's diagnostic.
	if configureCode := run([]string{"configure", "--root", root, "--state", state}, bytes.NewBufferString(`{"consent": false, "settings": {"model": {"enabled": false}}}`), &out); configureCode != 0 {
		t.Fatalf("disable model: %s", out.String())
	}
	if retryCode, retryResult := analyzeRoundCLI(t, root, state, `{"round_id":"`+roundID+`"}`); retryCode == 0 || retryResult["analyzed"] == true {
		t.Fatalf("disabled retry unexpectedly succeeded: code=%d result=%v", retryCode, retryResult)
	}
	out.Reset()
	if statusCode := run([]string{"status", "--root", root, "--state", state}, bytes.NewBuffer(nil), &out); statusCode != 0 {
		t.Fatalf("status after failed retry: %s", out.String())
	}
	var status map[string]any
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	actualState := status["state_directory"].(string)
	if err := os.WriteFile(filepath.Join(actualState, "analysis-diagnostic.json"), []byte(`{"round":"other-round","error":"keep me"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if configureCode := run([]string{"configure", "--root", root, "--state", state}, bytes.NewBufferString(config), &out); configureCode != 0 {
		t.Fatalf("re-enable model: %s", out.String())
	}
	code, result = analyzeRoundCLI(t, root, state, `{"round_id":"`+roundID+`"}`)
	if code != 0 || result["analyzed"] != true || calls.Load() != 4 {
		t.Fatalf("second analysis: code=%d result=%v calls=%d", code, result, calls.Load())
	}
	out.Reset()
	if code = run([]string{"status", "--root", root, "--state", state}, bytes.NewBuffer(nil), &out); code != 0 || !bytes.Contains(out.Bytes(), []byte("other-round")) || bytes.Contains(out.Bytes(), []byte(roundID)) {
		t.Fatalf("diagnostic cleanup scope wrong: code=%d output=%s", code, out.String())
	}
	out.Reset()
	if code = run([]string{"round-detail", "--root", root, "--state", state}, bytes.NewBufferString(`{"round_id":"`+roundID+`"}`), &out); code != 0 || json.Unmarshal(out.Bytes(), &round) != nil || round.Analysis == nil {
		t.Fatalf("round detail after regeneration: code=%d output=%s", code, out.String())
	}
	latest := round.Analysis.ID
	if latest == first {
		t.Fatalf("round analysis was not regenerated: first=%s latest=%s", first, latest)
	}
	// Unlike the other-round case above, this diagnostic must be removed.
	if err := os.WriteFile(filepath.Join(actualState, "analysis-diagnostic.json"), []byte(`{"round":"`+roundID+`","error":"stale failure"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if retryCode, retryResult := analyzeRoundCLI(t, root, state, `{"round_id":"`+roundID+`"}`); retryCode != 0 || retryResult["analyzed"] != true || calls.Load() != 6 {
		t.Fatalf("successful diagnostic retry: code=%d result=%v calls=%d", retryCode, retryResult, calls.Load())
	}
	if _, err := os.Stat(filepath.Join(actualState, "analysis-diagnostic.json")); !os.IsNotExist(err) {
		t.Fatalf("same-round diagnostic was not deleted: %v", err)
	}
	for _, command := range []string{"rounds", "status"} {
		out.Reset()
		if commandCode := run([]string{command, "--root", root, "--state", state}, bytes.NewBufferString("{}"), &out); commandCode != 0 || bytes.Contains(out.Bytes(), []byte("stale failure")) {
			t.Fatalf("%s retained stale failure: code=%d output=%s", command, commandCode, out.String())
		}
	}
	out.Reset()
	if code = run([]string{"explanation", "--root", root, "--state", state}, bytes.NewBufferString(`{"explanation_id":"`+first+`"}`), &out); code != 0 {
		t.Fatalf("old explanation became unreadable: %s", out.String())
	}
	var preserved map[string]any
	if err := json.Unmarshal(out.Bytes(), &preserved); err != nil || preserved["id"] != first {
		t.Fatalf("wrong old explanation: %s", out.String())
	}
}

func TestAnalyzeRoundDisabledOrBudgetRejectedWithoutHTTP(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled bool
		limit   int
		error   string
	}{
		{name: "disabled", enabled: false, limit: 1000000, error: "模型分析未启用"},
		{name: "budget", enabled: true, limit: 1, error: "预算不足"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int64
			transport := cliInMemoryTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			})}
			useModelTransport(t, transport)
			t.Setenv("INTENTLENS_E2E_REJECT_KEY", "local-only-key")
			config, err := json.Marshal(map[string]any{"consent": tc.enabled, "settings": map[string]any{"model": map[string]any{"enabled": tc.enabled, "provider": "openai-compatible", "base_url": "https://model.invalid", "api_key_env": "INTENTLENS_E2E_REJECT_KEY", "model": "fixture", "token_limit": tc.limit}}})
			if err != nil {
				t.Fatal(err)
			}
			root, state := t.TempDir(), t.TempDir()
			var out bytes.Buffer
			if code := run([]string{"configure", "--root", root, "--state", state}, bytes.NewReader(config), &out); code != 0 {
				t.Fatalf("configure: %s", out.String())
			}
			roundID := makeRound(t, root, state)
			code, result := analyzeRoundCLI(t, root, state, `{"round_id":"`+roundID+`"}`)
			message, _ := result["error"].(string)
			if code == 0 || result["analyzed"] == true || calls.Load() != 0 || !strings.Contains(message, tc.error) {
				t.Fatalf("wrong rejection or sent HTTP: code=%d result=%v calls=%d", code, result, calls.Load())
			}
		})
	}
}

func TestRepeatedStopHookAutomaticallyAnalyzesOnceAcrossAppRestarts(t *testing.T) {
	var calls atomic.Int64
	transport := cliInMemoryTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if call%2 == 1 {
			body, _ := io.ReadAll(r.Body)
			match := regexp.MustCompile(`id.{0,20}([a-f0-9]{20})`).FindSubmatch(body)
			if len(match) != 2 {
				t.Errorf("model prompt omitted evidence id: %s", body)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			content, _ := json.Marshal(map[string]any{"claims": []any{map[string]any{"id": "c1", "text": "adds F", "kind": "behavior", "evidence_ids": []string{string(match[1])}, "status": "supported"}}})
			response, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": string(content)}}}, "usage": map[string]int{"total_tokens": 1}})
			_, _ = w.Write(response)
		} else {
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"checks\":[]}"}}],"usage":{"total_tokens":1}}`))
		}
	})}
	root, state := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".intentignore"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	openCount := 0
	previousOpen, previousLaunch := openApp, launchHookAnalysis
	openApp = func(root, state string) (*app.App, error) {
		openCount++
		return app.OpenWithHTTPTransport(root, state, transport)
	}
	t.Cleanup(func() { openApp, launchHookAnalysis = previousOpen, previousLaunch })
	launchHookAnalysis = func(root, state, roundID string) error {
		var out bytes.Buffer
		if code := run([]string{"analyze-round", "--root", root, "--state", state, "--round", roundID, "--automatic"}, bytes.NewReader(nil), &out); code != 0 {
			return fmt.Errorf("automatic analysis failed: %s", out.String())
		}
		return nil
	}
	config := `{"consent":true,"settings":{"model":{"enabled":true,"provider":"openai-compatible","base_url":"https://model.invalid","api_key_env":"INTENTLENS_HOOK_TEST_KEY","model":"fixture","token_limit":1000000}}}`
	t.Setenv("INTENTLENS_HOOK_TEST_KEY", "local-only-key")
	var out bytes.Buffer
	if code := run([]string{"configure", "--root", root, "--state", state}, bytes.NewBufferString(config), &out); code != 0 {
		t.Fatalf("configure: %s", out.String())
	}
	event := func(name string) {
		t.Helper()
		payload, _ := json.Marshal(map[string]string{"hook_event_name": name, "session_id": "hook-session", "cwd": root})
		if err := handleHook(bytes.NewReader(payload), state); err != nil {
			t.Fatalf("%s hook: %v", name, err)
		}
	}
	event("UserPromptSubmit")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package p\nfunc F() int { return 1 }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	event("Stop")
	if calls.Load() != 2 {
		t.Fatalf("first Stop hook made %d model requests, want one two-stage analysis", calls.Load())
	}
	openedAfterFirstStop := openCount
	event("Stop")
	if calls.Load() != 2 {
		t.Fatalf("replayed Stop hook made duplicate model requests: %d", calls.Load())
	}
	if openCount <= openedAfterFirstStop {
		t.Fatal("replayed hook did not use a new App instance")
	}

	a, err := app.Open(root, state)
	if err != nil {
		t.Fatal(err)
	}
	rounds, err := a.List()
	if err != nil || len(rounds) != 1 || rounds[0].Analysis == nil {
		ids := make([]string, len(rounds))
		for i := range rounds {
			ids[i] = rounds[i].ID
		}
		t.Fatalf("round analysis missing or duplicated: rounds=%d ids=%v err=%v", len(rounds), ids, err)
	}
	entries, err := os.ReadDir(a.State)
	if err != nil {
		t.Fatal(err)
	}
	explanations := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "explanation-") && strings.HasSuffix(entry.Name(), ".json") {
			explanations++
		}
	}
	if explanations != 1 {
		t.Fatalf("saved %d explanations after duplicate hook, want 1", explanations)
	}
	markerPath := filepath.Join(a.State, "auto-analysis-round-"+rounds[0].ID+".json")
	marker, err := os.ReadFile(markerPath)
	var markerData map[string]string
	if err != nil || json.Unmarshal(marker, &markerData) != nil || markerData["round_id"] != rounds[0].ID {
		t.Fatalf("automatic analysis marker missing or not bound to the round: %v", err)
	}

	var manual bytes.Buffer
	if code := run([]string{"analyze-round", "--root", root, "--state", state, "--round", rounds[0].ID}, bytes.NewReader(nil), &manual); code != 0 {
		t.Fatalf("manual reanalysis was blocked: %s", manual.String())
	}
	if calls.Load() != 4 {
		t.Fatalf("manual reanalysis made %d total model requests, want 4", calls.Load())
	}
}
