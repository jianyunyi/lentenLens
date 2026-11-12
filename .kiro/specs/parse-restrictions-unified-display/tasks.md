# Implementation Plan: Parse Restrictions Unified Display

## Overview

This feature transforms IntentLens's restriction reporting from individual notifications to a unified, hierarchical display. The implementation follows a 5-phase approach: Go backend aggregator, CLI integration, VS Code extension frontend, testing/polish, and documentation/release.

## Tasks

### Phase 1: Go Backend Aggregator

- [x] 1. Create aggregator package structure
  - Create `internal/aggregator` directory
  - Create `aggregator.go` with package declaration
  - Create `types.go` for data structures
  - Create `aggregator_test.go` for unit tests
  - _Requirements: 1.1, 7.1, 7.2_

- [x] 2. Implement core data structures
  - [x] 2.1 Define Restriction struct with JSON tags
    - Implement `type`, `category`, `message`, `filePath`, `lineNumber`, `severity` fields
    - Add JSON struct tags for serialization
    - _Requirements: 7.2_
  
  - [x] 2.2 Define CategoryGroup and Summary structs
    - Implement `CategoryGroup` with name, icon, count, items
    - Implement `Summary` with totalCategories, totalRestrictions, categoryBreakdown
    - _Requirements: 7.3_
  
  - [x] 2.3 Define AggregatedResult struct
    - Implement top-level struct with summary, categories, timestamp
    - _Requirements: 7.3_
  
  - [x] 2.4 Define ItemSummary struct for missing imports
    - Implement type, count, sample, full, message fields
    - _Requirements: 2.1, 2.5_

- [ ] 3. Implement restriction classification
  - [ ] 3.1 Implement parseIssueString() function
    - Parse format "path/file.go:123: message" to extract filePath, line, message
    - Handle cases without file path or line number
    - Use regex for robust parsing
    - _Requirements: 1.5, 3.7_
  
  - [x] 3.2 Implement classifyRestriction() function
    - Classify "parse failed" → Parse Errors category
    - Classify "type check:" → Type Check Errors category
    - Classify "missing imports:" → Missing Imports subcategory
    - Classify "undefined:" or "not declared" → Undefined Identifiers
    - Classify "ambiguous" → Ambiguous Calls category
    - Default to "Other" category for unknown patterns
    - Assign severity (error, warning, info) based on category
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6_
  
  - [x] 3.3 Implement getIconForCategory() function
    - Map Parse Errors, Type Check Errors → "error" icon
    - Map Ambiguous Calls → "warning" icon
    - Map Other → "info" icon
    - _Requirements: 3.3_

- [x] 4. Implement restriction grouping logic
  - [x] 4.1 Implement groupByCategory() function
    - Group restrictions by category into map
    - Convert map to []CategoryGroup slice
    - Sort categories by severity rank (errors first, then warnings, then info)
    - _Requirements: 1.3_
  
  - [x] 4.2 Implement severityRank() helper function
    - Define ordering: Parse Errors (0), Type Check Errors (1), Missing Imports (2), Undefined Identifiers (3), Ambiguous Calls (4), Other (5)
    - _Requirements: 1.3_

- [x] 5. Implement Missing Imports summarization
  - [x] 5.1 Implement parseMissingImportsMessage() function
    - Parse pattern "type check: missing imports: pkg1, pkg2"
    - Parse pattern "type check: 10 missing imports (e.g., pkg1, pkg2, pkg3)"
    - Extract package names using regex
    - _Requirements: 2.6_
  
  - [x] 5.2 Implement extractMissingPackages() function
    - Extract all package names from restrictions
    - Deduplicate packages using map
    - Sort packages alphabetically
    - _Requirements: 2.2, 2.5_
  
  - [x] 5.3 Implement summarizeMissingImports() function
    - Count unique packages
    - Create sample array (first 3 packages if count > 3)
    - Create full array with all packages
    - Generate human-readable message
    - Return ItemSummary struct
    - _Requirements: 2.1, 2.3, 2.4, 2.5_
  
  - [x] 5.4 Implement formatMissingImportsMessage() function
    - Format: "Missing imports: pkg1, pkg2, pkg3" for ≤3 packages
    - Format: "10 missing imports (e.g., pkg1, pkg2, pkg3)" for >3 packages
    - _Requirements: 2.4_

- [x] 6. Implement aggregation orchestration
  - [x] 6.1 Implement parseRestrictions() function
    - Convert []string issues to []Restriction
    - Call classifyRestriction() for each issue
    - _Requirements: 1.1_
  
  - [x] 6.2 Implement calculateSummary() function
    - Count total categories
    - Count total restrictions (sum of all category counts)
    - Build categoryBreakdown map (category name → count)
    - _Requirements: 1.4, 3.2_
  
  - [x] 6.3 Implement Aggregate() function
    - Call parseRestrictions() to parse input
    - Call groupByCategory() to organize restrictions
    - Apply summarizeMissingImports() for Missing Imports category
    - Call calculateSummary() for statistics
    - Return AggregatedResult with timestamp
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6_

- [x] 7. Implement timeout protection
  - [x] 7.1 Implement AggregateWithTimeout() function
    - Run Aggregate() in goroutine
    - Use select with time.After() for timeout (default 200ms)
    - Recover from panics in goroutine
    - Return error on timeout or panic
    - _Requirements: 8.3, 10.3_

- [ ] 8. Checkpoint - Backend aggregator core complete
  - Ensure all aggregator functions implemented
  - Verify code compiles without errors
  - Run go vet and golangci-lint
  - Ask user if questions arise

- [ ]* 9. Write unit tests for aggregator
  - [ ]* 9.1 Test classifyRestriction() with various patterns
    - Test "parse failed" → Parse Errors
    - Test "type check:" → Type Check Errors
    - Test "missing imports:" → Missing Imports
    - Test "undefined:" → Undefined Identifiers
    - Test "ambiguous" → Ambiguous Calls
    - Test unknown pattern → Other
    - Test message without file path
    - Test message with file path and line number
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6_
  
  - [ ]* 9.2 Test parseIssueString() edge cases
    - Test "path/file.go:123: message" format
    - Test "message" (no location) format
    - Test "file.go: message" (missing line number)
    - Test empty string input
    - _Requirements: 1.5_
  
  - [ ]* 9.3 Test groupByCategory() logic
    - Test with mixed categories
    - Test with single category
    - Test with empty input
    - Test category sorting by severity
    - _Requirements: 1.3_
  
  - [ ]* 9.4 Test summarizeMissingImports() scenarios
    - Test with 0 missing imports
    - Test with 1 missing import
    - Test with 3 missing imports (exact boundary)
    - Test with 4 missing imports (sample truncation)
    - Test with 10 missing imports
    - Test with duplicate package names
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_
  
  - [ ]* 9.5 Test Aggregate() end-to-end
    - Test with realistic issue set (mixed categories)
    - Test with empty input
    - Test with 100 restrictions
    - Test with 1000 restrictions
    - Verify summary counts match category counts
    - _Requirements: 1.1, 1.2, 1.3, 1.4_
  
  - [ ]* 9.6 Test AggregateWithTimeout() behavior
    - Test successful completion within timeout
    - Test timeout triggers error return
    - Test panic recovery
    - _Requirements: 10.3_
  
  - [ ]* 9.7 Test JSON serialization/deserialization
    - Test round-trip: AggregatedResult → JSON → AggregatedResult
    - Verify no data loss
    - Test with various restriction counts
    - _Requirements: 7.5_

- [ ]* 10. Write property-based tests for aggregator
  - [ ]* 10.1 Property 1: Complete Collection Preservation
    - **Property 1: Complete Collection Preservation**
    - **Validates: Requirements 1.1, 1.5**
    - Use gopter to generate random string slices
    - Verify all input restrictions appear in output
    - Run minimum 100 iterations
  
  - [ ]* 10.2 Property 2: Classification Completeness
    - **Property 2: Classification Completeness**
    - **Validates: Requirements 1.2**
    - Generate random restriction strings
    - Verify each classified into valid category
    - Valid categories: Parse Errors, Type Check Errors, Missing Imports, Undefined Identifiers, Ambiguous Calls, Other
  
  - [ ]* 10.3 Property 3: Category Grouping Consistency
    - **Property 3: Category Grouping Consistency**
    - **Validates: Requirements 1.3**
    - Generate restrictions with same category
    - Verify all appear in same CategoryGroup
  
  - [ ]* 10.4 Property 4: Count Arithmetic Invariant
    - **Property 4: Count Arithmetic Invariant**
    - **Validates: Requirements 1.4**
    - Generate random issue lists
    - Verify sum of category counts equals total restrictions
  
  - [ ]* 10.5 Property 5: Missing Imports Summarization
    - **Property 5: Missing Imports Summarization**
    - **Validates: Requirements 2.1**
    - Generate missing import restrictions
    - Verify grouped into exactly one ItemSummary
  
  - [ ]* 10.6 Property 6: Unique Package Counting
    - **Property 6: Unique Package Counting**
    - **Validates: Requirements 2.2**
    - Generate missing imports with duplicates
    - Verify count equals unique package count
  
  - [ ]* 10.7 Property 7: Sample Size Limit
    - **Property 7: Sample Size Limit**
    - **Validates: Requirements 2.4**
    - Generate >3 missing imports
    - Verify sample array contains exactly 3 items
  
  - [ ]* 10.8 Property 8: Full List Preservation
    - **Property 8: Full List Preservation**
    - **Validates: Requirements 2.5**
    - Generate any number of missing imports
    - Verify full list contains all unique packages
  
  - [ ]* 10.9 Property 9: Pattern Classification Correctness
    - **Property 9: Pattern Classification Correctness**
    - **Validates: Requirements 2.6, 9.1, 9.2, 9.3, 9.4, 9.5**
    - Generate strings with "parse failed" → verify Parse Errors
    - Generate strings with "type check:" → verify Type Check Errors
    - Generate strings with "missing imports:" → verify Missing Imports
    - Generate strings with "undefined:" → verify Undefined Identifiers
    - Generate strings with "ambiguous" → verify Ambiguous Calls
  
  - [ ]* 10.10 Property 10: Default Classification
    - **Property 10: Default Classification**
    - **Validates: Requirements 9.6**
    - Generate strings not matching any pattern
    - Verify classified as Other category
  
  - [ ]* 10.11 Property 11: Severity Icon Mapping
    - **Property 11: Severity Icon Mapping**
    - **Validates: Requirements 3.3**
    - Generate restrictions with different severities
    - Verify error severity → "error" icon
    - Verify warning severity → "warning" icon
    - Verify info severity → "info" icon
  
  - [ ]* 10.12 Property 12: Location Display Completeness
    - **Property 12: Location Display Completeness**
    - **Validates: Requirements 3.7**
    - Generate restrictions with non-empty filePath
    - Verify both filePath and line number preserved
  
  - [ ]* 10.13 Property 13: JSON Serialization Round-Trip
    - **Property 13: JSON Serialization Round-Trip**
    - **Validates: Requirements 7.5**
    - Generate random AggregatedResult
    - Serialize to JSON, deserialize back
    - Verify no data loss (ignoring timestamp precision)
  
  - [ ]* 10.14 Property 14: Schema Field Completeness
    - **Property 14: Schema Field Completeness**
    - **Validates: Requirements 7.2**
    - Generate random restrictions
    - Verify all contain required fields: type, category, message, filePath, lineNumber, severity
  
  - [ ]* 10.15 Property 15: Aggregated Result Schema Completeness
    - **Property 15: Aggregated Result Schema Completeness**
    - **Validates: Requirements 7.3**
    - Generate random AggregatedResult
    - Verify contains summary, categories, timestamp fields

### Phase 2: CLI Integration

- [x] 11. Integrate aggregator into CLI output
  - [x] 11.1 Modify internal/codeindex index output
    - Import aggregator package
    - Call aggregator.Aggregate(index.Issues) after indexing
    - Add `Aggregated *aggregator.AggregatedResult` field to output struct
    - Maintain existing `Issues []string` field for backward compatibility
    - _Requirements: 4.2, 4.6_
  
  - [x] 11.2 Implement error handling for aggregation failures
    - Wrap Aggregate() call with error recovery
    - Log aggregation errors
    - On failure, set Aggregated to nil (fallback to Issues field)
    - _Requirements: 10.1_
  
  - [x] 11.3 Implement timeout protection
    - Call AggregateWithTimeout() with 200ms timeout
    - On timeout, log warning and fallback to Issues field
    - Limit to first 5 issues on timeout when >5 issues exist
    - _Requirements: 10.3, 10.4_

- [ ] 12. Test CLI integration
  - [ ]* 12.1 Test CLI output with valid restrictions
    - Run CLI on Go project with parse errors
    - Verify JSON output contains `aggregated` field
    - Verify `issues` field still present (backward compatibility)
    - Parse JSON and validate schema
    - _Requirements: 4.6, 7.1_
  
  - [ ]* 12.2 Test CLI with no restrictions
    - Run CLI on valid Go project
    - Verify no aggregated output or empty aggregated
    - _Requirements: 4.6_
  
  - [ ]* 12.3 Test CLI aggregation timeout
    - Create test with >1000 restrictions
    - Verify timeout protection triggers
    - Verify fallback to first 5 issues
    - _Requirements: 10.3, 10.4_

- [ ] 13. Checkpoint - CLI integration complete
  - Verify CLI outputs valid JSON with aggregated field
  - Test with multiple Go projects
  - Ensure backward compatibility maintained
  - Ask user if questions arise

### Phase 3: VS Code Extension Frontend

- [ ] 14. Create RestrictionTreeView component
  - [x] 14.1 Create extension/src/restrictionView.js file
    - Create file with module structure
    - Define type comment blocks for TypeScript hints
    - _Requirements: 3.1_
  
  - [ ] 14.2 Implement RestrictionTreeProvider class
    - Implement constructor with EventEmitter for tree updates
    - Implement refresh(data) method to update tree
    - Implement getTreeItem(element) method
    - Implement getChildren(element) method with 3 levels: root → categories → items
    - _Requirements: 3.1, 3.5_
  
  - [ ] 14.3 Implement tree item creation methods
    - Implement createSummaryItem() for root summary node
    - Implement createCategoryItem() for category nodes with collapse state
    - Implement createRestrictionItem() for individual restrictions
    - Implement createMissingImportsSummaryItem() for missing imports summary
    - Implement createPackageItem() for individual package names
    - _Requirements: 3.2, 3.4, 3.6_
  
  - [ ] 14.4 Implement icon mapping
    - Map "error" → VS Code error ThemeIcon
    - Map "warning" → VS Code warning ThemeIcon
    - Map "info" → VS Code info ThemeIcon
    - Map missing imports → package ThemeIcon
    - _Requirements: 3.3_
  
  - [ ] 14.5 Implement navigation command handling
    - Create command to open file at restriction location
    - Use vscode.workspace.openTextDocument()
    - Use vscode.window.showTextDocument()
    - Position cursor at line number
    - Reveal range in center of editor
    - _Requirements: 3.7, 6.5_

- [ ] 15. Create RestrictionDetailPanel webview
  - [x] 15.1 Create extension/src/restrictionDetailPanel.js file
    - Create RestrictionDetailPanel class
    - Implement constructor with VS Code API and context
    - _Requirements: 6.1_
  
  - [ ] 15.2 Implement webview panel management
    - Implement show(aggregated) method to create/reveal panel
    - Implement panel disposal handling
    - Enable webview scripts
    - _Requirements: 6.1_
  
  - [ ] 15.3 Implement webview HTML generation
    - Implement getWebviewContent(aggregated) method
    - Render summary section with statistics
    - Render category filter dropdown
    - Render categories with restrictions
    - Add CSS styling using VS Code theme variables
    - _Requirements: 6.2_
  
  - [ ] 15.4 Implement webview message handling
    - Handle "openFile" message to navigate to source
    - Handle "copyRestriction" message to copy to clipboard
    - _Requirements: 6.4, 6.5_
  
  - [ ] 15.5 Implement category filtering
    - Add filter dropdown with "All Categories" option
    - Implement filterCategories() JavaScript function
    - Show/hide category divs based on filter selection
    - _Requirements: 6.3_
  
  - [ ] 15.6 Implement no-restrictions state
    - Display "No issues found" message when empty
    - _Requirements: 6.6_

- [ ] 16. Integrate into extension.js
  - [ ] 16.1 Import new components
    - Require restrictionView.js
    - Require restrictionDetailPanel.js
    - _Requirements: 4.2_
  
  - [ ] 16.2 Implement showAggregatedRestrictions() function
    - Accept aggregated data and restrictionProvider
    - Update tree view with new data
    - Show single warning notification with summary
    - Add action buttons: "Expand Details", "Ignore", "Fix Suggestions"
    - _Requirements: 4.3, 5.1, 5.2, 5.3_
  
  - [ ] 16.3 Implement action button handlers
    - "Expand Details" → open tree view in explorer
    - "Ignore" → save acknowledgment to workspace state
    - "Fix Suggestions" → show placeholder notification (future enhancement)
    - _Requirements: 5.4, 5.5, 5.6, 5.7_
  
  - [ ] 16.4 Modify cli() function to handle aggregated field
    - Check for result.aggregated in CLI response
    - If present and valid, call showAggregatedRestrictions()
    - If absent, fall back to existing individual notification logic
    - Maintain backward compatibility with older CLI versions
    - _Requirements: 4.2, 4.4_
  
  - [ ] 16.5 Implement schema validation
    - Create validateAggregatedSchema() function
    - Check for required fields: summary, categories, timestamp
    - Check summary has totalRestrictions number
    - Check categories is array
    - Reject invalid data and log error
    - _Requirements: 7.6, 10.1_
  
  - [ ] 16.6 Implement error handling and fallback
    - Wrap showAggregatedRestrictions() in try-catch
    - On error, fall back to individual notifications
    - Log errors for debugging
    - Send telemetry for aggregation failures
    - _Requirements: 10.1, 10.2, 10.5, 10.6_
  
  - [ ] 16.7 Implement timeout fallback logic
    - When timeout indicated in response, show first 5 issues individually
    - Display warning about additional issues not shown
    - _Requirements: 10.4_

- [ ] 17. Register components in extension activation
  - [ ] 17.1 Register RestrictionTreeView
    - Create tree view provider instance
    - Register with vscode.window.createTreeView()
    - Set treeDataProvider and enable showCollapseAll
    - Add to context.subscriptions
    - _Requirements: 3.1_
  
  - [ ] 17.2 Register navigation command
    - Register 'intentlens.openRestrictionLocation' command
    - _Requirements: 6.5_
  
  - [ ] 17.3 Register detail panel commands
    - Create RestrictionDetailPanel instance
    - Register commands for opening detail panel
    - _Requirements: 6.1_

- [ ] 18. Update package.json for VS Code contributions
  - [ ] 18.1 Add views contribution
    - Add "intentlensRestrictions" tree view to explorer
    - Set view name and when clause
    - _Requirements: 3.1_
  
  - [ ] 18.2 Add commands contribution
    - Add "intentlens.openRestrictionLocation" command
    - Add "intentlens.refreshRestrictions" command
    - Add "intentlens.clearRestrictions" command
    - Define titles and icons
    - _Requirements: 5.1_
  
  - [ ] 18.3 Add menus contribution
    - Add refresh button to view/title for intentlensRestrictions
    - Add clear button to view/title
    - _Requirements: 5.1_
  
  - [ ] 18.4 Add configuration contributions
    - Add "intentlens.restrictionDisplay.enabled" boolean setting (default true)
    - Add "intentlens.restrictionDisplay.autoExpandDetails" boolean setting (default false)
    - Add "intentlens.restrictionDisplay.maxIndividualFallback" number setting (default 5)
    - _Requirements: 4.4_

- [ ] 19. Checkpoint - Frontend implementation complete
  - Verify tree view displays in VS Code explorer
  - Test notification with action buttons
  - Test detail panel opens and renders correctly
  - Test navigation to source files
  - Ensure all UI elements functional
  - Ask user if questions arise

### Phase 4: Testing & Polish

- [ ] 20. End-to-end integration testing
  - [ ]* 20.1 Test complete workflow with parse errors
    - Open Go project with parse errors
    - Trigger IntentLens analysis
    - Verify single aggregated notification appears
    - Verify tree view populates with restrictions
    - Click "Expand Details" → verify detail panel opens
    - _Requirements: 1.1, 3.1, 5.1, 6.1_
  
  - [ ]* 20.2 Test with missing imports scenario
    - Create Go file with 10+ missing imports
    - Run analysis
    - Verify missing imports summarized (not 10 individual notifications)
    - Verify summary shows count and sample packages
    - Expand missing imports → verify all packages listed
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_
  
  - [ ]* 20.3 Test category grouping and hierarchy
    - Create Go project with mixed restriction types
    - Verify restrictions grouped by category
    - Verify categories ordered by severity (errors first)
    - Test expand/collapse for each category
    - _Requirements: 1.2, 1.3, 3.4, 3.5_
  
  - [ ]* 20.4 Test navigation to source locations
    - Click on restriction with file path
    - Verify file opens in editor
    - Verify cursor positioned at correct line number
    - Verify line revealed in center of editor
    - _Requirements: 3.7, 6.5_
  
  - [ ]* 20.5 Test action buttons
    - Click "Expand Details" → verify tree view opens
    - Click "Ignore" → verify notification dismissed
    - Verify no duplicate notification appears (deduplication)
    - _Requirements: 5.4, 5.5, 5.7_
  
  - [ ]* 20.6 Test detail panel features
    - Open detail panel
    - Test category filter dropdown
    - Test copying restriction to clipboard
    - Test navigation links in detail panel
    - _Requirements: 6.3, 6.4, 6.5_

- [ ]* 21. Performance benchmarking
  - [ ]* 21.1 Benchmark aggregator with large inputs
    - Test with 100 restrictions → measure <10ms
    - Test with 1000 restrictions → measure <100ms
    - Run go test -bench for aggregator package
    - _Requirements: 1.6, 8.1_
  
  - [ ]* 21.2 Benchmark UI rendering
    - Test tree view rendering with 500 restrictions
    - Verify <50ms render time
    - Verify UI remains responsive during rendering
    - _Requirements: 8.2, 8.5_
  
  - [ ]* 21.3 Test UI non-blocking behavior
    - Trigger analysis with 500+ restrictions
    - Verify main UI thread not blocked
    - Verify VS Code remains responsive
    - _Requirements: 8.4, 8.5_
  
  - [ ]* 21.4 Test large restriction count warning
    - Create test with >500 restrictions
    - Verify warning message displayed
    - Verify UI remains responsive
    - _Requirements: 8.6_

- [ ]* 22. Backward compatibility testing
  - [ ]* 22.1 Test with older CLI (no aggregated field)
    - Use CLI version without aggregator support
    - Verify fallback to individual notifications
    - Verify no errors or crashes
    - _Requirements: 4.1, 4.4_
  
  - [ ]* 22.2 Test configuration settings
    - Disable unified display via setting
    - Verify fallback to legacy notifications
    - Re-enable and verify unified display works
    - _Requirements: 4.4_
  
  - [ ]* 22.3 Test existing IntentLens features
    - Test call path analysis
    - Test coverage insights
    - Test exercise mode
    - Verify no regressions introduced
    - _Requirements: 4.6_

- [ ]* 23. Error recovery scenario testing
  - [ ]* 23.1 Test aggregation failure fallback
    - Simulate aggregator error
    - Verify fallback to individual notifications
    - Verify error logged
    - _Requirements: 10.1, 10.2_
  
  - [ ]* 23.2 Test rendering failure fallback
    - Simulate tree view rendering error
    - Verify simplified text summary displayed
    - Verify no crash or data loss
    - _Requirements: 10.2_
  
  - [ ]* 23.3 Test timeout protection
    - Simulate aggregation timeout
    - Verify first 5 issues displayed individually
    - Verify timeout logged
    - _Requirements: 10.3, 10.4_
  
  - [ ]* 23.4 Test invalid JSON handling
    - Send malformed JSON from CLI
    - Verify schema validation rejects data
    - Verify error message displayed
    - _Requirements: 7.6, 10.1_
  
  - [ ]* 23.5 Test invalid file path navigation
    - Create restriction with non-existent file path
    - Click navigation link
    - Verify error message shown (not crash)
    - _Requirements: 6.5_

- [ ]* 24. UI/UX polish
  - [ ]* 24.1 Improve notification message formatting
    - Review notification text for clarity
    - Ensure summary is concise and actionable
    - _Requirements: 3.2_
  
  - [ ]* 24.2 Enhance tree view icons and labels
    - Verify icons visually distinct
    - Ensure labels readable and not truncated
    - Test with light and dark themes
    - _Requirements: 3.3, 3.4_
  
  - [ ]* 24.3 Improve detail panel styling
    - Ensure consistent with VS Code theme
    - Test responsive layout
    - Improve readability of restriction details
    - _Requirements: 6.2_
  
  - [ ]* 24.4 Add loading indicators
    - Show progress during aggregation (if >200ms)
    - Add "Loading..." state to tree view
    - _Requirements: 8.5_

- [ ] 25. Checkpoint - Testing and polish complete
  - All integration tests pass
  - Performance benchmarks meet requirements
  - Backward compatibility verified
  - Error scenarios handled gracefully
  - UI/UX polished and user-tested
  - Ask user if questions arise

### Phase 5: Documentation & Release

- [ ] 26. Update user documentation
  - [ ] 26.1 Update README.md
    - Add "Unified Restriction Display" feature section
    - Include screenshot of tree view
    - Include screenshot of detail panel
    - Explain action buttons and navigation
    - _Requirements: All_
  
  - [ ] 26.2 Update extension README
    - Update extension/README.md with new features
    - Add usage examples
    - Document configuration settings
    - _Requirements: All_
  
  - [ ] 26.3 Create feature demo materials
    - Capture GIF of restriction aggregation in action
    - Capture GIF of category expansion
    - Capture GIF of navigation to source
    - Add to docs/ directory
    - _Requirements: All_

- [ ] 27. Update developer documentation
  - [ ] 27.1 Document aggregator package API
    - Add godoc comments to all exported functions
    - Document data structures with examples
    - Add package-level documentation
    - _Requirements: 7.1, 7.2, 7.3_
  
  - [ ] 27.2 Update CONTRIBUTING.md
    - Document how to extend classification rules
    - Explain how to add new restriction categories
    - Document testing requirements for new features
    - _Requirements: 9.7_
  
  - [ ] 27.3 Update architecture documentation
    - Document three-tier architecture (Go → JSON → JS)
    - Document data flow and integration points
    - Add diagrams to docs/ directory
    - _Requirements: All_

- [ ] 28. Update CHANGELOG and release notes
  - [ ] 28.1 Update CHANGELOG.md
    - Add "## [0.6.2] - YYYY-MM-DD" section
    - List "Added" features: unified restriction display, tree view, detail panel
    - List "Changed" items: notification behavior
    - List "Fixed" items if any bugs resolved
    - _Requirements: All_
  
  - [ ] 28.2 Create release notes
    - Write release-notes-v0.6.2.txt with highlights
    - Emphasize improved user experience
    - Include breaking changes (none expected)
    - Include migration guide (none needed)
    - _Requirements: All_

- [ ] 29. Version bump and build
  - [ ] 29.1 Update version numbers
    - Update go.mod version comment if applicable
    - Update extension/package.json version to 0.6.2
    - Update any version constants in code
    - _Requirements: All_
  
  - [ ] 29.2 Build and package extension
    - Run npm run build in extension directory
    - Run vsce package to create .vsix
    - Verify package size reasonable
    - Test installation from .vsix file
    - _Requirements: All_
  
  - [ ] 29.3 Run final smoke tests
    - Install packaged extension in clean VS Code
    - Test all unified restriction display features
    - Test backward compatibility
    - Verify no console errors
    - _Requirements: All_

- [ ] 30. Final checkpoint - Release preparation complete
  - Documentation updated and reviewed
  - CHANGELOG and release notes finalized
  - Version bumped to 0.6.2
  - Extension built and packaged
  - All smoke tests pass
  - Ready for release
  - Ask user if questions arise

## Notes

- Tasks marked with `*` are optional testing and polish tasks. They can be skipped for faster MVP, but are recommended for production quality
- Each task references specific requirements for traceability
- Property-based tests (tasks 10.1-10.15) require gopter package: `go get github.com/leanovate/gopter`
- Frontend uses JavaScript (extension is already in JS, not TypeScript)
- Backend uses Go as selected by user
- Checkpoints ensure incremental validation and provide opportunities for user feedback
- Phase 1 (Backend) can be developed independently from Phase 3 (Frontend)
- Phase 2 (CLI Integration) bridges backend and frontend
- Phase 4 ensures production quality through comprehensive testing
- Phase 5 prepares for user-facing release with documentation

## Task Dependency Graph

```json
{
  "waves": [
    {
      "id": 0,
      "tasks": ["1.1"]
    },
    {
      "id": 1,
      "tasks": ["2.1", "2.2", "2.3", "2.4"]
    },
    {
      "id": 2,
      "tasks": ["3.1", "3.2", "3.3"]
    },
    {
      "id": 3,
      "tasks": ["4.1", "4.2"]
    },
    {
      "id": 4,
      "tasks": ["5.1", "5.2", "5.3", "5.4"]
    },
    {
      "id": 5,
      "tasks": ["6.1", "6.2", "6.3"]
    },
    {
      "id": 6,
      "tasks": ["7.1"]
    },
    {
      "id": 7,
      "tasks": ["9.1", "9.2", "9.3", "9.4", "9.5", "9.6", "9.7"]
    },
    {
      "id": 8,
      "tasks": ["10.1", "10.2", "10.3", "10.4", "10.5", "10.6", "10.7", "10.8", "10.9", "10.10", "10.11", "10.12", "10.13", "10.14", "10.15"]
    },
    {
      "id": 9,
      "tasks": ["11.1", "11.2", "11.3"]
    },
    {
      "id": 10,
      "tasks": ["12.1", "12.2", "12.3"]
    },
    {
      "id": 11,
      "tasks": ["14.1"]
    },
    {
      "id": 12,
      "tasks": ["14.2", "15.1"]
    },
    {
      "id": 13,
      "tasks": ["14.3", "14.4", "15.2"]
    },
    {
      "id": 14,
      "tasks": ["14.5", "15.3", "15.4"]
    },
    {
      "id": 15,
      "tasks": ["15.5", "15.6"]
    },
    {
      "id": 16,
      "tasks": ["16.1", "16.2"]
    },
    {
      "id": 17,
      "tasks": ["16.3", "16.4"]
    },
    {
      "id": 18,
      "tasks": ["16.5", "16.6", "16.7"]
    },
    {
      "id": 19,
      "tasks": ["17.1", "17.2", "17.3"]
    },
    {
      "id": 20,
      "tasks": ["18.1", "18.2", "18.3", "18.4"]
    },
    {
      "id": 21,
      "tasks": ["20.1", "20.2", "20.3", "20.4", "20.5", "20.6"]
    },
    {
      "id": 22,
      "tasks": ["21.1", "21.2", "21.3", "21.4"]
    },
    {
      "id": 23,
      "tasks": ["22.1", "22.2", "22.3"]
    },
    {
      "id": 24,
      "tasks": ["23.1", "23.2", "23.3", "23.4", "23.5"]
    },
    {
      "id": 25,
      "tasks": ["24.1", "24.2", "24.3", "24.4"]
    },
    {
      "id": 26,
      "tasks": ["26.1", "26.2", "26.3"]
    },
    {
      "id": 27,
      "tasks": ["27.1", "27.2", "27.3"]
    },
    {
      "id": 28,
      "tasks": ["28.1", "28.2"]
    },
    {
      "id": 29,
      "tasks": ["29.1"]
    },
    {
      "id": 30,
      "tasks": ["29.2", "29.3"]
    }
  ]
}
```
