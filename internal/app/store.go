package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"intentlens/internal/capture"
	"intentlens/internal/explain"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type App struct {
	prescanBeforePublish    func() error // Test seam before atomic replacement; never serialized.
	prescanExpiry           *time.Timer  // Guarded by scrubMu.
	prescanExpires          time.Time
	prescanExpiryGeneration string

	scrubMu        sync.Mutex
	prescanTake    func(bool) (capture.Snapshot, capture.Coverage, error) // Test seam; never serialized.
	prescanTimeout time.Duration                                          // Test seam, may only shorten the hard deadline.

	applyStageFault   func() error    // Test seam before a source rename.
	applyFault        func(int) error // In-process failure injection; never serialized.
	afterModelReserve func()          // Test seam after usage is reserved on disk.
	modelTransport    http.RoundTripper
	modelRun          func(context.Context, explain.Config, explain.Usage, string, string, []explain.Evidence) (explain.Document, explain.Usage, error)
	selectionRun      func(context.Context, explain.Config, explain.Usage, string, string, []explain.Evidence) (explain.Document, explain.Usage, error)
	exerciseRun       func(context.Context, explain.Config, explain.Usage, string, string) (string, explain.Usage, error)
	Root              string
	State             string
	scrubbedState     string              // Policy and record listing last scrubbed by this process.
	scrubbedEvidence  map[string]struct{} // Project paths cited by explanations at that scrub.

	// EditorModelTarget is set only for a key supplied by the VS Code extension.
	EditorModelTarget *EditorModelTarget
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
	if recoverErr := a.recoverWithPrescan(); recoverErr != nil {
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
	a.modelTransport = transport
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
	return readStateRecord(root, name, v, root.Open)
}

func readStateRecord(root *os.Root, name string, v any, open func(string) (*os.File, error)) error {
	for range 3 {
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("本地记录必须是普通文件")
		}
		if info.Size() > maxStateRecordBytes || (isPrescanManifest(name) && info.Size() > maxPrescanBytes) {
			return errors.New("本地记录超过读取范围")
		}
		f, err := open(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		openedInfo, err := f.Stat()
		if err != nil {
			f.Close()
			return err
		}
		if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
			f.Close()
			continue
		}
		defer f.Close()
		return decodeStateRecord(f, name, v)
	}
	return errors.New("本地记录路径在读取期间发生变化")
}

func decodeStateRecord(f *os.File, name string, v any) error {
	limit := int64(maxStateRecordBytes)
	if isPrescanManifest(name) {
		limit = maxPrescanBytes
	}
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil {
		return e
	}
	if int64(len(b)) > limit {
		return errors.New("本地记录超过读取范围")
	}
	if name == "usage.json" || name == "settings.json" {
		return validateStateRecord(name, b, v)
	}
	return json.Unmarshal(b, v)
}
func (a *App) write(name string, v any) error {
	return a.writeContext(context.Background(), name, v)
}
func (a *App) writeContext(ctx context.Context, name string, v any) error {
	release, err := a.lock("state")
	if err != nil {
		return err
	}
	defer release()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if isPrescanManifest(name) {
		if len(b) > maxPrescanBytes {
			return errors.New("预扫构建失败：缓存编码超过64 MiB")
		}
		// Include the old published slot while its replacement is staged. The
		// temporary second generation does not silently double the disk budget.
		total := int64(len(b))
		entries, err := os.ReadDir(a.State)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if isPrescanManifest(entry.Name()) {
				info, err := entry.Info()
				if err != nil {
					return err
				}
				total += info.Size()
			}
		}
		if total > maxPrescanBytes {
			return errors.New("预扫构建失败：新旧代际及暂存总编码超过64 MiB")
		}
		m, ok := v.(*prescanManifest)
		if !ok {
			return errors.New("预扫记录必须使用内部manifest")
		}
		if err := validatePrescan(m); err != nil {
			return err
		}
	}
	root, err := os.OpenRoot(a.State)
	if err != nil {
		return err
	}
	defer root.Close()
	if isPrescanManifest(name) {
		return atomicReplaceStaged(root, name, b, 0o600, nil, "", func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if a.prescanBeforePublish != nil {
				return a.prescanBeforePublish()
			}
			return nil
		})
	}
	return atomicReplace(root, name, b, 0o600, nil)
}
