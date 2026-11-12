# Design Document: Parse Restrictions Unified Display

## Overview

IntentLens currently generates parse restrictions (parsing errors, type check failures, missing imports) during Go code analysis and displays them as individual VS Code notifications. This creates UI clutter and poor user experience when analyzing code with multiple issues. This design introduces a three-tier architecture that aggregates restrictions in the Go backend, serializes them as JSON, and renders them in a hierarchical unified display in the VS Code extension frontend.

### Goals

1. **Eliminate notification spam**: Replace individual restriction notifications with a single aggregated notification
2. **Improve comprehension**: Group similar issues by category with statistical summaries
3. **Preserve detail access**: Enable users to drill down into individual restriction details
4. **Maintain backward compatibility**: Keep existing notification infrastructure intact
5. **Optimize performance**: Process aggregation in Go (fast) rather than JavaScript (slower)

### Non-Goals

- Automatic fix generation for restrictions (future enhancement)
- Real-time restriction updates during typing (remains post-analysis)
- Custom restriction filtering rules (future enhancement)

## Architecture

### System Components

```
┌─────────────────────────────────────────────────────────────┐
│                    VS Code Extension                         │
│  ┌────────────────────────────────────────────────────────┐ │
│  │         extension.js (Notification Layer)              │ │
│  │  - deduplicatedNotify() [existing]                     │ │
│  │  - showAggregatedRestrictions() [new]                  │ │
│  └────────────────────────────────────────────────────────┘ │
│                           │ ▲                                │
│                           ▼ │ JSON                           │
│  ┌────────────────────────────────────────────────────────┐ │
│  │         RestrictionTreeView (new component)            │ │
│  │  - Hierarchical tree rendering                         │ │
│  │  - Expand/collapse categories                          │ │
│  │  - Action buttons (Ignore, Fix, Details)               │ │
│  └────────────────────────────────────────────────────────┘ │
│                           │                                  │
│                           ▼                                  │
│  ┌────────────────────────────────────────────────────────┐ │
│  │         RestrictionDetailPanel (VS Code Webview)       │ │
│  │  - Detailed restriction list                           │ │
│  │  - Filtering by category                               │ │
│  │  - Source navigation                                   │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                           ▲
                           │ JSON over stdout
                           │
┌─────────────────────────────────────────────────────────────┐
│                    Go CLI Backend                            │
│  ┌────────────────────────────────────────────────────────┐ │
│  │         internal/codeindex (existing)                  │ │
│  │  - Parser and type checker                             │ │
│  │  - Generates []string issues                           │ │
│  └────────────────────────────────────────────────────────┘ │
│                           │                                  │
│                           ▼                                  │
│  ┌────────────────────────────────────────────────────────┐ │
│  │         internal/aggregator (new)                      │ │
│  │  - ParseRestrictionAggregator                          │ │
│  │  - Aggregate(restrictions) → AggregatedResult          │ │
│  │  - ClassifyRestriction() → Category                    │ │
│  │  - SummarizeMissingImports() → Summary                 │ │
│  └────────────────────────────────────────────────────────┘ │
│                           │                                  │
│                           ▼                                  │
│  ┌────────────────────────────────────────────────────────┐ │
│  │         JSON Output (stdio)                            │ │
│  │  - AggregatedResult serialized                         │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Data Flow

1. **Code Analysis**: `internal/codeindex/index.go` parses Go files and generates `Index.Issues []string`
2. **Aggregation**: New `internal/aggregator` package transforms `[]string` into `AggregatedResult`
3. **Serialization**: Go CLI outputs JSON to stdout
4. **Parsing**: `extension.js` receives JSON via existing `cli()` function
5. **Display**: `showAggregatedRestrictions()` invokes `RestrictionTreeView` component
6. **Detail Expansion**: User clicks "Expand Details" → opens `RestrictionDetailPanel` webview

### Integration Point

The aggregator integrates **after** code indexing but **before** JSON serialization:

```go
// In cmd/intentlens/main.go or internal/app/index.go
index := codeindex.Build(files)

// NEW: Aggregate restrictions
aggregated := aggregator.Aggregate(index.Issues)

// Output modified index with aggregated restrictions
output := struct {
    Functions  []codeindex.Function        `json:"functions"`
    Coverage   codeindex.Coverage          `json:"coverage"`
    Aggregated *aggregator.AggregatedResult `json:"aggregated"` // NEW field
}{
    Functions:  index.Functions,
    Coverage:   index.Coverage,
    Aggregated: &aggregated,
}
```

## Components and Interfaces

### 1. Aggregator (Go Backend)

**Location**: `internal/aggregator/aggregator.go`

#### Core Types

```go
package aggregator

import (
    "regexp"
    "sort"
    "strings"
    "time"
)

// Restriction represents a single parse restriction with full metadata
type Restriction struct {
    Type     string `json:"type"`     // "parse_error" | "type_check" | "ambiguous_call"
    Category string `json:"category"` // "Parse Errors" | "Type Check Errors" | "Missing Imports" | "Ambiguous Calls" | "Other"
    Message  string `json:"message"`  // Original error message
    FilePath string `json:"filePath"` // Source file path (may be empty)
    Line     int    `json:"lineNumber"` // Line number (0 if unavailable)
    Severity string `json:"severity"` // "error" | "warning" | "info"
}

// Summary provides statistical overview
type Summary struct {
    TotalCategories  int            `json:"totalCategories"`
    TotalRestrictions int           `json:"totalRestrictions"`
    CategoryBreakdown map[string]int `json:"categoryBreakdown"` // category name → count
}

// CategoryGroup represents a grouped set of restrictions
type CategoryGroup struct {
    Name  string        `json:"name"`  // "Parse Errors", "Missing Imports", etc.
    Icon  string        `json:"icon"`  // "error", "warning", "info" (VS Code icon names)
    Count int           `json:"count"` // Total restrictions in this category
    Items []interface{} `json:"items"` // Can be []Restriction or []ItemSummary
}

// ItemSummary represents a summarized group (e.g., missing imports)
type ItemSummary struct {
    Type    string   `json:"type"`    // "missing_imports_summary"
    Count   int      `json:"count"`   // Total missing imports
    Sample  []string `json:"sample"`  // First 3 package names
    Full    []string `json:"full"`    // Complete list of packages
    Message string   `json:"message"` // Human-readable summary
}

// AggregatedResult is the top-level output
type AggregatedResult struct {
    Summary    Summary         `json:"summary"`
    Categories []CategoryGroup `json:"categories"`
    Timestamp  time.Time       `json:"timestamp"`
}
```

#### Core Functions

```go
// Aggregate transforms raw issue strings into aggregated result
func Aggregate(issues []string) AggregatedResult {
    restrictions := parseRestrictions(issues)
    categories := groupByCategory(restrictions)
    summary := calculateSummary(categories)
    
    return AggregatedResult{
        Summary:    summary,
        Categories: categories,
        Timestamp:  time.Now(),
    }
}

// parseRestrictions converts string issues to structured Restrictions
func parseRestrictions(issues []string) []Restriction {
    restrictions := make([]Restriction, 0, len(issues))
    for _, issue := range issues {
        restrictions = append(restrictions, classifyRestriction(issue))
    }
    return restrictions
}

// classifyRestriction determines category and extracts metadata
func classifyRestriction(issue string) Restriction {
    // Extract file path and line number if present
    // Format: "path/file.go:123: message" or just "message"
    filePath, line, message := parseIssueString(issue)
    
    restriction := Restriction{
        Message:  message,
        FilePath: filePath,
        Line:     line,
    }
    
    // Classification rules (Requirement 9)
    switch {
    case strings.Contains(message, "parse failed"):
        restriction.Type = "parse_error"
        restriction.Category = "Parse Errors"
        restriction.Severity = "error"
    case strings.Contains(message, "type check:"):
        restriction.Type = "type_check"
        restriction.Severity = "error"
        if strings.Contains(message, "missing imports:") {
            restriction.Category = "Missing Imports"
        } else if strings.Contains(message, "undefined:") || strings.Contains(message, "not declared") {
            restriction.Category = "Undefined Identifiers"
        } else {
            restriction.Category = "Type Check Errors"
        }
    case strings.Contains(message, "ambiguous"):
        restriction.Type = "ambiguous_call"
        restriction.Category = "Ambiguous Calls"
        restriction.Severity = "warning"
    default:
        restriction.Type = "other"
        restriction.Category = "Other"
        restriction.Severity = "info"
    }
    
    return restriction
}

// groupByCategory organizes restrictions into CategoryGroups with summarization
func groupByCategory(restrictions []Restriction) []CategoryGroup {
    categoryMap := make(map[string][]Restriction)
    
    for _, r := range restrictions {
        categoryMap[r.Category] = append(categoryMap[r.Category], r)
    }
    
    groups := make([]CategoryGroup, 0, len(categoryMap))
    
    for category, items := range categoryMap {
        group := CategoryGroup{
            Name:  category,
            Icon:  getIconForCategory(category),
            Count: len(items),
        }
        
        // Special handling for Missing Imports (Requirement 2)
        if category == "Missing Imports" {
            group.Items = []interface{}{summarizeMissingImports(items)}
        } else {
            // Convert to interface{} slice
            group.Items = make([]interface{}, len(items))
            for i, item := range items {
                group.Items[i] = item
            }
        }
        
        groups = append(groups, group)
    }
    
    // Sort categories by severity: errors → warnings → info
    sort.Slice(groups, func(i, j int) bool {
        return severityRank(groups[i].Name) < severityRank(groups[j].Name)
    })
    
    return groups
}

// summarizeMissingImports creates ItemSummary for missing import errors
func summarizeMissingImports(restrictions []Restriction) ItemSummary {
    packages := extractMissingPackages(restrictions)
    count := len(packages)
    
    sample := packages
    if count > 3 {
        sample = packages[:3]
    }
    
    message := formatMissingImportsMessage(count, sample)
    
    return ItemSummary{
        Type:    "missing_imports_summary",
        Count:   count,
        Sample:  sample,
        Full:    packages,
        Message: message,
    }
}

// extractMissingPackages parses package names from restriction messages
func extractMissingPackages(restrictions []Restriction) []string {
    packageSet := make(map[string]bool)
    
    for _, r := range restrictions {
        // Parse "type check: missing imports: pkg1, pkg2, pkg3"
        // Or "type check: 10 missing imports (e.g., pkg1, pkg2, pkg3)"
        packages := parseMissingImportsMessage(r.Message)
        for _, pkg := range packages {
            packageSet[pkg] = true
        }
    }
    
    packages := make([]string, 0, len(packageSet))
    for pkg := range packageSet {
        packages = append(packages, pkg)
    }
    sort.Strings(packages)
    
    return packages
}

// parseMissingImportsMessage extracts package names from error message
func parseMissingImportsMessage(message string) []string {
    // Pattern 1: "type check: missing imports: pkg1, pkg2"
    if idx := strings.Index(message, "missing imports:"); idx != -1 {
        pkgPart := message[idx+len("missing imports:"):]
        pkgPart = strings.TrimSpace(pkgPart)
        return strings.Split(pkgPart, ", ")
    }
    
    // Pattern 2: "type check: 10 missing imports (e.g., pkg1, pkg2, pkg3)"
    re := regexp.MustCompile(`\(e\.g\.,\s*([^)]+)\)`)
    matches := re.FindStringSubmatch(message)
    if len(matches) > 1 {
        return strings.Split(matches[1], ", ")
    }
    
    return []string{}
}

// formatMissingImportsMessage creates human-readable summary
func formatMissingImportsMessage(count int, sample []string) string {
    if count <= 3 {
        return "Missing imports: " + strings.Join(sample, ", ")
    }
    return fmt.Sprintf("%d missing imports (e.g., %s)", count, strings.Join(sample, ", "))
}

// parseIssueString extracts file path, line number, and message from issue string
func parseIssueString(issue string) (filePath string, line int, message string) {
    // Format: "path/file.go:123: message" or "message"
    re := regexp.MustCompile(`^([^:]+):(\d+):\s*(.*)$`)
    matches := re.FindStringSubmatch(issue)
    
    if len(matches) == 4 {
        filePath = matches[1]
        line, _ = strconv.Atoi(matches[2])
        message = matches[3]
    } else {
        message = issue
    }
    
    return
}

// getIconForCategory returns VS Code icon name for category
func getIconForCategory(category string) string {
    switch category {
    case "Parse Errors", "Type Check Errors", "Undefined Identifiers":
        return "error"
    case "Ambiguous Calls":
        return "warning"
    default:
        return "info"
    }
}

// severityRank provides sorting order for categories
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
    default:
        return 5
    }
}

// calculateSummary computes statistical overview
func calculateSummary(categories []CategoryGroup) Summary {
    breakdown := make(map[string]int)
    totalRestrictions := 0
    
    for _, cat := range categories {
        breakdown[cat.Name] = cat.Count
        totalRestrictions += cat.Count
    }
    
    return Summary{
        TotalCategories:   len(categories),
        TotalRestrictions: totalRestrictions,
        CategoryBreakdown: breakdown,
    }
}
```

#### Performance Optimization

```go
// AggregateWithTimeout provides timeout protection (Requirement 10)
func AggregateWithTimeout(issues []string, timeout time.Duration) (AggregatedResult, error) {
    done := make(chan AggregatedResult, 1)
    errChan := make(chan error, 1)
    
    go func() {
        defer func() {
            if r := recover(); r != nil {
                errChan <- fmt.Errorf("aggregation panic: %v", r)
            }
        }()
        done <- Aggregate(issues)
    }()
    
    select {
    case result := <-done:
        return result, nil
    case err := <-errChan:
        return AggregatedResult{}, err
    case <-time.After(timeout):
        return AggregatedResult{}, fmt.Errorf("aggregation timeout after %v", timeout)
    }
}
```

### 2. RestrictionTreeView (TypeScript Frontend)

**Location**: `extension/src/restrictionView.js` (new file)

#### Core Interface

```typescript
// Type definitions
interface AggregatedRestrictions {
    summary: {
        totalCategories: number;
        totalRestrictions: number;
        categoryBreakdown: { [key: string]: number };
    };
    categories: CategoryGroup[];
    timestamp: string;
}

interface CategoryGroup {
    name: string;
    icon: string;
    count: number;
    items: (Restriction | ItemSummary)[];
}

interface Restriction {
    type: string;
    category: string;
    message: string;
    filePath: string;
    lineNumber: number;
    severity: string;
}

interface ItemSummary {
    type: string;
    count: number;
    sample: string[];
    full: string[];
    message: string;
}

// Tree view provider
class RestrictionTreeProvider {
    constructor(vscode) {
        this._onDidChangeTreeData = new vscode.EventEmitter();
        this.onDidChangeTreeData = this._onDidChangeTreeData.event;
        this.data = null;
    }
    
    refresh(data) {
        this.data = data;
        this._onDidChangeTreeData.fire();
    }
    
    getTreeItem(element) {
        return element;
    }
    
    getChildren(element) {
        if (!this.data) return [];
        
        if (!element) {
            // Root level: show summary + categories
            return [
                this.createSummaryItem(),
                ...this.data.categories.map(cat => this.createCategoryItem(cat))
            ];
        }
        
        if (element.contextValue === 'category') {
            // Category level: show items
            const category = this.data.categories.find(c => c.name === element.label);
            return category.items.map(item => {
                if (item.type === 'missing_imports_summary') {
                    return this.createSummaryItem(item);
                }
                return this.createRestrictionItem(item);
            });
        }
        
        if (element.contextValue === 'missing_imports_summary') {
            // Expand missing imports summary to show all packages
            const summary = element.summary;
            return summary.full.map(pkg => this.createPackageItem(pkg));
        }
        
        return [];
    }
    
    createSummaryItem() {
        const item = new vscode.TreeItem(
            `Analysis Issues: ${this.data.summary.totalRestrictions} total in ${this.data.summary.totalCategories} categories`,
            vscode.TreeItemCollapsibleState.None
        );
        item.iconPath = new vscode.ThemeIcon('info');
        item.contextValue = 'summary';
        return item;
    }
    
    createCategoryItem(category) {
        const item = new vscode.TreeItem(
            `${category.name} (${category.count})`,
            vscode.TreeItemCollapsibleState.Collapsed
        );
        item.iconPath = new vscode.ThemeIcon(category.icon);
        item.contextValue = 'category';
        return item;
    }
    
    createSummaryItem(summary) {
        const item = new vscode.TreeItem(
            summary.message,
            vscode.TreeItemCollapsibleState.Collapsed
        );
        item.iconPath = new vscode.ThemeIcon('package');
        item.contextValue = 'missing_imports_summary';
        item.summary = summary;
        return item;
    }
    
    createRestrictionItem(restriction) {
        const label = restriction.filePath 
            ? `${restriction.filePath}:${restriction.lineNumber} - ${restriction.message}`
            : restriction.message;
        
        const item = new vscode.TreeItem(label, vscode.TreeItemCollapsibleState.None);
        item.iconPath = new vscode.ThemeIcon(restriction.severity);
        item.contextValue = 'restriction';
        item.restriction = restriction;
        
        // Enable navigation to source
        if (restriction.filePath) {
            item.command = {
                command: 'intentlens.openRestrictionLocation',
                title: 'Open Location',
                arguments: [restriction]
            };
        }
        
        return item;
    }
    
    createPackageItem(packageName) {
        const item = new vscode.TreeItem(packageName, vscode.TreeItemCollapsibleState.None);
        item.iconPath = new vscode.ThemeIcon('package');
        item.contextValue = 'package';
        return item;
    }
}

// Registration in extension.js
function registerRestrictionView(context, vscode) {
    const provider = new RestrictionTreeProvider(vscode);
    const treeView = vscode.window.createTreeView('intentlensRestrictions', {
        treeDataProvider: provider,
        showCollapseAll: true
    });
    
    context.subscriptions.push(treeView);
    
    // Command to open restriction source location
    context.subscriptions.push(
        vscode.commands.registerCommand('intentlens.openRestrictionLocation', async (restriction) => {
            if (!restriction.filePath) return;
            
            const uri = vscode.Uri.file(restriction.filePath);
            const doc = await vscode.workspace.openTextDocument(uri);
            const editor = await vscode.window.showTextDocument(doc);
            
            const position = new vscode.Position(Math.max(0, restriction.lineNumber - 1), 0);
            editor.selection = new vscode.Selection(position, position);
            editor.revealRange(new vscode.Range(position, position), vscode.TextEditorRevealType.InCenter);
        })
    );
    
    return provider;
}
```

### 3. Notification Integration (Modified extension.js)

```javascript
// In extension.js

// NEW: Show aggregated restrictions
function showAggregatedRestrictions(aggregated, restrictionProvider) {
    if (!aggregated || !aggregated.categories || aggregated.categories.length === 0) {
        return; // No restrictions to display
    }
    
    // Update tree view
    restrictionProvider.refresh(aggregated);
    
    // Show single notification with action buttons
    const summary = `${aggregated.summary.totalRestrictions} analysis issues found in ${aggregated.summary.totalCategories} categories`;
    
    vscode.window.showWarningMessage(
        `IntentLens: ${summary}`,
        'Expand Details',
        'Ignore',
        'Fix Suggestions'
    ).then(selection => {
        switch (selection) {
            case 'Expand Details':
                vscode.commands.executeCommand('workbench.view.extension.intentlens-restrictions');
                break;
            case 'Ignore':
                // Mark as acknowledged (store in workspace state)
                context.workspaceState.update('restrictionsAcknowledged', aggregated.timestamp);
                break;
            case 'Fix Suggestions':
                showFixSuggestions(aggregated);
                break;
        }
    });
}

// Modified CLI result handling
async function cli(folder, command, input, options = {}) {
    // ... existing code ...
    
    const result = await runCLI(binary, folder.uri.fsPath, command, input, { 
        timeout: command === 'exercise-run' ? 240000 : 180000, 
        signal: options.signal, 
        env 
    });
    
    // NEW: Handle aggregated restrictions if present
    if (result.aggregated && restrictionProvider) {
        showAggregatedRestrictions(result.aggregated, restrictionProvider);
    }
    // BACKWARD COMPATIBLE: Fall back to individual notifications
    else if (result.issues && Array.isArray(result.issues)) {
        for (const issue of result.issues.slice(0, 5)) {
            deduplicatedNotify('error', issue, msg => 
                vscode.window.showErrorMessage(`IntentLens: ${msg}`)
            );
        }
    }
    
    return result;
}
```

### 4. Detail Panel (VS Code Webview)

**Location**: `extension/src/restrictionDetailPanel.js` (new file)

```javascript
class RestrictionDetailPanel {
    constructor(vscode, context) {
        this.vscode = vscode;
        this.context = context;
        this.panel = null;
    }
    
    show(aggregated) {
        if (this.panel) {
            this.panel.reveal(vscode.ViewColumn.Two);
        } else {
            this.panel = vscode.window.createWebviewPanel(
                'restrictionDetails',
                'Restriction Details',
                vscode.ViewColumn.Two,
                { enableScripts: true }
            );
            
            this.panel.onDidDispose(() => {
                this.panel = null;
            });
        }
        
        this.panel.webview.html = this.getWebviewContent(aggregated);
        
        // Handle messages from webview
        this.panel.webview.onDidReceiveMessage(message => {
            switch (message.command) {
                case 'openFile':
                    this.openFile(message.filePath, message.lineNumber);
                    break;
                case 'copyRestriction':
                    vscode.env.clipboard.writeText(message.text);
                    vscode.window.showInformationMessage('Copied to clipboard');
                    break;
            }
        });
    }
    
    async openFile(filePath, lineNumber) {
        const uri = vscode.Uri.file(filePath);
        const doc = await vscode.workspace.openTextDocument(uri);
        const editor = await vscode.window.showTextDocument(doc);
        const position = new vscode.Position(Math.max(0, lineNumber - 1), 0);
        editor.selection = new vscode.Selection(position, position);
        editor.revealRange(new vscode.Range(position, position), vscode.TextEditorRevealType.InCenter);
    }
    
    getWebviewContent(aggregated) {
        const nonce = getNonce();
        
        return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Restriction Details</title>
    <style>
        body {
            font-family: var(--vscode-font-family);
            padding: 20px;
            color: var(--vscode-foreground);
        }
        .summary {
            background: var(--vscode-editor-inactiveSelectionBackground);
            padding: 15px;
            border-radius: 5px;
            margin-bottom: 20px;
        }
        .category {
            margin-bottom: 30px;
        }
        .category-header {
            display: flex;
            align-items: center;
            font-size: 1.2em;
            font-weight: bold;
            margin-bottom: 10px;
            padding-bottom: 5px;
            border-bottom: 1px solid var(--vscode-panel-border);
        }
        .category-icon {
            margin-right: 8px;
        }
        .restriction {
            padding: 10px;
            margin: 5px 0;
            background: var(--vscode-editor-background);
            border-left: 3px solid var(--vscode-inputValidation-errorBorder);
            cursor: pointer;
        }
        .restriction:hover {
            background: var(--vscode-list-hoverBackground);
        }
        .restriction-message {
            font-family: var(--vscode-editor-font-family);
            margin-bottom: 5px;
        }
        .restriction-location {
            font-size: 0.9em;
            color: var(--vscode-descriptionForeground);
        }
        .actions {
            display: flex;
            gap: 10px;
            margin-top: 20px;
        }
        button {
            background: var(--vscode-button-background);
            color: var(--vscode-button-foreground);
            border: none;
            padding: 8px 16px;
            cursor: pointer;
            border-radius: 3px;
        }
        button:hover {
            background: var(--vscode-button-hoverBackground);
        }
        .filter {
            margin-bottom: 15px;
        }
        select {
            background: var(--vscode-dropdown-background);
            color: var(--vscode-dropdown-foreground);
            border: 1px solid var(--vscode-dropdown-border);
            padding: 5px;
        }
    </style>
</head>
<body>
    <div class="summary">
        <h2>Analysis Summary</h2>
        <p>Total Issues: ${aggregated.summary.totalRestrictions}</p>
        <p>Categories: ${aggregated.summary.totalCategories}</p>
        <p>Timestamp: ${new Date(aggregated.timestamp).toLocaleString()}</p>
    </div>
    
    <div class="filter">
        <label>Filter by category: </label>
        <select id="categoryFilter" onchange="filterCategories()">
            <option value="all">All Categories</option>
            ${aggregated.categories.map(cat => 
                `<option value="${cat.name}">${cat.name} (${cat.count})</option>`
            ).join('')}
        </select>
    </div>
    
    <div id="categories">
        ${aggregated.categories.map(cat => this.renderCategory(cat)).join('')}
    </div>
    
    <script nonce="${nonce}">
        const vscode = acquireVsCodeApi();
        
        function openFile(filePath, lineNumber) {
            vscode.postMessage({
                command: 'openFile',
                filePath: filePath,
                lineNumber: lineNumber
            });
        }
        
        function copyRestriction(text) {
            vscode.postMessage({
                command: 'copyRestriction',
                text: text
            });
        }
        
        function filterCategories() {
            const filter = document.getElementById('categoryFilter').value;
            const categories = document.querySelectorAll('.category');
            
            categories.forEach(cat => {
                if (filter === 'all' || cat.dataset.category === filter) {
                    cat.style.display = 'block';
                } else {
                    cat.style.display = 'none';
                }
            });
        }
    </script>
</body>
</html>`;
    }
    
    renderCategory(category) {
        const iconMap = {
            'error': '⛔',
            'warning': '⚠️',
            'info': 'ℹ️'
        };
        
        return `
            <div class="category" data-category="${category.name}">
                <div class="category-header">
                    <span class="category-icon">${iconMap[category.icon] || 'ℹ️'}</span>
                    <span>${category.name} (${category.count})</span>
                </div>
                ${category.items.map(item => {
                    if (item.type === 'missing_imports_summary') {
                        return this.renderMissingImportsSummary(item);
                    }
                    return this.renderRestriction(item);
                }).join('')}
            </div>
        `;
    }
    
    renderRestriction(restriction) {
        const location = restriction.filePath 
            ? `${restriction.filePath}:${restriction.lineNumber}`
            : 'Unknown location';
        
        const onclick = restriction.filePath 
            ? `onclick="openFile('${restriction.filePath}', ${restriction.lineNumber})"`
            : '';
        
        return `
            <div class="restriction" ${onclick}>
                <div class="restriction-message">${escapeHtml(restriction.message)}</div>
                <div class="restriction-location">${location}</div>
            </div>
        `;
    }
    
    renderMissingImportsSummary(summary) {
        return `
            <div class="restriction">
                <div class="restriction-message">${escapeHtml(summary.message)}</div>
                <div class="restriction-location">
                    Complete list: ${summary.full.join(', ')}
                </div>
            </div>
        `;
    }
}

function escapeHtml(text) {
    const map = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' };
    return text.replace(/[&<>"']/g, m => map[m]);
}

function getNonce() {
    let text = '';
    const possible = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
    for (let i = 0; i < 32; i++) {
        text += possible.charAt(Math.floor(Math.random() * possible.length));
    }
    return text;
}
```

## Data Models

### Go Backend Models

```go
// internal/aggregator/types.go

// Restriction represents a single parse/analysis issue
type Restriction struct {
    Type     string `json:"type"`      // Classification: parse_error, type_check, ambiguous_call, other
    Category string `json:"category"`  // Human-readable: "Parse Errors", "Missing Imports", etc.
    Message  string `json:"message"`   // Original error message
    FilePath string `json:"filePath"`  // Source file path (empty if not file-specific)
    Line     int    `json:"lineNumber"` // Line number (0 if unavailable)
    Severity string `json:"severity"`  // error, warning, info
}

// ItemSummary represents summarized items (e.g., missing imports)
type ItemSummary struct {
    Type    string   `json:"type"`    // Summary type: missing_imports_summary
    Count   int      `json:"count"`   // Total count of summarized items
    Sample  []string `json:"sample"`  // First N items for preview
    Full    []string `json:"full"`    // Complete list (for detail expansion)
    Message string   `json:"message"` // Human-readable summary text
}

// CategoryGroup groups restrictions by category
type CategoryGroup struct {
    Name  string        `json:"name"`  // Category name
    Icon  string        `json:"icon"`  // VS Code icon name (error, warning, info)
    Count int           `json:"count"` // Total restrictions in this category
    Items []interface{} `json:"items"` // Mixed: Restriction or ItemSummary
}

// Summary provides statistical overview
type Summary struct {
    TotalCategories   int            `json:"totalCategories"`
    TotalRestrictions int            `json:"totalRestrictions"`
    CategoryBreakdown map[string]int `json:"categoryBreakdown"` // category → count
}

// AggregatedResult is the top-level output
type AggregatedResult struct {
    Summary    Summary         `json:"summary"`
    Categories []CategoryGroup `json:"categories"`
    Timestamp  time.Time       `json:"timestamp"`
}
```

### TypeScript Frontend Models

```typescript
// extension/src/types.d.ts

interface Restriction {
    type: string;       // "parse_error" | "type_check" | "ambiguous_call" | "other"
    category: string;   // "Parse Errors" | "Type Check Errors" | etc.
    message: string;
    filePath: string;
    lineNumber: number;
    severity: string;   // "error" | "warning" | "info"
}

interface ItemSummary {
    type: string;       // "missing_imports_summary"
    count: number;
    sample: string[];
    full: string[];
    message: string;
}

interface CategoryGroup {
    name: string;
    icon: string;
    count: number;
    items: (Restriction | ItemSummary)[];
}

interface Summary {
    totalCategories: number;
    totalRestrictions: number;
    categoryBreakdown: { [key: string]: number };
}

interface AggregatedRestrictions {
    summary: Summary;
    categories: CategoryGroup[];
    timestamp: string;
}

interface IndexResult {
    functions: Function[];
    coverage: Coverage;
    aggregated?: AggregatedRestrictions; // NEW: Optional aggregated restrictions
    issues?: string[];                   // LEGACY: Kept for backward compatibility
}
```

### JSON Wire Format Example

```json
{
    "summary": {
        "totalCategories": 3,
        "totalRestrictions": 15,
        "categoryBreakdown": {
            "Parse Errors": 2,
            "Missing Imports": 10,
            "Ambiguous Calls": 3
        }
    },
    "categories": [
        {
            "name": "Parse Errors",
            "icon": "error",
            "count": 2,
            "items": [
                {
                    "type": "parse_error",
                    "category": "Parse Errors",
                    "message": "expected ';', found 'EOF'",
                    "filePath": "src/main.go",
                    "lineNumber": 42,
                    "severity": "error"
                },
                {
                    "type": "parse_error",
                    "category": "Parse Errors",
                    "message": "syntax error: unexpected )",
                    "filePath": "src/utils.go",
                    "lineNumber": 15,
                    "severity": "error"
                }
            ]
        },
        {
            "name": "Missing Imports",
            "icon": "error",
            "count": 10,
            "items": [
                {
                    "type": "missing_imports_summary",
                    "count": 10,
                    "sample": ["fmt", "os", "io"],
                    "full": ["fmt", "os", "io", "net/http", "encoding/json", "strings", "bytes", "time", "sync", "context"],
                    "message": "10 missing imports (e.g., fmt, os, io)"
                }
            ]
        },
        {
            "name": "Ambiguous Calls",
            "icon": "warning",
            "count": 3,
            "items": [
                {
                    "type": "ambiguous_call",
                    "category": "Ambiguous Calls",
                    "message": "ambiguous reference to Helper",
                    "filePath": "src/service.go",
                    "lineNumber": 78,
                    "severity": "warning"
                }
            ]
        }
    ],
    "timestamp": "2026-10-06T14:30:00Z"
}
```

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system-essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*


### Property 1: Complete Collection Preservation

*For any* list of restriction strings, the aggregator SHALL produce a result where all original restrictions appear in the output with no data loss.

**Validates: Requirements 1.1, 1.5**

### Property 2: Classification Completeness

*For any* restriction string, the aggregator SHALL classify it into exactly one valid category from the defined set (Parse Errors, Type Check Errors, Missing Imports, Ambiguous Calls, Other).

**Validates: Requirements 1.2**

### Property 3: Category Grouping Consistency

*For any* set of restrictions with the same category, they SHALL all appear in the same CategoryGroup in the aggregated result.

**Validates: Requirements 1.3**

### Property 4: Count Arithmetic Invariant

*For any* aggregated result, the sum of all category counts SHALL equal the total restrictions count in the summary.

**Validates: Requirements 1.4**

### Property 5: Missing Imports Summarization

*For any* list of restrictions classified as missing imports, they SHALL be grouped into exactly one ItemSummary entry in the output.

**Validates: Requirements 2.1**

### Property 6: Unique Package Counting

*For any* set of missing import restrictions (potentially with duplicate packages), the summary count SHALL equal the number of unique package names.

**Validates: Requirements 2.2**

### Property 7: Sample Size Limit

*For any* missing imports summary where the count exceeds 3, the sample array SHALL contain exactly 3 package names.

**Validates: Requirements 2.4**

### Property 8: Full List Preservation

*For any* number of missing import restrictions, the summary's full list SHALL contain all unique package names regardless of sample truncation.

**Validates: Requirements 2.5**

### Property 9: Pattern Classification Correctness

*For any* restriction string containing "parse failed", it SHALL be classified into the "Parse Errors" category. Similarly, strings containing "type check:" SHALL map to "Type Check Errors", "missing imports:" to "Missing Imports", "undefined:" or "not declared" to "Undefined Identifiers", and "ambiguous" to "Ambiguous Calls".

**Validates: Requirements 2.6, 9.1, 9.2, 9.3, 9.4, 9.5**

### Property 10: Default Classification

*For any* restriction string that does not match any defined pattern, it SHALL be classified into the "Other" category.

**Validates: Requirements 9.6**

### Property 11: Severity Icon Mapping

*For any* category with a given severity level (error, warning, info), the assigned icon SHALL correctly correspond to that severity.

**Validates: Requirements 3.3**

### Property 12: Location Display Completeness

*For any* restriction with a non-empty filePath field, the display representation SHALL include both the file path and line number.

**Validates: Requirements 3.7**

### Property 13: JSON Serialization Round-Trip

*For any* valid AggregatedResult object, serializing it to JSON and then deserializing SHALL produce an equivalent object with no data loss.

**Validates: Requirements 7.5**

### Property 14: Schema Field Completeness

*For any* generated Restriction, it SHALL contain all required fields: type, category, message, filePath, lineNumber, and severity.

**Validates: Requirements 7.2**

### Property 15: Aggregated Result Schema Completeness

*For any* generated AggregatedResult, it SHALL contain all required top-level fields: summary (with category counts), categories (array), and timestamp.

**Validates: Requirements 7.3**

## Error Handling

### Aggregator Error Handling

1. **Parse Failures**: If a restriction string cannot be parsed (malformed format), it is still included with category "Other" and original message preserved
2. **Timeout Protection**: Aggregation operations exceeding 200ms timeout return error, triggering fallback to individual notifications
3. **Memory Limits**: For restriction counts >1000, aggregator logs warning but continues processing (performance may degrade)
4. **Invalid Input**: Empty or nil restriction lists return empty AggregatedResult with zero counts

### Frontend Error Handling

1. **Invalid JSON**: If CLI returns malformed JSON, log error and fall back to displaying raw Issues array as individual notifications
2. **Schema Validation Failure**: If aggregated data missing required fields, reject entirely and show error notification
3. **Rendering Failure**: If tree view throws exception, display simplified text summary of restriction counts
4. **Navigation Errors**: If file path invalid or file doesn't exist, show error message instead of opening editor

### Fallback Strategy (Requirement 10)

```javascript
async function handleRestrictions(cliResult) {
    try {
        // Primary path: use aggregated restrictions
        if (cliResult.aggregated && validateAggregatedSchema(cliResult.aggregated)) {
            showAggregatedRestrictions(cliResult.aggregated, restrictionProvider);
            return;
        }
    } catch (error) {
        console.error('Aggregated display failed:', error);
        telemetry.logError('aggregation_display_error', error);
    }
    
    // Fallback: individual notifications (legacy behavior)
    if (cliResult.issues && Array.isArray(cliResult.issues)) {
        const maxIndividual = 5;
        const issues = cliResult.issues.slice(0, maxIndividual);
        
        for (const issue of issues) {
            deduplicatedNotify('error', issue, msg => 
                vscode.window.showErrorMessage(`IntentLens: ${msg}`)
            );
        }
        
        if (cliResult.issues.length > maxIndividual) {
            vscode.window.showWarningMessage(
                `IntentLens: ${cliResult.issues.length - maxIndividual} additional issues not shown`
            );
        }
    }
}

function validateAggregatedSchema(aggregated) {
    return (
        aggregated &&
        aggregated.summary &&
        typeof aggregated.summary.totalRestrictions === 'number' &&
        Array.isArray(aggregated.categories) &&
        aggregated.timestamp
    );
}
```

## Testing Strategy

### Unit Testing (Go Backend)

**Test Coverage for Aggregator Package:**

1. **Classification Tests**:
   - Test `classifyRestriction()` with various restriction patterns
   - Verify pattern matching for each category
   - Test edge cases: empty strings, malformed messages, special characters
   - Test default classification for unknown patterns

2. **Grouping Tests**:
   - Test `groupByCategory()` with mixed categories
   - Verify same-category items grouped together
   - Test empty input, single item, large inputs (1000+ items)

3. **Summarization Tests**:
   - Test `summarizeMissingImports()` with 0, 1, 3, 4, 10, 100 missing imports
   - Verify unique package counting with duplicates
   - Verify sample truncation at 3 items
   - Verify full list preservation

4. **Parsing Tests**:
   - Test `parseIssueString()` with various formats:
     - `"path/file.go:123: message"`
     - `"message"` (no location)
     - `"file.go: message"` (missing line number)
   - Test extraction of file paths and line numbers

5. **Integration Tests**:
   - Test `Aggregate()` end-to-end with realistic issue sets
   - Test JSON serialization/deserialization
   - Test timeout behavior with `AggregateWithTimeout()`

**Example Unit Test:**

```go
func TestClassifyRestriction_ParseError(t *testing.T) {
    restriction := classifyRestriction("src/main.go:42: parse failed: expected ';'")
    
    assert.Equal(t, "parse_error", restriction.Type)
    assert.Equal(t, "Parse Errors", restriction.Category)
    assert.Equal(t, "error", restriction.Severity)
    assert.Equal(t, "src/main.go", restriction.FilePath)
    assert.Equal(t, 42, restriction.Line)
}

func TestSummarizeMissingImports_MoreThanThree(t *testing.T) {
    restrictions := []Restriction{
        {Message: "type check: missing imports: fmt"},
        {Message: "type check: missing imports: os"},
        {Message: "type check: missing imports: io"},
        {Message: "type check: missing imports: net/http"},
    }
    
    summary := summarizeMissingImports(restrictions)
    
    assert.Equal(t, 4, summary.Count)
    assert.Equal(t, 3, len(summary.Sample))
    assert.Equal(t, 4, len(summary.Full))
    assert.Contains(t, summary.Message, "4 missing imports")
}
```

### Property-Based Testing (Go Backend)

**PBT Library**: Use [gopter](https://github.com/leanovate/gopter) for Go property-based testing

**Configuration**: Minimum 100 iterations per property test

**Property Tests:**

```go
// Property 1: Complete Collection Preservation
// Feature: parse-restrictions-unified-display, Property 1: Complete Collection Preservation
func TestProperty_CompleteCollectionPreservation(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("all input restrictions appear in output", prop.ForAll(
        func(issues []string) bool {
            result := Aggregate(issues)
            
            // Extract all restriction messages from result
            outputMessages := extractAllMessages(result)
            
            // Verify every input appears in output
            for _, issue := range issues {
                if !contains(outputMessages, issue) {
                    return false
                }
            }
            return true
        },
        gen.SliceOf(gen.Identifier()), // Generate random string slices
    ))
    
    properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Property 2: Classification Completeness
// Feature: parse-restrictions-unified-display, Property 2: Classification Completeness
func TestProperty_ClassificationCompleteness(t *testing.T) {
    properties := gopter.NewProperties(nil)
    validCategories := []string{"Parse Errors", "Type Check Errors", "Missing Imports", 
                                  "Undefined Identifiers", "Ambiguous Calls", "Other"}
    
    properties.Property("every restriction classified into valid category", prop.ForAll(
        func(issue string) bool {
            restriction := classifyRestriction(issue)
            return contains(validCategories, restriction.Category)
        },
        gen.AnyString(),
    ))
    
    properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Property 4: Count Arithmetic Invariant
// Feature: parse-restrictions-unified-display, Property 4: Count Arithmetic Invariant
func TestProperty_CountArithmeticInvariant(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("sum of category counts equals total restrictions", prop.ForAll(
        func(issues []string) bool {
            result := Aggregate(issues)
            
            sumCounts := 0
            for _, cat := range result.Categories {
                sumCounts += cat.Count
            }
            
            return sumCounts == result.Summary.TotalRestrictions
        },
        gen.SliceOf(gen.Identifier()),
    ))
    
    properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Property 9: Pattern Classification Correctness
// Feature: parse-restrictions-unified-display, Property 9: Pattern Classification Correctness
func TestProperty_PatternClassification(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("parse failed maps to Parse Errors", prop.ForAll(
        func(prefix, suffix string) bool {
            issue := prefix + "parse failed" + suffix
            restriction := classifyRestriction(issue)
            return restriction.Category == "Parse Errors"
        },
        gen.Identifier(),
        gen.Identifier(),
    ))
    
    properties.Property("type check: maps to Type Check Errors", prop.ForAll(
        func(prefix, suffix string) bool {
            issue := prefix + "type check:" + suffix
            restriction := classifyRestriction(issue)
            return restriction.Category == "Type Check Errors" || 
                   restriction.Category == "Missing Imports" ||
                   restriction.Category == "Undefined Identifiers"
        },
        gen.Identifier(),
        gen.Identifier(),
    ))
    
    properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Property 13: JSON Serialization Round-Trip
// Feature: parse-restrictions-unified-display, Property 13: JSON Serialization Round-Trip
func TestProperty_JSONRoundTrip(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("serialize then deserialize preserves data", prop.ForAll(
        func(issues []string) bool {
            original := Aggregate(issues)
            
            // Serialize
            jsonBytes, err := json.Marshal(original)
            if err != nil {
                return false
            }
            
            // Deserialize
            var restored AggregatedResult
            err = json.Unmarshal(jsonBytes, &restored)
            if err != nil {
                return false
            }
            
            // Compare (ignoring timestamp precision differences)
            return aggregatedResultsEqual(original, restored)
        },
        gen.SliceOf(gen.Identifier()),
    ))
    
    properties.TestingRun(t, gopter.ConsoleReporter(false))
}
```

### Integration Testing (Frontend)

**Test Coverage for Extension:**

1. **CLI Integration**:
   - Mock CLI responses with valid aggregated JSON
   - Verify `showAggregatedRestrictions()` called when aggregated field present
   - Verify fallback to individual notifications when aggregated missing
   - Test timeout handling

2. **Tree View Tests**:
   - Test `RestrictionTreeProvider.getChildren()` at each level
   - Verify correct item creation for summary, categories, restrictions
   - Test with empty data, single category, multiple categories

3. **Notification Tests**:
   - Verify single notification shown for aggregated data
   - Verify action buttons present
   - Test deduplication within TTL window

4. **Detail Panel Tests**:
   - Test webview HTML generation
   - Test message handling (openFile, copyRestriction)
   - Test with various data sizes (0, 10, 100, 500 restrictions)

5. **Error Handling Tests**:
   - Test with malformed JSON
   - Test with invalid schema
   - Test rendering exceptions
   - Verify telemetry logging

### Manual Testing

1. **End-to-End Workflow**:
   - Open Go project with parse errors
   - Trigger IntentLens analysis
   - Verify unified notification appears
   - Click "Expand Details" → verify tree view opens
   - Expand categories → verify restrictions shown
   - Click restriction → verify file opens at correct line

2. **Performance Testing**:
   - Test with project having 10, 100, 500, 1000 restrictions
   - Verify UI remains responsive
   - Measure aggregation time
   - Verify lazy loading for detail panel

3. **Backward Compatibility**:
   - Test with older Go CLI (no aggregated field)
   - Verify fallback to individual notifications
   - Verify no regressions in existing features

## Implementation Phases

### Phase 1: Go Backend Aggregator (1-2 days)

1. Create `internal/aggregator` package
2. Implement core types (Restriction, CategoryGroup, AggregatedResult)
3. Implement `classifyRestriction()` with pattern matching
4. Implement `groupByCategory()` with sorting
5. Implement `summarizeMissingImports()` with package extraction
6. Implement `Aggregate()` orchestration function
7. Add timeout protection with `AggregateWithTimeout()`
8. Write comprehensive unit tests (80%+ coverage)
9. Write property-based tests (minimum 100 iterations each)

### Phase 2: CLI Integration (0.5-1 day)

1. Modify CLI output in `cmd/intentlens/main.go` or relevant command handler
2. Add `aggregated` field to JSON output
3. Maintain backward compatible `issues` field
4. Test CLI output with various Go projects

### Phase 3: VS Code Extension Frontend (2-3 days)

1. Create `extension/src/restrictionView.js` with `RestrictionTreeProvider`
2. Create `extension/src/restrictionDetailPanel.js` with webview
3. Modify `extension.js`:
   - Add `showAggregatedRestrictions()` function
   - Modify `cli()` to handle aggregated field
   - Add fallback logic with error handling
4. Register tree view in `package.json` contributes section
5. Add commands for navigation and actions
6. Implement schema validation

### Phase 4: Testing & Polish (1-2 days)

1. Write integration tests for extension
2. Perform manual end-to-end testing
3. Performance testing with large projects
4. Fix bugs and edge cases
5. Update documentation (README, CHANGELOG)

### Phase 5: Documentation & Release (0.5 day)

1. Update user-facing documentation
2. Add release notes
3. Create demo screenshots/GIFs
4. Update CONTRIBUTING.md if needed

**Total Estimated Time**: 5-8 days

## Configuration

### Extension Settings (package.json)

```json
{
  "contributes": {
    "configuration": {
      "properties": {
        "intentlens.restrictionDisplay.enabled": {
          "type": "boolean",
          "default": true,
          "description": "Enable unified restriction display (disable for legacy individual notifications)"
        },
        "intentlens.restrictionDisplay.autoExpandDetails": {
          "type": "boolean",
          "default": false,
          "description": "Automatically open detail panel when restrictions are found"
        },
        "intentlens.restrictionDisplay.maxIndividualFallback": {
          "type": "number",
          "default": 5,
          "description": "Maximum number of individual notifications to show in fallback mode"
        }
      }
    },
    "views": {
      "explorer": [
        {
          "id": "intentlensRestrictions",
          "name": "IntentLens Restrictions",
          "when": "workspaceFolderCount > 0"
        }
      ]
    },
    "commands": [
      {
        "command": "intentlens.openRestrictionLocation",
        "title": "IntentLens: Open Restriction Location"
      },
      {
        "command": "intentlens.refreshRestrictions",
        "title": "IntentLens: Refresh Restrictions",
        "icon": "$(refresh)"
      },
      {
        "command": "intentlens.clearRestrictions",
        "title": "IntentLens: Clear Restrictions",
        "icon": "$(clear-all)"
      }
    ],
    "menus": {
      "view/title": [
        {
          "command": "intentlens.refreshRestrictions",
          "when": "view == intentlensRestrictions",
          "group": "navigation"
        },
        {
          "command": "intentlens.clearRestrictions",
          "when": "view == intentlensRestrictions",
          "group": "navigation"
        }
      ]
    }
  }
}
```

### Aggregator Configuration (Optional Future Enhancement)

```json
// .intentlens/aggregator-config.json
{
  "classificationRules": [
    {
      "pattern": "parse failed",
      "category": "Parse Errors",
      "severity": "error",
      "icon": "error"
    },
    {
      "pattern": "type check:",
      "category": "Type Check Errors",
      "severity": "error",
      "icon": "error",
      "subcategories": [
        {
          "pattern": "missing imports:",
          "category": "Missing Imports"
        },
        {
          "pattern": "undefined:|not declared",
          "category": "Undefined Identifiers"
        }
      ]
    },
    {
      "pattern": "ambiguous",
      "category": "Ambiguous Calls",
      "severity": "warning",
      "icon": "warning"
    }
  ],
  "summarizationRules": {
    "missingImports": {
      "enabled": true,
      "sampleSize": 3,
      "messageFormat": "{count} missing imports (e.g., {sample})"
    }
  }
}
```

## Dependencies

### Go Dependencies

- Standard library only (no new dependencies)
- Existing: `encoding/json`, `regexp`, `time`, `sort`, `strings`

### Frontend Dependencies

- VS Code API (existing)
- No new npm packages required

### Development Dependencies

- Go testing: `gopter` for property-based testing
- Go linting: `golangci-lint` (existing)
- TypeScript/JavaScript testing: existing test framework

## Performance Characteristics

### Aggregator Performance

- **Time Complexity**: O(n log n) where n = number of restrictions
  - O(n) for parsing and classification
  - O(n log n) for sorting categories
  - O(n) for summarization
- **Space Complexity**: O(n)
  - Stores all restrictions in memory
  - Additional O(k) for k categories (typically k << n)

### Expected Performance

- 100 restrictions: <10ms
- 1000 restrictions: <100ms
- 10000 restrictions: <1000ms (not typical for IntentLens use case)

### Optimization Strategies

1. **Early termination**: Stop processing if timeout approached
2. **Lazy evaluation**: Don't generate full item lists until category expanded
3. **Caching**: Cache aggregated results per analysis session
4. **Streaming**: For >1000 restrictions, consider streaming aggregation

## Security Considerations

1. **Input Validation**: Validate all restriction strings before processing (prevent injection)
2. **Path Sanitization**: Sanitize file paths before opening in editor (prevent path traversal)
3. **JSON Safety**: Use safe JSON parsing (prevent prototype pollution)
4. **Resource Limits**: Enforce memory limits for large restriction sets
5. **Telemetry Privacy**: Don't log sensitive file paths or code content in telemetry

## Backward Compatibility

1. **CLI Output**: Include both `aggregated` (new) and `issues` (legacy) fields in JSON
2. **Extension Detection**: Check for `aggregated` field; if absent, use legacy path
3. **Configuration**: Allow disabling unified display via setting
4. **Existing Commands**: All existing IntentLens commands remain unchanged
5. **Data Migration**: No migration needed (stateless feature)

## Future Enhancements

1. **Auto-fix Suggestions**: Implement `showFixSuggestions()` with actionable fixes
2. **Configurable Rules**: Allow user-defined classification patterns
3. **Export Reports**: Export restrictions to CSV/JSON file
4. **Restriction History**: Track restriction changes across analysis runs
5. **Quick Fixes**: Inline code actions for common restriction types (missing imports)
6. **Batch Operations**: "Fix All" for similar restrictions
7. **Custom Categories**: Allow users to define custom restriction categories
8. **Integration with Problem Panel**: Sync with VS Code's built-in Problems panel

## Open Questions

1. Should we use VS Code's built-in Problems panel instead of custom tree view?
   - **Decision**: Start with custom tree view for richer UI control; consider Problems panel integration in future
2. Should aggregation happen in Go CLI or VS Code extension?
   - **Decision**: Go CLI (better performance, language strengths)
3. Should we persist acknowledged restrictions across sessions?
   - **Decision**: No persistence in v1; consider workspace state in future
4. What icon library to use for category indicators?
   - **Decision**: VS Code ThemeIcons (consistent with IDE)

## Success Metrics

1. **User Experience**:
   - Reduction in notification count: Target 90%+ reduction for multi-issue projects
   - Time to comprehend issues: Target 50%+ faster than individual notifications
   - User satisfaction: Survey users after 1 month

2. **Performance**:
   - Aggregation time: <100ms for 1000 restrictions
   - UI rendering time: <50ms for aggregated view
   - No UI thread blocking

3. **Adoption**:
   - Feature usage rate: Track % of users with unified display enabled
   - Fallback rate: Track how often fallback path is used (target <5%)

4. **Quality**:
   - Bug rate: <1 bug per 1000 restriction aggregations
   - Test coverage: >80% for aggregator, >70% for frontend

