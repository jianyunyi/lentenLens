package capture

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Policy struct{ patterns []string }

// LoadPolicy accepts path globs, ** directory segments and trailing directory /.
// Basename patterns match every path component; slash patterns are root-relative.
// Every matched path and its descendants are excluded, including directory rules.
// Negation is deliberately unsupported so exclusions cannot be re-included.
func LoadPolicy(root string) (p Policy, retErr error) {
	p = Policy{patterns: []string{".git", ".intent", ".intentlens-write-*", ".intentignore", ".worktrees", ".cache", ".pnpm-store", "node_modules", "vendor", ".env*", "*.pem", "*.key"}}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return p, err
	}
	defer func() {
		if closeErr := dir.Close(); closeErr != nil {
			retErr = errors.Join(retErr, closeErr)
		}
	}()
	info, err := dir.Lstat(".intentignore")
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if !info.Mode().IsRegular() {
		return p, fmt.Errorf(".intentignore must be a regular file")
	}
	if info.Size() > 64*1024 {
		return p, fmt.Errorf(".intentignore exceeds 64 KiB")
	}
	f, err := dir.Open(".intentignore")
	if err != nil {
		return p, err
	}
	openedInfo, err := f.Stat()
	if err != nil {
		return p, errors.Join(err, f.Close())
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return p, errors.Join(fmt.Errorf(".intentignore changed while opening"), f.Close())
	}
	if openedInfo.Size() > 64*1024 {
		return p, errors.Join(fmt.Errorf(".intentignore exceeds 64 KiB"), f.Close())
	}
	data, readErr := io.ReadAll(io.LimitReader(f, 64*1024+1))
	closeErr := f.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return p, err
	}
	if len(data) > 64*1024 {
		return p, fmt.Errorf(".intentignore exceeds 64 KiB")
	}
	scan := bufio.NewScanner(bytes.NewReader(data))
	for scan.Scan() {
		s := strings.TrimSpace(scan.Text())
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if strings.HasPrefix(s, "!") || strings.Contains(s, "\\") || strings.HasPrefix(s, "/") {
			return p, fmt.Errorf("unsupported .intentignore pattern")
		}
		s = strings.TrimSuffix(s, "/")
		for _, part := range strings.Split(s, "/") {
			if part == "" || part == ".." || part == "." {
				return p, fmt.Errorf("invalid .intentignore pattern")
			}
			if part != "**" {
				if _, err := path.Match(part, ""); err != nil {
					return p, fmt.Errorf("invalid .intentignore pattern")
				}
			}
		}
		p.patterns = append(p.patterns, s)
	}
	return p, scan.Err()
}

func (p Policy) Allows(name string) bool {
	name = filepath.ToSlash(name)
	if name == "" || path.IsAbs(name) || strings.Contains(name, "\\") {
		return false
	}
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if part == "" || part == ".." || part == "." {
			return false
		}
	}
	for _, pattern := range p.patterns {
		if !strings.Contains(pattern, "/") {
			for _, part := range parts {
				ok, err := path.Match(pattern, part)
				if err != nil {
					// Invalid patterns are rejected by LoadPolicy; deny on a
					// malformed in-memory policy rather than weakening exclusion.
					return false
				}
				if ok {
					return false
				}
			}
			continue
		}
		for n := 1; n <= len(parts); n++ {
			if matchParts(strings.Split(pattern, "/"), parts[:n]) {
				return false
			}
		}
	}
	return true
}

func matchParts(pattern, parts []string) bool {
	if len(pattern) == 0 {
		return len(parts) == 0
	}
	if pattern[0] == "**" {
		return matchParts(pattern[1:], parts) || (len(parts) > 0 && matchParts(pattern, parts[1:]))
	}
	if len(parts) == 0 {
		return false
	}
	ok, err := path.Match(pattern[0], parts[0])
	if err != nil {
		return false
	}
	return ok && matchParts(pattern[1:], parts[1:])
}

func (p Policy) Filter(s Snapshot) Snapshot {
	out := Snapshot{Files: map[string]File{}, Omitted: []string{}, PolicyHash: p.Hash()}
	for name, f := range s.Files {
		if p.Allows(name) {
			out.Files[name] = f
		}
	}
	for _, skip := range s.Omitted {
		if p.Allows(skip) {
			out.Omitted = append(out.Omitted, skip)
		}
	}
	return out
}

func (p Policy) Hash() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte("capture-v1:1MiB:16MiB:UTF8:nolinks\n"+strings.Join(p.patterns, "\n"))))
}
