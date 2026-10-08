package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCLIStartStopJSON(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	var out bytes.Buffer
	if code := run([]string{"start", "--root", root, "--state", state}, bytes.NewBufferString(`{"session":"s"}`), &out); code != 0 {
		t.Fatal(out.String())
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package p\nfunc A() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run([]string{"stop", "--root", root, "--state", state}, bytes.NewBufferString(`{"session":"s"}`), &out); code != 0 {
		t.Fatal(out.String())
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["id"] == nil {
		t.Fatal("round not returned")
	}
}

func TestHookAlwaysFailOpenWithoutRawPayload(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"hook", "--state", t.TempDir()}, bytes.NewBufferString(`{"hook_event_name":"Stop","cwd":"missing","prompt":"secret"}`), &out); code != 0 {
		t.Fatal("hook blocked coding")
	}
	if out.Len() != 0 {
		t.Fatal("hook emits content to Claude")
	}
}

func TestHookConfigurationMatchesOfficialLifecycle(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"hooks-config"}, bytes.NewBufferString("{}"), &out); code != 0 {
		t.Fatal(out.String())
	}
	var cfg struct {
		Hooks map[string]any `json:"hooks"`
	}
	if err := json.Unmarshal(out.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Hooks["UserPromptSubmit"] == nil || cfg.Hooks["Stop"] == nil {
		t.Fatal("missing lifecycle events")
	}
}

func TestWindowsUTF8BOMInput(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"start", "--root", t.TempDir(), "--state", t.TempDir()}, bytes.NewBufferString("\xef\xbb\xbf"+`{"session":"bom"}`), &out)
	if code != 0 {
		t.Fatalf("UTF-8 BOM JSON rejected: %s", out.String())
	}
}
