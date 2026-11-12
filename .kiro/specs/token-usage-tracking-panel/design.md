# Design Document: Token Usage Tracking Panel

## Overview

The Token Usage Tracking Panel (TokenUsagePanel) is a WebView panel that provides users with comprehensive visibility into their API token consumption, associated costs, and budget tracking. It features automatic refresh capabilities, data export functionality, and multi-workspace synchronization to ensure consistent usage tracking across the user's development environment.

**Implementation Language**: JavaScript (Node.js runtime within VS Code)

**Architecture**: The panel uses the Webview API combined with VS Code Global State for persistent cross-session and cross-workspace data storage.

## Architecture

### Component Structure

```
Extension (extension.js)
    ├── Command Handler (intentlens.viewTokenUsage)
    ├── Global State Manager (token usage persistence)
    ├── Auto-Refresh Timer (5-minute interval)
    ├── CLI Wrapper (fetch token statistics)
    ├── Export Manager (CSV/JSON generation)
    └── TokenUsagePanel (WebView)
        ├── Session Statistics Display
        ├── Cumulative Statistics Display
        ├── Cost Summary Display
        ├── Budget Tracker (editable)
        ├── Progress Bar (budget visualization)
        ├── Export Data Button
        ├── Reset Statistics Button (advanced)
        ├── Auto-Refresh Status
        └── Manual Refresh Button
```

### Data Flow

1. **Panel Open**: User runs `intentlens.viewTokenUsage`
2. **Load Cached Data**: Panel reads token statistics from Global State
3. **Display Current Data**: Panel renders session, cumulative, cost, and budget info
4. **5-Minute Auto-Refresh**: Extension fetches fresh data from CLI
5. **Update UI**: Panel re-renders with new statistics
6. **User Actions**:
   - Edit budget → saved to Global State
   - Export data → trigger file download
   - Reset stats → clear Global State
   - Manual refresh → fetch and re-render immediately

### Message Protocol

**Extension → WebView**:
```javascript
{
  command: 'loadData',
  session: { inputTokens: 1000, outputTokens: 500, totalTokens: 1500 },
  cumulative: {
    today: 5000,
    week: 25000,
    month: 100000,
    allTime: 500000
  },
  costs: {
    inputPrice: 0.03,
    outputPrice: 0.06,
    todayCost: 3.50,
    monthCost: 25.00
  },
  budget: {
    monthlyBudget: 100.00,
    spent: 25.00,
    remaining: 75.00,
    percentage: 25
  },
  lastRefresh: '2024-01-15T10:30:00Z'
}

{
  command: 'refreshComplete',
  data: { /* same structure as loadData */ }
}
```

**WebView → Extension**:
```javascript
{
  command: 'updateBudget',
  monthlyBudget: 150.00
}

{
  command: 'exportData',
  format: 'csv' // or 'json'
}

{
  command: 'resetStatistics'
}

{
  command: 'manualRefresh'
}
```

## Components and Interfaces

### 1. TokenUsagePanel Class

**Responsibility**: Manage the WebView panel lifecycle, data updates, and user interactions

**Key Methods**:
- `constructor(panel, context)`: Initialize panel with auto-refresh timer
- `static show(context)`: Create or reveal the panel
- `_loadData()`: Fetch data from Global State and CLI
- `_getHtmlContent()`: Generate HTML with current statistics
- `_getStyles()`: Return CSS for Impeccable design
- `_handleMessage(message)`: Process WebView messages
- `_startAutoRefresh()`: Initialize 5-minute auto-refresh
- `_stopAutoRefresh()`: Cancel auto-refresh when panel hidden
- `_exportData(format)`: Generate CSV/JSON export
- `dispose()`: Clean up resources

**Properties**:
- `_panel`: VSCode WebViewPanel
- `_context`: ExtensionContext for global state
- `_refreshTimer`: Auto-refresh interval ID
- `_currentData`: Current displayed statistics
- `_isVisible`: Track panel visibility

### 2. Data Storage and Retrieval

**Global State Structure**:
```javascript
{
  'intentlens.tokenUsage': {
    session: {
      inputTokens: number,
      outputTokens: number,
      totalTokens: number,
      startTime: ISO8601String
    },
    cumulative: {
      today: number,
      week: number,
      month: number,
      allTime: number,
      daily: { // For detailed tracking
        [YYYY-MM-DD]: number,
        ...
      }
    },
    costs: {
      today: number,
      month: number
    },
    budget: {
      monthlyBudget: number,
      lastReset: ISO8601String
    },
    lastUpdate: ISO8601String
  }
}
```

**Access Methods**:
```javascript
// Load data
const data = await context.globalState.get('intentlens.tokenUsage', {});

// Update data
await context.globalState.update('intentlens.tokenUsage', updatedData);

// Track new usage
const addTokens = (input, output) => {
  const data = await context.globalState.get('intentlens.tokenUsage', {});
  data.session.inputTokens += input;
  data.session.outputTokens += output;
  data.session.totalTokens += (input + output);
  // ... update cumulative
  await context.globalState.update('intentlens.tokenUsage', data);
};
```

### 3. CLI Integration for Token Statistics

**Command**: `intentlens get-token-stats`

**Input** (JSON via stdin):
```json
{
  "period": "all" // or 'today', 'week', 'month'
}
```

**Output** (JSON via stdout):
```json
{
  "inputTokens": 1000,
  "outputTokens": 500,
  "totalTokens": 1500,
  "estimatedCost": 3.50,
  "period": "today"
}
```

### 4. UI Component Hierarchy

```
Container (max-width: 700px, padding: 24px)
  ├── Header
  │   ├── Title: "Token Usage"
  │   ├── Last Updated: "Updated 5 minutes ago"
  │   └── Refresh Button (with spinner during refresh)
  │
  ├── Session Statistics (📊)
  │   ├── "Session Statistics"
  │   ├── Input Tokens: 1,000 (large font)
  │   ├── Output Tokens: 500 (large font)
  │   └── Total Tokens: 1,500 (large font, highlight)
  │
  ├── Cumulative Statistics (📈)
  │   ├── "Cumulative Statistics"
  │   ├── Today: 5,000 tokens
  │   ├── This Week: 25,000 tokens
  │   ├── This Month: 100,000 tokens
  │   └── All Time: 500,000 tokens
  │
  ├── Cost Summary (💰)
  │   ├── "Cost Summary"
  │   ├── Pricing: "Input: $0.03 / 1M, Output: $0.06 / 1M"
  │   ├── Today's Cost: $3.50
  │   └── Month's Cost: $25.00
  │
  ├── Budget & Limits (🎯)
  │   ├── "Budget and Limits"
  │   ├── Monthly Budget: $100.00 (editable)
  │   ├── Spent: $25.00
  │   ├── Remaining: $75.00
  │   ├── Progress Bar (25% filled, green)
  │   └── "75% of budget remaining"
  │
  ├── Action Buttons
  │   ├── Export Data (secondary)
  │   ├── Reset Statistics (secondary, less prominent)
  │   └── Refresh (secondary, always visible)
  │
  └── Footer
      └── "Last updated: 2024-01-15 10:30:00"
```

## Data Models

### Token Usage State

```javascript
{
  // Session data (reset when extension reloads)
  session: {
    inputTokens: 1000,
    outputTokens: 500,
    totalTokens: 1500,
    startTime: '2024-01-15T08:00:00Z'
  },
  
  // Persistent cumulative data
  cumulative: {
    today: 5000,
    week: 25000,
    month: 100000,
    allTime: 500000,
    daily: {
      '2024-01-15': 5000,
      '2024-01-14': 3000,
      // ... historical daily data
    }
  },
  
  // Cost calculations
  costs: {
    inputPrice: 0.03,
    outputPrice: 0.06,
    todayCost: 3.50,
    monthCost: 25.00
  },
  
  // Budget tracking
  budget: {
    monthlyBudget: 100.00,
    spent: 25.00,
    remaining: 75.00,
    percentage: 25,
    budgetExceeded: false
  },
  
  // Metadata
  lastUpdate: '2024-01-15T10:30:00Z',
  lastSessionStart: '2024-01-15T08:00:00Z'
}
```

### Cumulative Calculation Logic

```javascript
// Dates are in UTC or user timezone (configurable)
const calculateCumulativeStats = (dailyData) => {
  const today = new Date().toISOString().split('T')[0];
  const weekStart = getWeekStart(); // Monday
  const monthStart = new Date().toISOString().substring(0, 7);
  
  return {
    today: sumTokensForDate(dailyData, today),
    week: sumTokensForDateRange(dailyData, weekStart, today),
    month: sumTokensForDateRange(dailyData, monthStart, today),
    allTime: sumAllTokens(dailyData)
  };
};
```

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Cumulative Invariant

**For any** valid token usage history, the cumulative statistics SHALL satisfy: Today ≥ 0, This Week ≥ Today, This Month ≥ This Week, All Time ≥ This Month.

**Validates: Requirements 2.2, 2.3, 2.4, 2.5, 2.6, 2.7**

### Property 2: Cost Calculation Accuracy

**For any** input tokens, output tokens, and pricing rates, the calculated cost SHALL equal (input_tokens × input_price + output_tokens × output_price) / 1,000,000, within floating-point precision.

**Validates: Requirements 3.6, 3.8**

### Property 3: Budget Percentage Accuracy

**For any** monthly budget amount and current month's spending, the budget percentage SHALL equal (spent / budget) × 100, and SHALL never exceed 100% for spending ≤ budget.

**Validates: Requirements 4.6, 4.9, 4.10**

### Property 4: Total Token Consistency

**For all** token statistics, Total_Tokens SHALL equal Input_Tokens + Output_Tokens at all times (session, today, week, month, all-time).

**Validates: Requirements 1.8, 2.2**

### Property 5: Budget Persistence Round-Trip

**For any** monthly budget value entered by the user, saving the budget and then reloading the panel SHALL display the exact same budget value without modification.

**Validates: Requirements 4.12, 4.13**

### Property 6: Auto-Refresh Interval Correctness

**For all** panels that remain visible, the auto-refresh SHALL fetch fresh data at 5-minute intervals, within 10 seconds tolerance. When the panel is hidden, no refresh SHALL occur.

**Validates: Requirements 5.1, 5.2, 5.3, 5.4**

### Property 7: Data Persistence Across Sessions

**For any** token usage data accumulated before VS Code closes, when VS Code reopens and the TokenUsagePanel is displayed, ALL cumulative statistics (Today, Week, Month, All-Time) SHALL match the values from the previous session.

**Validates: Requirements 12.1, 12.2, 12.3**

### Property 8: Export Data Round-Trip

**For any** exported token usage data (CSV or JSON), parsing the exported file SHALL produce records that match the displayed statistics for each time period.

**Validates: Requirements 6.3, 6.7**

## Testing Strategy

### Unit Tests

1. **Cumulative Statistics Calculation**
   - Test that cumulative stats increase monotonically
   - Test edge cases: zero tokens, large numbers, date boundaries
   - Test that daily summation equals cumulative today

2. **Cost Calculation**
   - Test cost = (input × input_price + output × output_price) / 1M
   - Test with various pricing rates and token amounts
   - Test floating-point precision handling

3. **Budget Tracking**
   - Test percentage calculation: (spent / budget) × 100
   - Test color transitions: green (0-50%), yellow (50-80%), orange (80-100%), red (>100%)
   - Test budget persistence and reload

4. **Data Formatting**
   - Test token count formatting with comma separators
   - Test currency formatting with 2 decimal places
   - Test timestamp formatting

### Property-Based Tests

1. **Cumulative Invariant** - Use fast-check to generate token sequences and verify cumulative stats always satisfy: Today ≤ Week ≤ Month ≤ AllTime

2. **Cost Accuracy** - Generate random input/output token pairs and pricing rates, verify cost calculation matches expected formula

3. **Budget Percentage** - Generate random budget and spending amounts, verify percentage calculation and color coding

4. **Data Persistence** - Simulate saves and loads of token usage data, verify round-trip equality

### Integration Tests

1. **CLI Communication**
   - Mock CLI token statistics fetch
   - Verify correct command invocation
   - Test parsing of CLI response

2. **Global State Persistence**
   - Test data storage to VS Code Global State
   - Test data retrieval after panel close/open
   - Test data retention across extension reload

3. **Auto-Refresh Mechanism**
   - Test refresh interval timing
   - Test panel visibility tracking
   - Test refresh inhibition when hidden
   - Test immediate refresh on visibility change

4. **Export Generation**
   - Test CSV export format and content
   - Test JSON export structure and content
   - Test file save dialog triggering

### End-to-End Test Scenarios

1. **Initial Setup**
   - User opens TokenUsagePanel for first time
   - Panel loads and displays all zeros
   - Budget defaults to configured value (100.00)
   - Last update shows current time

2. **Token Accumulation**
   - User performs API calls
   - Session tokens increase immediately
   - Auto-refresh triggers (5 minute interval)
   - Cumulative stats update
   - Costs recalculate

3. **Budget Tracking**
   - User edits monthly budget (e.g., 50.00)
   - Panel updates percentage display
   - Progress bar color changes if crossing thresholds
   - Warning displays if exceeding budget

4. **Data Export**
   - User clicks Export Data
   - User selects CSV format
   - File save dialog opens
   - CSV file generated with correct data
   - User can open and verify content

5. **Multi-Session Persistence**
   - User accumulates tokens in Session 1
   - User closes and reopens VS Code
   - TokenUsagePanel displays same cumulative stats
   - Session stats reset to zero (new session)

6. **Statistics Reset**
   - User clicks Reset Statistics
   - Confirmation dialog appears
   - User confirms reset
   - All statistics display as zero
   - Budget and settings retained

## Error Handling

### Error Scenarios

1. **CLI Unavailable**
   - Display: "✗ Failed to load token usage data (CLI unavailable)"
   - Action: Show cached data if available
   - Recovery: Retry button, or check CLI installation

2. **Data Corruption**
   - Display: "⚠ Could not load historical data (corrupted)"
   - Action: Reset to zero values
   - Recovery: User manually resets or restarts extension

3. **Pricing Data Unavailable**
   - Display: "Pricing data unavailable"
   - Action: Show tokens without cost
   - Recovery: Retry on next refresh

4. **Export Failed**
   - Display: "✗ Export failed: <reason>"
   - Action: Keep panel open, allow retry
   - Recovery: User retries export

5. **Refresh Failed**
   - Display: "✗ Failed to refresh data"
   - Action: Retain last known values
   - Recovery: User manually refreshes or waits for next auto-refresh

### Graceful Degradation

- If CLI is unavailable, use only cached Global State data
- If pricing unavailable, show token counts only
- If export fails, allow user to manually copy data
- If auto-refresh fails, maintain cached data and retry next cycle

## Accessibility

- All metric displays have clear labels
- Color indicators (green/yellow/orange/red) supplemented with text
- Progress bar has accessible percentage text
- Buttons have clear, descriptive labels
- Focus states clearly visible
- WCAG AA contrast ratios maintained

## Performance Considerations

- Cache Global State data in memory to avoid repeated reads
- Debounce budget updates at 500ms
- Limit daily history to 90 days (3 months) to prevent excessive storage
- Lazy-load historical charts/visualizations if added later
- Timeout CLI calls after 10 seconds to prevent UI hang

## Security Considerations

- Token statistics contain no sensitive data (only counts and costs)
- Export files should not include API keys or credentials
- Ensure Global State is not accessible to untrusted code
- Validate all data loaded from Global State for corruption
- Sanitize all user input (budget amount, export filename)

## Multi-Workspace Considerations

- Token statistics in Global State are shared across all workspaces
- Each workspace has its own session statistics (workspace-local)
- Budget setting is shared across all workspaces
- Export works independently per workspace
- Reset affects global cumulative data (with confirmation)
