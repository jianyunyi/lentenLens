package test

import (
	"fmt"
	"strings"

	"intentlens/internal/sandbox"
)

// VerifyResult contains the result of a sandbox verification test.
type VerifyResult struct {
	Passed           bool
	Description      string
	ExpectedBehavior string
	ActualBehavior   string
	ErrorMsg         string
}

// Verifier provides verification functions for sandbox behavior.
type Verifier struct{}

// NewVerifier creates a new verifier.
func NewVerifier() *Verifier {
	return &Verifier{}
}

// VerifyExecutionSuccess checks that code executed successfully.
func (v *Verifier) VerifyExecutionSuccess(result *sandbox.Result) VerifyResult {
	if result.ExitCode != 0 {
		return VerifyResult{
			Passed:           false,
			Description:      "Code execution failed",
			ExpectedBehavior: "Exit code 0",
			ActualBehavior:   fmt.Sprintf("Exit code %d", result.ExitCode),
			ErrorMsg:         result.Error + "\nStderr: " + result.Stderr,
		}
	}

	return VerifyResult{
		Passed:           true,
		Description:      "Code executed successfully",
		ExpectedBehavior: "Exit code 0",
		ActualBehavior:   "Exit code 0",
	}
}

// VerifyNoTimeout checks that execution did not timeout.
func (v *Verifier) VerifyNoTimeout(result *sandbox.Result) VerifyResult {
	if result.Timeout {
		return VerifyResult{
			Passed:           false,
			Description:      "Execution timeout",
			ExpectedBehavior: "Execution completes before timeout",
			ActualBehavior:   fmt.Sprintf("Timed out after %d ms", result.DurationMS),
			ErrorMsg:         "Process was terminated due to timeout",
		}
	}

	return VerifyResult{
		Passed:           true,
		Description:      "No timeout",
		ExpectedBehavior: "Execution completes before timeout",
		ActualBehavior:   fmt.Sprintf("Completed in %d ms", result.DurationMS),
	}
}

// VerifyMemoryLimit checks that memory limit was not exceeded.
func (v *Verifier) VerifyMemoryLimit(result *sandbox.Result) VerifyResult {
	if result.MemoryExceeded {
		return VerifyResult{
			Passed:           false,
			Description:      "Memory limit exceeded",
			ExpectedBehavior: "Memory usage within limits",
			ActualBehavior:   "Memory limit exceeded",
			ErrorMsg:         "Process exceeded memory limit and was terminated",
		}
	}

	return VerifyResult{
		Passed:           true,
		Description:      "Memory limit respected",
		ExpectedBehavior: "Memory usage within limits",
		ActualBehavior:   "Memory limit not exceeded",
	}
}

// VerifyResourceLimits checks that resource limits were enforced.
func (v *Verifier) VerifyResourceLimits(result *sandbox.Result) VerifyResult {
	if result.ResourceLimitExceeded {
		return VerifyResult{
			Passed:           false,
			Description:      "Resource limit exceeded",
			ExpectedBehavior: "All resources within limits",
			ActualBehavior:   "Resource limit exceeded",
			ErrorMsg:         "Process exceeded resource limits",
		}
	}

	return VerifyResult{
		Passed:           true,
		Description:      "Resource limits respected",
		ExpectedBehavior: "All resources within limits",
		ActualBehavior:   "All resources within limits",
	}
}

// VerifyOutputContains checks that output contains expected text.
func (v *Verifier) VerifyOutputContains(result *sandbox.Result, expected string) VerifyResult {
	if !strings.Contains(result.Stdout, expected) {
		return VerifyResult{
			Passed:           false,
			Description:      "Output does not contain expected text",
			ExpectedBehavior: fmt.Sprintf("Output contains: %q", expected),
			ActualBehavior:   fmt.Sprintf("Output: %q", result.Stdout),
		}
	}

	return VerifyResult{
		Passed:           true,
		Description:      "Output contains expected text",
		ExpectedBehavior: fmt.Sprintf("Output contains: %q", expected),
		ActualBehavior:   fmt.Sprintf("Output contains: %q", expected),
	}
}

// VerifyOutputExcludes checks that output does NOT contain text.
func (v *Verifier) VerifyOutputExcludes(result *sandbox.Result, forbidden string) VerifyResult {
	if strings.Contains(result.Stdout, forbidden) {
		return VerifyResult{
			Passed:           false,
			Description:      "Output contains forbidden text",
			ExpectedBehavior: fmt.Sprintf("Output should NOT contain: %q", forbidden),
			ActualBehavior:   fmt.Sprintf("Output contains: %q", forbidden),
		}
	}

	return VerifyResult{
		Passed:           true,
		Description:      "Output excludes forbidden text",
		ExpectedBehavior: fmt.Sprintf("Output should NOT contain: %q", forbidden),
		ActualBehavior:   "Forbidden text not found",
	}
}

// VerifyStderrExcludes checks that stderr does NOT contain text.
func (v *Verifier) VerifyStderrExcludes(result *sandbox.Result, forbidden string) VerifyResult {
	if strings.Contains(result.Stderr, forbidden) {
		return VerifyResult{
			Passed:           false,
			Description:      "Stderr contains forbidden text",
			ExpectedBehavior: fmt.Sprintf("Stderr should NOT contain: %q", forbidden),
			ActualBehavior:   fmt.Sprintf("Stderr contains: %q", forbidden),
		}
	}

	return VerifyResult{
		Passed:           true,
		Description:      "Stderr excludes forbidden text",
		ExpectedBehavior: fmt.Sprintf("Stderr should NOT contain: %q", forbidden),
		ActualBehavior:   "Forbidden text not found",
	}
}

// VerifyBlocked checks that an operation was blocked (error in output or stderr).
func (v *Verifier) VerifyBlocked(result *sandbox.Result, operationName string) VerifyResult {
	blockedIndicators := []string{"Blocked", "Permission denied", "not allowed", "EACCES", "EPERM"}

	found := false
	for _, indicator := range blockedIndicators {
		if strings.Contains(result.Stderr, indicator) || strings.Contains(result.Stdout, indicator) {
			found = true
			break
		}
	}

	if !found && result.ExitCode == 0 {
		return VerifyResult{
			Passed:           false,
			Description:      fmt.Sprintf("Operation %q was not blocked", operationName),
			ExpectedBehavior: "Operation should be blocked",
			ActualBehavior:   fmt.Sprintf("Operation completed successfully\nStdout: %q\nStderr: %q", result.Stdout, result.Stderr),
		}
	}

	return VerifyResult{
		Passed:           true,
		Description:      fmt.Sprintf("Operation %q was properly blocked", operationName),
		ExpectedBehavior: "Operation should be blocked",
		ActualBehavior:   "Operation was blocked",
	}
}

// VerifyDurationWithin checks that execution completed within time window.
func (v *Verifier) VerifyDurationWithin(result *sandbox.Result, minMs, maxMs int64) VerifyResult {
	if result.DurationMS < minMs || result.DurationMS > maxMs {
		return VerifyResult{
			Passed:           false,
			Description:      "Execution time outside expected range",
			ExpectedBehavior: fmt.Sprintf("Execution time between %d and %d ms", minMs, maxMs),
			ActualBehavior:   fmt.Sprintf("Execution time: %d ms", result.DurationMS),
		}
	}

	return VerifyResult{
		Passed:           true,
		Description:      "Execution time within expected range",
		ExpectedBehavior: fmt.Sprintf("Execution time between %d and %d ms", minMs, maxMs),
		ActualBehavior:   fmt.Sprintf("Execution time: %d ms", result.DurationMS),
	}
}

// TestSuite represents a collection of verification tests.
type TestSuite struct {
	Name   string
	Tests  []TestSuite
	Runner func() (VerifyResult, error)
}

// Run executes the test suite and returns results.
func (ts *TestSuite) Run() (passed int, failed int, errors int) {
	if ts.Runner != nil {
		result, err := ts.Runner()
		if err != nil {
			fmt.Printf("❌ %s: ERROR - %v\n", ts.Name, err)
			errors++
		} else if result.Passed {
			fmt.Printf("✓ %s\n", ts.Name)
			passed++
		} else {
			fmt.Printf("✗ %s\n", ts.Name)
			fmt.Printf("  Expected: %s\n", result.ExpectedBehavior)
			fmt.Printf("  Actual: %s\n", result.ActualBehavior)
			if result.ErrorMsg != "" {
				fmt.Printf("  Error: %s\n", result.ErrorMsg)
			}
			failed++
		}
	}

	for _, subtest := range ts.Tests {
		p, f, e := subtest.Run()
		passed += p
		failed += f
		errors += e
	}

	return
}
