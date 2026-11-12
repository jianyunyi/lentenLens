package aggregator

import (
	"encoding/json"
	"fmt"
	"time"
)

// ExampleAggregate demonstrates the basic usage of the Aggregate function
func ExampleAggregate() {
	issues := []string{
		"main.go:10:5: parse failed: expected ';', found 'EOF'",
		"handler.go:25:10: type check: missing imports: fmt, os",
		"service.go:15:20: type check: undefined: context",
		"util.go:30:15: ambiguous reference to New",
	}

	result := Aggregate(issues)

	fmt.Printf("Total restrictions: %d\n", result.Summary.TotalRestrictions)
	fmt.Printf("Total categories: %d\n", result.Summary.TotalCategories)
	fmt.Printf("First category: %s (count: %d)\n",
		result.Categories[0].Name,
		result.Categories[0].Count)

	// Output:
	// Total restrictions: 4
	// Total categories: 4
	// First category: Parse Errors (count: 1)
}

// ExampleAggregateWithTimeout demonstrates timeout protection
func ExampleAggregateWithTimeout() {
	issues := []string{
		"main.go:10: parse failed",
		"type check: missing imports: fmt",
	}

	// Use default timeout (200ms)
	result, err := AggregateWithTimeout(issues, 0)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Aggregated %d restrictions successfully\n", result.Summary.TotalRestrictions)

	// Output:
	// Aggregated 2 restrictions successfully
}

// ExampleAggregate_missingImportsSummarization demonstrates how missing imports are summarized
func ExampleAggregate_missingImportsSummarization() {
	issues := []string{
		"type check: missing imports: fmt, os, io",
		"type check: missing imports: context, crypto/sha256",
	}

	result := Aggregate(issues)

	// Find Missing Imports category
	for _, category := range result.Categories {
		if category.Name == "Missing Imports" {
			fmt.Printf("Category: %s\n", category.Name)
			fmt.Printf("Total restrictions: %d\n", category.Count)

			// The items are summarized into a single ItemSummary
			if len(category.Items) > 0 {
				if summary, ok := category.Items[0].(ItemSummary); ok {
					fmt.Printf("Summary type: %s\n", summary.Type)
					fmt.Printf("Unique packages: %d\n", summary.Count)
					fmt.Printf("Sample packages: %v\n", summary.SampleItems[:3])
				}
			}
		}
	}

	// Output:
	// Category: Missing Imports
	// Total restrictions: 2
	// Summary type: summary
	// Unique packages: 5
	// Sample packages: [context crypto/sha256 fmt]
}

// ExampleAggregate_jsonSerialization demonstrates JSON serialization
func ExampleAggregate_jsonSerialization() {
	issues := []string{
		"main.go:10: parse failed: expected ';'",
		"type check: missing imports: fmt",
	}

	result := Aggregate(issues)

	// Serialize to JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Printf("JSON error: %v\n", err)
		return
	}

	fmt.Printf("JSON output available: %t\n", len(jsonData) > 0)
	fmt.Printf("Contains summary: %t\n", len(result.Summary.CategoryBreakdown) > 0)
	fmt.Printf("Contains categories: %t\n", len(result.Categories) > 0)

	// Output:
	// JSON output available: true
	// Contains summary: true
	// Contains categories: true
}

// ExampleAggregateWithTimeout_timeout demonstrates timeout behavior
func ExampleAggregateWithTimeout_timeout() {
	// Create a reasonable set of issues
	issues := []string{
		"main.go:10: parse failed",
		"type check: missing imports: fmt",
	}

	// Use a very generous timeout that should always succeed
	result, err := AggregateWithTimeout(issues, 5*time.Second)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Success: %d restrictions aggregated\n", result.Summary.TotalRestrictions)

	// Output:
	// Success: 2 restrictions aggregated
}
