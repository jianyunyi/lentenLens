package codeindex

import "strings"

// RestoreDocPostings restores the nonserialized deterministic search table from
// validated retained paragraphs, without reading or segmenting source files.
func RestoreDocPostings(index DocIndex) DocIndex {
	index.postings = map[string][]int{}
	for i, p := range index.Paragraphs {
		for _, term := range docTerms(strings.Join(p.HeadingPath, " ") + "\n" + p.Source) {
			index.postings[term] = append(index.postings[term], i)
		}
	}
	return index
}
