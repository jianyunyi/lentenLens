# Design Document: API Configuration Panel

## Overview

The API Configuration Panel (AIConfigPanel) is a dedicated WebView panel that provides users with a secure, intuitive interface to configure AI API credentials, select models, and validate connectivity. It follows the architectural patterns established by existing IntentLens panels (e.g., RestrictionDetailPanel) while introducing secure credential handling through VS Code Secrets API.

**Implementation Language**: JavaScript (Node.js runtime within VS Code)

**Architecture**: The panel uses the Webview API to create an isolated UI that communicates with the extension backend through message passing.

## Architecture

### Component Structure

```
Extension (extension.js)
    ├── Command Handler (intentlens.configureAPI)
    ├── Secret Storage Manager (VS Code Secrets API)
    ├── Configuration Manager (Workspace Configuration)
    ├── CLI Wrapper (runCLI with validation command)
    └── AIConfigPanel (WebView)
        ├── Provider Selector (Dropdown)
        ├── Credential Input (Password field)
        ├── Advanced Options (Collapsible)
        │   └── Base URL Input
        ├── Model Selection (Combobox)
        ├── Test Connection (Button + Async Handler)
        └── Save/Apply (Button + Validator)
```

### Data Flow

1. **User Action**: User opens command palette and runs `intentlens.configureAPI`
2. **Panel Creation**: Extension creates AIConfigPanel WebView
3. **Load Configuration**: Panel reads current config from Workspace Settings + Secrets
4. **Display Current State**: Panel shows current provider, model (masked key)
5. **User Input**: User modifies fields (provider, credentials, model)
6. **Validation**: Panel validates input in real-time (debounced)
7. **Test Connection**: User clicks "Test Connection"
8. **CLI Invocation**: Extension calls CLI with credentials
9. **Result Display**: Panel shows success ✓ or failure ✗
10. **Save**: User clicks Save, extension stores credentials in Secrets + config in Settings

### Message Protocol

**Extension → WebView**:
```javascript
{
  command: 'loadConfig',
  provider: 'openai',
  modelId: 'gpt-4',
  hasApiKey: true,
  baseUrl: '',
  configStatus: 'configured',
  lastValidation: '2024-01-15T10:30:00Z'
}

{
  command: 'testResult',
  success: true,
  message: 'Connection successful',
  models: ['gpt-4', 'gpt-4-turbo', 'gpt-3.5-turbo']
}
```

**WebView → Extension**:
```javascript
{
  command: 'save',
  provider: 'openai',
  apiKey: 'sk-...',
  modelId: 'gpt-4',
  baseUrl: ''
}

{
  command: 'testConnection',
  provider: 'openai',
  apiKey: 'sk-...',
  baseUrl: '',
  modelId: 'gpt-4'
}

{
  command: 'fetchModels',
  provider: 'openai',
  apiKey: 'sk-...',
  baseUrl: ''
}
```

## Components and Interfaces

### 1. AIConfigPanel Class

**Responsibility**: Manage the WebView panel lifecycle and communication

**Key Methods**:
- `constructor(panel, context)`: Initialize panel with message handlers
- `static show(context)`: Create or reveal the panel
- `_getHtmlContent()`: Generate HTML with current configuration state
- `_getStyles()`: Return CSS for Impeccable design consistency
- `_handleMessage(message)`: Process WebView messages
- `dispose()`: Clean up resources

**Properties**:
- `_panel`: VSCode WebViewPanel
- `_context`: ExtensionContext for secrets storage
- `_currentConfig`: Current configuration state
- `_validationState`: Current validation results

### 2. Configuration Storage Layer

**Interface**:
```typescript
interface APIConfiguration {
  provider: 'openai' | 'anthropic' | 'other';
  modelId: string;
  baseUrl?: string;
  lastValidation?: ISO8601String;
  validationStatus: 'valid' | 'invalid' | 'untested';
}

// Storage:
// - apiKey: Stored in VS Code Secrets
// - provider, modelId, baseUrl: Stored in Workspace Configuration
// - lastValidation: Stored in Workspace Configuration
```

**Access Methods**:
```javascript
// Save API Key (secure)
await context.secrets.store('intentlens.apiKey', apiKey);

// Load API Key
const apiKey = await context.secrets.get('intentlens.apiKey');

// Save configuration
const config = vscode.workspace.getConfiguration('intentlens');
await config.update('apiProvider', provider, false);
```

### 3. CLI Integration for Validation

**Command**: `intentlens validate-api-config`

**Input** (JSON via stdin):
```json
{
  "provider": "openai",
  "apiKey": "sk-...",
  "baseUrl": "",
  "modelId": "gpt-4"
}
```

**Output** (JSON via stdout):
```json
{
  "success": true,
  "message": "Connection successful",
  "models": ["gpt-4", "gpt-4-turbo"],
  "pricing": {
    "inputPer1M": 0.03,
    "outputPer1M": 0.06
  }
}
```

### 4. UI Component Hierarchy

```
Container (max-width: 600px, padding: 24px)
  ├── Header
  │   ├── Title: "API Configuration"
  │   └── Status Badge (✓ Configured / ✗ Not Configured / ⚠ Needs Validation)
  │
  ├── Provider Selection
  │   ├── Label: "API Provider"
  │   └── Dropdown: ["OpenAI", "Anthropic Claude", "Other"]
  │
  ├── API Key Input
  │   ├── Label: "API Key"
  │   ├── Password Input (masked)
  │   └── Validation Error (if any)
  │
  ├── Advanced Options (Collapsible)
  │   └── Base URL Input
  │       ├── Label: "Base URL (Optional)"
  │       ├── Text Input
  │       └── Validation Error (if any)
  │
  ├── Model Selection
  │   ├── Label: "Model ID"
  │   ├── Combobox (fetched list or custom input)
  │   └── Loading Indicator (if fetching)
  │
  ├── Action Buttons
  │   ├── Test Connection Button (secondary)
  │   └── Save Button (primary)
  │
  ├── Test Result Display
  │   ├── Status Icon (✓ / ✗ / ⚠)
  │   ├── Message
  │   └── Duration
  │
  └── Footer
      └── Helper Text
```

## Data Models

### Configuration State

```javascript
{
  // Current UI state
  currentProvider: 'openai',
  currentApiKey: '', // Always empty in UI (security)
  currentModelId: 'gpt-4',
  currentBaseUrl: '',
  
  // Saved state
  hasApiKey: true, // Indicates if key exists
  savedProvider: 'openai',
  savedModelId: 'gpt-4',
  savedBaseUrl: '',
  
  // Validation state
  validationState: {
    isValid: true,
    lastValidation: '2024-01-15T10:30:00Z',
    models: ['gpt-4', 'gpt-4-turbo', 'gpt-3.5-turbo'],
    pricing: {
      inputPer1M: 0.03,
      outputPer1M: 0.06
    }
  },
  
  // Field validation errors
  errors: {
    apiKey: null,
    modelId: null,
    baseUrl: null
  }
}
```

### Provider Metadata

```javascript
const PROVIDER_CONFIG = {
  openai: {
    name: 'OpenAI',
    defaultUrl: 'https://api.openai.com/v1',
    icon: 'openai-icon',
    modelsFetchable: true
  },
  anthropic: {
    name: 'Anthropic Claude',
    defaultUrl: 'https://api.anthropic.com',
    icon: 'anthropic-icon',
    modelsFetchable: true
  },
  other: {
    name: 'Other',
    defaultUrl: '',
    icon: 'question-icon',
    modelsFetchable: false
  }
};
```

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Configuration Persistence Round-Trip

**For any** valid API configuration (provider, API key, model ID, and optional base URL), saving the configuration and then reloading the panel SHALL restore the exact same configuration (with API key masked).

**Validates: Requirements 2.3, 2.5, 6.6**

### Property 2: Validation Status Accuracy

**For any** API configuration that passes validation with the CLI, THE panel's validation status display SHALL show "✓ Connection successful" and remain in this state until the configuration is modified.

**Validates: Requirements 5.3, 5.4, 5.5**

### Property 3: Form Enablement State

**For all** input states, THE Save button SHALL be disabled if and only if any required field (Provider, API Key, or Model ID) is empty, regardless of validation status.

**Validates: Requirements 6.2, 6.3**

### Property 4: Error Message Clarity

**For any** validation failure scenario (invalid URL, missing field, API connection failure), THE panel SHALL display an error message that accurately describes the failure reason, not a generic error.

**Validates: Requirements 9.2, 9.3, 9.4, 9.5**

### Property 5: Base URL Validation

**For all** Base URL values that are entered, IF the URL is provided (non-empty), THE URL SHALL be validated as valid HTTP(S) format before allowing save. IF validation fails, an error message SHALL be displayed.

**Validates: Requirements 3.6, 3.7**

### Property 6: API Key Masking Security

**For all** saved API keys, when the panel is reopened, THE previously saved API key SHALL NOT be displayed in the input field in any form (not masked, not partial, not any substitute).

**Validates: Requirements 2.5, 2.6**

## Testing Strategy

### Unit Tests

1. **Configuration Validation**
   - Test that valid configurations pass validation
   - Test that invalid configurations (empty fields, malformed URLs) fail
   - Test error messages for each failure type
   - Test that validation errors clear when field is corrected

2. **Form State Management**
   - Test Save button enabled/disabled state transitions
   - Test that form retains user input during edits
   - Test that form resets after successful save

3. **Security**
   - Test that API keys are never logged or displayed
   - Test that VS Code Secrets API is called correctly
   - Test that secrets are properly retrieved and cleared

### Property-Based Tests

1. **Configuration Persistence** - Use fast-check to generate random valid configurations, save them, reload, and verify the stored values match

2. **Validation State Machine** - Generate sequences of form inputs and verify the validation state transitions are correct (e.g., empty → error, filled → can validate)

3. **URL Format Validation** - Use generators to create various URL patterns and verify that valid HTTP(S) URLs pass validation while invalid patterns fail

### Integration Tests

1. **CLI Communication**
   - Mock the CLI command invocation
   - Verify correct parameters are passed
   - Verify response handling (success, failure, model list)

2. **Message Protocol**
   - Verify WebView ↔ Extension message passing
   - Test message ordering and completeness

3. **Workspace Integration**
   - Test configuration persistence across panel close/open
   - Test configuration visible to other extension components

### End-to-End Test Scenarios

1. **New User Setup**
   - User opens command palette
   - User runs `intentlens.configureAPI`
   - User selects OpenAI
   - User enters API key
   - User selects model from fetched list
   - User clicks Test Connection → success
   - User clicks Save
   - User reopens panel → configuration preserved

2. **Configuration Update**
   - Existing configuration is loaded
   - User changes API key
   - User clicks Save
   - New credentials are used for subsequent API calls

3. **Error Recovery**
   - User enters invalid API key
   - User clicks Test Connection → failure with error
   - User corrects API key
   - User clicks Test Connection → success
   - User clicks Save

## Error Handling

### Error Scenarios

1. **Missing API Key**
   - Display: "API Key is required"
   - Action: Prevent save, highlight field
   - Recovery: User enters key

2. **Invalid Base URL**
   - Display: "Invalid URL format (must be https://...)"
   - Action: Prevent save, highlight field
   - Recovery: User corrects URL

3. **API Connection Failed**
   - Display: "✗ Connection failed: 401 Unauthorized (check API key)"
   - Action: Allow user to retry test or save anyway
   - Recovery: User corrects credentials

4. **Model Fetch Failed**
   - Display: Model input becomes text-only without dropdown
   - Action: Allow user to enter model ID manually
   - Recovery: User types model ID or retries test

5. **CLI Invocation Failed**
   - Display: "✗ Failed to reach validation service"
   - Action: Disable Test Connection button
   - Recovery: User checks if CLI is installed, retries

### Graceful Degradation

- If model fetching fails, allow manual text input
- If pricing fetch fails, show costs as "Unavailable"
- If validation fails, allow save anyway (validate later during use)

## Accessibility

- All form inputs have associated labels
- Color indicators (green/yellow/red) supplemented with text/icons
- Buttons have clear, descriptive text
- Focus states clearly visible
- WCAG AA contrast ratios maintained

## Performance Considerations

- Debounce validation input at 500ms to reduce unnecessary validation
- Lazy-load advanced options section
- Cache model list to reduce API calls
- Timeout model fetch after 5 seconds with fallback to text input

## Security Considerations

- API keys never logged, displayed, or transmitted except to configured API service
- Use VS Code Secrets API for credential storage (encrypted at rest)
- Sanitize all user input before passing to CLI
- Use HTTPS-only for base URLs in production
- Mask API key in UI after 3 characters: "sk-•••••••••"
