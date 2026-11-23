package main

import (
	"reflect"
	"testing"

	"intentlens/internal/explain"
)

func TestScore(t *testing.T) {
	c := Case{
		Expect: []Rule{{ID: "lock", Any: []string{"Mutex"}}, {ID: "evict", Any: []string{"最早"}}},
		Forbid: []Rule{{ID: "lru", Any: []string{"LRU"}}, {ID: "ttl", Any: []string{"过期"}}},
	}
	ev := []explain.Evidence{{ID: "e1", File: "cache.go", StartLine: 3, EndLine: 9}}
	doc := explain.Document{Claims: []explain.Claim{
		{ID: "c1", Text: "持有 mutex 后写入", EvidenceIDs: []string{"e1"}, Status: "supported"},
		{ID: "c2", Text: "按 LRU 淘汰", EvidenceIDs: []string{"e1", "e9"}, Status: "supported"},
		{ID: "c3", Text: "淘汰最早插入的键", Status: "uncertain"},
	}}
	got := score(c, doc, ev)
	want := Score{
		Claims:           3,
		Statuses:         map[string]int{"supported": 2, "uncertain": 1},
		InvalidCitations: []string{"c2->e9"},
		Uncited:          []string{"c3"},
		ExpectMet:        []string{"lock"},
		ExpectMissed:     []string{"evict"},
		ForbidHit:        []string{"lru"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("score =\n%+v\nwant\n%+v", got, want)
	}
	if refs := evidenceRefs([]string{"e1", "e9"}, ev); refs != "cache.go:3-9 e9" {
		t.Fatalf("refs = %q", refs)
	}
}

func TestFunctionLine(t *testing.T) {
	src := "package p\n\nfunc (c *Cache) Put(k string) {}\nfunc Put2() {}\n"
	if n := functionLine(src, "Put"); n != 3 {
		t.Fatalf("Put line = %d", n)
	}
	if n := functionLine(src, "Missing"); n != 0 {
		t.Fatalf("Missing line = %d", n)
	}
}
