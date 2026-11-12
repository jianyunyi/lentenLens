package aggregator

import (
	"time"
)

// Restriction represents a single parse/type-check issue identified during code analysis.
// It contains complete metadata about the issue including its type, category, message,
// location information, and severity level.
type Restriction struct {
	Type     string `json:"type"`       // Issue type: "parse" | "type_check" | "ambiguous" | "other"
	Category string `json:"category"`   // Category name: "Parse Errors" | "Type Check Errors" | "Missing Imports" | "Ambiguous Calls" | "Other"
	Message  string `json:"message"`    // Full error message text
	FilePath string `json:"filePath"`   // Absolute or relative file path (empty string if location unknown)
	Line     int    `json:"lineNumber"` // Line number where issue occurs (0 if unknown)
	Severity string `json:"severity"`   // Severity level: "error" | "warning" | "info"
}

// Summary provides a statistical overview of all restrictions.
// It includes total counts and a breakdown by category for quick analysis.
type Summary struct {
	TotalCategories   int            `json:"totalCategories"`   // Number of distinct categories present
	TotalRestrictions int            `json:"totalRestrictions"` // Total count of all restrictions
	CategoryBreakdown map[string]int `json:"categoryBreakdown"` // Map of category name → count
}

// ItemSummary represents a summarized collection of similar items.
// Used primarily for missing imports to show counts and samples rather than
// listing every individual occurrence.
type ItemSummary struct {
	Type        string   `json:"type"`        // Summary type identifier: "summary"
	Count       int      `json:"count"`       // Total number of items summarized
	SampleItems []string `json:"sampleItems"` // First 3 items as preview
	FullList    []string `json:"fullList"`    // Complete list of all items for expansion
	Message     string   `json:"message"`     // Human-readable summary message
}

// CategoryGroup represents a collection of restrictions grouped by category.
// It can contain either individual Restriction items or ItemSummary entries
// for aggregated display of similar items.
type CategoryGroup struct {
	Name  string        `json:"name"`  // Category display name
	Icon  string        `json:"icon"`  // Icon identifier for UI display (❌ ⚠️ 🔍 ℹ️)
	Count int           `json:"count"` // Total number of items in this category
	Items []interface{} `json:"items"` // Mixed array of Restriction or ItemSummary objects
}

// AggregatedResult is the top-level output structure for aggregated restrictions.
// It combines statistical summary, categorized restrictions, and metadata.
type AggregatedResult struct {
	Summary    Summary         `json:"summary"`    // Statistical overview
	Categories []CategoryGroup `json:"categories"` // Array of categorized restriction groups
	Timestamp  time.Time       `json:"timestamp"`  // When this aggregation was performed
}

// parseIssueString parses a raw issue string from IntentLens CLI output.
// Expected format examples:
// - "distributed-task/cmd/api/main.go:49:2: declared and not used: fingerprint"
// - "distributed-task/cmd/api/main.go: parse failed: expected ';', found 'EOF'"
// - "type check: missing imports: context, crypto/sha256"
// - "调用目标未唯一确定：errors.New"
// - "C:\Users\project\main.go:10:5: syntax error" (Windows absolute path)
//
// Returns filePath (empty if not present), line number (0 if not present), and the message.
func parseIssueString(issue string) (filePath string, line int, message string) {
	// Try to match format: "path/file.go:123:col: message" or "path/file.go:123: message"
	// Pattern: filepath followed by optional :line:col or :line, then message
	
	// Special case: Windows absolute path starts with drive letter (e.g., C:\)
	// Need to skip the first colon if it's part of a Windows drive letter
	colonIdx := -1
	startSearch := 0
	
	// Check if this looks like a Windows absolute path (X:\ where X is a letter)
	if len(issue) >= 3 && issue[1] == ':' && (issue[2] == '\\' || issue[2] == '/') {
		if (issue[0] >= 'A' && issue[0] <= 'Z') || (issue[0] >= 'a' && issue[0] <= 'z') {
			// This is a Windows absolute path, skip the drive letter colon
			startSearch = 2
		}
	}
	
	// Find the first colon after the drive letter (if any)
	for i := startSearch; i < len(issue); i++ {
		if issue[i] == ':' {
			colonIdx = i
			break
		}
	}
	
	// If no colon found, treat entire string as message
	if colonIdx == -1 {
		message = issue
		return
	}
	
	// Check if the part before first colon looks like a file path
	// (contains / or \ or ends with .go)
	potentialPath := issue[:colonIdx]
	if len(potentialPath) > 0 && (containsPathSeparator(potentialPath) || endsWithGoExt(potentialPath)) {
		filePath = potentialPath
		remainder := issue[colonIdx+1:]
		
		// Try to parse line number from remainder
		if len(remainder) > 0 {
			// Look for next colon to find where line number ends
			nextColonIdx := -1
			for i := 0; i < len(remainder); i++ {
				if remainder[i] == ':' {
					nextColonIdx = i
					break
				}
			}
			
			if nextColonIdx > 0 {
				// Try to parse the part before next colon as line number
				lineStr := remainder[:nextColonIdx]
				var err error
				line, err = atoi(lineStr)
				if err == nil {
					// Successfully parsed line number
					// Now check if there's a column number or message
					messageStart := remainder[nextColonIdx+1:]
					
					// Check if next part is a column number (another number before colon)
					if len(messageStart) > 0 {
						thirdColonIdx := -1
						for i := 0; i < len(messageStart); i++ {
							if messageStart[i] == ':' {
								thirdColonIdx = i
								break
							}
						}
						
						if thirdColonIdx > 0 {
							colStr := messageStart[:thirdColonIdx]
							_, colErr := atoi(colStr)
							if colErr == nil {
								// It's a column number, message starts after this colon
								message = trimSpace(messageStart[thirdColonIdx+1:])
								return
							}
						}
						
						// No column number, this is the message
						message = trimSpace(messageStart)
						return
					}
				} else {
					// Not a line number, treat remainder as message
					line = 0
					message = trimSpace(remainder)
					return
				}
			} else {
				// No second colon, remainder might be just line number or message
				possibleLine, err := atoi(trimSpace(remainder))
				if err == nil {
					line = possibleLine
					message = ""
					return
				}
				// Not a number, treat as message
				message = trimSpace(remainder)
				return
			}
		}
	} else {
		// First part doesn't look like a file path, treat entire string as message
		message = issue
		return
	}
	
	return
}

// Helper functions for parseIssueString
func containsPathSeparator(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' || s[i] == '\\' {
			return true
		}
	}
	return false
}

func endsWithGoExt(s string) bool {
	return len(s) >= 3 && s[len(s)-3:] == ".go"
}

func trimSpace(s string) string {
	// Trim leading and trailing whitespace
	start := 0
	end := len(s)
	
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	
	return s[start:end]
}

func atoi(s string) (int, error) {
	if len(s) == 0 {
		return 0, &parseError{"empty string"}
	}
	
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, &parseError{"invalid digit"}
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, nil
}

type parseError struct {
	msg string
}

func (e *parseError) Error() string {
	return e.msg
}

// classifyRestriction determines the category and metadata for a restriction.
// Classification rules based on Requirement 9:
// - "parse failed" → Parse Errors
// - "type check:" → Type Check Errors
// - "missing imports:" → Missing Imports (subcategory)
// - "undefined:" or "not declared" → Undefined Identifiers
// - "ambiguous" or "调用目标未唯一确定" → Ambiguous Calls
// - Other → Other category
func classifyRestriction(r *Restriction) {
	msg := toLower(r.Message)
	
	if contains(msg, "parse failed") {
		r.Type = "parse"
		r.Category = "Parse Errors"
		r.Severity = "error"
	} else if contains(msg, "type check:") {
		r.Type = "type_check"
		r.Severity = "error"
		
		if contains(msg, "missing imports:") {
			r.Category = "Missing Imports"
		} else if contains(msg, "undefined:") || contains(msg, "not declared") {
			r.Category = "Undefined Identifiers"
		} else {
			r.Category = "Type Check Errors"
		}
	} else if contains(msg, "ambiguous") || contains(r.Message, "调用目标未唯一确定") {
		// Note: Chinese text is case-sensitive, so we check the original message
		r.Type = "ambiguous"
		r.Category = "Ambiguous Calls"
		r.Severity = "warning"
	} else {
		r.Type = "other"
		r.Category = "Other"
		r.Severity = "info"
	}
}

// Helper functions for classifyRestriction
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + ('a' - 'A')
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// getIconForCategory returns the appropriate icon identifier for a restriction category.
// Icons are used in the VS Code tree view to visually distinguish category types.
// Returns: "❌" for errors, "⚠️" for warnings, "🔍" for ambiguous calls, "ℹ️" for info.
func getIconForCategory(category string) string {
	switch category {
	case "Parse Errors":
		return "❌"
	case "Type Check Errors", "Missing Imports", "Undefined Identifiers":
		return "⚠️"
	case "Ambiguous Calls":
		return "🔍"
	default:
		return "ℹ️"
	}
}

// severityRank returns a sort order for categories.
// Lower values = higher priority (shown first).
func severityRank(category string) int {
	switch category {
	case "Parse Errors":
		return 0
	case "Type Check Errors":
		return 1
	case "Missing Imports":
		return 2
	case "Undefined Identifiers":
		return 3
	case "Ambiguous Calls":
		return 4
	case "Other":
		return 5
	default:
		return 6
	}
}

// groupByCategory groups restrictions by category and creates CategoryGroup slices.
// Categories are sorted by severity rank (errors first, then warnings, then info).
func groupByCategory(restrictions []Restriction) []CategoryGroup {
	// Create map: category -> []Restriction
	categoryMap := make(map[string][]Restriction)

	for i := range restrictions {
		category := restrictions[i].Category
		categoryMap[category] = append(categoryMap[category], restrictions[i])
	}

	// Convert map to slice
	var groups []CategoryGroup
	for category, items := range categoryMap {
		// Convert []Restriction to []interface{}
		interfaceItems := make([]interface{}, len(items))
		for i, item := range items {
			interfaceItems[i] = item
		}

		groups = append(groups, CategoryGroup{
			Name:  category,
			Icon:  getIconForCategory(category),
			Count: len(items),
			Items: interfaceItems,
		})
	}

	// Sort groups by severity rank
	for i := 0; i < len(groups); i++ {
		for j := i + 1; j < len(groups); j++ {
			if severityRank(groups[i].Name) > severityRank(groups[j].Name) {
				groups[i], groups[j] = groups[j], groups[i]
			}
		}
	}

	return groups
}

// parseMissingImportsMessage extracts package names from "type check: missing imports: pkg1, pkg2, pkg3".
// Returns nil if the message doesn't contain "missing imports:".
func parseMissingImportsMessage(message string) []string {
	// Look for "missing imports:" pattern (case-insensitive)
	prefix := "missing imports:"
	lowerMsg := toLower(message)
	idx := -1

	// Find "missing imports:" in the lowercase version
	for i := 0; i <= len(lowerMsg)-len(prefix); i++ {
		match := true
		for j := 0; j < len(prefix); j++ {
			if lowerMsg[i+j] != prefix[j] {
				match = false
				break
			}
		}
		if match {
			idx = i
			break
		}
	}

	if idx == -1 {
		return nil
	}

	// Extract package list after "missing imports:" using original message for correct case
	packagePart := message[idx+len(prefix):]
	packagePart = trimSpace(packagePart)

	// Split by comma
	var packages []string
	start := 0
	for i := 0; i <= len(packagePart); i++ {
		if i == len(packagePart) || packagePart[i] == ',' {
			if i > start {
				pkg := trimSpace(packagePart[start:i])
				if pkg != "" {
					packages = append(packages, pkg)
				}
			}
			start = i + 1
		}
	}

	return packages
}

// extractMissingPackages collects all missing packages from restrictions, deduplicates, and sorts.
func extractMissingPackages(restrictions []Restriction) []string {
	packageSet := make(map[string]bool)

	for _, r := range restrictions {
		if r.Category == "Missing Imports" {
			packages := parseMissingImportsMessage(r.Message)
			for _, pkg := range packages {
				packageSet[pkg] = true
			}
		}
	}

	// Convert to sorted slice
	var packages []string
	for pkg := range packageSet {
		packages = append(packages, pkg)
	}

	// Sort packages alphabetically
	for i := 0; i < len(packages); i++ {
		for j := i + 1; j < len(packages); j++ {
			if packages[i] > packages[j] {
				packages[i], packages[j] = packages[j], packages[i]
			}
		}
	}

	return packages
}

// summarizeMissingImports creates an ItemSummary from missing import restrictions.
func summarizeMissingImports(restrictions []Restriction) ItemSummary {
	packages := extractMissingPackages(restrictions)
	count := len(packages)

	var sampleItems []string
	if count <= 3 {
		sampleItems = packages
	} else {
		sampleItems = packages[:3]
	}

	message := formatMissingImportsMessage(count, sampleItems)

	return ItemSummary{
		Type:        "summary",
		Count:       count,
		SampleItems: sampleItems,
		FullList:    packages,
		Message:     message,
	}
}

// formatMissingImportsMessage formats the human-readable summary message.
func formatMissingImportsMessage(count int, sampleItems []string) string {
	if count <= 3 {
		return "Missing imports: " + join(sampleItems, ", ")
	}
	
	// Format: "10 missing imports (e.g., pkg1, pkg2, pkg3)"
	countStr := itoa(count)
	sample := join(sampleItems, ", ")
	return countStr + " missing imports (e.g., " + sample + ")"
}

// Helper function to join strings with a separator
func join(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}

	// Calculate total length
	totalLen := 0
	for _, s := range strs {
		totalLen += len(s)
	}
	totalLen += len(sep) * (len(strs) - 1)

	// Build result
	result := make([]byte, 0, totalLen)
	for i, s := range strs {
		if i > 0 {
			result = append(result, sep...)
		}
		result = append(result, s...)
	}

	return string(result)
}

// Helper function to convert int to string
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	// Handle negative numbers
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}

	// Count digits
	digits := 0
	temp := n
	for temp > 0 {
		digits++
		temp /= 10
	}

	// Build string
	var result []byte
	if negative {
		result = make([]byte, digits+1)
		result[0] = '-'
	} else {
		result = make([]byte, digits)
	}

	// Fill digits from right to left
	pos := len(result) - 1
	for n > 0 {
		result[pos] = byte('0' + (n % 10))
		n /= 10
		pos--
	}

	return string(result)
}

// parseRestrictions converts []string issue messages to []Restriction
func parseRestrictions(issues []string) []Restriction {
	restrictions := make([]Restriction, 0, len(issues))

	for _, issue := range issues {
		if issue == "" {
			continue
		}

		filePath, line, message := parseIssueString(issue)

		r := Restriction{
			Message:  message,
			FilePath: filePath,
			Line:     line,
		}

		classifyRestriction(&r)
		restrictions = append(restrictions, r)
	}

	return restrictions
}

// calculateSummary generates statistics from CategoryGroup slice
func calculateSummary(groups []CategoryGroup) Summary {
	categoryBreakdown := make(map[string]int)
	totalRestrictions := 0

	for _, group := range groups {
		categoryBreakdown[group.Name] = group.Count
		totalRestrictions += group.Count
	}

	return Summary{
		TotalCategories:   len(groups),
		TotalRestrictions: totalRestrictions,
		CategoryBreakdown: categoryBreakdown,
	}
}

// Aggregate aggregates parse restrictions into structured, hierarchical display
func Aggregate(issues []string) AggregatedResult {
	// Step 1: Parse and classify
	restrictions := parseRestrictions(issues)

	// Step 2: Group by category
	groups := groupByCategory(restrictions)

	// Step 3: Apply Missing Imports summarization
	for i := range groups {
		if groups[i].Name == "Missing Imports" {
			// Convert Items back to []Restriction
			restrictions := make([]Restriction, 0, len(groups[i].Items))
			for _, item := range groups[i].Items {
				if r, ok := item.(Restriction); ok {
					restrictions = append(restrictions, r)
				}
			}

			// Create summary
			summary := summarizeMissingImports(restrictions)

			// Replace Items with summary
			groups[i].Items = []interface{}{summary}
			// Keep count as total restrictions, not summary count
		}
	}

	// Step 4: Calculate summary statistics
	summary := calculateSummary(groups)

	// Step 5: Build result
	return AggregatedResult{
		Summary:    summary,
		Categories: groups,
		Timestamp:  time.Now(),
	}
}

// AggregateWithTimeout runs Aggregate() with timeout protection (default 200ms)
func AggregateWithTimeout(issues []string, timeout time.Duration) (AggregatedResult, error) {
	if timeout == 0 {
		timeout = 200 * time.Millisecond
	}

	type result struct {
		agg AggregatedResult
		err error
	}

	resultChan := make(chan result, 1)

	// Run aggregation in goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- result{
					err: formatError("aggregation panicked: ", r),
				}
			}
		}()

		agg := Aggregate(issues)
		resultChan <- result{agg: agg, err: nil}
	}()

	// Wait for result or timeout
	select {
	case res := <-resultChan:
		return res.agg, res.err
	case <-time.After(timeout):
		return AggregatedResult{}, formatTimeoutError(timeout)
	}
}

// formatError creates an error from a panic value
func formatError(prefix string, r interface{}) error {
	return &aggregationError{msg: prefix + formatPanicValue(r)}
}

// formatPanicValue converts a panic value to string
func formatPanicValue(r interface{}) string {
	switch v := r.(type) {
	case string:
		return v
	case error:
		return v.Error()
	default:
		// Convert to string representation
		return "unknown panic"
	}
}

// formatTimeoutError creates a timeout error message
func formatTimeoutError(timeout time.Duration) error {
	ms := timeout.Nanoseconds() / 1000000
	return &aggregationError{msg: "aggregation timeout after " + itoa(int(ms)) + "ms"}
}

// aggregationError represents an aggregation error
type aggregationError struct {
	msg string
}

func (e *aggregationError) Error() string {
	return e.msg
}
