package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"intentlens/internal/capture"
	"intentlens/internal/codeindex"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func prescanFixture(t *testing.T) (*App, string) {
	t.Helper()
	a, root := fixture(t)
	put(t, root, "main.go", "package p\nfunc A() { B() }\nfunc B() {}\n")
	putCoverage(t, root, "docs/design.md", "# Design\nA calls B for an example.\n")
	return a, root
}
func scanForTest(t *testing.T, a *App, refresh bool) PrescanResult {
	t.Helper()
	r, e := a.Prescan(context.Background(), true, refresh)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func readManifestForTest(t *testing.T, a *App) (string, prescanManifest) {
	t.Helper()
	entries, e := os.ReadDir(a.State)
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if isPrescanManifest(entry.Name()) {
			var m prescanManifest
			if e = a.read(entry.Name(), &m); e != nil {
				t.Fatal(e)
			}
			return entry.Name(), m
		}
	}
	t.Fatal("manifest not published")
	return "", prescanManifest{}
}
func TestPrescanManifestAndExplicitVerificationHit(t *testing.T) {
	a, root := prescanFixture(t)
	if _, e := a.Prescan(context.Background(), false, false); e == nil {
		t.Fatal("retention consent bypassed")
	}
	old, e := a.Index()
	if e != nil {
		t.Fatal(e)
	}
	first := scanForTest(t, a, false)
	name, m := readManifestForTest(t, a)
	if e = validatePrescan(&m); e != nil {
		t.Fatal(e)
	}
	if name != prescanName(m.PolicyHash) || m.Schema != 1 || m.Config.GoParser == "" || m.Config.Markdown == "" || m.Config.Tokenizer == "" || m.Generation == "" || m.Workspace != hash(root) || len(m.Files) != 2 || m.Files["main.go"].Bytes == 0 || m.AuthorizationVersion != 1 || m.Integrity == "" {
		t.Fatalf("incomplete manifest: %+v", m)
	}
	if !reflect.DeepEqual(old.Functions, first.Go.Functions) || !reflect.DeepEqual(old.Coverage, first.Go.Coverage) || !reflect.DeepEqual(old.Issues, first.Go.Issues) {
		t.Fatal("legacy Go result changed")
	}
	if first.Coverage.Go.Scope != "go-only" || first.Coverage.Docs.Scope != "docs-only" || first.Coverage.GoReasons == nil || first.Coverage.DocsReasons == nil || first.Coverage.ParseOmissions == nil {
		t.Fatal("coverage missing")
	}
	foundEdge := false
	for k, v := range m.Body.Sources {
		if strings.HasPrefix(k, "call:") && len(v) >= 1 && v[0].StartLine == 1 && v[0].EndLine >= 3 {
			foundEdge = true
		}
	}
	if !foundEdge {
		t.Fatal("target declaration omitted from call lineage")
	}
	b, _ := os.ReadFile(filepath.Join(a.State, name))
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(b, &raw)
	if _, ok := raw["snapshot"]; ok || strings.Contains(string(b), `"content":`) {
		t.Fatal("generic full-file snapshot persisted")
	}
	info, _ := os.Stat(filepath.Join(a.State, name))
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	second := scanForTest(t, a, false)
	if second.Prescan.State != "hit" || !second.Prescan.Reused || second.Prescan.Generation != first.Prescan.Generation || !reflect.DeepEqual(first.Go, second.Go) {
		t.Fatalf("verification did not reuse: %+v", second.Prescan)
	}
	// A later process only knows snapshot time, regardless of how recently it ran.
	reopened, e := Open(root, filepath.Dir(a.State))
	if e != nil {
		t.Fatal(e)
	}
	reused, e := reopened.CachedPrescan()
	if e != nil {
		t.Fatal(e)
	}
	if reused.Prescan.State != "unknown" || !strings.Contains(reused.Prescan.Message, "未验证") {
		t.Fatal(reused.Prescan)
	}
	if hits := reused.Docs.Search(codeindex.DocQuery{Text: "example"}); len(hits) != 1 {
		t.Fatal("postings lost on JSON restore", hits)
	}
}
func TestPrescanInvalidationThreeConditionsAndSingleGeneration(t *testing.T) {
	for _, kind := range []string{"policy", "markdown", "add", "delete"} {
		t.Run(kind, func(t *testing.T) {
			a, root := prescanFixture(t)
			first := scanForTest(t, a, false)
			oldName, _ := readManifestForTest(t, a)
			switch kind {
			case "policy":
				put(t, root, ".intentignore", "docs/design.md\n")
			case "markdown":
				putCoverage(t, root, "docs/design.md", "# Changed\nNew design.\n")
			case "add":
				put(t, root, "new.go", "package p\nfunc New() {}\n")
			case "delete":
				if e := os.Remove(filepath.Join(root, "docs/design.md")); e != nil {
					t.Fatal(e)
				}
			}
			if kind == "policy" {
				r, e := a.CachedPrescan()
				if e != nil {
					t.Fatal(e)
				}
				if r.Prescan.State != "stale" || len(r.Go.Functions) != 0 || len(r.Docs.Paragraphs) != 0 {
					t.Fatal(r)
				}
				if _, e = os.Stat(filepath.Join(a.State, oldName)); !errors.Is(e, os.ErrNotExist) {
					t.Fatal("old policy retained")
				}
			}
			r := scanForTest(t, a, false)
			if r.Prescan.State != "hit" || !r.Prescan.Rebuilt || r.Prescan.Generation == first.Prescan.Generation || r.Prescan.PreviousState != "stale" {
				t.Fatal(r.Prescan)
			}
			entries, _ := os.ReadDir(a.State)
			n := 0
			for _, entry := range entries {
				if isPrescanManifest(entry.Name()) {
					n++
				}
			}
			if n != 1 {
				t.Fatalf("%d published generations", n)
			}
			if kind == "policy" {
				b, _ := os.ReadFile(filepath.Join(a.State, prescanName(prescanPolicyHash(mustPolicy(t, root).Hash()))))
				if strings.Contains(string(b), "A calls B for an example") {
					t.Fatal("revoked source retained")
				}
			}
		})
	}
}
func mustPolicy(t *testing.T, root string) capture.Policy {
	t.Helper()
	p, e := capture.LoadPolicy(root)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestPrescanFourStatesNoHotSourceReadsAndRevocation(t *testing.T) {
	a, root := prescanFixture(t)
	r, e := a.CachedPrescan()
	if e != nil || r.Prescan.State != "missing" || !strings.Contains(r.Prescan.Message, "需先预扫") {
		t.Fatal(r, e)
	}
	hit := scanForTest(t, a, false)
	if hit.Prescan.State != "hit" {
		t.Fatal(hit.Prescan)
	}
	a.prescanTake = func(bool) (capture.Snapshot, capture.Coverage, error) {
		t.Error("hot path captured source")
		return capture.Snapshot{}, capture.Coverage{}, errors.New("no reads")
	}
	// Unobserved external edits must stay unknown, never falsely claim a hit.
	putCoverage(t, root, "docs/design.md", "# External edit\nFresh contents.\n")
	r, e = a.CachedPrescan()
	if e != nil || r.Prescan.State != "unknown" || !strings.Contains(r.Docs.Paragraphs[0].Source, "example") {
		t.Fatal(r, e)
	}
	i, e := a.PrescanIndex()
	if e != nil || len(i.Functions) == 0 || i.Prescan.State != "unknown" {
		t.Fatal(i, e)
	}
	info, e := a.PrescanInspect("main.go", 2)
	if e != nil || info.Function.Name != "A" || len(info.History) != 0 {
		t.Fatal(info, e)
	}
	if e = a.MarkPrescanStale(); e != nil {
		t.Fatal(e)
	}
	r, e = a.CachedPrescan()
	if e != nil || r.Prescan.State != "stale" || len(r.Go.Functions) != 0 || len(r.Docs.Paragraphs) != 0 {
		t.Fatal(r, e)
	}
	if e = a.ClearPrescan(); e != nil {
		t.Fatal(e)
	}
	r, e = a.CachedPrescan()
	if e != nil || r.Prescan.State != "missing" {
		t.Fatal(r, e)
	}
	if _, e = a.Prescan(context.Background(), false, false); e == nil {
		t.Fatal("revoked permission reused")
	}
}
func TestPrescanCorruptionRecoveryPreservesHistory(t *testing.T) {
	for _, kind := range []string{"json", "integrity", "schema", "permission", "expiry"} {
		t.Run(kind, func(t *testing.T) {
			a, root := prescanFixture(t)
			scanForTest(t, a, false)
			name, m := readManifestForTest(t, a)
			history := []byte(`{"id":"old","evidence":[]}`)
			if e := os.WriteFile(filepath.Join(a.State, "explanation-old.json"), history, 0600); e != nil {
				t.Fatal(e)
			}
			state := "missing"
			switch kind {
			case "json":
				if e := os.WriteFile(filepath.Join(a.State, name), []byte(`{"body":"PRIVATE_BAD_CACHE"`), 0600); e != nil {
					t.Fatal(e)
				}
			case "integrity":
				m.Body.Go.Functions[0].Source = "tampered"
			case "schema":
				m.Schema++
				state = "stale"
			case "permission":
				p, e := a.prescanPermission()
				if e != nil {
					t.Fatal(e)
				}
				p.Epoch = newID()
				if e = a.write(prescanPermissionName, p); e != nil {
					t.Fatal(e)
				}
			case "expiry":
				m.Started = time.Now().Add(-prescanLifetime - time.Second)
				m.Finished = m.Started.Add(time.Millisecond)
				m.Integrity = manifestDigest(&m)
				state = "stale"
			}
			if kind == "integrity" || kind == "schema" || kind == "expiry" {
				b, _ := json.Marshal(m)
				if e := os.WriteFile(filepath.Join(a.State, name), b, 0600); e != nil {
					t.Fatal(e)
				}
			}
			reopened, e := Open(root, filepath.Dir(a.State))
			if e != nil {
				t.Fatal(e)
			}
			r, e := reopened.CachedPrescan()
			if e != nil || r.Prescan.State != state || len(r.Go.Functions) > 0 {
				t.Fatal(r, e)
			}
			after, _ := os.ReadFile(filepath.Join(a.State, "explanation-old.json"))
			if !reflect.DeepEqual(history, after) {
				t.Fatal("history damaged")
			}
			if _, e = os.Stat(filepath.Join(a.State, name)); !errors.Is(e, os.ErrNotExist) {
				t.Fatal("invalid source cache retained")
			}
			entries, _ := os.ReadDir(filepath.Join(a.State, "quarantine"))
			for _, entry := range entries {
				if strings.Contains(entry.Name(), "prescan-") {
					t.Fatal("source quarantined instead of removed")
				}
			}
		})
	}
}
func TestPrescanConcurrentRefreshCoalescesAndReplacesGeneration(t *testing.T) {
	a, _ := prescanFixture(t)
	first := scanForTest(t, a, false)
	var calls atomic.Int32
	entered := make(chan struct{})
	unblock := make(chan struct{})
	var once sync.Once
	a.prescanTake = func(docs bool) (capture.Snapshot, capture.Coverage, error) {
		calls.Add(1)
		once.Do(func() { close(entered); <-unblock })
		if docs {
			return capture.TakeDocs(a.Root)
		}
		return capture.TakeGo(a.Root)
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan PrescanResult, 6)
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r, e := a.Prescan(context.Background(), false, true)
			results <- r
			errs <- e
		}()
	}
	close(start)
	<-entered
	// All contenders have entered before the shared generation is published.
	time.Sleep(30 * time.Millisecond)
	close(unblock)
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	gen := ""
	for r := range results {
		if r.Prescan.State != "hit" {
			t.Fatal(r.Prescan)
		}
		if gen == "" {
			gen = r.Prescan.Generation
		}
		if gen != r.Prescan.Generation {
			t.Fatal("overlapping refreshes published different generations")
		}
	}
	if gen == first.Prescan.Generation || calls.Load() != 4 {
		t.Fatal(gen, calls.Load())
	}
	_, m := readManifestForTest(t, a)
	if m.Generation != gen {
		t.Fatal("old generation still published")
	}
}
func TestPrescanInconsistentCaptureAndDeadlineNeverPublish(t *testing.T) {
	for _, kind := range []string{"policy", "files", "deadline"} {
		t.Run(kind, func(t *testing.T) {
			a, _ := prescanFixture(t)
			first := scanForTest(t, a, false)
			name, _ := readManifestForTest(t, a)
			before, _ := os.ReadFile(filepath.Join(a.State, name))
			var calls int
			done := make(chan struct{})
			a.prescanTimeout = 10 * time.Millisecond
			a.prescanTake = func(docs bool) (capture.Snapshot, capture.Coverage, error) {
				calls++
				if kind == "deadline" && calls == 1 {
					<-done
				}
				var s capture.Snapshot
				var c capture.Coverage
				var e error
				if docs {
					s, c, e = capture.TakeDocs(a.Root)
				} else {
					s, c, e = capture.TakeGo(a.Root)
				}
				if kind == "policy" && docs {
					s.PolicyHash = "different"
				}
				if kind == "files" && calls == 3 {
					s.Files["late.go"] = capture.NewFile("package p")
				}
				return s, c, e
			}
			_, e := a.Prescan(context.Background(), false, true)
			if e == nil {
				t.Fatal("unsafe build accepted")
			}
			if kind == "deadline" {
				if !strings.Contains(e.Error(), "120秒") {
					t.Fatal(e)
				}
				close(done)
				time.Sleep(30 * time.Millisecond)
			}
			after, _ := os.ReadFile(filepath.Join(a.State, name))
			if !reflect.DeepEqual(before, after) {
				t.Fatal("failed build damaged old generation")
			}
			if first.Prescan.Generation == "" {
				t.Fatal("fixture")
			}
		})
	}
}
func TestPrescanEncodingLimitFailsBeforeAtomicReplacement(t *testing.T) {
	a, _ := prescanFixture(t)
	scanForTest(t, a, false)
	name, m := readManifestForTest(t, a)
	before, _ := os.ReadFile(filepath.Join(a.State, name))
	// JSON escaping expands bounded text beyond the encoded 64 MiB limit.
	m.Body.Go.Functions[0].Source = strings.Repeat("\x01", (maxPrescanBytes/6)+1)
	m.Integrity = manifestDigest(&m)
	if e := a.write(name, &m); e == nil || !strings.Contains(e.Error(), "64 MiB") {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(filepath.Join(a.State, name))
	if !reflect.DeepEqual(before, after) {
		t.Fatal("oversize write replaced old cache")
	}
}
func TestPrescanCoverageOmissionsSurviveJSON(t *testing.T) {
	a, root := prescanFixture(t)
	put(t, root, "bad.go", "package p\nfunc broken(")
	putCoverage(t, root, "docs/long.md", strings.Repeat("z", codeindex.MaxDocParagraphBytes+1))
	putCoverage(t, root, "docs/big.md", strings.Repeat("z", capture.MaxFileBytes+1))
	first := scanForTest(t, a, false)
	second, e := a.CachedPrescan()
	if e != nil {
		t.Fatal(e)
	}
	if second.Coverage.Go.Complete || second.Coverage.Docs.Complete || second.Coverage.ParseOmissions["bad.go"] != "parse_error" || second.Coverage.DocsReasons["docs/big.md"] != "file_too_large" || len(second.Docs.Omitted) != 1 {
		t.Fatal(second.Coverage, second.Docs.Omitted)
	}
	if !reflect.DeepEqual(first.Coverage, second.Coverage) {
		t.Fatal("internal omissions lost across JSON")
	}
}

func TestPrescanAtomicPublicationFailureAndExpiredCleanup(t *testing.T) {
	a, root := prescanFixture(t)
	scanForTest(t, a, false)
	name, m := readManifestForTest(t, a)
	before, _ := os.ReadFile(filepath.Join(a.State, name))
	a.prescanBeforePublish = func() error { return errors.New("simulated disk full before rename") }
	if _, e := a.Prescan(context.Background(), false, true); e == nil || !strings.Contains(e.Error(), "disk full") {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(filepath.Join(a.State, name))
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed atomic publish overwrote cache")
	}
	entries, _ := os.ReadDir(a.State)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".intentlens-write-") {
			t.Fatal("failed staging source retained")
		}
	}
	a.prescanBeforePublish = nil
	m.Started = time.Now().Add(-prescanLifetime + 200*time.Millisecond)
	m.Finished = m.Started.Add(time.Millisecond)
	m.Integrity = manifestDigest(&m)
	if e := a.write(name, &m); e != nil {
		t.Fatal(e)
	}
	reopened, e := Open(root, filepath.Dir(a.State))
	if e != nil {
		t.Fatal(e)
	}
	// No query triggers deletion; the long-lived host must expire it itself.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, e = os.Stat(filepath.Join(a.State, name)); errors.Is(e, os.ErrNotExist) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, e = os.Stat(filepath.Join(a.State, name)); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("online expiry failed")
	}
	r, e := reopened.CachedPrescan()
	if e != nil || r.Prescan.State != "stale" {
		t.Fatal(r, e)
	}
}

func TestPrescanProcessHelper(t *testing.T) {
	if os.Getenv("INTENTLENS_PRESCAN_HELPER") == "" {
		return
	}
	a, e := Open(os.Getenv("INTENTLENS_PRESCAN_ROOT"), os.Getenv("INTENTLENS_PRESCAN_STATE"))
	if e != nil {
		t.Fatal(e)
	}
	gate := os.Getenv("INTENTLENS_PRESCAN_GATE")
	id := os.Getenv("INTENTLENS_PRESCAN_ID")
	if e = os.WriteFile(filepath.Join(gate, "ready-"+id), nil, 0600); e != nil {
		t.Fatal(e)
	}
	limit := time.Now().Add(10 * time.Second)
	for {
		if _, e = os.Stat(filepath.Join(gate, "start")); e == nil {
			break
		}
		if time.Now().After(limit) {
			t.Fatal("gate timed out")
		}
		time.Sleep(time.Millisecond)
	}
	var once sync.Once
	a.prescanTake = func(docs bool) (capture.Snapshot, capture.Coverage, error) {
		once.Do(func() { time.Sleep(60 * time.Millisecond) })
		if docs {
			return capture.TakeDocs(a.Root)
		}
		return capture.TakeGo(a.Root)
	}
	r, e := a.Prescan(context.Background(), false, true)
	if e != nil {
		t.Fatal(e)
	}
	fmt.Println("PRESCAN_GENERATION=" + r.Prescan.Generation)
}
func TestPrescanConcurrentCLIProcessesSharePublishedGeneration(t *testing.T) {
	a, root := prescanFixture(t)
	first := scanForTest(t, a, false)
	gate := t.TempDir()
	commands := []*exec.Cmd{}
	outputs := make([]strings.Builder, 2)
	for i, id := range []string{"a", "b"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestPrescanProcessHelper$")
		cmd.Env = append(os.Environ(), "INTENTLENS_PRESCAN_HELPER=1", "INTENTLENS_PRESCAN_ROOT="+root, "INTENTLENS_PRESCAN_STATE="+filepath.Dir(a.State), "INTENTLENS_PRESCAN_GATE="+gate, "INTENTLENS_PRESCAN_ID="+id)
		cmd.Stdout = &outputs[i]
		cmd.Stderr = &outputs[i]
		if e := cmd.Start(); e != nil {
			t.Fatal(e)
		}
		commands = append(commands, cmd)
		t.Cleanup(func() { _ = cmd.Process.Kill() })
		deadline := time.Now().Add(10 * time.Second)
		for {
			if _, e := os.Stat(filepath.Join(gate, "ready-"+id)); e == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("helper did not open")
			}
			time.Sleep(time.Millisecond)
		}
	}
	if e := os.WriteFile(filepath.Join(gate, "start"), nil, 0600); e != nil {
		t.Fatal(e)
	}
	gen := ""
	for i, cmd := range commands {
		if e := cmd.Wait(); e != nil {
			t.Fatal(e, outputs[i].String())
		}
		s := outputs[i].String()
		at := strings.Index(s, "PRESCAN_GENERATION=")
		if at < 0 {
			t.Fatal(s)
		}
		current := strings.Fields(s[at:])[0]
		if gen == "" {
			gen = current
		} else if gen != current {
			t.Fatal("processes rebuilt independently", gen, current)
		}
	}
	if strings.HasSuffix(gen, first.Prescan.Generation) {
		t.Fatal("explicit refresh was skipped")
	}
}

func TestPrescanLineageIncludesFilesWithoutFunctionsAndSharesSourceSets(t *testing.T) {
	a, root := prescanFixture(t)
	put(t, root, "types.go", "package p\ntype T struct{}\n")
	scanForTest(t, a, false)
	name, m := readManifestForTest(t, a)
	for id, sources := range m.Body.Sources {
		if !strings.HasPrefix(id, "function:") && !strings.HasPrefix(id, "call:") {
			continue
		}
		found := false
		for _, s := range sources {
			if s.File == "types.go" {
				found = true
			}
		}
		if !found {
			t.Fatal("type context missing from source closure", id)
		}
	}
	b, e := os.ReadFile(filepath.Join(a.State, name))
	if e != nil {
		t.Fatal(e)
	}
	var raw struct {
		Body struct {
			Sets    map[string][]prescanSource `json:"source_sets"`
			Sources map[string][]string        `json:"sources"`
		} `json:"body"`
	}
	if e = json.Unmarshal(b, &raw); e != nil {
		t.Fatal(e)
	}
	if len(raw.Body.Sets) >= len(raw.Body.Sources) {
		t.Fatal("source closures not normalized")
	}
	for id, refs := range raw.Body.Sources {
		if len(refs) != 1 || len(raw.Body.Sets[refs[0]]) == 0 {
			t.Fatal("broken source reference", id)
		}
	}
}
func TestPrescanSymlinkCacheIsRemovedWithoutTouchingTarget(t *testing.T) {
	a, root := prescanFixture(t)
	scanForTest(t, a, false)
	name, _ := readManifestForTest(t, a)
	outside := filepath.Join(t.TempDir(), "keep.json")
	if e := os.WriteFile(outside, []byte("KEEP_OUTSIDE"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(filepath.Join(a.State, name)); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(outside, filepath.Join(a.State, name)); e != nil {
		t.Skipf("symlinks unavailable: %v", e)
	}
	reopened, e := Open(root, filepath.Dir(a.State))
	if e != nil {
		t.Fatal(e)
	}
	r, e := reopened.CachedPrescan()
	if e != nil || r.Prescan.State != "missing" {
		t.Fatal(r, e)
	}
	b, e := os.ReadFile(outside)
	if e != nil || string(b) != "KEEP_OUTSIDE" {
		t.Fatal("outside target damaged", e)
	}
}

func TestPrescanTotalBudgetIncludesOldGenerationWhileStaging(t *testing.T) {
	a, _ := prescanFixture(t)
	scanForTest(t, a, false)
	name, m := readManifestForTest(t, a)
	// Sparse size is sufficient to exercise disk accounting without filling tmpfs.
	f, e := os.OpenFile(filepath.Join(a.State, name), os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Truncate(maxPrescanBytes); e != nil {
		f.Close()
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	if e = a.write(name, &m); e == nil || !strings.Contains(e.Error(), "总编码超过64 MiB") {
		t.Fatal(e)
	}
	info, e := os.Stat(filepath.Join(a.State, name))
	if e != nil || info.Size() != maxPrescanBytes {
		t.Fatal("budget failure altered old slot", e)
	}
}
