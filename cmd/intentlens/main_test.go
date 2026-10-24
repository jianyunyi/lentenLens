package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type failOnRead struct{}

func (failOnRead) Read([]byte) (int, error) { return 0, errors.New("stdin was read") }

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

func TestSharingCLIRejectsUnconfirmedOrMissingVersions(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	for _, command := range []string{"share-confirm", "share-prepare", "flag", "revise", "adopt-shared", "explanation"} {
		var out bytes.Buffer
		code := run([]string{command, "--root", root, "--state", state}, bytes.NewBufferString(`{"explanation_id":"../bad","confirm":false}`), &out)
		var response map[string]any
		if code == 0 || json.Unmarshal(out.Bytes(), &response) != nil || response["error"] == nil {
			t.Fatalf("%s: %s", command, out.String())
		}
	}
	var out bytes.Buffer
	if code := run([]string{"shared", "--root", root, "--state", state}, bytes.NewBuffer(nil), &out); code != 0 || out.String() != "[]\n" {
		t.Fatal(out.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".intent")); !os.IsNotExist(err) {
		t.Fatal("read/rejected operations wrote repository")
	}
}

func TestExerciseCLINeverExecutesWithoutConfirmation(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	for _, command := range []string{"exercise-run", "exercise-apply-confirm", "exercise-share-confirm", "exercise-generate", "exercise-import"} {
		var out bytes.Buffer
		if code := run([]string{command, "--root", root, "--state", state}, bytes.NewBufferString(`{"exercise_id":"../bad","confirm":false}`), &out); code == 0 {
			t.Fatalf("%s unexpectedly succeeded", command)
		}
	}
	var out bytes.Buffer
	if code := run([]string{"exercises", "--root", root, "--state", state}, bytes.NewBuffer(nil), &out); code != 0 || out.String() != "[]\n" {
		t.Fatal(out.String())
	}
}

func TestAnalyzeRoundAcceptsJSONRoundID(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	var out bytes.Buffer
	if code := run([]string{"start", "--root", root, "--state", state}, bytes.NewBufferString(`{"session":"s"}`), &out); code != 0 {
		t.Fatal(out.String())
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package p\nfunc F() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run([]string{"stop", "--root", root, "--state", state}, bytes.NewBufferString(`{"session":"s"}`), &out); code != 0 {
		t.Fatal(out.String())
	}
	var round map[string]any
	if err := json.Unmarshal(out.Bytes(), &round); err != nil {
		t.Fatal(err)
	}
	id, ok := round["id"].(string)
	if !ok || id == "" {
		t.Fatal("round id missing")
	}
	out.Reset()
	if code := run([]string{"analyze-round", "--root", root, "--state", state}, bytes.NewBufferString(`{"round_id":"`+id+`"}`), &out); code == 0 || bytes.Contains(out.Bytes(), []byte("轮次不存在")) {
		t.Fatalf("round_id was not dispatched: %s", out.String())
	}
}

func TestAnalyzeRoundFlagDoesNotReadStdin(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package p\nfunc F() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := run([]string{"start", "--root", root, "--state", state}, bytes.NewBufferString(`{"session":"s"}`), &out); code != 0 {
		t.Fatal(out.String())
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package p\nfunc F() { println(1) }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run([]string{"stop", "--root", root, "--state", state}, bytes.NewBufferString(`{"session":"s"}`), &out); code != 0 {
		t.Fatal(out.String())
	}
	var round map[string]any
	if err := json.Unmarshal(out.Bytes(), &round); err != nil {
		t.Fatal(err)
	}
	id := round["id"].(string)
	out.Reset()
	if code := run([]string{"analyze-round", "--root", root, "--state", state, "--round", id}, io.Reader(failOnRead{}), &out); code == 0 || bytes.Contains(out.Bytes(), []byte("stdin was read")) {
		t.Fatalf("explicit --round consumed stdin: %s", out.String())
	}
}
