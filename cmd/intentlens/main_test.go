package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"intentlens/internal/app"
	"io"
	"net/http"
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
	var started map[string]any
	if err := json.Unmarshal(out.Bytes(), &started); err != nil || started["id"] == nil {
		t.Fatalf("start did not return round ID: %s", out.String())
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package p\nfunc A() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	stopInput, _ := json.Marshal(map[string]string{"session": "s", "round_id": started["id"].(string)})
	if code := run([]string{"stop", "--root", root, "--state", state}, bytes.NewReader(stopInput), &out); code != 0 {
		t.Fatal(out.String())
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["id"] == nil || result["change_count"] == nil || result["omitted_count"] == nil || result["changes"] != nil {
		t.Fatalf("Stop must return a compact receipt: %s", out.String())
	}
}

func TestRoundTransferContractBoundsStopPagesAndReadsFilesOnDemand(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	const fileCount, fileBytes = 16, 1 << 20
	before := strings.Repeat("a", fileBytes)
	after := strings.Repeat("b", fileBytes)
	for i := 0; i < fileCount; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("file-%02d.txt", i)), []byte(before), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if code := run([]string{"start", "--root", root, "--state", state}, strings.NewReader(`{"session":"max-snapshot"}`), &out); code != 0 {
		t.Fatal(out.String())
	}
	var start struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out.Bytes(), &start); err != nil || start.ID == "" {
		t.Fatalf("invalid start result: %v %s", err, out.String())
	}
	for i := 0; i < fileCount; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("file-%02d.txt", i)), []byte(after), 0600); err != nil {
			t.Fatal(err)
		}
	}
	out.Reset()
	stopInput, _ := json.Marshal(map[string]string{"session": "max-snapshot", "round_id": start.ID})
	if code := run([]string{"stop", "--root", root, "--state", state}, bytes.NewReader(stopInput), &out); code != 0 {
		t.Fatal(out.String())
	}
	if out.Len() >= 32*1024 || strings.Contains(out.String(), before[:128]) || strings.Contains(out.String(), after[:128]) {
		t.Fatalf("Stop response contains large snapshot data: %d bytes", out.Len())
	}
	var receipt app.RoundReceipt
	if err := json.Unmarshal(out.Bytes(), &receipt); err != nil || receipt.ID != start.ID || receipt.ChangeCount != fileCount || receipt.ModifiedCount != fileCount {
		t.Fatalf("invalid receipt: %+v, %v", receipt, err)
	}
	readCommand := func(command string, input any) []byte {
		t.Helper()
		encoded, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		out.Reset()
		if code := run([]string{command, "--root", root, "--state", state}, bytes.NewReader(encoded), &out); code != 0 {
			t.Fatalf("%s failed: %s", command, out.String())
		}
		if out.Len() >= 32*1024*1024 {
			t.Fatalf("%s unexpectedly large: %d bytes", command, out.Len())
		}
		return append([]byte(nil), out.Bytes()...)
	}
	var page app.RoundPage
	if err := json.Unmarshal(readCommand("rounds", map[string]any{"limit": 2}), &page); err != nil || len(page.Items) != 1 || page.Items[0].ChangeCount != fileCount {
		t.Fatalf("summary page: %+v, %v", page, err)
	}
	detailBytes := readCommand("round-detail", map[string]any{"round_id": start.ID, "limit": fileCount})
	var detail app.RoundDetail
	if err := json.Unmarshal(detailBytes, &detail); err != nil || detail.ID != start.ID || len(detail.Changes) != fileCount || detail.Changes[0].Before.Bytes != fileBytes {
		t.Fatalf("round detail: %+v, %v", detail, err)
	}
	if strings.Contains(string(detailBytes), before[:128]) || strings.Contains(string(detailBytes), after[:128]) {
		t.Fatal("round detail included snapshot content")
	}
	var file app.RoundFileResult
	if err := json.Unmarshal(readCommand("round-file", map[string]string{"round_id": start.ID, "file": "file-00.txt", "side": "after"}), &file); err != nil || !file.Exists || file.File == nil || file.File.Content != after {
		t.Fatalf("on-demand file read: exists=%v, err=%v", file.Exists, err)
	}

	opened, err := app.Open(root, state)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		session := fmt.Sprintf("history-%d", i)
		active, err := opened.Start(session)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "history.txt"), []byte(fmt.Sprintf("%d", i)), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := opened.Stop(session, active.ID); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	cursor := ""
	for {
		input := map[string]any{"limit": 3}
		if cursor != "" {
			input["cursor"] = cursor
		}
		encoded, _ := json.Marshal(input)
		out.Reset()
		if code := run([]string{"rounds", "--root", root, "--state", state}, bytes.NewReader(encoded), &out); code != 0 {
			t.Fatal(out.String())
		}
		page = app.RoundPage{}
		if out.Len() >= 32*1024 || json.Unmarshal(out.Bytes(), &page) != nil || len(page.Items) > 3 {
			t.Fatalf("unbounded or invalid page: bytes=%d", out.Len())
		}
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatalf("duplicate round across pages: %s", item.ID)
			}
			seen[item.ID] = true
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(seen) != 9 {
		t.Fatalf("history pages missed rounds: got %d, want 9", len(seen))
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

func TestEditorModelTargetRejectsConfigChangedAfterPreflight(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	t.Setenv("INTENTLENS_API_KEY", "fixture-api-key")
	var out bytes.Buffer
	configure := func(base, model string) {
		t.Helper()
		out.Reset()
		stdin := strings.NewReader(`{"settings":{"model":{"enabled":true,"provider":"openai-compatible","base_url":"` + base + `","model":"` + model + `","api_key_env":"INTENTLENS_API_KEY","token_limit":4096}},"consent":true}`)
		if code := run([]string{"configure", "--root", root, "--state", state}, stdin, &out); code != 0 {
			t.Fatalf("configure failed: %s", &out)
		}
	}
	args := []string{"test-model", "--root", root, "--state", state, "--editor-model-target",
		`{"provider":"openai-compatible","base_url":"https://original.fixture/v1","model":"fixture"}`}
	configure("https://changed.fixture/v1", "fixture")
	out.Reset()
	if code := run(args, failOnRead{}, &out); code != 1 || !strings.Contains(out.String(), "模型服务配置已变化") {
		t.Fatalf("mismatched target: exit=%d output=%s", code, &out)
	}
	configure("https://original.fixture/v1", "changed-model")
	out.Reset()
	if code := run(args, failOnRead{}, &out); code != 1 || !strings.Contains(out.String(), "模型服务配置已变化") {
		t.Fatalf("mismatched model: exit=%d output=%s", code, &out)
	}
	// A matching target gets past the binding guard to the normal key check.
	configure("https://original.fixture/v1", "fixture")
	t.Setenv("INTENTLENS_API_KEY", "")
	out.Reset()
	if code := run(args, failOnRead{}, &out); code != 1 || !strings.Contains(out.String(), "未提供此工作区配置的 API Key") {
		t.Fatalf("matching target: exit=%d output=%s", code, &out)
	}
}

func TestCLIEditorCredentialBinding(t *testing.T) {
	const validTarget = `{"provider":"openai-compatible","base_url":"https://original.fixture/v1","model":"fixture"}`
	for _, tc := range []struct {
		name, flag, target, keyEnv, wantError string
		editor                                bool
	}{
		{name: "editor missing target", editor: true, wantError: "模型密钥绑定无效"},
		{name: "editor empty target", editor: true, flag: "--editor-model-target=", wantError: "模型密钥绑定无效"},
		{name: "explicit empty equals", flag: "--editor-model-target=", wantError: "模型密钥绑定无效"},
		{name: "explicit empty argument", flag: "--editor-model-target", wantError: "模型密钥绑定无效"},
		{name: "malformed JSON", editor: true, flag: "--editor-model-target", target: "{", wantError: "模型密钥绑定无效"},
		{name: "null target", editor: true, flag: "--editor-model-target", target: "null", wantError: "模型密钥绑定无效"},
		{name: "empty object", editor: true, flag: "--editor-model-target", target: "{}", wantError: "模型密钥绑定无效"},
		{name: "missing base", editor: true, flag: "--editor-model-target", target: `{"provider":"openai-compatible"}`, wantError: "模型密钥绑定无效"},
		{name: "missing model", editor: true, flag: "--editor-model-target", target: `{"provider":"openai-compatible","base_url":"https://original.fixture/v1"}`, wantError: "模型服务配置已变化"},
		{name: "wrong type", editor: true, flag: "--editor-model-target", target: `{"provider":1,"base_url":"https://original.fixture/v1"}`, wantError: "模型密钥绑定无效"},
		{name: "invalid URL", editor: true, flag: "--editor-model-target", target: `{"provider":"openai-compatible","base_url":"not-a-url","model":"fixture"}`, wantError: "模型服务配置已变化"},
		{name: "provider mismatch", editor: true, flag: "--editor-model-target", target: `{"provider":"openai","base_url":"https://original.fixture/v1","model":"fixture"}`, wantError: "模型服务配置已变化"},
		{name: "base mismatch", editor: true, flag: "--editor-model-target", target: `{"provider":"openai-compatible","base_url":"https://changed.fixture/v1","model":"fixture"}`, wantError: "模型服务配置已变化"},
		{name: "model mismatch", editor: true, flag: "--editor-model-target", target: `{"provider":"openai-compatible","base_url":"https://original.fixture/v1","model":"other-model"}`, wantError: "模型服务配置已变化"},
		{name: "key env mismatch", editor: true, flag: "--editor-model-target", target: validTarget, keyEnv: "INTENTLENS_CLI_TEST_KEY", wantError: "模型服务配置已变化"},
		{name: "matching editor target", editor: true, flag: "--editor-model-target", target: validTarget},
		{name: "ordinary CLI default key"},
		{name: "ordinary CLI custom key", keyEnv: "INTENTLENS_CLI_TEST_KEY"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Keep configuration on the ordinary CLI path, as in the extension.
			t.Setenv(app.EditorAPIKeyEnv, "")
			if err := os.Unsetenv(app.EditorAPIKeyEnv); err != nil {
				t.Fatal(err)
			}
			t.Setenv("INTENTLENS_API_KEY", "cli-fixture-key")
			t.Setenv("INTENTLENS_CLI_TEST_KEY", "cli-fixture-key")
			calls := 0
			previous := http.DefaultTransport
			http.DefaultTransport = cliInMemoryTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				wantKey := "cli-fixture-key"
				if tc.editor {
					wantKey = "editor-fixture-key"
				}
				if r.URL.String() != "https://original.fixture/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer "+wantKey {
					t.Error("request used the wrong destination or credential channel")
				}
				_, _ = io.WriteString(w, `{"choices":[{"finish_reason":"stop","message":{"content":"{\"intentlens_connection_test\":\"ok\"}"}}],"usage":{"total_tokens":9}}`)
			})}
			t.Cleanup(func() { http.DefaultTransport = previous })
			root, state := t.TempDir(), t.TempDir()
			var out bytes.Buffer
			config := `{"settings":{"model":{"enabled":true,"provider":"openai-compatible","base_url":" https://original.fixture/v1/// ","model":"fixture","api_key_env":"` + tc.keyEnv + `","token_limit":4096}},"consent":true}`
			if code := run([]string{"configure", "--root", root, "--state", state}, strings.NewReader(config), &out); code != 0 {
				t.Fatalf("configure: %s", &out)
			}
			if tc.editor {
				t.Setenv(app.EditorAPIKeyEnv, "editor-fixture-key")
			}
			args := []string{"test-model", "--root", root, "--state", state}
			if tc.flag != "" {
				args = append(args, tc.flag)
				if tc.flag == "--editor-model-target" {
					args = append(args, tc.target)
				}
			}
			out.Reset()
			code := run(args, failOnRead{}, &out)
			if tc.wantError != "" {
				if code != 1 || !strings.Contains(out.String(), tc.wantError) || calls != 0 {
					t.Fatalf("exit=%d requests=%d output=%s", code, calls, &out)
				}
			} else if code != 0 || calls != 1 || !strings.Contains(out.String(), `"tested":true`) {
				t.Fatalf("exit=%d requests=%d output=%s", code, calls, &out)
			}
			if strings.Contains(out.String(), "fixture-key") {
				t.Fatal("credential leaked into command output")
			}
		})
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
