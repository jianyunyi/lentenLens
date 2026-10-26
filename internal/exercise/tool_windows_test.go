//go:build windows

package exercise

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestToolThroughWindowsJunction(t *testing.T) {
	path, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	link := filepath.Join(base, "toolchain")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, filepath.Dir(filepath.Dir(path))).CombinedOutput(); err != nil {
		t.Fatalf("create toolchain junction: %v: %s", err, out)
	}
	t.Setenv("PATH", filepath.Join(link, "bin"))
	tool, err := Tool()
	if err != nil {
		t.Fatal(err)
	}
	if tool.Path != filepath.Join(link, "bin", filepath.Base(path)) {
		t.Fatalf("lookup path not retained: %s", tool.Path)
	}
	dir := filepath.Join(base, "workspace")
	if err := Materialize(dir, fixture().Baseline); err != nil {
		t.Fatal(err)
	}
	result := Run(context.Background(), tool, dir, filepath.Join(base, "cache"))
	if !result.Passed {
		t.Fatalf("junction tool cannot execute: %+v", result)
	}
}
