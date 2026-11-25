package test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// GoTestRunner runs Go code in a test harness.
type GoTestRunner struct {
	tmpDir string
}

// NewGoTestRunner creates a new test runner.
func NewGoTestRunner() (*GoTestRunner, error) {
	tmpDir, err := os.MkdirTemp("", "sandbox-test-")
	if err != nil {
		return nil, err
	}
	return &GoTestRunner{tmpDir: tmpDir}, nil
}

// RunCode executes Go code directly.
func (r *GoTestRunner) RunCode(code string) (stdout string, stderr string, exitCode int, err error) {
	return r.runCode(code, 10*time.Second)
}

// RunCodeWithTimeout executes Go code with a custom timeout.
func (r *GoTestRunner) RunCodeWithTimeout(code string, timeout time.Duration) (stdout string, stderr string, exitCode int, err error) {
	return r.runCode(code, timeout)
}

// RunTest executes Go test code.
func (r *GoTestRunner) RunTest(code string) (passed bool, output string, err error) {
	return r.runTest(code, 30*time.Second)
}

// Cleanup removes temporary resources.
func (r *GoTestRunner) Cleanup() error {
	return os.RemoveAll(r.tmpDir)
}

// private functions

func (r *GoTestRunner) runCode(code string, timeout time.Duration) (string, string, int, error) {
	// Create main.go
	mainPath := filepath.Join(r.tmpDir, "main.go")
	if err := os.WriteFile(mainPath, []byte(code), 0644); err != nil {
		return "", "", -1, err
	}

	// Run: go run main.go
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", mainPath)
	cmd.Dir = r.tmpDir

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	exitCode := 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", outputStr, -1, fmt.Errorf("timeout after %v", timeout)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", outputStr, -1, err
		}
	}

	return outputStr, "", exitCode, nil
}

func (r *GoTestRunner) runTest(code string, timeout time.Duration) (bool, string, error) {
	// Create main_test.go
	testPath := filepath.Join(r.tmpDir, "main_test.go")
	if err := os.WriteFile(testPath, []byte(code), 0644); err != nil {
		return false, "", err
	}

	// Run: go test -v
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "-v")
	cmd.Dir = r.tmpDir

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return false, outputStr, fmt.Errorf("timeout after %v", timeout)
		}
		// Test failed
		return false, outputStr, nil
	}

	return true, outputStr, nil
}
