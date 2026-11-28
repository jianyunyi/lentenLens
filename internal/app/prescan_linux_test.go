//go:build linux

package app

import (
	"errors"
	"intentlens/internal/capture"
	"intentlens/internal/explain"
	"path/filepath"
	"syscall"
	"testing"
)

// Observe the actual source-file open boundary, including Open/recovery/scrub.
// The capture test seam alone would not catch an accidental os.ReadFile here.
func TestPrescanHotQueriesDoNotOpenGoOrMarkdown(t *testing.T) {
	a, root := prescanFixture(t)
	scanForTest(t, a, false)
	if e := a.write("explanation-cachehistory.json", Explanation{ID: "cachehistory", File: "main.go", FunctionID: "main.go:A", Evidence: []explain.Evidence{{File: "main.go", Source: "OLD_SAVED", Hash: "old"}}}); e != nil {
		t.Fatal(e)
	}
	fd, e := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if e != nil {
		t.Fatal(e)
	}
	defer syscall.Close(fd)
	for _, p := range []string{"main.go", "docs/design.md"} {
		if _, e = syscall.InotifyAddWatch(fd, filepath.Join(root, p), syscall.IN_OPEN); e != nil {
			t.Fatal(e)
		}
	}
	reopened, e := Open(root, filepath.Dir(a.State))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = reopened.PrescanIndex(); e != nil {
		t.Fatal(e)
	}
	info, e := reopened.PrescanInspect("main.go", 2)
	if e != nil {
		t.Fatal(e)
	}
	if len(info.History) != 1 || info.History[0].Freshness != "基于预扫快照；历史证据当前性未验证" {
		t.Fatal(info.History)
	}
	b := make([]byte, 4096)
	n, e := syscall.Read(fd, b)
	if n > 0 || !errors.Is(e, syscall.EAGAIN) {
		t.Fatalf("hot query opened source: n=%d err=%v", n, e)
	}
	// Positive control: an explicitly allowed capture must be observable.
	if _, _, e = capture.TakeGo(root); e != nil {
		t.Fatal(e)
	}
	n, e = syscall.Read(fd, b)
	if e != nil || n <= 0 {
		t.Fatalf("watch was not observing opens: %d %v", n, e)
	}
}
