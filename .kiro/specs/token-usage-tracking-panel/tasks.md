# Implementation Plan: Token Usage Tracking Panel

## Overview

This implementation plan converts the Token Usage Tracking Panel design into a series of discrete coding tasks. The panel will track API token consumption, calculate costs, monitor budgets, and provide data export and reset capabilities. Data persistence uses VS Code Global State for cross-session storage.

## Tasks

- [~] 1. Set up TokenUsagePanel module structure and core class
  - Create `extension/src/tokenUsagePanel.js` with TokenUsagePanel class skeleton
  - Implement `static show(context)` method to create or reveal panel
  - Implement panel lifecycle methods: `constructor`, `dispose`, `_startAutoRefresh()`, `_stopAutoRefresh()`
  - Register command `intentlens.viewTokenUsage` in extension.js
  - Verify panel can be opened, closed, and auto-refresh timer works
  - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

- [~] 2. Implement HTML rendering and Impeccable design styles
  - Create `_getHtmlContent()` method to generate panel HTML structure
  - Implement `_getStyles()` with Impeccable design colors and responsive layout
  - Create UI sections: Header, Session Statistics, Cumulative Statistics, Cost Summary, Budget Tracker, Action Buttons
  - Use large fonts for metric displays (session tokens, costs)
  - Use color transitions in progress bar: Green (0-50%), Yellow (50-80%), Orange (80-100%), Red (>100%)
  - Ensure responsive layout and proper spacing
  - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5, 10.6, 10.9_

- [~] 3. Implement WebView message protocol
  - Create `_handleMessage()` to process WebView messages
  - Implement message handlers: `updateBudget`, `exportData`, `resetStatistics`, `manualRefresh`
  - Create message types: `loadData`, `refreshComplete`, `error`
  - Test message passing between extension and WebView
  - _Requirements: 8.1, 8.8_

- [~] 4. Implement Global State data loading and persistence
  - Create data structure for token usage in Global State
  - Implement `_loadData()` to read from Global State
  - Implement `_saveData()` to persist to Global State
  - Handle initial load with default values (all zeros)
  - Support data retention across multiple sessions
  - _Requirements: 12.1, 12.2, 12.3, 12.5, 12.6_

- [~] 5. Implement Session Token Statistics display
  - Create session tracking for Input Tokens, Output Tokens, Total Tokens
  - Initialize session stats to zero on extension start
  - Display session stats in large, easy-to-read format
  - Ensure Total = Input + Output
  - Update session stats when new API interactions occur
  - Session stats should NOT reset until VS Code is closed/reopened
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9_

- [~] 6. Implement token tracking integration with existing API calls
  - Hook into CLI responses to extract token usage information
  - Accumulate tokens into session and cumulative stats
  - Update Global State with new token counts
  - Test that tokens are accurately tracked across multiple API calls
  - _Requirements: 1.6, 1.7, 2.2_

- [~] 7. Implement Cumulative Time-Based Statistics
  - Implement "Today" calculation (tokens from 00:00:00 UTC to current time)
  - Implement "This Week" calculation (Monday 00:00:00 to current time)
  - Implement "This Month" calculation (1st of month 00:00:00 to current time)
  - Implement "All Time" calculation (sum of all tokens ever)
  - Store daily breakdown in Global State for detailed tracking
  - Display cumulative stats with proper formatting (comma separators)
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8_

- [~] 8. Implement Cost Calculation and Display
  - Create pricing data structure (input_price per 1M, output_price per 1M)
  - Implement cost calculation formula: (input × input_price + output × output_price) / 1,000,000
  - Display pricing information: "Input: $X.XX / 1M tokens", "Output: $Y.YY / 1M tokens"
  - Calculate and display Today's cost
  - Calculate and display This Month's cost
  - Handle case when pricing data unavailable: show "Pricing data unavailable"
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8, 3.9_

- [~] 9. Implement Monthly Budget Tracking and Progress Bar
  - Display configurable "Monthly Budget" amount
  - Calculate and display "Spent This Month"
  - Calculate and display "Remaining" budget
  - Implement progress bar visualization showing budget usage percentage
  - Color progress bar based on usage: Green (0-50%), Yellow (50-80%), Orange (80-100%), Red (>100%)
  - Display percentage text on progress bar
  - Display warning when budget exceeded: "⚠ Budget exceeded by $XX.XX"
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 4.8, 4.9, 4.10, 4.13_

- [~] 10. Implement editable Monthly Budget field
  - Make budget amount clickable/editable
  - Implement inline budget editor
  - Validate budget input (positive number, reasonable bounds)
  - Save updated budget to Global State
  - Persist budget across sessions
  - Ensure budget changes trigger recalculation of all dependent displays
  - _Requirements: 4.11, 4.12, 4.13_

- [~] 11. Implement Auto-Refresh mechanism (5-minute interval)
  - Create auto-refresh timer that triggers every 5 minutes
  - Implement panel visibility tracking (pause refresh when panel hidden)
  - Resume refresh immediately when panel regains focus
  - Fetch fresh token data from CLI during refresh
  - Update all displayed statistics with new data
  - Handle refresh failures gracefully (show warning, retain cached data)
  - Display last refresh timestamp
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 5.8, 5.9_

- [~] 12. Implement Manual Refresh button
  - Add "Refresh" button to UI
  - Show loading state with spinner during refresh
  - Disable button while refresh is in progress
  - Handle refresh failures with error message display
  - Re-enable button after refresh completes
  - _Requirements: 5.6, 5.7_

- [~] 13. Implement Data Export functionality (CSV)
  - Create "Export Data" button with dropdown options
  - Implement CSV export generation with columns: Date, Time Period, Input Tokens, Output Tokens, Total Tokens, Cost USD
  - Include all cumulative statistics in export
  - Generate filename with date: "intentlens-token-usage-YYYY-MM-DD.csv"
  - Trigger VS Code file save dialog
  - Test CSV file can be opened and parsed correctly
  - _Requirements: 6.1, 6.2, 6.3, 6.5, 6.7, 6.8_

- [~] 14. Implement Data Export functionality (JSON)
  - Implement JSON export generation with hierarchical structure
  - Include session, cumulative, cost, and budget data
  - Generate filename with date: "intentlens-token-usage-YYYY-MM-DD.json"
  - Trigger VS Code file save dialog
  - Test JSON file can be parsed and contains expected structure
  - _Requirements: 6.1, 6.2, 6.4, 6.5, 6.7_

- [~] 15. Implement Statistics Reset with confirmation dialog
  - Add "Reset Statistics" button (in advanced/less prominent section)
  - Show confirmation dialog: "Are you sure you want to reset all token usage statistics?"
  - Provide "Cancel" and "Reset" buttons in dialog
  - Clear all token usage data from Global State on confirmation
  - Reset all displayed metrics to zero
  - Preserve budget and configuration settings
  - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7, 7.8, 7.9_

- [~] 16. Implement Multi-Workspace Synchronization
  - Ensure token data in Global State is shared across all VS Code instances
  - Test that token updates in one instance appear in others
  - Session stats remain isolated per instance
  - Verify cumulative stats are consistent across instances
  - Handle graceful degradation if sync unavailable
  - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6_

- [~] 17. Implement error handling and user feedback
  - Display error for CLI unavailable: "✗ Failed to load token usage data (CLI unavailable)"
  - Display error for corrupted data: "⚠ Could not load historical data (corrupted)"
  - Display error for failed export: "✗ Export failed: <reason>"
  - Display error for failed refresh: "✗ Failed to refresh data"
  - Add Retry button for failed operations
  - Use consistent error color (#F14C4C) and warning color (#F5A623)
  - _Requirements: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7_

- [~] 18. Implement data validation and corruption recovery
  - Validate all data loaded from Global State
  - Detect and handle corrupted data gracefully
  - Reset to zero values if data is invalid
  - Log corruption events for debugging
  - _Requirements: 12.4, 12.6_

- [~] 19. Update extension.js with panel registration and token tracking
  - Import TokenUsagePanel class in extension.js
  - Register command `intentlens.viewTokenUsage` with proper handler
  - Integrate token tracking with existing CLI calls (explainSelection, etc.)
  - Verify command appears in command palette
  - Test command execution opens panel correctly
  - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

- [~] 20. Implement CLI integration for token statistics retrieval
  - Create CLI command invocation: `intentlens get-token-stats`
  - Pass period parameter (today, week, month, all)
  - Parse CLI response for token counts and costs
  - Handle CLI timeout after 10 seconds
  - Fall back to cached data if CLI call fails
  - _Requirements: 2.8, 3.8_

- [~] 21. Implement accessibility features
  - Add proper labels to all data displays
  - Ensure progress bar has accessible percentage text
  - Verify color indicators supplemented with text
  - Ensure focus states are clearly visible
  - Verify WCAG AA contrast ratios for all text
  - Test with screen reader for accessibility
  - _Requirements: 10.7, 10.8_

- [~] 22. Checkpoint - Ensure panel functionality and tests pass
  - Run all property tests for cumulative invariants, cost accuracy, budget percentage, data persistence
  - Verify no regressions in existing extension functionality
  - Test panel opening, closing, auto-refresh
  - Test budget editing and persistence
  - Test export functionality (CSV and JSON)
  - Test statistics reset with confirmation
  - Ask user if questions arise

- [~] 23. Create integration test for end-to-end workflow
  - Test: Open panel → View token statistics → Verify numbers display
  - Test: Edit budget → Save → Verify persistence after reload
  - Test: Export data → Verify CSV/JSON files generated correctly
  - Test: Reset statistics → Confirm → Verify stats reset to zero
  - Test: Multi-session persistence → Close/reopen VS Code → Verify cumulative stats retained
  - _Requirements: 1.1, 2.1, 3.1, 4.1, 5.1, 6.1, 7.1, 8.1, 12.1_

- [~] 24. Documentation and cleanup
  - Add JSDoc comments to TokenUsagePanel class and all methods
  - Document message protocol in code comments
  - Document Global State data structure in code
  - Create USAGE.md explaining token tracking features
  - Clean up any temporary test files or debugging code
  - _Requirements: All_

## Notes

- Tasks marked with `*` are optional (if they had an asterisk) and can be skipped for MVP
- The implementation uses VS Code Global State for reliable cross-session persistence
- Property tests should verify: cumulative invariants, cost calculations, budget percentages, and data persistence
- Auto-refresh is critical for real-time monitoring but must not hang UI when panel hidden
- Export functionality should support both CSV (simple) and JSON (detailed) formats
- All error messages follow Impeccable color coding standards

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "2.1", "4.1"] },
    { "id": 1, "tasks": ["3.1", "5.1", "7.1"] },
    { "id": 2, "tasks": ["6.1", "8.1", "9.1", "10.1"] },
    { "id": 3, "tasks": ["11.1", "12.1", "13.1", "14.1"] },
    { "id": 4, "tasks": ["15.1", "16.1", "17.1", "18.1"] },
    { "id": 5, "tasks": ["19.1", "20.1", "21.1", "23.1"] }
  ]
}
```
