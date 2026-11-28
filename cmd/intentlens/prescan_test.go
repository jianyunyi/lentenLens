package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPrescanCommandEndToEnd(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	if e := os.WriteFile(filepath.Join(root, "main.go"), []byte("package p\nfunc F() {}\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Design\nF is an example.\n"), 0600); e != nil {
		t.Fatal(e)
	}
	runCommand := func(command string, extra ...string) map[string]any {
		t.Helper()
		var out bytes.Buffer
		args := append([]string{command, "--root", root, "--state", state}, extra...)
		input := "{}"
		if command == "prescan-inspect" {
			input = `{"file":"main.go","line":2}`
		}
		if run(args, strings.NewReader(input), &out) != 0 {
			t.Fatal(command, out.String())
		}
		var r map[string]any
		if e := json.Unmarshal(out.Bytes(), &r); e != nil {
			t.Fatal(e)
		}
		return r
	}
	missing := runCommand("prescan-index")
	if missing["prescan"].(map[string]any)["state"] != "missing" {
		t.Fatal(missing)
	}
	first := runCommand("prescan", "--consent")
	s := first["prescan"].(map[string]any)
	if s["state"] != "hit" || first["function_count"] != float64(1) || first["paragraph_count"] != float64(1) {
		t.Fatal(first)
	}
	manifests, e := filepath.Glob(filepath.Join(state, "*", "prescan-*.json"))
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, name := range manifests {
		b, e := os.ReadFile(name)
		if e != nil {
			t.Fatal(e)
		}
		var m map[string]any
		if e = json.Unmarshal(b, &m); e != nil {
			t.Fatal(e)
		}
		if _, ok := m["body"]; !ok {
			continue
		}
		found = true
		for _, key := range []string{"schema", "config", "generation", "started", "finished", "workspace", "policy_hash", "capture_policy_hash", "authorization_version", "authorization_epoch", "files", "coverage", "integrity", "body"} {
			if _, ok := m[key]; !ok {
				t.Fatal("missing", key)
			}
		}
	}
	if !found {
		t.Fatal("manifest missing")
	}
	before := runCommand("index")
	after := runCommand("index")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("cached index differs")
	}
	second := runCommand("prescan")
	ss := second["prescan"].(map[string]any)
	if ss["reused"] != true || ss["generation"] != s["generation"] {
		t.Fatal(second)
	}
	cached := runCommand("prescan-index")
	if cached["prescan"].(map[string]any)["state"] != "unknown" {
		t.Fatal(cached)
	}
	inspect := runCommand("prescan-inspect")
	if inspect["function"].(map[string]any)["name"] != "F" {
		t.Fatal(inspect)
	}
	if e = os.WriteFile(filepath.Join(root, "README.md"), []byte("# Changed\nNew text.\n"), 0600); e != nil {
		t.Fatal(e)
	}
	// Legacy explicit index verifies documents too once prescan was authorized.
	runCommand("index")
	third := runCommand("prescan")
	if third["prescan"].(map[string]any)["generation"] == s["generation"] {
		t.Fatal("Markdown edit failed to rebuild")
	}
	runCommand("prescan-invalidate")
	stale := runCommand("prescan-index")
	if stale["prescan"].(map[string]any)["state"] != "stale" || len(stale["functions"].([]any)) != 0 {
		t.Fatal(stale)
	}
	runCommand("prescan-refresh")
	runCommand("prescan-clear")
	missing = runCommand("prescan-index")
	if missing["prescan"].(map[string]any)["state"] != "missing" {
		t.Fatal(missing)
	}
}
