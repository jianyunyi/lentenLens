package main

import (
	"fmt"
	"sort"
	"strings"

	"intentlens/internal/explain"
)

// Rule matches a claim when its text contains any keyword (case-insensitive).
type Rule struct {
	ID  string   `json:"id"`
	Any []string `json:"any"`
}

type Case struct {
	File     string `json:"file"`
	Function string `json:"function"`
	Question string `json:"question,omitempty"`
	Expect   []Rule `json:"expect"`
	Forbid   []Rule `json:"forbid"`
}

type Score struct {
	Claims           int            `json:"claims"`
	Statuses         map[string]int `json:"statuses"`
	InvalidCitations []string       `json:"invalid_citations,omitempty"`
	Uncited          []string       `json:"uncited,omitempty"`
	ExpectMet        []string       `json:"expect_met"`
	ExpectMissed     []string       `json:"expect_missed"`
	ForbidHit        []string       `json:"forbid_hit,omitempty"`
}

func matches(r Rule, text string) bool {
	text = strings.ToLower(text)
	for _, k := range r.Any {
		if k != "" && strings.Contains(text, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

// score checks citations structurally. Keyword checks are a coarse first
// pass; the review sheet holds the human judgment.
func score(c Case, doc explain.Document, evidence []explain.Evidence) Score {
	ids := map[string]bool{}
	for _, ev := range evidence {
		ids[ev.ID] = true
	}
	s := Score{Statuses: map[string]int{}, ExpectMet: []string{}, ExpectMissed: []string{}}
	var supported []string
	for _, cl := range doc.Claims {
		s.Claims++
		s.Statuses[cl.Status]++
		if len(cl.EvidenceIDs) == 0 {
			s.Uncited = append(s.Uncited, cl.ID)
		}
		for _, id := range cl.EvidenceIDs {
			if !ids[id] {
				s.InvalidCitations = append(s.InvalidCitations, cl.ID+"->"+id)
			}
		}
		if cl.Status == "supported" {
			supported = append(supported, cl.Text)
		}
	}
	for _, r := range c.Expect {
		met := false
		for _, t := range supported {
			if matches(r, t) {
				met = true
				break
			}
		}
		if met {
			s.ExpectMet = append(s.ExpectMet, r.ID)
		} else {
			s.ExpectMissed = append(s.ExpectMissed, r.ID)
		}
	}
	for _, r := range c.Forbid {
		for _, t := range supported {
			if matches(r, t) {
				s.ForbidHit = append(s.ForbidHit, r.ID)
				break
			}
		}
	}
	return s
}

func evidenceRefs(ids []string, evidence []explain.Evidence) string {
	byID := map[string]explain.Evidence{}
	for _, ev := range evidence {
		byID[ev.ID] = ev
	}
	var refs []string
	for _, id := range ids {
		if ev, ok := byID[id]; ok && ev.File != "" {
			refs = append(refs, fmt.Sprintf("%s:%d-%d", ev.File, ev.StartLine, ev.EndLine))
		} else {
			refs = append(refs, id)
		}
	}
	sort.Strings(refs)
	return strings.Join(refs, " ")
}
