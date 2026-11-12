# Requirements Document

## Introduction

IntentLens is a VS Code extension that analyzes Go code and provides insights about code dependencies, call graphs, and potential issues. During code analysis, the system generates parse restrictions (errors, warnings, and limitations) that currently display as individual notifications, resulting in UI clutter and poor user experience. This feature will aggregate parse restrictions into a unified, hierarchical display that groups similar issues, provides statistical summaries, and enables users to quickly understand and act on code analysis results.

## Glossary

- **IntentLens**: The VS Code extension system comprising a Go CLI backend and Node.js extension frontend
- **Parse_Restriction**: An issue, limitation, or error identified during code analysis (e.g., parse errors, type check failures, missing imports)
- **Aggregator**: The component responsible for collecting and grouping parse restrictions by type and category
- **Unified_Display**: The consolidated UI component that presents all parse restrictions in a hierarchical, grouped format
- **Restriction_Category**: A classification of parse restrictions (e.g., Parse Errors, Type Check Errors, Missing Imports, Ambiguous Calls)
- **Issue_Summary**: A statistical overview of restriction counts by category
- **Detail_Panel**: An expandable UI section showing individual restriction details
- **VS_Code_Extension**: The frontend Node.js component that runs within VS Code
- **Go_CLI**: The backend Go command-line tool that performs code analysis
- **Code_Index**: The Go module (internal/codeindex/index.go) that performs code parsing and type checking
- **Notification_System**: The existing VS Code notification mechanism in extension.js

## Requirements

### Requirement 1: Parse Restriction Aggregation

**User Story:** As a developer using IntentLens, I want all parse restrictions to be collected and grouped by category, so that I can understand the overall analysis results without being overwhelmed by individual messages.

#### Acceptance Criteria

1. WHEN THE Code_Index generates parse restrictions, THE Aggregator SHALL collect all restrictions into a single data structure
2. THE Aggregator SHALL classify each restriction into one of these categories: Parse Errors, Type Check Errors, Missing Imports, Ambiguous Calls, or Other
3. THE Aggregator SHALL group restrictions within each category by similarity (e.g., all missing import errors together)
4. THE Aggregator SHALL calculate count statistics for each category and subcategory
5. THE Aggregator SHALL preserve the original restriction message and location metadata for each individual restriction
6. THE Aggregator SHALL complete aggregation processing within 100 milliseconds for up to 1000 restrictions

### Requirement 2: Missing Import Summarization

**User Story:** As a developer, I want missing import errors to be summarized with a count and sample packages, so that I don't see 10+ identical "missing imports" messages.

#### Acceptance Criteria

1. WHEN THE Aggregator processes multiple missing import restrictions, THE Aggregator SHALL group them into a single summary entry
2. THE Aggregator SHALL count the total number of missing imports
3. WHEN there are 3 or fewer missing imports, THE Aggregator SHALL list all package names
4. WHEN there are more than 3 missing imports, THE Aggregator SHALL display the count and list the first 3 packages followed by "..."
5. THE Aggregator SHALL preserve the complete list of missing packages for detail expansion
6. THE Aggregator SHALL match the format pattern "type check: missing imports: <package_name>" when identifying missing import errors

### Requirement 3: Hierarchical Display Rendering

**User Story:** As a developer, I want parse restrictions displayed in a hierarchical tree format with category headers and expandable sections, so that I can quickly scan the overall status and drill into specific issues.

#### Acceptance Criteria

1. THE Unified_Display SHALL render restrictions in a tree hierarchy with categories as top-level nodes
2. THE Unified_Display SHALL display an Issue_Summary header showing total category count and total restriction count
3. THE Unified_Display SHALL use visual indicators (icons or symbols) to distinguish category types (errors, warnings, information)
4. THE Unified_Display SHALL show summarized counts for each category in the collapsed state
5. THE Unified_Display SHALL support expand/collapse toggle for each category
6. WHEN a category is expanded, THE Unified_Display SHALL show individual restriction details or subcategory summaries
7. THE Unified_Display SHALL display file paths and line numbers for restrictions that have location information

### Requirement 4: Backward Compatible Notification Integration

**User Story:** As a system maintainer, I want the unified display to integrate with the existing notification system, so that we don't break existing functionality while improving the user experience.

#### Acceptance Criteria

1. THE VS_Code_Extension SHALL maintain the existing deduplicatedNotify() function without breaking changes
2. WHEN parse restrictions are received from Go_CLI, THE VS_Code_Extension SHALL invoke the Aggregator before displaying notifications
3. THE Unified_Display SHALL replace individual restriction notifications with a single aggregated notification
4. THE VS_Code_Extension SHALL preserve notification deduplication logic to prevent duplicate unified displays within the TTL window
5. WHEN users click on any notification resembling the unified display format, THE VS_Code_Extension SHALL open the Detail_Panel regardless of whether it was generated by the Aggregator
6. THE VS_Code_Extension SHALL maintain all existing IntentLens features without regression

### Requirement 5: Interactive Action Buttons

**User Story:** As a developer, I want action buttons on the unified display to quickly ignore issues or get fix suggestions, so that I can efficiently manage code analysis results.

#### Acceptance Criteria

1. THE Unified_Display SHALL provide an "Expand Details" action button
2. THE Unified_Display SHALL provide an "Ignore" action button
3. THE Unified_Display SHALL provide a "Fix Suggestions" action button
4. WHEN the user clicks "Expand Details", THE VS_Code_Extension SHALL open the Detail_Panel showing all individual restrictions
5. WHEN the user clicks "Ignore", THE VS_Code_Extension SHALL dismiss the notification and mark restrictions as acknowledged
6. WHEN the user clicks "Fix Suggestions", THE VS_Code_Extension SHALL provide context-aware suggestions for resolving common restriction types
7. THE action buttons SHALL remain accessible and functional in both collapsed and expanded display states

### Requirement 6: Detail Panel Implementation

**User Story:** As a developer, I want to view complete details of all restrictions including file paths, line numbers, and specific error messages, so that I can investigate and resolve issues systematically.

#### Acceptance Criteria

1. THE Detail_Panel SHALL display all restrictions in a structured list format
2. THE Detail_Panel SHALL show the restriction category, file path, line number, and full error message for each restriction
3. THE Detail_Panel SHALL support filtering restrictions by category
4. THE Detail_Panel SHALL support copying individual restriction details to clipboard
5. THE Detail_Panel SHALL provide navigation links that open the relevant source file at the error location
6. WHEN no restrictions exist, THE Detail_Panel SHALL display a "No issues found" message

### Requirement 7: Parse Restriction Data Format

**User Story:** As a system developer, I want a standardized data format for aggregated restrictions, so that the frontend and backend can communicate effectively.

#### Acceptance Criteria

1. THE Go_CLI SHALL output parse restrictions in JSON format compatible with aggregation
2. THE restriction data format SHALL include fields: type, category, message, filePath, lineNumber, and severity
3. THE aggregated restriction object SHALL include fields: summary (category counts), categories (array of category objects), and timestamp
4. THE category object SHALL include fields: name, count, icon, items (array of restriction objects or summaries)
5. THE data format SHALL support serialization and deserialization without data loss
6. WHEN THE VS_Code_Extension detects that received restriction data fails validation, THE VS_Code_Extension SHALL reject the data entirely and display an error message to the user

### Requirement 8: Performance and Scalability

**User Story:** As a developer working with large codebases, I want the unified display to handle hundreds of restrictions efficiently, so that analysis results appear quickly without freezing the editor.

#### Acceptance Criteria

1. THE Aggregator SHALL process and aggregate up to 1000 restrictions within 100 milliseconds
2. THE Unified_Display SHALL render the aggregated view within 50 milliseconds after receiving data
3. THE Detail_Panel SHALL support lazy loading when displaying more than 100 individual restrictions
4. THE VS_Code_Extension SHALL not block the main UI thread during aggregation or rendering
5. THE system SHALL maintain responsive UI interaction during restriction processing
6. WHEN restriction count exceeds 500, THE Unified_Display SHALL show an informational warning about potential code analysis scope WHILE maintaining UI responsiveness and non-blocking behavior

### Requirement 9: Restriction Category Classification Rules

**User Story:** As a system developer, I want clear classification rules for categorizing parse restrictions, so that the aggregation logic is consistent and maintainable.

#### Acceptance Criteria

1. THE Aggregator SHALL classify restrictions matching "parse failed" pattern as Parse Errors category
2. THE Aggregator SHALL classify restrictions matching "type check:" pattern as Type Check Errors category
3. THE Aggregator SHALL classify restrictions containing "missing imports:" as Missing Imports subcategory within Type Check Errors
4. THE Aggregator SHALL classify restrictions containing "undefined:" or "not declared" as Undefined Identifiers subcategory
5. THE Aggregator SHALL classify restrictions containing "ambiguous" as Ambiguous Calls category
6. THE Aggregator SHALL classify restrictions not matching defined patterns as Other category
7. THE classification rules SHALL be configurable through a mapping configuration file

### Requirement 10: Error Recovery and Fallback

**User Story:** As a user, I want the system to gracefully handle aggregation failures, so that I still receive parse restriction information even if the unified display fails.

#### Acceptance Criteria

1. IF THE Aggregator encounters an error during processing, THEN THE VS_Code_Extension SHALL log the error and fall back to individual notifications
2. IF THE Unified_Display fails to render, THEN THE VS_Code_Extension SHALL display a simplified text summary of restriction counts
3. THE VS_Code_Extension SHALL implement timeout protection for aggregation operations exceeding 200 milliseconds
4. WHEN aggregation timeout occurs AND there are more than 5 restrictions, THE VS_Code_Extension SHALL discard restrictions beyond the first 5 and display only the first 5 individually with no further access path
5. THE error recovery mechanism SHALL not prevent users from accessing the raw restriction data
6. THE VS_Code_Extension SHALL report aggregation failures to telemetry for debugging purposes
