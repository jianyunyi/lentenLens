package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"intentlens/internal/explain"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type App struct {
	applyStageFault func() error    // Test seam before a source rename.
	applyFault      func(int) error // In-process failure injection; never serialized.
	modelRun        func(context.Context, explain.Config, explain.Usage, string, string, []explain.Evidence) (explain.Document, explain.Usage, error)
	selectionRun    func(context.Context, explain.Config, explain.Usage, string, string, []explain.Evidence) (explain.Document, explain.Usage, error)
	exerciseRun     func(context.Context, explain.Config, explain.Usage, string, string) (string, explain.Usage, error)
	Root            string
	State           string
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
	if mkdirErr := os.MkdirAll(state, 0o700); mkdirErr != nil {
		return nil, mkdirErr
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
	if mkdirErr := os.MkdirAll(state, 0o700); mkdirErr != nil {
		return nil, mkdirErr
	}
	canonical, err := filepath.EvalSymlinks(state)
	if err != nil {
		return nil, err
	}
	if canonical != state || contained(root, canonical) {
		return nil, errors.New("项目私有状态目录不能是链接")
	}
	a := &App{Root: root, State: state}
	if recoverErr := a.recoverState(); recoverErr != nil {
		return nil, recoverErr
	}
	if scrubErr := a.scrubExcluded(); scrubErr != nil {
		return nil, scrubErr
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
		a.selectionRun = func(ctx context.Context, cfg explain.Config, usage explain.Usage, key, question string, evidence []explain.Evidence) (explain.Document, explain.Usage, error) {
			cfg.HTTPTransport = transport
			return explain.RunSelection(ctx, cfg, usage, key, question, evidence)
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

func (a *App) read(name string, v any) error {
	root, e := os.OpenRoot(a.State)
	if e != nil {
		return e
	}
	defer root.Close()
	info, e := root.Lstat(name)
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() {
		return errors.New("本地记录必须是普通文件")
	}
	if info.Size() > maxStateRecordBytes {
		return errors.New("本地记录超过读取范围")
	}
	f, e := root.Open(name)
	if e != nil {
		return e
	}
	defer f.Close()
	openedInfo, e := f.Stat()
	if e != nil {
		return e
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return errors.New("本地记录路径在读取期间发生变化")
	}
	b, e := io.ReadAll(io.LimitReader(f, maxStateRecordBytes+1))
	if e != nil {
		return e
	}
	if int64(len(b)) > maxStateRecordBytes {
		return errors.New("本地记录超过读取范围")
	}
	if name == "usage.json" || name == "settings.json" {
		return validateStateRecord(name, b, v)
	}
	return json.Unmarshal(b, v)
}
func (a *App) write(name string, v any) error {
	release, err := a.lock("state")
	if err != nil {
		return err
	}
	defer release()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(a.State)
	if err != nil {
		return err
	}
	defer root.Close()
	return atomicReplace(root, name, b, 0o600, nil)
}
