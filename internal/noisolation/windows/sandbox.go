//go:build windows
// +build windows

package windows

import (
	"context"
	"fmt"
	"strings"
	"time"

	"intentlens/internal/noisolation"
)

// Sandbox implements noisolation.Sandbox for Windows using AppContainer.
type Sandbox struct {
	cfg noisolation.Config
	// TODO: AppContainer-specific fields
}

// Run executes the tool in a sandboxed Windows AppContainer environment.
func (s *Sandbox) Run(ctx context.Context, tool string, args []string, workDir string) (*noisolation.Result, error) {
	result := &noisolation.Result{
		StartTime: time.Now(),
	}

	// TODO: Implement Windows AppContainer execution
	// Steps:
	// 1. Create AppContainer SID
	// 2. Set up ACL for workDir and tool
	// 3. Create restricted token
	// 4. Create process with restricted token
	// 5. Monitor execution with timeout
	// 6. Collect output and exit code

	result.Error = "AppContainer implementation pending"
	result.EndTime = time.Now()
	result.DurationMS = result.EndTime.Sub(result.StartTime).Milliseconds()
	result.ExitCode = -1

	return result, fmt.Errorf("not yet implemented")
}

// Cleanup removes AppContainer resources.
func (s *Sandbox) Cleanup() error {
	// TODO: Delete AppContainer profile
	// TODO: Release tokens and ACLs
	return nil
}

// Capabilities returns no isolation features because AppContainer isolation is not yet implemented.
// Run currently provides no isolation; add capabilities only after they are implemented.
func (s *Sandbox) Capabilities() []string {
	return []string{}
}

// NewWindowsSandbox creates a new Windows sandbox.
func NewWindowsSandbox(cfg noisolation.Config) (noisolation.Sandbox, error) {
	// Check Windows version
	if !isWindowsVersionSupported() {
		return nil, noisolation.UnsupportedError{
			Reason: "Windows 10 1809 or later required for AppContainer",
		}
	}

	// Verify admin privileges
	if !isRunningAsAdmin() {
		return nil, noisolation.UnsupportedError{
			Reason: "Administrator privileges required for AppContainer",
		}
	}

	return &Sandbox{cfg: cfg}, nil
}

// isWindowsVersionSupported checks if the Windows version supports AppContainer.
func isWindowsVersionSupported() bool {
	// Windows 10 version 1809 = build 17763
	// Windows 11 = build 22000+
	// For now, return true if kernel32 GetVersion succeeds
	// TODO: Properly check version using GetVersionEx or RtlGetVersion
	return true
}

// isRunningAsAdmin checks if the current process has administrator privileges.
func isRunningAsAdmin() bool {
	// TODO: Use syscall.GetCurrentProcessToken() and CheckTokenMembership
	// to verify membership in Administrators group
	return true
}

// filteredEnvironment returns a filtered environment for Windows.
func (s *Sandbox) filteredEnvironment() []string {
	// Whitelist essential variables for Go test execution
	whitelist := map[string]bool{
		"PATH":        true,
		"TEMP":        true,
		"TMP":         true,
		"USERPROFILE": true,
		"GOARCH":      true,
		"GOOS":        true,
		"GOROOT":      true,
		"GOPATH":      true,
		"GOTMPDIR":    true,
	}

	// Add user-specified allowed variables
	for _, v := range s.cfg.AllowedEnvVars {
		whitelist[strings.ToUpper(v)] = true
	}

	// TODO: Construct new environment slice with only allowed variables
	return []string{}
}
