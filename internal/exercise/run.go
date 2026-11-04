package exercise

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type Executable struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Result struct {
	NoTests    bool              `json:"no_tests,omitempty"`
	Passed     bool              `json:"passed"`
	Failure    string            `json:"failure,omitempty"`
	Tests      map[string]string `json:"tests"`
	Expected   map[string]bool   `json:"expected_failures,omitempty"`
	DurationMS int64             `json:"duration_ms"`
	ExitCode   int               `json:"exit_code"`
}

func Args() []string { return []string{"test", "-json", "-count=1", "-timeout=10s", "."} }

func Tool() (Executable, error) {
	path, err := exec.LookPath("go")
	if err != nil {
		return Executable{}, errors.New("需要本机 Go 1.25+；不会自动安装或下载")
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return Executable{}, err
	}
	// Keep the absolute lookup path: Windows tool caches may be reached through
	// junctions that EvalSymlinks cannot canonicalize across volumes. Hashing and
	// hashing the executable at this same path preserves a stable preview identity.
	f, err := os.Open(path) // #nosec G304 -- path is the absolute executable resolved by LookPath("go"), which this feature intentionally fingerprints.
	if err != nil {
		return Executable{}, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return Executable{}, err
	}
	return Executable{Path: path, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

var ErrSandboxUnavailable = errors.New("exercise-run 已关闭：当前没有经过验证的操作系统沙箱，不会启动本机 Go 测试")

// There is deliberately no OS sandbox adapter in this version. Do not change
// this to true unless a sandbox boundary is implemented and independently verified.
const verifiedOSSandboxAvailable = false

func OSSandboxAvailable() bool { return verifiedOSSandboxAvailable }

// runWhenSandboxVerified is the single gate to any future execution callback.
// A false or unverified sandbox must return without invoking code that could
// start a child process.
func runWhenSandboxVerified(verified bool, run func() Result) Result {
	if !verified || run == nil {
		return Result{
			Failure:  "sandbox_unavailable",
			Tests:    map[string]string{},
			Expected: map[string]bool{},
			ExitCode: -1,
		}
	}
	return run()
}

// Run is intentionally fail-closed. A Go test process is native code execution,
// not a sandbox; confirmation, timeouts, and environment filtering do not make it safe.
func Run(_ context.Context, _ Executable, _, _ string) Result {
	return runWhenSandboxVerified(verifiedOSSandboxAvailable, nil)
}
