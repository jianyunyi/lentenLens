# Requirements Document

## Introduction

IntentLens integrates with AI APIs to provide code analysis, which consumes API tokens and incurs costs. Users need visibility into their token consumption, associated costs, and remaining budget to manage API usage effectively and avoid unexpected expenses. This feature introduces a dedicated Token Usage Tracking Panel (TokenUsagePanel) - a WebView panel that displays session and cumulative token statistics, cost calculations, budget tracking, and provides export and reset capabilities for token usage data.

## Glossary

- **IntentLens**: The VS Code extension system comprising a Go CLI backend and Node.js extension frontend
- **API_Token**: A unit of text processed by the AI API (either input or output)
- **Input_Token**: A token consumed by processing user's request content
- **Output_Token**: A token consumed by the AI model generating a response
- **Token_Usage_Data**: The record of tokens consumed during API interactions
- **Session**: A continuous period of VS Code editor activity without closing the extension
- **Cumulative_Statistics**: Token usage aggregated across multiple time periods (today, week, month, all-time)
- **Cost_Calculation**: Monetary value computed from token count and per-token pricing rates
- **Monthly_Budget**: User-configured maximum spending limit for a calendar month
- **Budget_Percentage**: The ratio of tokens spent to the monthly budget, expressed as a percentage
- **TokenUsagePanel**: The WebView panel UI for displaying token consumption and budget tracking
- **CLI**: The Go command-line tool (intentlens binary) that provides token usage data
- **Global_State**: VS Code's extension global state storage for persistent cross-session data
- **Auto_Refresh**: Automatic update of displayed data at regular intervals (every 5 minutes)
- **Export_Format**: File format for exporting token usage data (CSV or JSON)
- **Impeccable_Design**: The UI design system used throughout IntentLens featuring consistent colors, icons, and interactions

## Requirements

### Requirement 1: Session Token Statistics

**User Story:** As a user, I want to see my token consumption during the current session, so that I can monitor API usage in real-time.

#### Acceptance Criteria

1. THE TokenUsagePanel SHALL display a "Session Statistics" section at the top
2. THE Panel SHALL show three metrics: Input Tokens, Output Tokens, and Total Tokens
3. EACH metric SHALL display the current session count in large, easy-to-read font
4. THE Panel SHALL update session statistics immediately when new API interactions occur
5. WHEN the editor is first opened, THE session statistics SHALL show: Input: 0, Output: 0, Total: 0
6. THE Input_Token count SHALL increase when IntentLens sends requests to the API
7. THE Output_Token count SHALL increase when IntentLens receives responses from the API
8. THE Total_Tokens count SHALL equal Input_Tokens plus Output_Tokens
9. THE session statistics SHALL NOT reset until the user closes and reopens VS Code

### Requirement 2: Cumulative Time-Based Statistics

**User Story:** As a user, I want to see token consumption aggregated by time period, so that I can understand my usage patterns across different timeframes.

#### Acceptance Criteria

1. THE TokenUsagePanel SHALL display a "Cumulative Statistics" section below Session Statistics
2. THE Cumulative_Statistics section SHALL show the following time periods: Today, This Week, This Month, All Time
3. FOR each time period, THE Panel SHALL display: Total_Tokens consumed during that period
4. THE "Today" statistic SHALL include all tokens from 00:00:00 to current time (UTC-based or user timezone)
5. THE "This Week" statistic SHALL include all tokens from Monday 00:00:00 to current time
6. THE "This Month" statistic SHALL include all tokens from the 1st of the month 00:00:00 to current time
7. THE "All Time" statistic SHALL include all tokens ever consumed by this workspace
8. THE statistics data SHALL be retrieved from the CLI and stored in Global_State for persistence
9. WHEN configuration changes (e.g., API provider switch), THE statistics SHALL retain historical data

### Requirement 3: Cost Calculation Display

**User Story:** As a user, I want to see the monetary cost of my token consumption, so that I can understand the financial impact of my usage.

#### Acceptance Criteria

1. THE TokenUsagePanel SHALL display a "Cost Summary" section
2. WITHIN Cost_Summary, THE Panel SHALL show per-token pricing for the current API Provider
3. THE Panel SHALL display current pricing as: "Input: $X.XX / 1M tokens", "Output: $Y.YY / 1M tokens"
4. THE Panel SHALL calculate and display cost for Today: $Z.ZZ
5. THE Panel SHALL calculate and display cost for This Month: $A.AA
6. THE cost calculation formula SHALL be: (Input_Tokens * Input_Price + Output_Tokens * Output_Price) / 1,000,000
7. THE pricing data SHALL be fetched from the API Provider or stored as configurable constants
8. THE Panel SHALL update costs whenever tokens statistics are updated
9. IF pricing data is unavailable, THE Panel SHALL display "Pricing data unavailable" and skip cost calculations

### Requirement 4: Monthly Budget Tracking

**User Story:** As a user, I want to set and monitor a monthly budget for API usage, so that I can control my spending.

#### Acceptance Criteria

1. THE TokenUsagePanel SHALL display a "Budget and Limits" section
2. WITHIN Budget_and_Limits, THE Panel SHALL show: "Monthly Budget: $XXX.XX"
3. THE Panel SHALL display the configured monthly budget amount (editable)
4. THE Panel SHALL calculate and display: "Spent This Month: $YY.YY"
5. THE Panel SHALL calculate and display: "Remaining: $ZZ.ZZ"
6. THE Panel SHALL display a progress bar showing budget usage percentage
7. WHEN budget used is 0-50%, THE progress bar SHALL display in green color
8. WHEN budget used is 50-80%, THE progress bar SHALL display in yellow color
9. WHEN budget used is 80-100%, THE progress bar SHALL display in orange color
10. WHEN budget used exceeds 100%, THE progress bar SHALL display in red color and show percentage > 100%
11. WHEN the user clicks on the budget amount, THE Panel SHALL allow inline editing
12. WHEN the user updates the budget and saves, THE VS_Code_Extension SHALL persist the new budget value in Global_State
13. IF monthly budget is exceeded, THE Panel SHALL display a warning: "⚠ Budget exceeded by $XX.XX"

### Requirement 5: Auto-Refresh Capability

**User Story:** As a user, I want the token usage data to update automatically, so that I can monitor usage without manually refreshing.

#### Acceptance Criteria

1. THE TokenUsagePanel SHALL automatically refresh all statistics every 5 minutes
2. WHEN the panel is in focus, THE auto-refresh interval SHALL be 5 minutes
3. WHEN the panel is not in focus (hidden), THE auto-refresh SHALL pause
4. WHEN the panel regains focus, THE auto-refresh SHALL resume and fetch fresh data immediately
5. THE Panel SHALL display the timestamp of the last refresh at the bottom: "Last updated: HH:MM:SS"
6. THE user SHALL be able to manually refresh by clicking a "Refresh" button
7. WHILE a refresh is in progress, THE refresh button SHALL show a loading state with spinner
8. IF auto-refresh fails, THE Panel SHALL display a warning icon and "Failed to refresh data"
9. THE auto-refresh SHALL NOT interfere with user interactions (e.g., editing budget)

### Requirement 6: Data Export Functionality

**User Story:** As a user, I want to export my token usage data, so that I can analyze it with external tools or for record-keeping.

#### Acceptance Criteria

1. THE TokenUsagePanel SHALL provide an "Export Data" button
2. WHEN the user clicks "Export Data", THE Panel SHALL display a dropdown with export format options: "CSV" and "JSON"
3. WHEN the user selects "CSV", THE VS_Code_Extension SHALL generate a CSV file with the following columns: Date, Time Period, Input_Tokens, Output_Tokens, Total_Tokens, Cost_USD
4. WHEN the user selects "JSON", THE VS_Code_Extension SHALL generate a JSON file containing all token usage data structured hierarchically
5. THE export file name SHALL include the export date: "intentlens-token-usage-YYYY-MM-DD.csv" or ".json"
6. AFTER generating the export file, THE VS_Code_Extension SHALL trigger a "Save File" dialog
7. THE exported data SHALL include all cumulative statistics and session data up to the export time
8. IF there is no data to export, THE Export button SHALL be disabled

### Requirement 7: Statistics Reset and Cleanup

**User Story:** As a user (administrator), I want to reset token usage statistics when needed, so that I can start fresh tracking for a new billing period.

#### Acceptance Criteria

1. THE TokenUsagePanel SHALL provide a "Reset Statistics" button
2. THE Reset_Statistics button SHALL be placed in an advanced section or menu (not prominent)
3. WHEN the user clicks "Reset Statistics", THE Panel SHALL display a confirmation dialog: "Are you sure you want to reset all token usage statistics?"
4. THE confirmation dialog SHALL provide "Cancel" and "Reset" buttons
5. WHEN the user confirms reset, THE VS_Code_Extension SHALL clear all token usage data from Global_State
6. AFTER reset, THE Panel SHALL display all metrics as zero
7. THE reset SHALL only affect statistics, not configuration or budget settings
8. THE reset action SHALL be logged for audit purposes (if audit logging is available)
9. AFTER reset, THE cumulative statistics for Today, Week, Month, and All-Time SHALL be recalculated from zero

### Requirement 8: Panel Access and Commands

**User Story:** As a user, I want to open the Token Usage Panel using a command, so that I can access it easily from the command palette.

#### Acceptance Criteria

1. THE VS_Code_Extension SHALL register a command: `intentlens.viewTokenUsage`
2. WHEN the user runs `intentlens.viewTokenUsage` from the command palette, THE TokenUsagePanel SHALL open in a dedicated column
3. THE command SHALL be available in the VS Code Command Palette with title: "IntentLens: View Token Usage"
4. IF the panel is already open, THE panel SHALL be revealed (brought to foreground)
5. IF a panel is already open for another workspace, THE new panel SHALL be created for the active workspace
6. THE Panel title bar SHALL display "IntentLens: Token Usage"

### Requirement 9: Multi-Workspace Synchronization

**User Story:** As a user with multiple VS Code instances or workspaces, I want token usage data to be shared and synchronized, so that I see consistent totals across my work environment.

#### Acceptance Criteria

1. THE token usage data SHALL be stored in Global_State to be shared across multiple VS Code instances
2. WHEN token usage changes in one VS Code instance, THE changes SHALL be persisted to Global_State
3. WHEN a different VS Code instance running IntentLens performs an API call, THE statistics in all instances SHALL be updated
4. THE TokenUsagePanel in each instance SHALL reflect the same cumulative statistics (Today, Week, Month, All-Time)
5. THE Session_Statistics SHALL remain isolated per VS Code instance
6. IF multiple instances cannot access shared storage, THE sync system SHALL gracefully degrade and use local-only statistics

### Requirement 10: UI Design and Impeccable Consistency

**User Story:** As a user, I want the Token Usage Panel to follow IntentLens design standards, so that it feels consistent with the rest of the extension.

#### Acceptance Criteria

1. THE TokenUsagePanel SHALL use VS Code color scheme variables for all UI elements
2. THE Panel SHALL use Impeccable design system colors: ✓ green (#58B368), ⚠ yellow (#F5A623), ✗ red (#F14C4C)
3. THE Panel layout SHALL be responsive and readable with proper spacing and typography
4. THE Panel progress bar SHALL use color transitions: Green → Yellow → Orange → Red based on budget usage
5. THE metric displays SHALL use large, readable fonts for the primary numbers
6. THE Panel buttons SHALL have hover and active states with smooth transitions
7. THE Panel input fields SHALL have focus states with visible outlines
8. ALL text SHALL be centered and readable with appropriate contrast ratios (WCAG AA minimum)
9. THE Panel SHALL display icons for each section: 💰 for Cost, 📊 for Statistics, 🎯 for Budget, 📥 for Export

### Requirement 11: Error Handling and User Feedback

**User Story:** As a user, I want clear error messages when data retrieval fails, so that I understand what went wrong and can take action.

#### Acceptance Criteria

1. IF token usage data retrieval fails, THE Panel SHALL display: "✗ Failed to load token usage data"
2. THE error message SHALL include a reason, e.g., "CLI unavailable", "Configuration missing"
3. THE error message SHALL provide a "Retry" button to attempt data retrieval again
4. IF the API call to fetch pricing fails, THE Panel SHALL display costs as "Unavailable"
5. IF export fails, THE Panel SHALL display: "✗ Export failed: <error reason>"
6. ALL error messages SHALL use consistent red color (#F14C4C) and error icon
7. THE Panel SHALL display warnings in yellow (#F5A623) for non-critical issues

### Requirement 12: Data Persistence and Initial Load

**User Story:** As a user, I want my token usage data to persist between sessions, so that I don't lose historical tracking information.

#### Acceptance Criteria

1. WHEN the TokenUsagePanel is first opened, THE Panel SHALL load historical data from Global_State
2. IF no historical data exists, THE Panel SHALL display all metrics as zero
3. WHEN the panel closes and reopens, THE statistics SHALL remain the same (persisted)
4. THE Global_State storage SHALL use encryption if available through VS Code API
5. WHEN the workspace or extension is updated, THE historical data SHALL be preserved
6. THE Panel SHALL gracefully handle corrupted or incomplete data by resetting to zero
