// Package noisolation runs exercise tools WITHOUT any OS-level isolation.
// It is not a security boundary and must not be wired into an execution path.
package noisolation

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

// Sandbox defines the interface for executing code in an isolated environment.
type Sandbox interface {
	// Run executes the Go test tool with the given arguments in a sandboxed environment.
	// The working directory is limited to workDir.
	Run(ctx context.Context, tool string, args []string, workDir string) (*Result, error)

	// Cleanup removes any resources created by the sandbox.
	// This should be called after Run completes.
	Cleanup() error

	// Capabilities returns what this sandbox can isolate.
	Capabilities() []string
}

// Result describes the outcome of a sandboxed execution.
type Result struct {
	// ExitCode is the process exit code. -1 if not available.
	ExitCode int `json:"exit_code"`

	// Stdout is the captured standard output.
	Stdout string `json:"stdout"`

	// Stderr is the captured standard error.
	Stderr string `json:"stderr"`

	// Timeout indicates if the execution was terminated due to timeout.
	Timeout bool `json:"timeout"`

	// MemoryExceeded indicates if memory limit was hit.
	MemoryExceeded bool `json:"memory_exceeded"`

	// ResourceLimitExceeded indicates if any resource limit was exceeded.
	ResourceLimitExceeded bool `json:"resource_limit_exceeded"`

	// Error is set if execution failed (e.g., sandbox creation failed).
	Error string `json:"error,omitempty"`

	// StartTime is when the execution started.
	StartTime time.Time `json:"start_time"`

	// EndTime is when the execution ended.
	EndTime time.Time `json:"end_time"`

	// DurationMS is the execution duration in milliseconds.
	DurationMS int64 `json:"duration_ms"`
}

// Config defines sandbox parameters.
type Config struct {
	// WorkDir is the root directory for the sandboxed process.
	// The process can only access this directory and its subdirectories.
	WorkDir string

	// Timeout is the maximum execution duration.
	// Default: 10 seconds.
	Timeout time.Duration

	// MemoryLimitMB is the memory limit in megabytes.
	// Default: 500 MB.
	MemoryLimitMB int

	// MaxFDs is the maximum number of file descriptors.
	// Default: 256.
	MaxFDs int

	// MaxProcesses limits the number of child processes.
	// Default: 1 (only the main process).
	MaxProcesses int

	// AllowedEnvVars is a whitelist of environment variables to pass through.
	// Common vars like PATH and HOME are automatically included.
	AllowedEnvVars []string

	// DenyNetwork, if true, blocks all network access.
	// Default: true.
	DenyNetwork bool
}

// DefaultConfig returns a safe default configuration.
func DefaultConfig(workDir string) Config {
	return Config{
		WorkDir:        workDir,
		Timeout:        10 * time.Second,
		MemoryLimitMB:  500,
		MaxFDs:         256,
		MaxProcesses:   1,
		AllowedEnvVars: []string{},
		DenyNetwork:    true,
	}
}

// Validate checks that the configuration is valid and fills in defaults.
func (c *Config) Validate() error {
	// Validate WorkDir
	if c.WorkDir == "" {
		return fmt.Errorf("sandbox config: WorkDir is required")
	}

	// Validate Timeout
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	if c.Timeout > 5*time.Minute {
		return fmt.Errorf("sandbox config: Timeout cannot exceed 5 minutes")
	}

	// Validate memory limit
	if c.MemoryLimitMB <= 0 {
		c.MemoryLimitMB = 500
	}
	if c.MemoryLimitMB < 50 {
		return fmt.Errorf("sandbox config: MemoryLimitMB must be at least 50 MB")
	}
	if c.MemoryLimitMB > 10000 {
		return fmt.Errorf("sandbox config: MemoryLimitMB cannot exceed 10000 MB")
	}

	// Validate MaxFDs
	if c.MaxFDs <= 0 {
		c.MaxFDs = 256
	}
	if c.MaxFDs < 64 {
		return fmt.Errorf("sandbox config: MaxFDs must be at least 64")
	}

	// Validate MaxProcesses
	if c.MaxProcesses <= 0 {
		c.MaxProcesses = 1
	}
	if c.MaxProcesses > 100 {
		return fmt.Errorf("sandbox config: MaxProcesses cannot exceed 100")
	}

	return nil
}

// Duration returns the execution duration.
func (r *Result) Duration() time.Duration {
	return r.EndTime.Sub(r.StartTime)
}

// Platform-specific constructors (set by init functions in platform files)
var (
	newWindowsSandbox func(Config) (Sandbox, error)
	newLinuxSandbox   func(Config) (Sandbox, error)
	newMacOSSandbox   func(Config) (Sandbox, error)
)

// RegisterWindowsSandbox registers the Windows sandbox implementation.
func RegisterWindowsSandbox(fn func(Config) (Sandbox, error)) {
	newWindowsSandbox = fn
}

// RegisterLinuxSandbox registers the Linux sandbox implementation.
func RegisterLinuxSandbox(fn func(Config) (Sandbox, error)) {
	newLinuxSandbox = fn
}

// RegisterMacOSSandbox registers the macOS sandbox implementation.
func RegisterMacOSSandbox(fn func(Config) (Sandbox, error)) {
	newMacOSSandbox = fn
}

// New creates a new sandbox for the current platform.
func New(cfg Config) (Sandbox, error) {
	// Validate configuration first
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	switch runtime.GOOS {
	case "windows":
		if newWindowsSandbox == nil {
			return nil, UnsupportedError{Reason: "Windows sandbox not available"}
		}
		return newWindowsSandbox(cfg)
	case "linux":
		if newLinuxSandbox == nil {
			return nil, UnsupportedError{Reason: "Linux sandbox not available"}
		}
		return newLinuxSandbox(cfg)
	case "darwin":
		if newMacOSSandbox == nil {
			return nil, UnsupportedError{Reason: "macOS sandbox not available"}
		}
		return newMacOSSandbox(cfg)
	default:
		return nil, fmt.Errorf("sandbox not supported on %s", runtime.GOOS)
	}
}

// NotImplementedError is returned when a sandbox feature is not available.
type NotImplementedError struct {
	Platform string
	Feature  string
}

func (e NotImplementedError) Error() string {
	return fmt.Sprintf("sandbox not implemented for %s: %s", e.Platform, e.Feature)
}

// UnsupportedError is returned when sandboxing is not supported in the environment.
type UnsupportedError struct {
	Reason string
}

func (e UnsupportedError) Error() string {
	return fmt.Sprintf("sandboxing not supported: %s", e.Reason)
}
