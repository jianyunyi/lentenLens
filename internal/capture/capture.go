// Package capture records bounded text snapshots without executing project code.
package capture

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const MaxFileBytes = 1 << 20
const MaxSnapshotBytes = 16 << 20

type File struct {
	Content string `json:"content"`
	Hash    string `json:"hash"`
}
type Snapshot struct {
	Files      map[string]File `json:"files"`
	Omitted    []string        `json:"omitted"`
	PolicyHash string          `json:"policy_hash"`
}
type CoverageOmission struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}
type Coverage struct {
	Complete        bool               `json:"complete"`
	Scope           string             `json:"scope"`
	BudgetBytes     int                `json:"budget_bytes"`
	CapturedBytes   int                `json:"captured_bytes"`
	OmittedCount    int                `json:"omitted_count"`
	ExcludedCount   int                `json:"excluded_count"`
	Omitted         []CoverageOmission `json:"omitted"`
	Truncated       bool               `json:"truncated"`
	OmissionReasons map[string]string  `json:"-"`
}
type Change struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Before *File  `json:"before,omitempty"`
	After  *File  `json:"after,omitempty"`
	Folded bool   `json:"folded"`
}

func NewFile(content string) File {
	return File{Content: content, Hash: fmt.Sprintf("%x", sha256.Sum256([]byte(content)))}
}
func Take(root string) (Snapshot, error) {
	snapshot, _, err := TakeWithPolicy(root)
	return snapshot, err
}

// TakeWithPolicy returns the snapshot and the policy used to capture it.
func TakeWithPolicy(root string) (Snapshot, Policy, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Snapshot{}, Policy{}, err
	}
	if safeOK, safeErr := safePath(root, ""); safeErr != nil || !safeOK {
		return Snapshot{}, Policy{}, fmt.Errorf("capture root must not traverse links")
	}
	policy, err := LoadPolicy(root)
	if err != nil {
		return Snapshot{}, Policy{}, err
	}
	s := Snapshot{Files: map[string]File{}, Omitted: []string{}, PolicyHash: policy.Hash()}
	total := 0
	confined, err := os.OpenRoot(root)
	if err != nil {
		return Snapshot{}, Policy{}, err
	}
	defer confined.Close()
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if p == root {
			return walkErr
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		rel = filepath.ToSlash(rel)
		if !policy.Allows(rel) {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if walkErr != nil {
			return fmt.Errorf("cannot inspect capture path %q", rel)
		}
		if d.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			s.Omitted = append(s.Omitted, rel)
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return fmt.Errorf("cannot inspect capture path %q", rel)
		}
		if !info.Mode().IsRegular() || info.Size() > MaxFileBytes || info.Size() > int64(MaxSnapshotBytes-total) {
			s.Omitted = append(s.Omitted, rel)
			return nil
		}
		ok, e := safePath(root, rel)
		if e != nil {
			return e
		}
		if !ok {
			s.Omitted = append(s.Omitted, rel)
			return nil
		}
		data, e := readCaptureFile(confined, rel, info)
		if e == errFileChanged {
			s.Omitted = append(s.Omitted, rel)
			return nil
		}
		if e != nil {
			return e
		}
		if len(data) > MaxFileBytes || len(data) > MaxSnapshotBytes-total || !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			s.Omitted = append(s.Omitted, rel)
			return nil
		}
		total += len(data)
		s.Files[rel] = NewFile(string(data))
		return nil
	})
	if err != nil {
		return Snapshot{}, Policy{}, err
	}
	sort.Strings(s.Omitted)
	return s, policy, nil
}

// TakeGo captures only Go source for indexing. Non-Go files never consume the
// source budget; all ordinary path and privacy checks still apply.
func TakeGo(root string) (Snapshot, Coverage, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Snapshot{}, Coverage{}, err
	}
	if safeOK, safeErr := safePath(root, ""); safeErr != nil || !safeOK {
		return Snapshot{}, Coverage{}, fmt.Errorf("capture root must not traverse links")
	}
	policy, err := LoadPolicy(root)
	if err != nil {
		return Snapshot{}, Coverage{}, err
	}
	s := Snapshot{Files: map[string]File{}, Omitted: []string{}, PolicyHash: policy.Hash()}
	c := Coverage{Complete: true, Scope: "go-only", BudgetBytes: MaxSnapshotBytes, Omitted: []CoverageOmission{}, OmissionReasons: map[string]string{}}
	total := 0
	confined, err := os.OpenRoot(root)
	if err != nil {
		return Snapshot{}, Coverage{}, err
	}
	defer confined.Close()
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if p == root {
			return walkErr
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		rel = filepath.ToSlash(rel)
		if !policy.Allows(rel) {
			c.ExcludedCount++
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if walkErr != nil {
			return fmt.Errorf("cannot inspect capture path %q", rel)
		}
		if d.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			// Windows junctions can be ModeIrregular rather than ModeSymlink.
			// Both exclude a directory from the index and must affect coverage.
			if target, statErr := os.Stat(p); statErr == nil && target.IsDir() {
				c.ExcludedCount++
				if d.IsDir() {
					return filepath.SkipDir
				}
				// WalkDir does not follow non-directory link entries. SkipDir
				// here would skip the remaining entries in their parent.
				return nil
			}
			if strings.HasSuffix(rel, ".go") {
				addCoverageOmission(&c, rel, "unsafe_path")
			}
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(rel, ".go") {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return fmt.Errorf("cannot inspect capture path %q", rel)
		}
		if !info.Mode().IsRegular() {
			addCoverageOmission(&c, rel, "unsafe_path")
			return nil
		}
		if info.Size() > MaxFileBytes {
			addCoverageOmission(&c, rel, "file_too_large")
			return nil
		}
		if info.Size() > int64(MaxSnapshotBytes-total) {
			addCoverageOmission(&c, rel, "snapshot_budget")
			return nil
		}
		ok, e := safePath(root, rel)
		if e != nil {
			return e
		}
		if !ok {
			addCoverageOmission(&c, rel, "unsafe_path")
			return nil
		}
		data, e := readCaptureFile(confined, rel, info)
		if e == errFileChanged {
			s.Omitted = append(s.Omitted, rel)
			addCoverageOmission(&c, rel, "file_changed")
			return nil
		}
		if e != nil {
			return e
		}
		if len(data) > MaxFileBytes {
			addCoverageOmission(&c, rel, "file_too_large")
			return nil
		}
		if len(data) > MaxSnapshotBytes-total {
			addCoverageOmission(&c, rel, "snapshot_budget")
			return nil
		}
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			addCoverageOmission(&c, rel, "binary_or_invalid_utf8")
			return nil
		}
		total += len(data)
		s.Files[rel] = NewFile(string(data))
		return nil
	})
	if err != nil {
		return Snapshot{}, Coverage{}, err
	}
	c.CapturedBytes = total
	c.Complete = c.OmittedCount == 0 && c.ExcludedCount == 0
	sort.Strings(s.Omitted)
	return s, c, nil
}

var errFileChanged = errors.New("file changed during capture")

func readCaptureFile(root *os.Root, relative string, walked fs.FileInfo) (data []byte, err error) {
	f, err := root.Open(filepath.FromSlash(relative))
	if err != nil {
		return nil, fmt.Errorf("cannot read capture path %q", relative)
	}
	defer func() {
		closeErr := f.Close()
		if err == errFileChanged && closeErr == nil {
			return
		}
		if combined := errors.Join(err, closeErr); combined != nil {
			data = nil
			err = fmt.Errorf("cannot read capture path %q: %w", relative, combined)
		}
	}()

	before, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || !os.SameFile(walked, before) || !sameFileVersion(walked, before) {
		return nil, errFileChanged
	}
	data, err = io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !sameFileVersion(before, after) {
		return nil, errFileChanged
	}
	return data, nil
}

func sameFileVersion(a, b fs.FileInfo) bool {
	return a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func addCoverageOmission(c *Coverage, file, reason string) {
	if c.OmissionReasons == nil {
		c.OmissionReasons = map[string]string{}
	}
	c.OmissionReasons[file] = reason
	c.OmittedCount++
	if len(c.Omitted) < 20 {
		c.Omitted = append(c.Omitted, CoverageOmission{File: file, Reason: reason})
	} else {
		// truncated describes the public omission sample, not source-budget use.
		c.Truncated = true
	}
}

func safePath(root, relative string) (bool, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return false, err
	}
	// Check ancestors too: a root supplied through a link is not a safe root.
	for p := abs; ; p = filepath.Dir(p) {
		i, e := os.Lstat(p)
		if e != nil {
			return false, e
		}
		if i.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return false, nil
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	p := abs
	for _, part := range strings.Split(filepath.FromSlash(relative), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		p = filepath.Join(p, part)
		i, e := os.Lstat(p)
		if os.IsNotExist(e) {
			return true, nil
		}
		if e != nil {
			return false, e
		}
		if i.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return false, nil
		}
	}
	return true, nil
}
func Allowed(root, relative string) (bool, error) {
	p, e := LoadPolicy(root)
	if e != nil {
		return false, e
	}
	return p.Allowed(root, relative)
}

// Allowed checks the loaded policy and the current path safety.
func (p Policy) Allowed(root, relative string) (bool, error) {
	if !filepath.IsLocal(relative) || !p.Allows(relative) {
		return false, nil
	}
	return safePath(root, relative)
}
func Diff(before, after Snapshot) []Change {
	omitted := map[string]bool{}
	for _, snapshot := range []Snapshot{before, after} {
		for _, p := range snapshot.Omitted {
			omitted[p] = true
		}
	}
	// An omitted path is unknown, including every descendant of a skipped directory.
	covered := func(p string) bool {
		for {
			if omitted[p] {
				return true
			}
			i := strings.LastIndexByte(p, '/')
			if i < 0 {
				return false
			}
			p = p[:i]
		}
	}
	paths := map[string]bool{}
	for p := range before.Files {
		paths[p] = true
	}
	for p := range after.Files {
		paths[p] = true
	}
	names := make([]string, 0, len(paths))
	for p := range paths {
		names = append(names, p)
	}
	sort.Strings(names)
	out := []Change{}
	for _, p := range names {
		if covered(p) {
			continue
		}
		a, aok := before.Files[p]
		b, bok := after.Files[p]
		if aok && bok && a.Content == b.Content {
			continue
		}
		c := Change{Path: p, Kind: "modified"}
		if aok {
			c.Before = &a
		}
		if bok {
			c.After = &b
		}
		if !aok {
			c.Kind = "added"
		}
		if !bok {
			c.Kind = "deleted"
		}
		c.Folded = generated(a.Content) || generated(b.Content)
		if aok && bok && strings.HasSuffix(p, ".go") {
			x, e := format.Source([]byte(a.Content))
			y, f := format.Source([]byte(b.Content))
			c.Folded = c.Folded || (e == nil && f == nil && bytes.Equal(x, y))
		}
		out = append(out, c)
	}
	return out
}
func generated(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "package ") {
			break
		}
		if strings.HasPrefix(line, "// Code generated ") && strings.HasSuffix(line, " DO NOT EDIT.") {
			return true
		}
	}
	return false
}
