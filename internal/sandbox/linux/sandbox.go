//go:build linux
// +build linux

package linux

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"intentlens/internal/sandbox"
)

// Sandbox implements sandbox.Sandbox for Linux without process isolation.
type Sandbox struct {
	cfg        sandbox.Config
	cgroupPath string
}

// Run executes the tool with a timeout and filtered environment, without isolation.
func (s *Sandbox) Run(ctx context.Context, tool string, args []string, workDir string) (*sandbox.Result, error) {
	result := &sandbox.Result{
		StartTime: time.Now(),
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	// Prepare the command
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Dir = workDir
	cmd.Env = s.filteredEnvironment()

	// Set up output capture
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// TODO: Set up seccomp filter
	// cmd.SysProcAttr = &syscall.SysProcAttr{
	//     SeccompFilter: ...,
	// }

	// TODO: Set up resource limits via cgroup
	// s.setupCgroup(cmd.Process.Pid)

	// Run the command
	err := cmd.Run()
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	// Capture exit code
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				result.ExitCode = status.ExitStatus()
			}
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

// Cleanup removes the cgroup directory if one has been recorded.
func (s *Sandbox) Cleanup() error {
	// TODO: Remove cgroup directory
	if s.cgroupPath != "" {
		return os.RemoveAll(s.cgroupPath)
	}
	return nil
}

// Capabilities returns no isolation features because seccomp/cgroup isolation is not yet implemented.
// Run currently provides no isolation; add capabilities only after they are implemented.
func (s *Sandbox) Capabilities() []string {
	return []string{}
}

// NewLinuxSandbox creates a new Linux sandbox.
func NewLinuxSandbox(cfg sandbox.Config) (sandbox.Sandbox, error) {
	// Check if running on Linux
	if runtime.GOOS != "linux" {
		return nil, sandbox.NotImplementedError{
			Platform: runtime.GOOS,
			Feature:  "Linux sandbox",
		}
	}

	// Check for cgroup v2
	if !hasCgroupV2() {
		return nil, sandbox.UnsupportedError{
			Reason: "cgroup v2 not available; requires Linux 5.8+ or cgroup v2 explicitly enabled",
		}
	}

	// Check for seccomp support
	if !hasSeccomp() {
		return nil, sandbox.UnsupportedError{
			Reason: "seccomp-bpf not available",
		}
	}

	return &Sandbox{cfg: cfg}, nil
}

// hasCgroupV2 checks if cgroup v2 is available.
func hasCgroupV2() bool {
	// Check if /sys/fs/cgroup/cgroup.controllers exists
	_, err := os.Stat("/sys/fs/cgroup/cgroup.controllers")
	return err == nil
}

// hasSeccomp checks if seccomp is available.
func hasSeccomp() bool {
	// Query the current seccomp mode; a successful query indicates support.
	// prctl takes five arguments; the final zero pads RawSyscall6's sixth slot.
	_, _, errno := syscall.RawSyscall6(syscall.SYS_PRCTL, syscall.PR_GET_SECCOMP, 0, 0, 0, 0, 0)
	return errno == 0
}

// filteredEnvironment returns a filtered environment with only allowed variables.
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
