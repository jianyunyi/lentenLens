package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegrationInfoNeedsNeitherProjectNorInput(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"integration-info", "--root", filepath.Join(t.TempDir(), "missing")}, nil, &out)
	var info struct {
		Schema        int      `json:"schema"`
		Entries       []string `json:"entries"`
		ModelCommands []string `json:"model_commands"`
		ModelRequired bool     `json:"model_required_for_local_evidence"`
		Event         struct {
			Command string   `json:"command"`
			Events  []string `json:"events"`
		} `json:"capture_event"`
	}
	if code != 0 || json.Unmarshal(out.Bytes(), &info) != nil || info.Schema != 1 || info.ModelRequired || strings.Join(info.Entries, ",") != "ide,skill,claude" || info.Event.Command != "capture-event" || strings.Join(info.Event.Events, ",") != "start,stop" || !strings.Contains(strings.Join(info.ModelCommands, ","), "test-model") {
		t.Fatalf("invalid integration contract: %s", out.String())
	}
}

func TestIntegrationInfoDescribesIndexCoverage(t *testing.T) {
	var info struct {
		IndexCoverage struct {
			Responses           []string `json:"responses"`
			Fields              []string `json:"fields"`
			OmittedExamplesMax  int      `json:"omitted_examples_max"`
			ExcludedPathsListed bool     `json:"excluded_paths_listed"`
			CallStatuses        []string `json:"call_statuses"`
			PartialIndexWarning string   `json:"partial_index_warning"`
			CallStatusWarning   string   `json:"call_status_warning"`
		} `json:"index_coverage"`
	}
	b, err := json.Marshal(integrationInfo())
	if err != nil || json.Unmarshal(b, &info) != nil {
		t.Fatalf("invalid integration-info JSON: %v", err)
	}
	fields := map[string]bool{}
	for _, field := range info.IndexCoverage.Fields {
		fields[field] = true
	}
	statuses := strings.Join(info.IndexCoverage.CallStatuses, ",")
	if !strings.Contains(strings.Join(info.IndexCoverage.Responses, ","), "index.coverage") || !strings.Contains(strings.Join(info.IndexCoverage.Responses, ","), "inspect.coverage") || !fields["complete"] || !fields["omitted_count"] || !fields["excluded_count"] || !fields["truncated"] || info.IndexCoverage.OmittedExamplesMax != 20 || info.IndexCoverage.ExcludedPathsListed || statuses != "resolved,unresolved,ambiguous" || info.IndexCoverage.PartialIndexWarning == "" || info.IndexCoverage.CallStatusWarning == "" {
		t.Fatalf("incomplete index coverage contract: %+v", info.IndexCoverage)
	}
}

func TestGenericCaptureEventsDoNotRequireClaude(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	call := func(payload string) map[string]any {
		t.Helper()
		var out bytes.Buffer
		if code := run([]string{"capture-event", "--root", root, "--state", state}, strings.NewReader(payload), &out); code != 0 {
			t.Fatalf("%s: %s", payload, out.String())
		}
		var response map[string]any
		if err := json.Unmarshal(out.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response
	}
	before := call(`{"event":"start","session":"skill-session","prompt":"PRIVATE_SKILL_PROMPT","transcript":"PRIVATE_SKILL_TRANSCRIPT"}`)
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package p\nfunc F() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	after := call(`{"event":"stop","session":"skill-session"}`)
	changes, ok := after["changes"].([]any)
	if !ok || len(changes) != 1 || before["id"] != after["id"] || after["source"] != "unconfirmed" {
		t.Fatalf("invalid round: %v", after)
	}
	if err := filepath.WalkDir(state, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, e := os.ReadFile(p)
		if strings.Contains(string(b), "PRIVATE_SKILL_") {
			t.Fatalf("raw agent input persisted: %s", p)
		}
		return e
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGenericCaptureErrorsAreNotFailOpen(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	for _, payload := range []string{`{`, `{"event":"unknown","session":"s"}`, `{"event":"start"}`, `{"event":"start","session":"  "}`, `{"event":"stop","session":"missing"}`} {
		var out bytes.Buffer
		code := run([]string{"capture-event", "--root", root, "--state", state}, strings.NewReader(payload), &out)
		var response map[string]any
		if code == 0 || json.Unmarshal(out.Bytes(), &response) != nil || response["error"] == nil {
			t.Fatalf("failure hidden: %q %s", payload, out.String())
		}
	}
}
