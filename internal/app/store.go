package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"intentlens/internal/explain"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type App struct {
	Root        string
	State       string
	modelRun    func(context.Context, explain.Config, explain.Usage, string, string, []explain.Evidence) (explain.Document, explain.Usage, error)
	exerciseRun func(context.Context, explain.Config, explain.Usage, string, string) (string, explain.Usage, error)
}

func Open(root, state string) (*App, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, errors.New("项目目录不存在")
	}
	if state == "" {
		base, e := os.UserConfigDir()
		if e != nil {
			return nil, e
		}
		state = filepath.Join(base, "IntentLens", "projects")
	}
	state, err = filepath.Abs(state)
	if err != nil {
		return nil, err
	}
	if contained(root, state) {
		return nil, errors.New("本地私有状态目录必须位于项目之外")
	}
	if err = os.MkdirAll(state, 0700); err != nil {
		return nil, err
	}
	state, err = filepath.EvalSymlinks(state)
	if err != nil {
		return nil, err
	}
	if contained(root, state) {
		return nil, errors.New("本地私有状态目录不能通过链接落入项目")
	}
	identity := root
	if runtime.GOOS == "windows" {
		identity = strings.ToLower(root)
	}
	state = filepath.Join(state, hash(identity)[:24])
	if err = os.MkdirAll(state, 0700); err != nil {
		return nil, err
	}
	canonical, err := filepath.EvalSymlinks(state)
	if err != nil {
		return nil, err
	}
	if canonical != state || contained(root, canonical) {
		return nil, errors.New("项目私有状态目录不能是链接")
	}
	a := &App{Root: root, State: state}
	if err = a.scrubExcluded(); err != nil {
		return nil, err
	}
	return a, nil
}

// OpenWithHTTPTransport uses the normal model runner with an in-process HTTP transport.
// The transport is process-local and is never persisted with project settings.
func OpenWithHTTPTransport(root, state string, transport http.RoundTripper) (*App, error) {
	a, err := Open(root, state)
	if err != nil {
		return nil, err
	}
	if transport != nil {
		a.modelRun = func(ctx context.Context, cfg explain.Config, usage explain.Usage, key, question string, evidence []explain.Evidence) (explain.Document, explain.Usage, error) {
			cfg.HTTPTransport = transport
			return explain.Run(ctx, cfg, usage, key, question, evidence)
		}
	}
	return a, nil
}

func contained(root, p string) bool {
	rel, e := filepath.Rel(root, p)
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
func hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func newID() string {
	var b [12]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}

func (a *App) lock(name string) (func(), error) {
	p := filepath.Join(a.State, name+".lock")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("项目操作进行中；若上次进程异常退出，请检查本地 %s 锁", name)
	}
	_, _ = fmt.Fprintf(f, "pid=%d\n", os.Getpid())
	_ = f.Close()
	return func() { _ = os.Remove(p) }, nil
}

func (a *App) read(name string, v any) error {
	info, e := os.Lstat(filepath.Join(a.State, name))
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() {
		return errors.New("本地记录必须是普通文件")
	}
	b, e := os.ReadFile(filepath.Join(a.State, name))
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func (a *App) write(name string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(a.State, ".tmp-")
	if e != nil {
		return e
	}
	p := f.Name()
	defer os.Remove(p)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	return os.Rename(p, filepath.Join(a.State, name))
}
