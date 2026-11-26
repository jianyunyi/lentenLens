package codeindex

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const MaxDocParagraphs = 20_000
const MaxDocParagraphBytes = 4 << 10

// DocParagraph is a verbatim Markdown section, including its heading and line
// endings. HeadingPath is structural context, not an inferred summary/version.
// File + HeadingPath disambiguate names; ID also distinguishes repeated headings.
type DocParagraph struct {
	ID          string   `json:"id"`
	File        string   `json:"file"`
	StartLine   int      `json:"start_line"`
	EndLine     int      `json:"end_line"`
	HeadingPath []string `json:"heading_path"`
	Source      string   `json:"source"`
	Hash        string   `json:"hash"`
}

type DocOmission struct {
	File        string   `json:"file"`
	StartLine   int      `json:"start_line"`
	EndLine     int      `json:"end_line"`
	HeadingPath []string `json:"heading_path"`
	Bytes       int      `json:"bytes"`
	Reason      string   `json:"reason"`
}

// DocIndex is independent of the Go index and capture coverage. Complete only
// describes the supplied Markdown; the caller must also report capture omissions.
// Treat the index and its paragraph slice as read-only after construction.
type DocIndex struct {
	Paragraphs []DocParagraph `json:"paragraphs"`
	Omitted    []DocOmission  `json:"omitted"`
	Complete   bool           `json:"complete"`
	postings   map[string][]int
}

type DocMatch struct {
	Paragraph    DocParagraph
	MatchedTerms int
}

// DocQuery optionally narrows lexical matches by exact file and heading path.
// Matches are mentions, not proof that a document describes a particular symbol.
type DocQuery struct {
	Text        string
	File        string
	HeadingPath []string
}

// BuildDocs accepts captured file contents, just like BuildDetailed. It performs
// no I/O or policy filtering, follows no links, and never invokes a model.
// Overlong sections are omitted whole (never truncated/split). All omissions
// retain their range and reason, including every section past the count limit.
func BuildDocs(files map[string]string) DocIndex {
	out := DocIndex{Paragraphs: []DocParagraph{}, Omitted: []DocOmission{}, Complete: true, postings: map[string][]int{}}
	names := make([]string, 0, len(files))
	for name := range files {
		if strings.HasSuffix(name, ".md") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		source := files[name]
		lines := strings.SplitAfter(source, "\n")
		if lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		start := 0
		heading := []string{}
		levels := []int{}
		emit := func(end int) {
			if end <= start {
				return
			}
			text := strings.Join(lines[start:end], "")
			if strings.TrimSpace(text) == "" {
				return
			}
			hp := append([]string{}, heading...)
			reason := ""
			if len(text) > MaxDocParagraphBytes {
				reason = "paragraph_bytes_exceeded"
			} else if len(out.Paragraphs) >= MaxDocParagraphs {
				reason = "paragraph_count_exceeded"
			}
			if reason != "" {
				out.Complete = false
				out.Omitted = append(out.Omitted, DocOmission{name, start + 1, end, hp, len(text), reason})
				return
			}
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
			// Length-prefixed file plus physical range is unambiguous even when paths
			// or titles contain punctuation or repeated headings occur in one file.
			id := fmt.Sprintf("doc:%d:%s:%d-%d", len(name), name, start+1, end)
			n := len(out.Paragraphs)
			out.Paragraphs = append(out.Paragraphs, DocParagraph{id, name, start + 1, end, hp, text, hash})
			for _, term := range docTerms(strings.Join(hp, " ") + "\n" + text) {
				out.postings[term] = append(out.postings[term], n)
			}
		}
		var fence byte
		fenceLen := 0
		for i := 0; i < len(lines); i++ {
			line := strings.TrimSuffix(strings.TrimSuffix(lines[i], "\n"), "\r")
			trimmed := strings.TrimLeft(line, " ")
			indent := len(line) - len(trimmed)
			if indent <= 3 && len(trimmed) > 0 && (trimmed[0] == '`' || trimmed[0] == '~') {
				ch := trimmed[0]
				count := 0
				for count < len(trimmed) && trimmed[count] == ch {
					count++
				}
				if fence != 0 {
					if ch == fence && count >= fenceLen && strings.TrimSpace(trimmed[count:]) == "" {
						fence = 0
					}
				} else if count >= 3 && (ch != '`' || !strings.Contains(trimmed[count:], "`")) {
					fence, fenceLen = ch, count
				}
				continue
			}
			if fence != 0 {
				continue
			}
			level, title := docATX(line)
			setext := false
			if level == 0 && indent <= 3 && strings.TrimSpace(line) != "" && i+1 < len(lines) {
				level = docSetext(lines[i+1])
				if level != 0 {
					title = strings.TrimSpace(line)
					setext = true
				}
			}
			if level == 0 {
				continue
			}
			emit(i)
			start = i
			for len(levels) > 0 && levels[len(levels)-1] >= level {
				levels = levels[:len(levels)-1]
				heading = heading[:len(heading)-1]
			}
			levels = append(levels, level)
			heading = append(heading, title)
			if setext {
				i++
			}
		}
		emit(len(lines))
	}
	return out
}

func docATX(line string) (int, string) {
	s := strings.TrimLeft(line, " ")
	if len(line)-len(s) > 3 {
		return 0, ""
	}
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || (n < len(s) && s[n] != ' ' && s[n] != '\t') {
		return 0, ""
	}
	title := strings.TrimSpace(s[n:])
	end := len(title)
	for end > 0 && title[end-1] == '#' {
		end--
	}
	if end == 0 || (end < len(title) && (title[end-1] == ' ' || title[end-1] == '\t')) {
		title = strings.TrimSpace(title[:end])
	}
	return n, title
}

func docSetext(line string) int {
	s := strings.TrimRight(line, " \t\r\n")
	t := strings.TrimLeft(s, " ")
	if len(s)-len(t) > 3 || t == "" || (t[0] != '=' && t[0] != '-') {
		return 0
	}
	for i := range t {
		if t[i] != t[0] {
			return 0
		}
	}
	if t[0] == '=' {
		return 1
	}
	return 2
}

// docTerms uses Han bigrams (single characters only for single-Han runs),
// case-folded whole identifiers, and underscore/camel-case identifier parts.
// It indexes the entire admitted section, including untrusted fenced text.
// This is deterministic lexical retrieval, not Chinese semantic segmentation.
func docTerms(text string) []string {
	set := map[string]bool{}
	r := []rune(text)
	for i := 0; i < len(r); {
		if unicode.Is(unicode.Han, r[i]) {
			j := i + 1
			for j < len(r) && unicode.Is(unicode.Han, r[j]) {
				j++
			}
			if j-i == 1 {
				set[string(r[i])] = true
			}
			for k := i; k+1 < j; k++ {
				set[string(r[k:k+2])] = true
			}
			i = j
			continue
		}
		if !unicode.IsLetter(r[i]) && !unicode.IsDigit(r[i]) && r[i] != '_' {
			i++
			continue
		}
		j := i + 1
		for j < len(r) && !unicode.Is(unicode.Han, r[j]) && (unicode.IsLetter(r[j]) || unicode.IsDigit(r[j]) || r[j] == '_') {
			j++
		}
		set[strings.ToLower(string(r[i:j]))] = true
		part := i
		for k := i; k <= j; k++ {
			boundary := k == j || r[k] == '_'
			if k > part && k < j && unicode.IsUpper(r[k]) && (unicode.IsLower(r[k-1]) || (k+1 < j && unicode.IsLower(r[k+1]))) {
				boundary = true
			}
			if boundary {
				if k > part {
					set[strings.ToLower(string(r[part:k]))] = true
				}
				part = k
				if k < j && r[k] == '_' {
					part++
				}
			}
		}
		i = j
	}
	terms := make([]string, 0, len(set))
	for term := range set {
		terms = append(terms, term)
	}
	sort.Strings(terms)
	return terms
}

// Search returns OR matches ranked by distinct matched terms descending, then
// file and physical start line ascending. Map iteration never determines order.
func (idx DocIndex) Search(query DocQuery) []DocMatch {
	counts := map[int]int{}
	for _, term := range docTerms(query.Text) {
		for _, n := range idx.postings[term] {
			counts[n]++
		}
	}
	ids := make([]int, 0, len(counts))
	for n := range counts {
		p := idx.Paragraphs[n]
		if query.File != "" && p.File != query.File {
			continue
		}
		if query.HeadingPath != nil && !equalDocPath(p.HeadingPath, query.HeadingPath) {
			continue
		}
		ids = append(ids, n)
	}
	sort.Slice(ids, func(i, j int) bool {
		if counts[ids[i]] != counts[ids[j]] {
			return counts[ids[i]] > counts[ids[j]]
		}
		return ids[i] < ids[j]
	})
	matches := make([]DocMatch, 0, len(ids))
	for _, n := range ids {
		matches = append(matches, DocMatch{idx.Paragraphs[n], counts[n]})
	}
	return matches
}

func equalDocPath(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
