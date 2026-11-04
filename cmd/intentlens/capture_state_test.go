package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

type captureStatusNoInput struct{ t *testing.T }

func (r captureStatusNoInput) Read([]byte) (int, error) {
	r.t.Fatal("capture-status read stdin")
	return 0, nil
}

func TestCaptureStatusDoesNotReadStdinAndReturnsNull(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"capture-status", "--root", t.TempDir(), "--state", t.TempDir()}, captureStatusNoInput{t}, &out); code != 0 {
		t.Fatal(out.String())
	}
	if out.String() != "null\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestCaptureAbortJSONAndErrors(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	var out bytes.Buffer
	if code := run([]string{"start", "--root", root, "--state", state}, bytes.NewBufferString(`{"session":"s"}`), &out); code != 0 {
		t.Fatal(out.String())
	}
	var started struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out.Bytes(), &started); err != nil || started.ID == "" {
		t.Fatalf("start response: %s", out.String())
	}
	out.Reset()
	if code := run([]string{"capture-abort", "--root", root, "--state", state}, bytes.NewBufferString(`{"session":"wrong","round_id":"`+started.ID+`","confirm":true}`), &out); code == 0 {
		t.Fatal("mismatched abort accepted")
	}
	var failure map[string]string
	if err := json.Unmarshal(out.Bytes(), &failure); err != nil || failure["error"] == "" {
		t.Fatalf("error response: %s", out.String())
	}
	out.Reset()
	if code := run([]string{"capture-abort", "--root", root, "--state", state}, bytes.NewBufferString(`{"session":"s","round_id":"`+started.ID+`","confirm":true}`), &out); code != 0 || out.String() != "{\"aborted\":true}\n" {
		t.Fatalf("abort response: %d %s", code, out.String())
	}
}

func TestCaptureStatusReturnsDirectActiveState(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	var out bytes.Buffer
	if code := run([]string{"start", "--root", root, "--state", state}, bytes.NewBufferString(`{"session":"s"}`), &out); code != 0 {
		t.Fatal(out.String())
	}
	out.Reset()
	if code := run([]string{"capture-status", "--root", root, "--state", state}, bytes.NewBuffer(nil), &out); code != 0 {
		t.Fatal(out.String())
	}
	var status struct {
		Active  any    `json:"active"`
		ID      string `json:"id"`
		Session string `json:"session"`
		Started string `json:"started"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.ID == "" || status.Session != "s" || status.Started == "" || status.Active != nil {
		t.Fatalf("unexpected direct capture status: %s", out.String())
	}
}
