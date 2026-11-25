package codeindex

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestDocsHeadingRangesAndCRLF(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		t.Run(fmt.Sprintf("newline_%q", newline), func(t *testing.T) {
			source := strings.Join([]string{"序言", "", "# Design", "body", "## Retry ###", "RetryPolicy", "Other", "-----", "last"}, newline)
			idx := BuildDocs(map[string]string{"docs/design.md": source, "main.go": "package p"})
			if !idx.Complete || len(idx.Paragraphs) != 4 {
				t.Fatalf("index: %+v", idx)
			}
			wantRanges := [][2]int{{1, 2}, {3, 4}, {5, 6}, {7, 9}}
			wantPaths := [][]string{{}, {"Design"}, {"Design", "Retry"}, {"Design", "Other"}}
			for i, p := range idx.Paragraphs {
				if p.StartLine != wantRanges[i][0] || p.EndLine != wantRanges[i][1] || !reflect.DeepEqual(p.HeadingPath, wantPaths[i]) {
					t.Fatalf("paragraph %d: %+v", i, p)
				}
				if p.Hash == "" || p.ID == "" {
					t.Fatalf("missing identity: %+v", p)
				}
			}
			var reconstructed string
			for _, p := range idx.Paragraphs {
				reconstructed += p.Source
			}
			if reconstructed != source {
				t.Fatalf("source changed: %q", reconstructed)
			}
		})
	}
}

func TestDocsATXAndSetextSyntax(t *testing.T) {
	idx := BuildDocs(map[string]string{"a.md": "Title\n=====\ntext\n   ## Child ##\n#not-heading\n####### seven\n    # indented\n## C#\n#\nend\n"})
	want := [][]string{{"Title"}, {"Title", "Child"}, {"Title", "C#"}, {""}}
	if len(idx.Paragraphs) != len(want) {
		t.Fatalf("paragraphs: %+v", idx.Paragraphs)
	}
	for i, p := range idx.Paragraphs {
		if !reflect.DeepEqual(p.HeadingPath, want[i]) {
			t.Fatalf("path: %+v", p)
		}
	}
}

func TestDocsFencedPseudoHeadings(t *testing.T) {
	for _, fence := range []string{"```", "~~~~"} {
		source := "# Real\n" + fence + "go\n# Fake\nFake setext\n=====\n" + fence[:2] + "\n## Still fake\n" + fence + " trailing\n# Also fake\n" + fence + "\n## Next\nbody\n"
		idx := BuildDocs(map[string]string{"a.md": source})
		if len(idx.Paragraphs) != 2 || idx.Paragraphs[0].EndLine != 10 || idx.Paragraphs[1].StartLine != 11 {
			t.Fatalf("fence %q: %+v", fence, idx.Paragraphs)
		}
		if hits := idx.Search(DocQuery{Text: "Fake"}); len(hits) != 1 || hits[0].Paragraph.StartLine != 1 {
			t.Fatalf("fenced body should remain indexed: %+v", hits)
		}
	}
	idx := BuildDocs(map[string]string{"a.md": "# Real\n```\n# Unclosed\nTitle\n---\n"})
	if len(idx.Paragraphs) != 1 {
		t.Fatalf("unclosed fence: %+v", idx)
	}
}

func TestDocsParagraphByteLimitOmitsWholeSection(t *testing.T) {
	exact := "# Exact\n" + strings.Repeat("x", MaxDocParagraphBytes-len("# Exact\n"))
	over := "# Huge\n" + strings.Repeat("中", MaxDocParagraphBytes/3)
	idx := BuildDocs(map[string]string{"exact.md": exact, "huge.md": over + "\n## Small\nkeep\n"})
	if idx.Complete || len(idx.Paragraphs) != 2 || len(idx.Omitted) != 1 {
		t.Fatalf("limits: %+v", idx)
	}
	o := idx.Omitted[0]
	if o.File != "huge.md" || o.StartLine != 1 || o.EndLine != 2 || o.Reason != "paragraph_bytes_exceeded" || o.Bytes != len(over)+1 || !reflect.DeepEqual(o.HeadingPath, []string{"Huge"}) {
		t.Fatalf("omission: %+v", o)
	}
	if len(idx.Search(DocQuery{Text: "中"})) != 0 || len(idx.Search(DocQuery{Text: "keep"})) != 1 {
		t.Fatal("omitted text indexed or following section lost")
	}
	for _, p := range idx.Paragraphs {
		if len(p.Source) > MaxDocParagraphBytes {
			t.Fatal("oversized admitted paragraph")
		}
	}
	// CRLF bytes count toward the limit too.
	crlf := strings.ReplaceAll(exact, "\n", "\r\n")
	if got := BuildDocs(map[string]string{"a.md": crlf}); got.Complete || len(got.Omitted) != 1 {
		t.Fatal("CRLF bytes not counted")
	}
}

func TestDocsParagraphCountLimitReportsEveryOmission(t *testing.T) {
	var source strings.Builder
	for i := 0; i < MaxDocParagraphs+3; i++ {
		fmt.Fprintf(&source, "# H%d\nneedle\n", i)
	}
	idx := BuildDocs(map[string]string{"a.md": source.String(), "z.md": "# Last\nneedle\n"})
	if idx.Complete || len(idx.Paragraphs) != MaxDocParagraphs || len(idx.Omitted) != 4 {
		t.Fatalf("counts: paragraphs=%d omissions=%d complete=%v", len(idx.Paragraphs), len(idx.Omitted), idx.Complete)
	}
	for i, o := range idx.Omitted {
		if o.Reason != "paragraph_count_exceeded" {
			t.Fatalf("reason: %+v", o)
		}
		if i < 3 && (o.File != "a.md" || o.StartLine != 2*(MaxDocParagraphs+i)+1) {
			t.Fatalf("range: %+v", o)
		}
	}
	if idx.Omitted[3].File != "z.md" || len(idx.Search(DocQuery{Text: "needle"})) != MaxDocParagraphs {
		t.Fatal("count cap not deterministic or omitted text indexed")
	}
}

func TestDocsChineseAndIdentifiers(t *testing.T) {
	idx := BuildDocs(map[string]string{"a.md": "# 重试设计\n支付失败使用 RetryPolicy HTTPClient retry_limit。\n## Details\n恢复服务\n"})
	for _, query := range []string{"支付", "失败", "支付失败", "RetryPolicy", "retrypolicy", "retry", "policy", "HTTPClient", "http", "client", "retry_limit", "limit", "重试"} {
		if hits := idx.Search(DocQuery{Text: query}); len(hits) == 0 {
			t.Errorf("no hits for %q", query)
		}
	}
	if hits := idx.Search(DocQuery{Text: "重试", HeadingPath: []string{"重试设计", "Details"}}); len(hits) != 1 {
		t.Fatal("ancestor heading terms not indexed")
	}
	if len(idx.Search(DocQuery{Text: "absent"})) != 0 {
		t.Fatal("unexpected hit")
	}
}

func TestDocsDuplicateHeadingsDisambiguateAndRankingIsStable(t *testing.T) {
	files := map[string]string{
		"b.md": "# Same\nRetryPolicy needle\n",
		"a.md": "# A\n## Same\nRetryPolicy needle\n## Same\nRetryPolicy needle extra\n# B\n## Same\nRetryPolicy needle\n",
	}
	for run := 0; run < 30; run++ {
		idx := BuildDocs(files)
		hits := idx.Search(DocQuery{Text: "needle extra needle"})
		if len(hits) != 4 {
			t.Fatalf("hits: %+v", hits)
		}
		got := []string{}
		ids := map[string]bool{}
		for _, h := range hits {
			got = append(got, fmt.Sprintf("%s:%d", h.Paragraph.File, h.Paragraph.StartLine))
			if ids[h.Paragraph.ID] {
				t.Fatal("duplicate ID")
			}
			ids[h.Paragraph.ID] = true
		}
		if !reflect.DeepEqual(got, []string{"a.md:4", "a.md:2", "a.md:7", "b.md:1"}) {
			t.Fatalf("order: %v", got)
		}
		if hits[0].MatchedTerms != 2 || hits[1].MatchedTerms != 1 {
			t.Fatal("duplicate query terms changed ranking")
		}
		scoped := idx.Search(DocQuery{Text: "RetryPolicy", File: "a.md", HeadingPath: []string{"B", "Same"}})
		if len(scoped) != 1 || scoped[0].Paragraph.StartLine != 7 {
			t.Fatalf("disambiguation: %+v", scoped)
		}
	}
}

func TestDocsEmptyInputAndNoHeadings(t *testing.T) {
	idx := BuildDocs(map[string]string{"empty.md": "", "blank.md": "\n \r\n", "plain.md": "plain body\n", "other.go": "package p"})
	if !idx.Complete || len(idx.Paragraphs) != 1 || idx.Paragraphs[0].StartLine != 1 || idx.Paragraphs[0].EndLine != 1 || len(idx.Paragraphs[0].HeadingPath) != 0 {
		t.Fatalf("plain/empty: %+v", idx)
	}
	if len(BuildDocs(nil).Search(DocQuery{Text: ""})) != 0 {
		t.Fatal("empty query")
	}
}
