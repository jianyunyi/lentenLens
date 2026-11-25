package app

import (
	"intentlens/internal/capture"
	"intentlens/internal/codeindex"
	"testing"
)

// Exercise the actual policy/capture -> map -> Go/docs builders boundary. The
// pure builders deliberately do not know .intentignore or read the repository.
func TestDocsIndexConsumesOnlyPolicyFilteredCapture(t *testing.T) {
	_, root := fixture(t)
	putCoverage(t, root, ".intentignore", "docs/private.md\n")
	putCoverage(t, root, "docs/private.md", "# Private\nsecretcanary\n")
	putCoverage(t, root, "docs/public.md", "# Public\npublicneedle\n")
	putCoverage(t, root, "main.go", "package p\nfunc Visible() {}\n")
	snapshot, coverage, err := capture.TakeGoAndDocs(root)
	if err != nil {
		t.Fatal(err)
	}
	if coverage.Scope != "go+docs" || coverage.Components == nil || coverage.Components.Docs.ExcludedCount == 0 {
		t.Fatalf("coverage: %+v", coverage)
	}
	files := map[string]string{}
	for name, file := range snapshot.Files {
		files[name] = file.Content
	}
	if _, ok := files["docs/private.md"]; ok {
		t.Fatal("excluded document captured")
	}
	docs := codeindex.BuildDocs(files)
	if len(docs.Search(codeindex.DocQuery{Text: "secretcanary"})) != 0 {
		t.Fatal("excluded content entered index")
	}
	hits := docs.Search(codeindex.DocQuery{Text: "publicneedle"})
	if len(hits) != 1 || hits[0].Paragraph.File != "docs/public.md" {
		t.Fatalf("public document missing: %+v", hits)
	}
	goIndex, _ := codeindex.BuildDetailed(files)
	found := false
	for _, fn := range goIndex.Functions {
		if fn.File == "main.go" && fn.Name == "Visible" {
			found = true
		}
	}
	if !found {
		t.Fatal("coexisting Go index lost function")
	}
}
