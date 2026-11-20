//go:build darwin
// +build darwin

package macos

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"intentlens/internal/sandbox"
)

// Sandbox implements sandbox.Sandbox for macOS using Sandbox.
type Sandbox struct {
	cfg       sandbox.Config
	sandboxID string
}

// Run executes the tool in a sandboxed macOS Sandbox environment.
func (s *Sandbox) Run(ctx context.Context, tool string, args []string, workDir string) (*sandbox.Result, error) {
	result := &sandbox.Result{
		StartTime: time.Now(),
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	// TODO: Load and apply sandbox profile
	// sandboxProfile := s.generateSandboxProfile(workDir)
	// err := s.applySandbox(sandboxProfile)
	// if err != nil {
	//     result.Error = err.Error()
	//     return result, err
	// }

	// Prepare the command
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Dir = workDir
	cmd.Env = s.filteredEnvironment()

	// Run the command
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			result.Timeout = true
			result.ExitCode = -1
		} else {
			result.Error = err.Error()
			result.ExitCode = -1
		}
	} else {
		result.ExitCode = 0
	}

	result.EndTime = time.Now()
	result.DurationMS = result.EndTime.Sub(result.StartTime).Milliseconds()

	return result, nil
}

// Cleanup removes sandbox resources.
func (s *Sandbox) Cleanup() error {
	// TODO: Remove sandbox profile if created
	return nil
}

// Capabilities returns no isolation features because sandbox profile isolation is not yet implemented.
// Run currently provides no isolation; add capabilities only after they are implemented.
func (s *Sandbox) Capabilities() []string {
	return []string{}
}

// NewMacOSSandbox creates a new macOS sandbox.
func NewMacOSSandbox(cfg sandbox.Config) (sandbox.Sandbox, error) {
	// Check macOS version
	if !isMacOSVersionSupported() {
		return nil, sandbox.UnsupportedError{
			Reason: "macOS 10.14 or later required for Sandbox",
		}
	}

	// Generate sandbox ID
	sandboxID := "com.intentlens.exercise-" + randomSuffix()

	return &Sandbox{
		cfg:       cfg,
		sandboxID: sandboxID,
	}, nil
}

// isMacOSVersionSupported checks if the macOS version supports Sandbox.
func isMacOSVersionSupported() bool {
	// TODO: Check macOS version using syscall.SysctlUint32("kern.osrelease")
	// macOS 10.14 = Darwin 18.0.0
	return true
}

// generateSandboxProfile creates a macOS sandbox profile for the given workDir.
func (s *Sandbox) generateSandboxProfile(workDir string) string {
	profile := fmt.Sprintf(`
(version 1)
(deny default (with no-log))

;; Allow essential processes
(allow process-exec (literal "/usr/bin/go"))
(allow process-info-pidinfo)
(allow process-info-pidfdinfo)

;; Allow work directory access
(allow file-read* (subpath "%s"))
(allow file-write* (subpath "%s"))

;; Allow system libraries
(allow file-read-data (glob "/usr/lib/libSystem*"))
(allow file-read-data (glob "/usr/lib/libobjc*"))
(allow file-read-metadata (subpath "/usr/lib"))
(allow file-read-metadata (subpath "/System/Library"))

;; Basic file operations
(allow file-read-metadata)
(allow file-write-xattr (only-writes-xattr "com.apple.macl"))

;; Deny network
(deny network* (all))

;; Deny other processes and privileged operations
(deny process-exec)
(deny syscall-unix)
`, workDir, workDir)

	return profile
}

// applySandbox loads the sandbox profile.
func (s *Sandbox) applySandbox(profile string) error {
	// TODO: Use sandbox_init() to apply the profile
	// This requires cgo bindings to libsandbox
	return nil
}

// filteredEnvironment returns a filtered environment for macOS.
func (s *Sandbox) filteredEnvironment() []string {
	whitelist := map[string]bool{
		"PATH":     true,
		"TEMP":     true,
		"TMPDIR":   true,
		"HOME":     true,
		"LANG":     true,
		"LC_ALL":   true,
		"GOARCH":   true,
		"GOOS":     true,
		"GOROOT":   true,
		"GOPATH":   true,
		"GOTMPDIR": true,
	}

	// Add user-specified allowed variables
	for _, v := range s.cfg.AllowedEnvVars {
		whitelist[strings.ToUpper(v)] = true
	}

	var filtered []string
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 && whitelist[parts[0]] {
			filtered = append(filtered, env)
		}
	}

	return filtered
}

// randomSuffix generates a random suffix for sandbox ID.
func randomSuffix() string {
	// TODO: Generate random hex string
	return "temp"
}
