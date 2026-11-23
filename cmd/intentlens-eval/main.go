// Command intentlens-eval runs fixed explanation cases against a real model
// and writes a machine score plus a sheet for human review.
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"intentlens/internal/app"
	"intentlens/internal/explain"
)

type result struct {
	Case        string             `json:"case"`
	Error       string             `json:"error,omitempty"`
	Seconds     float64            `json:"seconds"`
	Score       *Score             `json:"score,omitempty"`
	Document    explain.Document   `json:"document"`
	Evidence    []explain.Evidence `json:"evidence,omitempty"`
	TokensSpent int                `json:"tokens_spent"`
}

func main() {
	cases := flag.String("cases", "eval/explain/cases", "case directory")
	out := flag.String("out", "eval-out", "output directory")
	provider := flag.String("provider", "openai-compatible", "anthropic | openai | openai-compatible | gemini")
	baseURL := flag.String("base-url", "https://openrouter.ai/api/v1", "API base URL")
	model := flag.String("model", "", "model ID (required)")
	keyEnv := flag.String("key-env", "INTENTLENS_API_KEY", "environment variable holding the API key")
	mode := flag.String("mode", explain.ModeLight, "verification mode: light | fast | strict")
	tokens := flag.Int("token-limit", 200000, "token budget per case")
	only := flag.String("only", "", "comma-separated case names to run")
	timeout := flag.Duration("timeout", 5*time.Minute, "timeout per case")
	flag.Parse()
	if err := run(*cases, *out, *only, explain.Config{
		Provider: *provider, BaseURL: *baseURL, Model: *model, APIKeyEnv: *keyEnv,
		Enabled: true, TokenLimit: *tokens, VerificationMode: *mode,
	}, *timeout); err != nil {
		fmt.Fprintln(os.Stderr, "intentlens-eval:", err)
		os.Exit(1)
	}
}

func run(casesDir, outDir, only string, cfg explain.Config, timeout time.Duration) error {
	if cfg.Model == "" {
		return errors.New("-model is required")
	}
	key := os.Getenv(cfg.APIKeyEnv)
	if key == "" {
		return fmt.Errorf("environment variable %s is empty", cfg.APIKeyEnv)
	}
	names, err := caseNames(casesDir, only)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	var results []result
	for _, name := range names {
		fmt.Fprintf(os.Stderr, "case %s ... ", name)
		r := runCase(filepath.Join(casesDir, name), cfg, key, timeout)
		r.Case = name
		if r.Error != "" {
			fmt.Fprintln(os.Stderr, "error:", r.Error)
		} else {
			fmt.Fprintf(os.Stderr, "%d claims, expect %d/%d, forbidden %d\n", r.Score.Claims,
				len(r.Score.ExpectMet), len(r.Score.ExpectMet)+len(r.Score.ExpectMissed), len(r.Score.ForbidHit))
		}
		results = append(results, r)
	}
	if err := writeJSON(filepath.Join(outDir, "report.json"), map[string]any{
		"provider": cfg.Provider, "base_url": cfg.BaseURL, "model": cfg.Model,
		"mode": cfg.VerificationMode, "generated": time.Now().UTC().Format(time.RFC3339), "results": results,
	}); err != nil {
		return err
	}
	if err := writeSheet(filepath.Join(outDir, "review.csv"), results); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "summary.md"), []byte(summary(cfg, results)), 0o644)
}

func caseNames(dir, only string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, n := range strings.Split(only, ",") {
		if n = strings.TrimSpace(n); n != "" {
			want[n] = true
		}
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && (len(want) == 0 || want[e.Name()]) {
			names = append(names, e.Name())
			delete(want, e.Name())
		}
	}
	for n := range want {
		return nil, fmt.Errorf("unknown case %q", n)
	}
	sort.Strings(names)
	return names, nil
}

func runCase(dir string, cfg explain.Config, key string, timeout time.Duration) (r result) {
	start := time.Now()
	defer func() { r.Seconds = time.Since(start).Seconds() }()
	var c Case
	raw, err := os.ReadFile(filepath.Join(dir, "case.json"))
	if err == nil {
		err = json.Unmarshal(raw, &c)
	}
	if err != nil {
		r.Error = "case.json: " + err.Error()
		return r
	}
	work, err := os.MkdirTemp("", "intentlens-eval-")
	if err != nil {
		r.Error = err.Error()
		return r
	}
	defer os.RemoveAll(work)
	root, state := filepath.Join(work, "project"), filepath.Join(work, "state")
	if err := copyTree(dir, root); err != nil {
		r.Error = err.Error()
		return r
	}
	src, err := os.ReadFile(filepath.Join(root, c.File))
	if err != nil {
		r.Error = err.Error()
		return r
	}
	line := functionLine(string(src), c.Function)
	if line == 0 {
		r.Error = fmt.Sprintf("function %s not found in %s", c.Function, c.File)
		return r
	}
	a, err := app.Open(root, state)
	if err == nil {
		err = a.Configure(app.Settings{Model: cfg}, true)
	}
	if err != nil {
		r.Error = err.Error()
		return r
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	exp, err := a.Explain(ctx, c.File, line, c.Question, key)
	if u, uerr := a.Usage(); uerr == nil {
		r.TokensSpent = u.ActualTokens + u.UncertainTokens
	}
	if err != nil {
		r.Error = err.Error()
		return r
	}
	s := score(c, exp.Document, exp.Evidence)
	r.Score, r.Document, r.Evidence = &s, exp.Document, exp.Evidence
	return r
}

func functionLine(src, name string) int {
	re := regexp.MustCompile(`^func\s+(\([^)]*\)\s*)?` + regexp.QuoteMeta(name) + `\s*[\[(]`)
	for i, l := range strings.Split(src, "\n") {
		if re.MatchString(l) {
			return i + 1
		}
	}
	return 0
}

func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		dst := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if rel == "case.json" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// writeSheet emits one row per claim; reviewers fill the last four columns.
func writeSheet(path string, results []result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString("\uFEFF"); err != nil { // Excel needs a BOM to read UTF-8.
		return err
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"case", "claim_id", "section", "kind", "status", "text", "evidence",
		"correct (0-2)", "grounded (y/n)", "useful (0-2)", "notes"})
	for _, r := range results {
		for _, cl := range r.Document.Claims {
			_ = w.Write([]string{r.Case, cl.ID, cl.Section, cl.Kind, cl.Status, cl.Text,
				evidenceRefs(cl.EvidenceIDs, r.Evidence), "", "", "", ""})
		}
		for i, u := range r.Document.Unknowns {
			_ = w.Write([]string{r.Case, fmt.Sprintf("unknown-%d", i+1), "unknowns", "", "", u, "", "", "", "", ""})
		}
	}
	w.Flush()
	return w.Error()
}

func summary(cfg explain.Config, results []result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Explanation eval: %s (%s, %s mode)\n\n", cfg.Model, cfg.BaseURL, cfg.VerificationMode)
	b.WriteString("| case | claims | supported | uncertain | unsupported | invalid citations | expected met | forbidden hit | tokens | seconds |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range results {
		if r.Error != "" {
			fmt.Fprintf(&b, "| %s | error: %s | | | | | | | %d | %.1f |\n", r.Case, strings.ReplaceAll(r.Error, "|", "/"), r.TokensSpent, r.Seconds)
			continue
		}
		s := r.Score
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %d/%d | %s | %d | %.1f |\n", r.Case, s.Claims,
			s.Statuses["supported"], s.Statuses["uncertain"], s.Statuses["unsupported"], len(s.InvalidCitations),
			len(s.ExpectMet), len(s.ExpectMet)+len(s.ExpectMissed), strings.Join(s.ForbidHit, ", "), r.TokensSpent, r.Seconds)
	}
	b.WriteString("\nKeyword checks are a first pass only. Score each claim in review.csv: correct 0-2, grounded y/n (the cited lines support it), useful 0-2.\n")
	return b.String()
}
