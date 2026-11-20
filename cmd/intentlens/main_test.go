package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failOnRead struct{}

func (failOnRead) Read([]byte) (int, error) { return 0, errors.New("stdin was read") }

func TestCLIRejectsPositionalArgs(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"integration-info", "unexpected", "--root", t.TempDir()}, failOnRead{}, &out)
	if code != 1 || !strings.Contains(out.String(), "unexpected") {
		t.Fatalf("exit = %d, output = %s", code, &out)
	}
	var failure map[string]string
	if err := json.Unmarshal(out.Bytes(), &failure); err != nil || failure["error"] == "" {
		t.Fatalf("expected JSON error, got %s", &out)
	}
}

func TestCLIRejectsInvalidRequests(t *testing.T) {
	for _, tc := range []struct {
		name, stdin string
	}{
		{"empty", ""},
		{"null", "null"},
		{"second object", "{} {}"},
		{"trailing junk", "{} junk"},
		{"over limit whitespace", "{}" + strings.Repeat(" ", 32767)},
		{"junk past limit", "{}" + strings.Repeat(" ", 32766) + "x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			code := run([]string{"start", "--root", t.TempDir(), "--state", t.TempDir()}, strings.NewReader(tc.stdin), &out)
			if code != 1 || !strings.Contains(out.String(), "需要有效的 JSON 输入") {
				t.Fatalf("exit = %d, output = %s", code, &out)
			}
		})
	}
}

func TestDecodeRequestAtSizeLimit(t *testing.T) {
	for command, limit := range map[string]int{"start": 32768, "analyze-round": 32768, "explain-selection": 128 * 1024} {
		t.Run(command, func(t *testing.T) {
			const payload = `{"round_id":"abc"}`
			stdin := payload + strings.Repeat(" ", limit-len(payload))
			var round string
			request, err := decodeRequest(command, strings.NewReader(stdin), &round)
			if err != nil || request.RoundID != "abc" {
				t.Fatalf("request round ID = %q, error = %v", request.RoundID, err)
			}
			round = ""
			if _, err := decodeRequest(command, strings.NewReader(stdin+" "), &round); err == nil {
				t.Fatal("accepted request one byte over the limit")
			}
		})
	}
}

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

func TestTestModelCommandIsRegisteredAndDoesNotReadStdin(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"test-model", "--root", t.TempDir(), "--state", t.TempDir()}, failOnRead{}, &out)
	if code == 0 || strings.Contains(out.String(), "未知命令") || !strings.Contains(out.String(), "显式配置模型") {
		t.Fatalf("test-model command was not dispatched to its handler: exit=%d output=%s", code, out.String())
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

func TestExplainSelectionCLIRejectsIntentignoredFileBeforeModel(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".intentignore"), []byte("private.ts\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code := run([]string{"explain-selection", "--root", root, "--state", state}, bytes.NewBufferString(`{"file":"private.ts"}`), &out)
	if code == 0 || !strings.Contains(out.String(), "当前文件已排除") || !strings.Contains(out.String(), ".intentignore") {
		t.Fatalf("CLI did not enforce the local file exclusion before model setup: exit=%d output=%s", code, out.String())
	}
}
