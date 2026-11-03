package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestExplainSelectionCommandRejectsEmptySelection(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	var out bytes.Buffer
	code := run([]string{"explain-selection", "--root", root, "--state", state}, bytes.NewBufferString(`{"code":"","language":"typescript","start_line":1,"end_line":1}`), &out)
	if code == 0 {
		t.Fatalf("empty selection unexpectedly succeeded: %s", out.String())
	}
	var response map[string]string
	if err := json.Unmarshal(out.Bytes(), &response); err != nil || !strings.Contains(response["error"], "请先选中非空代码") {
		t.Fatalf("unexpected CLI error response: %q (%v)", out.String(), err)
	}
}
