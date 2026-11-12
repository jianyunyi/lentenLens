package aggregator

import (
	"testing"
	"time"
)

// Test parseIssueString with various input formats
func TestParseIssueString(t *testing.T) {
	tests := []struct {
		name         string
		issue        string
		wantFilePath string
		wantLine     int
		wantMessage  string
	}{
		{
			name:         "full format with line and column",
			issue:        "distributed-task/cmd/api/main.go:49:2: declared and not used: fingerprint",
			wantFilePath: "distributed-task/cmd/api/main.go",
			wantLine:     49,
			wantMessage:  "declared and not used: fingerprint",
		},
		{
			name:         "format with line only",
			issue:        "internal/service/task_service.go:123: undefined: context",
			wantFilePath: "internal/service/task_service.go",
			wantLine:     123,
			wantMessage:  "undefined: context",
		},
		{
			name:         "format without line number",
			issue:        "main.go: parse failed: expected ';', found 'EOF'",
			wantFilePath: "main.go",
			wantLine:     0,
			wantMessage:  "parse failed: expected ';', found 'EOF'",
		},
		{
			name:         "message without file path",
			issue:        "type check: missing imports: context, crypto/sha256",
			wantFilePath: "",
			wantLine:     0,
			wantMessage:  "type check: missing imports: context, crypto/sha256",
		},
		{
			name:         "Chinese error message without path",
			issue:        "调用目标未唯一确定：errors.New",
			wantFilePath: "",
			wantLine:     0,
			wantMessage:  "调用目标未唯一确定：errors.New",
		},
		{
			name:         "empty string",
			issue:        "",
			wantFilePath: "",
			wantLine:     0,
			wantMessage:  "",
		},
		{
			name:         "Windows path format",
			issue:        "C:\\Users\\project\\main.go:10:5: syntax error",
			wantFilePath: "C:\\Users\\project\\main.go",
			wantLine:     10,
			wantMessage:  "syntax error",
		},
		{
			name:         "path with spaces",
			issue:        "my project/main.go:15: error here",
			wantFilePath: "my project/main.go",
			wantLine:     15,
			wantMessage:  "error here",
		},
		{
			name:         "file path only without line",
			issue:        "internal/handler.go: some message",
			wantFilePath: "internal/handler.go",
			wantLine:     0,
			wantMessage:  "some message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFilePath, gotLine, gotMessage := parseIssueString(tt.issue)

			if gotFilePath != tt.wantFilePath {
				t.Errorf("parseIssueString() filePath = %v, want %v", gotFilePath, tt.wantFilePath)
			}
			if gotLine != tt.wantLine {
				t.Errorf("parseIssueString() line = %v, want %v", gotLine, tt.wantLine)
			}
			if gotMessage != tt.wantMessage {
				t.Errorf("parseIssueString() message = %v, want %v", gotMessage, tt.wantMessage)
			}
		})
	}
}

// Test classifyRestriction with various restriction patterns
func TestClassifyRestriction(t *testing.T) {
	tests := []struct {
		name         string
		message      string
		wantType     string
		wantCategory string
		wantSeverity string
	}{
		{
			name:         "parse error",
			message:      "parse failed: expected ';', found 'EOF'",
			wantType:     "parse",
			wantCategory: "Parse Errors",
			wantSeverity: "error",
		},
		{
			name:         "type check error generic",
			message:      "type check: cannot use x as string",
			wantType:     "type_check",
			wantCategory: "Type Check Errors",
			wantSeverity: "error",
		},
		{
			name:         "missing imports",
			message:      "type check: missing imports: context, crypto/sha256",
			wantType:     "type_check",
			wantCategory: "Missing Imports",
			wantSeverity: "error",
		},
		{
			name:         "undefined identifier",
			message:      "type check: undefined: context",
			wantType:     "type_check",
			wantCategory: "Undefined Identifiers",
			wantSeverity: "error",
		},
		{
			name:         "not declared",
			message:      "type check: variable not declared: foo",
			wantType:     "type_check",
			wantCategory: "Undefined Identifiers",
			wantSeverity: "error",
		},
		{
			name:         "ambiguous call",
			message:      "ambiguous reference to New",
			wantType:     "ambiguous",
			wantCategory: "Ambiguous Calls",
			wantSeverity: "warning",
		},
		{
			name:         "ambiguous call Chinese",
			message:      "调用目标未唯一确定：errors.New",
			wantType:     "ambiguous",
			wantCategory: "Ambiguous Calls",
			wantSeverity: "warning",
		},
		{
			name:         "other category",
			message:      "some unknown error message",
			wantType:     "other",
			wantCategory: "Other",
			wantSeverity: "info",
		},
		{
			name:         "case insensitive parse error",
			message:      "PARSE FAILED: something wrong",
			wantType:     "parse",
			wantCategory: "Parse Errors",
			wantSeverity: "error",
		},
		{
			name:         "case insensitive type check",
			message:      "TYPE CHECK: some error",
			wantType:     "type_check",
			wantCategory: "Type Check Errors",
			wantSeverity: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Restriction{
				Message: tt.message,
			}

			classifyRestriction(r)

			if r.Type != tt.wantType {
				t.Errorf("classifyRestriction() Type = %v, want %v", r.Type, tt.wantType)
			}
			if r.Category != tt.wantCategory {
				t.Errorf("classifyRestriction() Category = %v, want %v", r.Category, tt.wantCategory)
			}
			if r.Severity != tt.wantSeverity {
				t.Errorf("classifyRestriction() Severity = %v, want %v", r.Severity, tt.wantSeverity)
			}
		})
	}
}

// Test getIconForCategory
func TestGetIconForCategory(t *testing.T) {
	tests := []struct {
		name     string
		category string
		wantIcon string
	}{
		{
			name:     "parse errors icon",
			category: "Parse Errors",
			wantIcon: "❌",
		},
		{
			name:     "type check errors icon",
			category: "Type Check Errors",
			wantIcon: "⚠️",
		},
		{
			name:     "missing imports icon",
			category: "Missing Imports",
			wantIcon: "⚠️",
		},
		{
			name:     "undefined identifiers icon",
			category: "Undefined Identifiers",
			wantIcon: "⚠️",
		},
		{
			name:     "ambiguous calls icon",
			category: "Ambiguous Calls",
			wantIcon: "🔍",
		},
		{
			name:     "other category icon",
			category: "Other",
			wantIcon: "ℹ️",
		},
		{
			name:     "unknown category icon",
			category: "Unknown Category",
			wantIcon: "ℹ️",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIcon := getIconForCategory(tt.category)

			if gotIcon != tt.wantIcon {
				t.Errorf("getIconForCategory() = %v, want %v", gotIcon, tt.wantIcon)
			}
		})
	}
}

// Test helper functions
func TestContains(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{"substring present", "hello world", "world", true},
		{"substring not present", "hello world", "foo", false},
		{"empty substring", "hello", "", true},
		{"empty string", "", "test", false},
		{"exact match", "test", "test", true},
		{"case sensitive", "Hello", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contains(tt.s, tt.substr); got != tt.want {
				t.Errorf("contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToLower(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"all uppercase", "HELLO", "hello"},
		{"mixed case", "HeLLo WoRLd", "hello world"},
		{"all lowercase", "hello", "hello"},
		{"with numbers", "Test123", "test123"},
		{"with special chars", "Hello@World!", "hello@world!"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toLower(tt.s); got != tt.want {
				t.Errorf("toLower() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAtoi(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		want    int
		wantErr bool
	}{
		{"valid number", "123", 123, false},
		{"zero", "0", 0, false},
		{"large number", "9999", 9999, false},
		{"invalid - letters", "abc", 0, true},
		{"invalid - mixed", "12a3", 0, true},
		{"empty string", "", 0, true},
		{"negative not supported", "-5", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := atoi(tt.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("atoi() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("atoi() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"leading spaces", "  hello", "hello"},
		{"trailing spaces", "hello  ", "hello"},
		{"both sides", "  hello  ", "hello"},
		{"tabs and spaces", "\t hello \t", "hello"},
		{"newlines", "\nhello\n", "hello"},
		{"mixed whitespace", " \t\n hello world \n\t ", "hello world"},
		{"no whitespace", "hello", "hello"},
		{"all whitespace", "   \t\n  ", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trimSpace(tt.s); got != tt.want {
				t.Errorf("trimSpace() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Integration test: parse and classify together
func TestParseAndClassifyIntegration(t *testing.T) {
	tests := []struct {
		name         string
		issue        string
		wantFilePath string
		wantLine     int
		wantCategory string
		wantType     string
		wantSeverity string
	}{
		{
			name:         "parse error with location",
			issue:        "main.go:10: parse failed: expected ';'",
			wantFilePath: "main.go",
			wantLine:     10,
			wantCategory: "Parse Errors",
			wantType:     "parse",
			wantSeverity: "error",
		},
		{
			name:         "missing imports without location",
			issue:        "type check: missing imports: fmt, os",
			wantFilePath: "",
			wantLine:     0,
			wantCategory: "Missing Imports",
			wantType:     "type_check",
			wantSeverity: "error",
		},
		{
			name:         "ambiguous call with location",
			issue:        "handler.go:25:10: ambiguous reference to New",
			wantFilePath: "handler.go",
			wantLine:     25,
			wantCategory: "Ambiguous Calls",
			wantType:     "ambiguous",
			wantSeverity: "warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the issue string
			filePath, line, message := parseIssueString(tt.issue)

			// Create restriction and classify
			r := &Restriction{
				Message:  message,
				FilePath: filePath,
				Line:     line,
			}
			classifyRestriction(r)

			// Verify results
			if r.FilePath != tt.wantFilePath {
				t.Errorf("FilePath = %v, want %v", r.FilePath, tt.wantFilePath)
			}
			if r.Line != tt.wantLine {
				t.Errorf("Line = %v, want %v", r.Line, tt.wantLine)
			}
			if r.Category != tt.wantCategory {
				t.Errorf("Category = %v, want %v", r.Category, tt.wantCategory)
			}
			if r.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", r.Type, tt.wantType)
			}
			if r.Severity != tt.wantSeverity {
				t.Errorf("Severity = %v, want %v", r.Severity, tt.wantSeverity)
			}
		})
	}
}

// Test severityRank
func TestSeverityRank(t *testing.T) {
	tests := []struct {
		name     string
		category string
		wantRank int
	}{
		{"parse errors highest priority", "Parse Errors", 0},
		{"type check errors second", "Type Check Errors", 1},
		{"missing imports third", "Missing Imports", 2},
		{"undefined identifiers fourth", "Undefined Identifiers", 3},
		{"ambiguous calls fifth", "Ambiguous Calls", 4},
		{"other sixth", "Other", 5},
		{"unknown lowest", "Unknown Category", 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := severityRank(tt.category); got != tt.wantRank {
				t.Errorf("severityRank() = %v, want %v", got, tt.wantRank)
			}
		})
	}
}

// Test groupByCategory
func TestGroupByCategory(t *testing.T) {
	tests := []struct {
		name             string
		restrictions     []Restriction
		wantGroupCount   int
		wantFirstGroup   string // Should be highest severity
		wantCategoryCount map[string]int
	}{
		{
			name: "mixed categories",
			restrictions: []Restriction{
				{Category: "Parse Errors", Message: "error 1"},
				{Category: "Missing Imports", Message: "error 2"},
				{Category: "Parse Errors", Message: "error 3"},
				{Category: "Ambiguous Calls", Message: "warning 1"},
			},
			wantGroupCount: 3,
			wantFirstGroup: "Parse Errors",
			wantCategoryCount: map[string]int{
				"Parse Errors":    2,
				"Missing Imports": 1,
				"Ambiguous Calls": 1,
			},
		},
		{
			name: "single category",
			restrictions: []Restriction{
				{Category: "Type Check Errors", Message: "error 1"},
				{Category: "Type Check Errors", Message: "error 2"},
			},
			wantGroupCount: 1,
			wantFirstGroup: "Type Check Errors",
			wantCategoryCount: map[string]int{
				"Type Check Errors": 2,
			},
		},
		{
			name:             "empty input",
			restrictions:     []Restriction{},
			wantGroupCount:   0,
			wantFirstGroup:   "",
			wantCategoryCount: map[string]int{},
		},
		{
			name: "all categories",
			restrictions: []Restriction{
				{Category: "Other", Message: "msg"},
				{Category: "Ambiguous Calls", Message: "msg"},
				{Category: "Undefined Identifiers", Message: "msg"},
				{Category: "Missing Imports", Message: "msg"},
				{Category: "Type Check Errors", Message: "msg"},
				{Category: "Parse Errors", Message: "msg"},
			},
			wantGroupCount: 6,
			wantFirstGroup: "Parse Errors", // Highest severity
			wantCategoryCount: map[string]int{
				"Parse Errors":          1,
				"Type Check Errors":     1,
				"Missing Imports":       1,
				"Undefined Identifiers": 1,
				"Ambiguous Calls":       1,
				"Other":                 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := groupByCategory(tt.restrictions)

			// Check group count
			if len(groups) != tt.wantGroupCount {
				t.Errorf("groupByCategory() group count = %v, want %v", len(groups), tt.wantGroupCount)
			}

			// Check first group (highest severity)
			if tt.wantGroupCount > 0 && groups[0].Name != tt.wantFirstGroup {
				t.Errorf("groupByCategory() first group = %v, want %v", groups[0].Name, tt.wantFirstGroup)
			}

			// Check category counts
			for _, group := range groups {
				wantCount, exists := tt.wantCategoryCount[group.Name]
				if !exists {
					t.Errorf("groupByCategory() unexpected category: %v", group.Name)
					continue
				}
				if group.Count != wantCount {
					t.Errorf("groupByCategory() category %v count = %v, want %v", group.Name, group.Count, wantCount)
				}
				if len(group.Items) != wantCount {
					t.Errorf("groupByCategory() category %v items length = %v, want %v", group.Name, len(group.Items), wantCount)
				}
			}

			// Verify sorting order
			for i := 1; i < len(groups); i++ {
				if severityRank(groups[i-1].Name) > severityRank(groups[i].Name) {
					t.Errorf("groupByCategory() groups not sorted by severity: %v comes before %v", groups[i-1].Name, groups[i].Name)
				}
			}
		})
	}
}

// Test parseMissingImportsMessage
func TestParseMissingImportsMessage(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		wantPkgs []string
	}{
		{
			name:     "two packages",
			message:  "type check: missing imports: context, crypto/sha256",
			wantPkgs: []string{"context", "crypto/sha256"},
		},
		{
			name:     "single package",
			message:  "type check: missing imports: fmt",
			wantPkgs: []string{"fmt"},
		},
		{
			name:     "three packages with spaces",
			message:  "type check: missing imports: fmt, os, io",
			wantPkgs: []string{"fmt", "os", "io"},
		},
		{
			name:     "packages with no spaces after comma",
			message:  "type check: missing imports: fmt,os,io",
			wantPkgs: []string{"fmt", "os", "io"},
		},
		{
			name:     "not a missing imports message",
			message:  "type check: undefined variable",
			wantPkgs: nil,
		},
		{
			name:     "empty message",
			message:  "",
			wantPkgs: nil,
		},
		{
			name:     "case insensitive",
			message:  "type check: MISSING IMPORTS: fmt, os",
			wantPkgs: []string{"fmt", "os"},
		},
		{
			name:     "with extra whitespace",
			message:  "type check: missing imports:   fmt  ,  os  ",
			wantPkgs: []string{"fmt", "os"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPkgs := parseMissingImportsMessage(tt.message)

			if len(gotPkgs) != len(tt.wantPkgs) {
				t.Errorf("parseMissingImportsMessage() length = %v, want %v", len(gotPkgs), len(tt.wantPkgs))
				return
			}

			for i := range gotPkgs {
				if gotPkgs[i] != tt.wantPkgs[i] {
					t.Errorf("parseMissingImportsMessage()[%d] = %v, want %v", i, gotPkgs[i], tt.wantPkgs[i])
				}
			}
		})
	}
}

// Test extractMissingPackages
func TestExtractMissingPackages(t *testing.T) {
	tests := []struct {
		name         string
		restrictions []Restriction
		wantPkgs     []string
	}{
		{
			name: "single restriction",
			restrictions: []Restriction{
				{
					Category: "Missing Imports",
					Message:  "type check: missing imports: fmt, os",
				},
			},
			wantPkgs: []string{"fmt", "os"},
		},
		{
			name: "multiple restrictions with duplicates",
			restrictions: []Restriction{
				{
					Category: "Missing Imports",
					Message:  "type check: missing imports: fmt, os",
				},
				{
					Category: "Missing Imports",
					Message:  "type check: missing imports: os, io",
				},
			},
			wantPkgs: []string{"fmt", "io", "os"}, // Sorted and deduplicated
		},
		{
			name: "mixed categories",
			restrictions: []Restriction{
				{
					Category: "Missing Imports",
					Message:  "type check: missing imports: fmt",
				},
				{
					Category: "Parse Errors",
					Message:  "parse failed",
				},
				{
					Category: "Missing Imports",
					Message:  "type check: missing imports: os",
				},
			},
			wantPkgs: []string{"fmt", "os"},
		},
		{
			name:         "empty input",
			restrictions: []Restriction{},
			wantPkgs:     []string{},
		},
		{
			name: "no missing imports category",
			restrictions: []Restriction{
				{
					Category: "Parse Errors",
					Message:  "parse failed",
				},
			},
			wantPkgs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPkgs := extractMissingPackages(tt.restrictions)

			if len(gotPkgs) != len(tt.wantPkgs) {
				t.Errorf("extractMissingPackages() length = %v, want %v", len(gotPkgs), len(tt.wantPkgs))
				return
			}

			for i := range gotPkgs {
				if gotPkgs[i] != tt.wantPkgs[i] {
					t.Errorf("extractMissingPackages()[%d] = %v, want %v", i, gotPkgs[i], tt.wantPkgs[i])
				}
			}
		})
	}
}

// Test summarizeMissingImports
func TestSummarizeMissingImports(t *testing.T) {
	tests := []struct {
		name             string
		restrictions     []Restriction
		wantCount        int
		wantSampleCount  int
		wantMessageStart string
	}{
		{
			name: "three or fewer packages",
			restrictions: []Restriction{
				{
					Category: "Missing Imports",
					Message:  "type check: missing imports: fmt, os",
				},
			},
			wantCount:        2,
			wantSampleCount:  2,
			wantMessageStart: "Missing imports:",
		},
		{
			name: "more than three packages",
			restrictions: []Restriction{
				{
					Category: "Missing Imports",
					Message:  "type check: missing imports: fmt, os, io, context",
				},
			},
			wantCount:        4,
			wantSampleCount:  3,
			wantMessageStart: "4 missing imports",
		},
		{
			name: "exactly three packages",
			restrictions: []Restriction{
				{
					Category: "Missing Imports",
					Message:  "type check: missing imports: fmt, os, io",
				},
			},
			wantCount:        3,
			wantSampleCount:  3,
			wantMessageStart: "Missing imports:",
		},
		{
			name:             "empty input",
			restrictions:     []Restriction{},
			wantCount:        0,
			wantSampleCount:  0,
			wantMessageStart: "Missing imports:",
		},
		{
			name: "ten packages",
			restrictions: []Restriction{
				{
					Category: "Missing Imports",
					Message:  "type check: missing imports: a, b, c, d, e, f, g, h, i, j",
				},
			},
			wantCount:        10,
			wantSampleCount:  3,
			wantMessageStart: "10 missing imports",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := summarizeMissingImports(tt.restrictions)

			// Check type
			if summary.Type != "summary" {
				t.Errorf("summarizeMissingImports() Type = %v, want summary", summary.Type)
			}

			// Check count
			if summary.Count != tt.wantCount {
				t.Errorf("summarizeMissingImports() Count = %v, want %v", summary.Count, tt.wantCount)
			}

			// Check sample count
			if len(summary.SampleItems) != tt.wantSampleCount {
				t.Errorf("summarizeMissingImports() SampleItems length = %v, want %v", len(summary.SampleItems), tt.wantSampleCount)
			}

			// Check full list count
			if len(summary.FullList) != tt.wantCount {
				t.Errorf("summarizeMissingImports() FullList length = %v, want %v", len(summary.FullList), tt.wantCount)
			}

			// Check message format
			if tt.wantCount > 0 {
				msgStart := summary.Message
				if len(msgStart) > len(tt.wantMessageStart) {
					msgStart = msgStart[:len(tt.wantMessageStart)]
				}
				if msgStart != tt.wantMessageStart {
					t.Errorf("summarizeMissingImports() Message starts with %v, want %v", msgStart, tt.wantMessageStart)
				}
			}

			// Verify sample is subset of full list
			for _, sample := range summary.SampleItems {
				found := false
				for _, full := range summary.FullList {
					if sample == full {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("summarizeMissingImports() sample item %v not in full list", sample)
				}
			}
		})
	}
}

// Test formatMissingImportsMessage
func TestFormatMissingImportsMessage(t *testing.T) {
	tests := []struct {
		name        string
		count       int
		sampleItems []string
		want        string
	}{
		{
			name:        "one package",
			count:       1,
			sampleItems: []string{"fmt"},
			want:        "Missing imports: fmt",
		},
		{
			name:        "two packages",
			count:       2,
			sampleItems: []string{"fmt", "os"},
			want:        "Missing imports: fmt, os",
		},
		{
			name:        "three packages",
			count:       3,
			sampleItems: []string{"fmt", "os", "io"},
			want:        "Missing imports: fmt, os, io",
		},
		{
			name:        "four packages",
			count:       4,
			sampleItems: []string{"fmt", "os", "io"},
			want:        "4 missing imports (e.g., fmt, os, io)",
		},
		{
			name:        "ten packages",
			count:       10,
			sampleItems: []string{"a", "b", "c"},
			want:        "10 missing imports (e.g., a, b, c)",
		},
		{
			name:        "zero packages",
			count:       0,
			sampleItems: []string{},
			want:        "Missing imports: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMissingImportsMessage(tt.count, tt.sampleItems)

			if got != tt.want {
				t.Errorf("formatMissingImportsMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test helper functions for new implementations
func TestJoin(t *testing.T) {
	tests := []struct {
		name string
		strs []string
		sep  string
		want string
	}{
		{"empty slice", []string{}, ", ", ""},
		{"single element", []string{"hello"}, ", ", "hello"},
		{"two elements", []string{"hello", "world"}, ", ", "hello, world"},
		{"three elements", []string{"a", "b", "c"}, "-", "a-b-c"},
		{"with empty separator", []string{"hello", "world"}, "", "helloworld"},
		{"with space separator", []string{"foo", "bar", "baz"}, " ", "foo bar baz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := join(tt.strs, tt.sep); got != tt.want {
				t.Errorf("join() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want string
	}{
		{"zero", 0, "0"},
		{"single digit", 5, "5"},
		{"two digits", 42, "42"},
		{"three digits", 123, "123"},
		{"large number", 9999, "9999"},
		{"negative number", -5, "-5"},
		{"negative large", -123, "-123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := itoa(tt.n); got != tt.want {
				t.Errorf("itoa() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test parseRestrictions function
func TestParseRestrictions(t *testing.T) {
	tests := []struct {
		name             string
		issues           []string
		wantCount        int
		wantFirstMessage string
		wantFirstCat     string
	}{
		{
			name: "mixed issue types",
			issues: []string{
				"main.go:10: parse failed: expected ';'",
				"type check: missing imports: fmt, os",
				"handler.go:25: ambiguous reference to New",
				"",
				"service.go:15: type check: undefined: context",
			},
			wantCount:        4, // Empty string is skipped
			wantFirstMessage: "parse failed: expected ';'",
			wantFirstCat:     "Parse Errors",
		},
		{
			name:      "empty input",
			issues:    []string{},
			wantCount: 0,
		},
		{
			name:      "only empty strings",
			issues:    []string{"", "", ""},
			wantCount: 0,
		},
		{
			name: "all same category",
			issues: []string{
				"file1.go:10: parse failed",
				"file2.go:20: parse failed",
			},
			wantCount:        2,
			wantFirstMessage: "parse failed",
			wantFirstCat:     "Parse Errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restrictions := parseRestrictions(tt.issues)

			if len(restrictions) != tt.wantCount {
				t.Errorf("parseRestrictions() count = %v, want %v", len(restrictions), tt.wantCount)
			}

			if tt.wantCount > 0 {
				if restrictions[0].Message != tt.wantFirstMessage {
					t.Errorf("parseRestrictions()[0].Message = %v, want %v", restrictions[0].Message, tt.wantFirstMessage)
				}
				if restrictions[0].Category != tt.wantFirstCat {
					t.Errorf("parseRestrictions()[0].Category = %v, want %v", restrictions[0].Category, tt.wantFirstCat)
				}
			}

			// Verify all restrictions are classified
			for i, r := range restrictions {
				if r.Category == "" {
					t.Errorf("parseRestrictions()[%d] has empty Category", i)
				}
				if r.Type == "" {
					t.Errorf("parseRestrictions()[%d] has empty Type", i)
				}
				if r.Severity == "" {
					t.Errorf("parseRestrictions()[%d] has empty Severity", i)
				}
			}
		})
	}
}

// Test calculateSummary function
func TestCalculateSummary(t *testing.T) {
	tests := []struct {
		name              string
		groups            []CategoryGroup
		wantCategories    int
		wantRestrictions  int
		wantBreakdownKeys []string
	}{
		{
			name: "multiple categories",
			groups: []CategoryGroup{
				{Name: "Parse Errors", Count: 5},
				{Name: "Missing Imports", Count: 3},
				{Name: "Ambiguous Calls", Count: 2},
			},
			wantCategories:   3,
			wantRestrictions: 10,
			wantBreakdownKeys: []string{"Parse Errors", "Missing Imports", "Ambiguous Calls"},
		},
		{
			name: "single category",
			groups: []CategoryGroup{
				{Name: "Type Check Errors", Count: 7},
			},
			wantCategories:    1,
			wantRestrictions:  7,
			wantBreakdownKeys: []string{"Type Check Errors"},
		},
		{
			name:              "empty groups",
			groups:            []CategoryGroup{},
			wantCategories:    0,
			wantRestrictions:  0,
			wantBreakdownKeys: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := calculateSummary(tt.groups)

			if summary.TotalCategories != tt.wantCategories {
				t.Errorf("calculateSummary().TotalCategories = %v, want %v", summary.TotalCategories, tt.wantCategories)
			}

			if summary.TotalRestrictions != tt.wantRestrictions {
				t.Errorf("calculateSummary().TotalRestrictions = %v, want %v", summary.TotalRestrictions, tt.wantRestrictions)
			}

			if len(summary.CategoryBreakdown) != len(tt.wantBreakdownKeys) {
				t.Errorf("calculateSummary().CategoryBreakdown length = %v, want %v", len(summary.CategoryBreakdown), len(tt.wantBreakdownKeys))
			}

			// Verify breakdown contains all expected keys with correct counts
			for _, key := range tt.wantBreakdownKeys {
				found := false
				for _, group := range tt.groups {
					if group.Name == key {
						if summary.CategoryBreakdown[key] != group.Count {
							t.Errorf("calculateSummary().CategoryBreakdown[%v] = %v, want %v", key, summary.CategoryBreakdown[key], group.Count)
						}
						found = true
						break
					}
				}
				if !found {
					t.Errorf("calculateSummary().CategoryBreakdown missing key: %v", key)
				}
			}
		})
	}
}

// Test Aggregate function (end-to-end)
func TestAggregate(t *testing.T) {
	tests := []struct {
		name               string
		issues             []string
		wantGroupCount     int
		wantTotalRestrict  int
		wantFirstCategory  string
		checkMissingImport bool
	}{
		{
			name: "realistic mixed issues",
			issues: []string{
				"main.go:10: parse failed: expected ';'",
				"main.go:15: parse failed: missing '}",
				"type check: missing imports: fmt, os, io",
				"handler.go:25: ambiguous reference to New",
				"service.go:15: type check: undefined: context",
			},
			wantGroupCount:     4, // Parse Errors, Missing Imports, Ambiguous Calls, Undefined Identifiers
			wantTotalRestrict:  5,
			wantFirstCategory:  "Parse Errors", // Highest severity
			checkMissingImport: true,
		},
		{
			name:              "empty input",
			issues:            []string{},
			wantGroupCount:    0,
			wantTotalRestrict: 0,
		},
		{
			name: "only parse errors",
			issues: []string{
				"file1.go:10: parse failed",
				"file2.go:20: parse failed",
				"file3.go:30: parse failed",
			},
			wantGroupCount:    1,
			wantTotalRestrict: 3,
			wantFirstCategory: "Parse Errors",
		},
		{
			name: "multiple missing imports",
			issues: []string{
				"type check: missing imports: fmt",
				"type check: missing imports: os",
				"type check: missing imports: io, context",
			},
			wantGroupCount:     1,
			wantTotalRestrict:  3,
			wantFirstCategory:  "Missing Imports",
			checkMissingImport: true,
		},
		{
			name: "all categories",
			issues: []string{
				"main.go:1: parse failed",
				"main.go:2: type check: error",
				"type check: missing imports: fmt",
				"main.go:3: type check: undefined: foo",
				"main.go:4: ambiguous call",
				"main.go:5: some other error",
			},
			wantGroupCount:    6,
			wantTotalRestrict: 6,
			wantFirstCategory: "Parse Errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Aggregate(tt.issues)

			// Check summary statistics
			if result.Summary.TotalCategories != tt.wantGroupCount {
				t.Errorf("Aggregate().Summary.TotalCategories = %v, want %v", result.Summary.TotalCategories, tt.wantGroupCount)
			}

			if result.Summary.TotalRestrictions != tt.wantTotalRestrict {
				t.Errorf("Aggregate().Summary.TotalRestrictions = %v, want %v", result.Summary.TotalRestrictions, tt.wantTotalRestrict)
			}

			// Check category groups
			if len(result.Categories) != tt.wantGroupCount {
				t.Errorf("Aggregate().Categories length = %v, want %v", len(result.Categories), tt.wantGroupCount)
			}

			if tt.wantGroupCount > 0 {
				// Check first category is highest severity
				if result.Categories[0].Name != tt.wantFirstCategory {
					t.Errorf("Aggregate().Categories[0].Name = %v, want %v", result.Categories[0].Name, tt.wantFirstCategory)
				}

				// Verify timestamp is set
				if result.Timestamp.IsZero() {
					t.Errorf("Aggregate().Timestamp is zero")
				}

				// Verify all groups have icons
				for i, group := range result.Categories {
					if group.Icon == "" {
						t.Errorf("Aggregate().Categories[%d].Icon is empty", i)
					}
				}
			}

			// Check Missing Imports summarization
			if tt.checkMissingImport {
				for _, group := range result.Categories {
					if group.Name == "Missing Imports" {
						if len(group.Items) != 1 {
							t.Errorf("Missing Imports group should have exactly 1 item (summary), got %v", len(group.Items))
						}

						// Verify it's an ItemSummary
						if summary, ok := group.Items[0].(ItemSummary); ok {
							if summary.Type != "summary" {
								t.Errorf("Missing Imports item should be ItemSummary type, got %v", summary.Type)
							}
							if summary.Count == 0 {
								t.Errorf("Missing Imports summary should have count > 0")
							}
						} else {
							t.Errorf("Missing Imports item is not ItemSummary type")
						}
					}
				}
			}

			// Verify summary counts match category counts
			totalFromCategories := 0
			for _, group := range result.Categories {
				totalFromCategories += group.Count
			}
			if totalFromCategories != result.Summary.TotalRestrictions {
				t.Errorf("Sum of category counts (%v) != TotalRestrictions (%v)", totalFromCategories, result.Summary.TotalRestrictions)
			}
		})
	}
}

// Test AggregateWithTimeout - success case
func TestAggregateWithTimeout_Success(t *testing.T) {
	issues := []string{
		"main.go:10: parse failed",
		"type check: missing imports: fmt, os",
		"handler.go:25: ambiguous call",
	}

	result, err := AggregateWithTimeout(issues, 1*time.Second)

	if err != nil {
		t.Errorf("AggregateWithTimeout() unexpected error: %v", err)
	}

	if result.Summary.TotalRestrictions != 3 {
		t.Errorf("AggregateWithTimeout() TotalRestrictions = %v, want 3", result.Summary.TotalRestrictions)
	}

	if len(result.Categories) == 0 {
		t.Errorf("AggregateWithTimeout() returned empty categories")
	}
}

// Test AggregateWithTimeout - default timeout
func TestAggregateWithTimeout_DefaultTimeout(t *testing.T) {
	issues := []string{
		"main.go:10: parse failed",
	}

	// Pass 0 timeout to use default (200ms)
	result, err := AggregateWithTimeout(issues, 0)

	if err != nil {
		t.Errorf("AggregateWithTimeout() with default timeout unexpected error: %v", err)
	}

	if result.Summary.TotalRestrictions != 1 {
		t.Errorf("AggregateWithTimeout() TotalRestrictions = %v, want 1", result.Summary.TotalRestrictions)
	}
}

// Test AggregateWithTimeout - timeout scenario
func TestAggregateWithTimeout_Timeout(t *testing.T) {
	// This test is tricky because we need to make Aggregate() take longer than timeout
	// For now, we test with a very short timeout that might trigger
	// In a real scenario, you would inject a slow operation

	// Create a large number of issues to process
	issues := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		issues[i] = "main.go:" + itoa(i) + ": parse failed"
	}

	// Use an extremely short timeout (1 nanosecond)
	_, err := AggregateWithTimeout(issues, 1*time.Nanosecond)

	// We expect either success (if fast enough) or timeout error
	if err != nil {
		// Verify it's a timeout error
		if err.Error() == "" {
			t.Errorf("AggregateWithTimeout() timeout error has empty message")
		}
		// Check error message contains "timeout"
		if !contains(err.Error(), "timeout") {
			t.Errorf("AggregateWithTimeout() error should contain 'timeout', got: %v", err.Error())
		}
	}
	// If no error, that's also acceptable - the operation was fast enough
}

// Test AggregateWithTimeout - empty input
func TestAggregateWithTimeout_Empty(t *testing.T) {
	result, err := AggregateWithTimeout([]string{}, 100*time.Millisecond)

	if err != nil {
		t.Errorf("AggregateWithTimeout() with empty input unexpected error: %v", err)
	}

	if result.Summary.TotalRestrictions != 0 {
		t.Errorf("AggregateWithTimeout() TotalRestrictions = %v, want 0", result.Summary.TotalRestrictions)
	}

	if len(result.Categories) != 0 {
		t.Errorf("AggregateWithTimeout() Categories length = %v, want 0", len(result.Categories))
	}
}

// Benchmark Aggregate function
func BenchmarkAggregate_100(b *testing.B) {
	issues := make([]string, 100)
	for i := 0; i < 100; i++ {
		issues[i] = "file.go:" + itoa(i) + ": parse failed"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Aggregate(issues)
	}
}

func BenchmarkAggregate_1000(b *testing.B) {
	issues := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		issues[i] = "file.go:" + itoa(i) + ": parse failed"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Aggregate(issues)
	}
}

func BenchmarkAggregate_MixedCategories(b *testing.B) {
	issues := []string{
		"main.go:10: parse failed",
		"type check: missing imports: fmt, os, io, context",
		"handler.go:25: ambiguous call",
		"service.go:15: type check: undefined: foo",
		"util.go:30: type check: error",
	}

	// Replicate to get 100 issues
	allIssues := make([]string, 0, 500)
	for i := 0; i < 100; i++ {
		allIssues = append(allIssues, issues...)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Aggregate(allIssues)
	}
}
