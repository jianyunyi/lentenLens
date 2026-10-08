package project

import (
	"fmt"
	"strings"

	"intentlens/internal/capture"
)

type Evidence struct {
	RoundID          string `json:"round_id"`
	Path             string `json:"path"`
	Side             string `json:"side"`
	StartLine        int    `json:"start_line"`
	EndLine          int    `json:"end_line"`
	SourceVersion    string `json:"source_version"`
	SnippetVersion   string `json:"snippet_version"`
	Text             string `json:"text"`
	Status           string `json:"status"`
	CurrentStartLine int    `json:"current_start_line,omitempty"`
	Scope            string `json:"scope"`
}

func (s *Store) Evidence(id, name, side string, start, end int) (Evidence, error) {
	var result Evidence
	err := s.transact(func(st *state, p capture.Policy) error {
		if !p.Allows(name) {
			return fmt.Errorf("related context is excluded")
		}
		if side != "before" && side != "after" {
			return fmt.Errorf("side must be before or after")
		}
		var source *capture.File
		for _, r := range st.Rounds {
			if r.ID != id {
				continue
			}
			for _, c := range r.Changes {
				if c.Path == name {
					if side == "before" {
						source = c.Before
					} else {
						source = c.After
					}
				}
			}
		}
		if source == nil {
			return fmt.Errorf("no recorded evidence for this round, path and side")
		}
		lines := strings.SplitAfter(source.Content, "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		if start < 1 || end < start || end > len(lines) {
			return fmt.Errorf("invalid line range; recorded source has %d lines", len(lines))
		}
		excerpt := strings.Join(lines[start-1:end], "")
		result = Evidence{RoundID: id, Path: name, Side: side, StartLine: start, EndLine: end, SourceVersion: source.Hash, SnippetVersion: capture.NewFile(excerpt).Hash, Text: excerpt, Status: "stale", Scope: "only_this_referenced_snippet_not_all_dependencies"}
		current, err := capture.Take(s.Root)
		if err != nil {
			return err
		}
		f, exists := current.Files[name]
		if !exists {
			result.Status = "unavailable"
			return nil
		}
		// Prefer the original location when it still matches; otherwise require a unique whole-line match.
		currentLines := strings.SplitAfter(f.Content, "\n")
		if end <= len(currentLines) && strings.Join(currentLines[start-1:end], "") == excerpt {
			result.Status = "referenced_evidence_unchanged"
			result.CurrentStartLine = start
			return nil
		}
		matches := []int{}
		for i := range currentLines {
			count := end - start + 1
			if i+count <= len(currentLines) && strings.Join(currentLines[i:i+count], "") == excerpt {
				matches = append(matches, i+1)
			}
		}
		if len(matches) == 1 {
			result.Status = "referenced_evidence_unchanged"
			result.CurrentStartLine = matches[0]
		} else if len(matches) > 1 {
			result.Status = "ambiguous_location"
		}
		return nil
	})
	return result, err
}
