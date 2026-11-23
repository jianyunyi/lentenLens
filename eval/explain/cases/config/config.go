package evalcase

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Parse reads key=value lines. Blank lines and lines starting with # are skipped.
func Parse(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	s := bufio.NewScanner(r)
	for n := 1; s.Scan(); n++ {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, fmt.Errorf("line %d: expected key=value", n)
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out, s.Err()
}
