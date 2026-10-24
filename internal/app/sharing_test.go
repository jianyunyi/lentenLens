package app

import (
	"context"
	"encoding/json"
	"intentlens/internal/explain"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func seedExplanation(t *testing.T, a *App, root string) Explanation {
	t.Helper()
	source := "func F() int { return 1 }"
	put(t, root, "main.go", "package p\n"+source+"\n")
	e := Explanation{ID: newID(), File: "main.go", FunctionID: "main.go:F", Created: "2026-10-10", Document: explain.Document{Claims: []explain.Claim{{ID: "c1", Text: "PRIVATE_ORIGINAL_TEXT", Kind: "behavior", EvidenceIDs: []string{"code"}, Status: "supported"}}, Unknowns: []string{"PRIVATE_QUESTION"}}, Evidence: []explain.Evidence{{ID: "code", File: "main.go", StartLine: 2, EndLine: 2, Hash: hash(source), Source: source}, {ID: "private", Kind: "prompt", Visibility: "local", Hash: hash("PRIVATE_PROMPT"), Source: "PRIVATE_PROMPT"}}}
	if err := a.write("explanation-"+e.ID+".json", e); err != nil {
		t.Fatal(err)
	}
	if err := a.Configure(Settings{Model: explain.Config{Enabled: true, Model: "fixture", TokenLimit: 1000000}}, true); err != nil {
		t.Fatal(err)
	}
	a.modelRun = func(ctx context.Context, cfg explain.Config, u explain.Usage, key, q string, ev []explain.Evidence) (explain.Document, explain.Usage, error) {
		if strings.Contains(q, "PRIVATE_") {
			t.Fatal("local explanation or reason leaked into generation prompt")
		}
		ids := []string{}
		for _, item := range ev {
			ids = append(ids, item.ID)
		}
		u.ActualTokens += 20
		return explain.Document{Claims: []explain.Claim{{ID: "new", Text: "返回 1", Kind: "behavior", EvidenceIDs: ids, Status: "supported"}}, Unknowns: []string{}}, u, nil
	}
	return e
}

func TestSharedPreviewUsesOnlyAllowedEvidenceAndRequiresConfirmation(t *testing.T) {
	a, root := fixture(t)
	source := seedExplanation(t, a, root)
	preview, err := a.PrepareShare(context.Background(), source.ID, nil, "key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, ".intent")); !os.IsNotExist(err) {
		t.Fatal("prepare wrote into repository")
	}
	data, _ := json.Marshal(preview.Record)
	if strings.Contains(string(data), "PRIVATE_") || len(preview.Record.Evidence) != 1 {
		t.Fatalf("private content in shared version: %s", data)
	}
	if _, err = a.ConfirmShare(preview.ID, preview.Digest, false); err == nil {
		t.Fatal("confirmation bypassed")
	}
	if _, err = a.ConfirmShare(preview.ID, "wrong", true); err == nil {
		t.Fatal("preview mismatch accepted")
	}
	record, err := a.ConfirmShare(preview.ID, preview.Digest, true)
	if err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(filepath.Join(root, ".intent", "shared", record.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"PRIVATE_", "token_limit", "actual_tokens", a.State, root, "corrections"} {
		if strings.Contains(string(written), secret) {
			t.Fatalf("local data exported: %s", secret)
		}
	}
	receiver, err := Open(root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	records, err := receiver.Shared()
	if err != nil || len(records) != 1 {
		t.Fatalf("receiver cannot read: %+v %v", records, err)
	}
}

func TestPrivateEvidenceNeedsExplicitSelection(t *testing.T) {
	a, root := fixture(t)
	source := seedExplanation(t, a, root)
	preview, err := a.PrepareShare(context.Background(), source.ID, []string{"private"}, "key")
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Record.Evidence) != 2 {
		t.Fatal("selected private evidence missing from preview")
	}
	if _, err = a.PrepareShare(context.Background(), source.ID, []string{"invented"}, "key"); err == nil {
		t.Fatal("unknown selection accepted")
	}
}

func TestPreviewInvalidatesOnCorrectionPolicyAndCodeChange(t *testing.T) {
	for _, change := range []string{"correction", "policy", "code"} {
		t.Run(change, func(t *testing.T) {
			a, root := fixture(t)
			source := seedExplanation(t, a, root)
			p, err := a.PrepareShare(context.Background(), source.ID, nil, "key")
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "correction":
				_, err = a.FlagClaim(source.ID, "c1", "private correction reason")
			case "policy":
				put(t, root, ".intentignore", "unrelated/\n")
			case "code":
				put(t, root, "main.go", "package p\nfunc F() int { return 2 }\n")
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = a.ConfirmShare(p.ID, p.Digest, true); err == nil {
				t.Fatal("changed preview published")
			}
		})
	}
}

func TestFlagIsAppendOnlyAndRevisionPreservesParent(t *testing.T) {
	a, root := fixture(t)
	source := seedExplanation(t, a, root)
	before, _ := os.ReadFile(filepath.Join(a.State, "explanation-"+source.ID+".json"))
	if _, err := a.FlagClaim(source.ID, "unknown", "reason"); err == nil {
		t.Fatal("unknown claim marked")
	}
	if _, err := a.FlagClaim(source.ID, "c1", "PRIVATE_REASON"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(a.State, "explanation-"+source.ID+".json"))
	if string(before) != string(after) {
		t.Fatal("historical explanation rewritten")
	}
	if _, err := a.PrepareShare(context.Background(), source.ID, nil, "key"); err == nil {
		t.Fatal("flagged version shared")
	}
	revised, err := a.Revise(context.Background(), source.ID, "key")
	if err != nil {
		t.Fatal(err)
	}
	if revised.ID == source.ID || revised.Supersedes != source.ID {
		t.Fatal("revision lost lineage")
	}
	old, err := a.ExplanationByID(source.ID)
	if err != nil || len(old.Corrections) != 1 {
		t.Fatalf("correction lost: %+v %v", old, err)
	}
}

func TestSharedReaderMissingEvidenceAndChangedSource(t *testing.T) {
	a, root := fixture(t)
	s := seedExplanation(t, a, root)
	p, e := a.PrepareShare(context.Background(), s.ID, nil, "key")
	if e != nil {
		t.Fatal(e)
	}
	record, e := a.ConfirmShare(p.ID, p.Digest, true)
	if e != nil {
		t.Fatal(e)
	}
	put(t, root, "main.go", "package p\nfunc F() int { return 3 }\n")
	list, e := a.Shared()
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(list[0].Freshness, "过期") {
		t.Fatal("stale export shown current")
	}
	record.Evidence = nil
	b, _ := json.Marshal(record)
	if e = os.WriteFile(filepath.Join(root, ".intent", "shared", record.ID+".json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	list, e = a.Shared()
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(strings.Join(list[0].Issues, " "), "依据未共享") {
		t.Fatal("missing reference not visible")
	}
}

func TestSharingRejectsTraversal(t *testing.T) {
	a, root := fixture(t)
	seedExplanation(t, a, root)
	if _, e := a.PrepareShare(context.Background(), "../settings", nil, "key"); e == nil {
		t.Fatal("unsafe id accepted")
	}
	if _, e := a.ConfirmShare("../settings", "x", true); e == nil {
		t.Fatal("unsafe draft id accepted")
	}
}

func TestSharedDirectoryRejectsLinks(t *testing.T) {
	a, root := fixture(t)
	s := seedExplanation(t, a, root)
	d, err := a.PrepareShare(context.Background(), s.ID, nil, "key")
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	link := filepath.Join(root, ".intent")
	if runtime.GOOS == "windows" {
		quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
		err = exec.Command("powershell", "-NoProfile", "-Command", "New-Item -ItemType Junction -Path "+quote(link)+" -Target "+quote(outside)).Run()
	} else {
		err = os.Symlink(outside, link)
	}
	if err != nil {
		t.Fatalf("create link fixture: %v", err)
	}
	if _, err = a.ConfirmShare(d.ID, d.Digest, true); err == nil {
		t.Fatal("linked shared directory accepted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("wrote outside repository: %v", err)
	}
}

func TestRevocationPurgesDependentLocalRecords(t *testing.T) {
	a, root := fixture(t)
	s := seedExplanation(t, a, root)
	d, err := a.PrepareShare(context.Background(), s.ID, nil, "key")
	if err != nil {
		t.Fatal(err)
	}
	c, err := a.FlagClaim(s.ID, "c1", "PRIVATE_REASON")
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, ".intentignore", "main.go\n")
	if err = a.scrubExcluded(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"explanation-" + s.ID + ".json", "share-draft-" + d.ID + ".json", "correction-" + c.ID + ".json"} {
		if _, err = os.Stat(filepath.Join(a.State, name)); !os.IsNotExist(err) {
			t.Fatalf("revoked local content retained: %s", name)
		}
	}
}

func TestSharedHistoryCannotBeOverwrittenAndRevisionRequiresNewPreview(t *testing.T) {
	a, root := fixture(t)
	s := seedExplanation(t, a, root)
	d, err := a.PrepareShare(context.Background(), s.ID, nil, "key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.ConfirmShare(d.ID, d.Digest, true); err != nil {
		t.Fatal(err)
	}
	if _, err = a.ConfirmShare(d.ID, d.Digest, true); err != nil {
		t.Fatal("idempotent confirmation", err)
	}
	if _, err = a.FlagClaim(s.ID, "c1", "reason"); err != nil {
		t.Fatal(err)
	}
	views, err := a.Shared()
	if err != nil || len(views) != 1 || len(views[0].Issues) == 0 {
		t.Fatal("shared correction warning missing", err)
	}
	r, err := a.Revise(context.Background(), s.ID, "key")
	if err != nil {
		t.Fatal(err)
	}
	next, err := a.PrepareShare(context.Background(), r.ID, nil, "key")
	if err != nil {
		t.Fatal(err)
	}
	if next.Record.Supersedes != d.ID {
		t.Fatal("shared lineage missing")
	}
	entries, _ := os.ReadDir(filepath.Join(root, ".intent", "shared"))
	if len(entries) != 1 {
		t.Fatal("revision auto-published")
	}
	put(t, root, ".intent/shared/"+next.ID+".json", "existing content")
	if _, err = a.ConfirmShare(next.ID, next.Digest, true); err == nil {
		t.Fatal("existing record overwritten")
	}
	data, _ := os.ReadFile(filepath.Join(root, ".intent", "shared", next.ID+".json"))
	if string(data) != "existing content" {
		t.Fatal("existing bytes changed")
	}
}

func TestRecipientCanCorrectAndReviseSharedVersion(t *testing.T) {
	a, root := fixture(t)
	s := seedExplanation(t, a, root)
	d, err := a.PrepareShare(context.Background(), s.ID, nil, "key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.ConfirmShare(d.ID, d.Digest, true); err != nil {
		t.Fatal(err)
	}
	receiver, err := Open(root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = receiver.AdoptShared(d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = receiver.FlagClaim(d.ID, "new", "recipient private reason"); err != nil {
		t.Fatal(err)
	}
	views, err := receiver.Shared()
	if err != nil || len(views[0].Issues) == 0 {
		t.Fatal("recipient flag invisible", err)
	}
	cfg, _ := a.Config()
	if err = receiver.Configure(cfg, true); err != nil {
		t.Fatal(err)
	}
	receiver.modelRun = a.modelRun
	r, err := receiver.Revise(context.Background(), d.ID, "key")
	if err != nil {
		t.Fatal(err)
	}
	next, err := receiver.PrepareShare(context.Background(), r.ID, nil, "key")
	if err != nil || next.Record.Supersedes != d.ID {
		t.Fatal("recipient revision lost shared parent", err)
	}
	second, err := receiver.Revise(context.Background(), r.ID, "key")
	if err != nil {
		t.Fatal(err)
	}
	next, err = receiver.PrepareShare(context.Background(), second.ID, nil, "key")
	if err != nil || next.Record.Supersedes != d.ID {
		t.Fatal("multiple local revisions lost shared ancestor", err)
	}
}

func TestDeletedEvidenceDetectsUncapturedReplacement(t *testing.T) {
	for _, kind := range []string{"directory", "binary", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			a, root := fixture(t)
			ev := []explain.Evidence{{ID: "deleted", File: "deleted.go", Role: "before", Source: "old", Hash: hash("old")}}
			if a.freshness(ev) != "已引用证据未变" {
				t.Fatal("absent path not recognized")
			}
			switch kind {
			case "directory":
				if err := os.Mkdir(filepath.Join(root, "deleted.go"), 0700); err != nil {
					t.Fatal(err)
				}
			case "binary":
				put(t, root, "deleted.go", "\x00\xff")
			case "oversize":
				put(t, root, "deleted.go", strings.Repeat("x", (1<<20)+1))
			}
			if !strings.Contains(a.freshness(ev), "过期") {
				t.Fatal("replacement path shown unchanged")
			}
		})
	}
}

func TestWholeFileEvidenceIncludesEmptyFileAndDetectsAppend(t *testing.T) {
	for _, source := range []string{"", "package p\n"} {
		a, root := fixture(t)
		put(t, root, "empty.go", source)
		ev := []explain.Evidence{{ID: "after", File: "empty.go", Role: "after", Source: source, Hash: hash(source)}}
		if a.freshness(ev) != "已引用证据未变" {
			t.Fatal("unchanged whole file shown stale")
		}
		put(t, root, "empty.go", source+"// changed\n")
		if !strings.Contains(a.freshness(ev), "过期") {
			t.Fatal("appended whole file shown unchanged")
		}
	}
}
