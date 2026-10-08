package capture

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
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
func LoadPolicy(root string) (Policy, error) {
	p := Policy{patterns: []string{".git", ".intent", ".intentignore", "node_modules", "vendor", ".env*", "*.pem", "*.key"}}
	name := filepath.Join(root, ".intentignore")
	info, err := os.Lstat(name)
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
	f, err := os.Open(name)
	if err != nil {
		return p, err
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
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
				if ok, _ := path.Match(pattern, part); ok {
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
	ok, _ := path.Match(pattern[0], parts[0])
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
