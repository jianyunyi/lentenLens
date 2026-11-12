# Implementation Plan: API Configuration Panel

## Overview

This implementation plan converts the API Configuration Panel design into a series of discrete coding tasks. The panel will be built following the same WebView pattern used by existing panels in IntentLens (RestrictionDetailPanel) and will integrate with VS Code's Secrets API for secure credential storage.

## Tasks

- [x] 1. Set up AIConfigPanel module structure and core class
  - Create `extension/src/aiConfigPanel.js` with AIConfigPanel class skeleton
  - Implement `static show(context)` method to create or reveal panel
  - Implement panel lifecycle methods: `constructor`, `dispose`
  - Register command `intentlens.configureAPI` in extension.js
  - Verify panel can be opened and closed without errors
  - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 8.6_

- [x] 2. Implement HTML rendering and Impeccable design styles
  - Create `_getHtmlContent()` method to generate panel HTML structure
  - Implement `_getStyles()` with Impeccable design colors and responsive layout
  - Create UI sections: Header, Provider Selection, API Key, Advanced Options, Model Selection, Buttons
  - Ensure color scheme uses VS Code variables (background, foreground, text colors)
  - Test responsive layout at various panel widths
  - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5_

- [x] 3. Implement WebView message protocol
  - Create `_handleMessage()` to process WebView messages
  - Implement message handlers: `save`, `testConnection`, `fetchModels`
  - Create message types: `loadConfig`, `testResult`, `validationError`
  - Test message passing between extension and WebView
  - _Requirements: 8.1, 8.8_

- [x] 4. Implement configuration loading and display
  - Read current configuration from Workspace Settings (`intentlens.apiProvider`, `intentlens.apiBaseUrl`)
  - Read configuration status from Workspace Settings (`intentlens.configStatus`)
  - Load API key presence flag from VS Code Secrets
  - Render current provider, model, and status in HTML
  - Display status badge: ✓ Configured, ✗ Not Configured, or ⚠ Needs Validation
  - _Requirements: 1.2, 2.5, 2.6, 3.5, 7.1, 7.2, 7.3, 7.6_

- [x] 5. Implement Provider selection dropdown
  - Render dropdown with options: OpenAI, Anthropic Claude, Other
  - Handle provider selection changes
  - Update displayed options based on selected provider
  - Store selected provider in Workspace Configuration
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6_

- [x] 6. Implement API Key input field with password masking
  - Create password-masked input field
  - Display validation errors for empty API key
  - Implement error message: "API Key is required"
  - When panel reopens, do NOT display the previously saved key
  - Display masked indicator when key is configured
  - _Requirements: 2.1, 2.2, 2.5, 2.6, 2.7_

- [x] 7. Implement VS Code Secrets storage for API Key
  - Store API key in VS Code Secrets using workspace-scoped key
  - Retrieve API key from Secrets when needed (for testing)
  - Handle secret deletion when user clears API key
  - Test that keys are encrypted and persist across sessions
  - _Requirements: 2.3, 2.4, 2.5, 2.7_

- [x] 8. Implement Advanced Options collapsible section with Base URL
  - Create collapsible "Advanced Options" section
  - Implement Base URL input field (optional, not required)
  - Validate Base URL format (must be valid HTTP/HTTPS URL)
  - Display validation error: "Invalid URL format"
  - Store Base URL in Workspace Configuration
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7_

- [x] 9. Implement Model ID selection and input
  - Create combobox/dropdown for Model ID selection
  - Implement Model ID text input fallback
  - Display validation error: "Model ID is required"
  - Store selected/entered model ID in Workspace Configuration
  - _Requirements: 4.1, 4.5, 4.6, 4.7, 4.8_

- [x] 10. Implement model list fetching from API Provider
  - Create CLI command invocation: `intentlens test-api-config`
  - Pass provider, API key, base URL, model ID to CLI
  - Parse CLI response for available models list
  - Populate dropdown with fetched models
  - If fetch fails, allow manual text input
  - Handle timeout after 5 seconds
  - _Requirements: 4.2, 4.3, 4.4_

- [x] 11. Implement Test Connection button and handler
  - Add "Test Connection" button to UI
  - Implement async connection test logic
  - Show loading state with spinner: "Testing..."
  - Call CLI with `intentlens test-api-config` command
  - Display success message: "✓ Connection successful"
  - Display failure message: "✗ Connection failed: <error_detail>"
  - Handle timeout and error cases gracefully
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 5.8_

- [x] 12. Implement Save button with validation
  - Implement form validation: all required fields (Provider, API Key, Model ID) must be filled
  - Enable Save button only when all required fields are valid
  - Disable Save button and show visual feedback when fields invalid
  - Implement save logic: persist to VS Code Secrets and Workspace Configuration
  - Display success message: "✓ Configuration saved"
  - Auto-close panel after 2 seconds on success OR allow immediate close
  - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 6.8_

- [x] 13. Implement inline field validation with error messages
  - Implement debounced validation (500ms) as user types
  - Display inline error messages below each field (red text)
  - Show specific error messages: "API Key is required", "Invalid URL format", "Model ID is required"
  - Clear error messages when field is corrected
  - Use consistent error color (#F14C4C) and success color (#58B368)
  - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6, 9.7_

- [x] 14. Update extension.js with panel registration
  - Import AIConfigPanel class in extension.js
  - Register command `intentlens.configureAPI` with proper handler
  - Verify command appears in command palette
  - Test command execution opens panel correctly
  - Ensure existing functionality is not broken
  - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 4.2_

- [x] 15. Implement configuration sync to active IntentLens features
  - After successful save, ensure IntentLens CLI uses new credentials
  - Test that subsequent API calls use stored configuration
  - Verify existing explainSelection and other features work with new config
  - _Requirements: 6.8_

- [x] 16. Checkpoint - Ensure panel functionality and tests pass
  - Run all property tests for configuration persistence, validation, and form state
  - Verify no regressions in existing extension functionality
  - Test panel opening, closing, configuration loading/saving
  - Check error messages display correctly
  - Ask user if questions arise

- [x] 17. Implement accessibility features
  - Add proper labels to all form inputs
  - Ensure focus states are clearly visible
  - Verify WCAG AA contrast ratios for all text
  - Test keyboard navigation through form fields
  - Test with screen reader for accessibility
  - _Requirements: 10.4, 10.5, 10.7_

- [x] 18. Create integration test for end-to-end workflow
  - Test: Open panel → Select provider → Enter credentials → Test connection → Save
  - Test: Reopen panel → Verify configuration loaded correctly
  - Test: Change API key → Save → Verify new key is used
  - Verify no sensitive data is logged during tests
  - _Requirements: 1.1, 2.1, 4.1, 5.1, 6.1, 8.1_

- [x] 19. Documentation and cleanup
  - Add JSDoc comments to AIConfigPanel class and all methods
  - Document message protocol in code comments
  - Create USAGE.md explaining how to configure API
  - Clean up any temporary test files or debugging code
  - _Requirements: All_

## Notes

- Tasks marked with `*` are optional (if they had an asterisk) and can be skipped for MVP
- The implementation follows the existing RestrictionDetailPanel pattern for consistency
- Property tests should verify configuration persistence round-trip, form validation state, and API key masking
- Security is critical: API keys must never be logged or displayed in any form
- All error messages should follow the Impeccable color coding standard

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "2.1"] },
    { "id": 1, "tasks": ["3.1", "4.1", "5.1"] },
    { "id": 2, "tasks": ["6.1", "7.1", "8.1", "9.1"] },
    { "id": 3, "tasks": ["10.1", "11.1", "12.1"] },
    { "id": 4, "tasks": ["13.1", "14.1"] },
    { "id": 5, "tasks": ["15.1", "17.1", "18.1"] }
  ]
}
```
