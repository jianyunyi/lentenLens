package main

import (
	"bytes"
	"testing"
)

type forbiddenInput struct{ t *testing.T }

func (r forbiddenInput) Read([]byte) (int, error) {
	r.t.Fatal("command without input tried to read stdin")
	return 0, nil
}
func TestMetadataCommandsDoNotWaitForInput(t *testing.T) {
	var out bytes.Buffer
	if run([]string{"hooks-config"}, forbiddenInput{t}, &out) != 0 {
		t.Fatal(out.String())
	}
}
