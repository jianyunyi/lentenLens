package exercise

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
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
		return Executable{}, errors.New("需要本机 Go 1.24+；不会自动安装或下载")
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return Executable{}, err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return Executable{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return Executable{}, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return Executable{}, err
	}
	return Executable{path, hex.EncodeToString(h.Sum(nil))}, nil
}

func verifyTool(tool Executable) error {
	if !filepath.IsAbs(tool.Path) {
		return errors.New("练习执行工具路径无效")
	}
	expected, err := hex.DecodeString(tool.SHA256)
	if err != nil || len(expected) != sha256.Size {
		return errors.New("练习执行工具摘要无效")
	}
	f, err := os.Open(tool.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return err
	}
	if !bytes.Equal(h.Sum(nil), expected) {
		return errors.New("Go 执行器在确认后发生变化")
	}
	return nil
}

type limitedOutput struct {
	mu       sync.Mutex
	b        bytes.Buffer
	overflow bool
}

func (w *limitedOutput) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.b.Len()+len(b) > 1<<20 {
		w.overflow = true
		return 0, errors.New("测试输出超过 1 MiB")
	}
	return w.b.Write(b)
}
func Run(ctx context.Context, tool Executable, dir, cache string) Result {
	started := time.Now()
	r := Result{Tests: map[string]string{}, Expected: map[string]bool{}, ExitCode: -1}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := verifyTool(tool); err != nil {
		r.Failure = "tool_changed"
		r.DurationMS = time.Since(started).Milliseconds()
		return r
	}
	if err := os.MkdirAll(cache, 0700); err != nil {
		r.Failure = "environment_error"
		return r
	}
	tmp := filepath.Join(cache, "tmp")
	if err := os.MkdirAll(tmp, 0700); err != nil {
		r.Failure = "environment_error"
		return r
	}
	cmd := exec.CommandContext(ctx, tool.Path, Args()...)
	cmd.Dir = dir
	cmd.Env = []string{"GOPROXY=off", "GOSUMDB=off", "GOWORK=off", "GOENV=off", "GOTOOLCHAIN=local", "CGO_ENABLED=0", "GOFLAGS=", "GOTELEMETRY=off", "GOCACHE=" + cache, "GOPATH=" + filepath.Join(cache, "gopath"), "HOME=" + cache, "USERPROFILE=" + cache, "TMP=" + tmp, "TEMP=" + tmp, "TMPDIR=" + tmp, "PATH=" + filepath.Dir(tool.Path)}
	for _, key := range []string{"SystemRoot", "WINDIR"} {
		if v := os.Getenv(key); v != "" {
			cmd.Env = append(cmd.Env, key+"="+v)
		}
	}
	configureProcess(cmd)
	cmd.WaitDelay = 2 * time.Second
	var output limitedOutput
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	r.DurationMS = time.Since(started).Milliseconds()
	if cmd.ProcessState != nil {
		r.ExitCode = cmd.ProcessState.ExitCode()
	}
	if ctx.Err() != nil {
		r.Failure = "timeout"
		return r
	}
	if output.overflow {
		r.Failure = "output_limit"
		return r
	}
	scanner := bufio.NewScanner(bytes.NewReader(output.b.Bytes()))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	packagePass := false
	failedTest := false
	for scanner.Scan() {
		var event struct {
			Action string
			Test   string
			Output string
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		if event.Test != "" {
			switch event.Action {
			case "pass", "fail", "skip":
				r.Tests[event.Test] = event.Action
				if event.Action == "fail" {
					failedTest = true
				}
			}
			if bytes.Contains([]byte(event.Output), []byte(MissingMarker)) {
				r.Expected[event.Test] = true
			}
		} else if event.Action == "skip" {
			r.NoTests = true
		} else if event.Action == "pass" {
			packagePass = true
		}
	}
	r.Passed = err == nil && (packagePass || r.NoTests) && scanner.Err() == nil
	if !r.Passed {
		r.Failure = "build_or_environment_error"
		if failedTest {
			r.Failure = "tests_failed"
		}
	}
	return r
}
