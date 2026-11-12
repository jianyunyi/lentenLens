# Requirements Document

## Introduction

IntentLens is a VS Code extension that provides AI-powered code analysis for Go projects. To enable this functionality, users need to configure API credentials and select their preferred AI model. This feature introduces a dedicated API Configuration Panel (AIConfigPanel) - a WebView panel that allows users to securely enter API credentials, select AI providers and models, and validate their configuration. This unified interface replaces scattered configuration prompts and provides clear feedback on API connectivity and configuration status.

## Glossary

- **IntentLens**: The VS Code extension system comprising a Go CLI backend and Node.js extension frontend
- **API_Provider**: A third-party AI service that IntentLens communicates with (e.g., OpenAI, Anthropic Claude)
- **API_Key**: Authentication credential required to access the API Provider
- **Base_URL**: Optional endpoint URL for custom or self-hosted API services
- **Model_ID**: Identifier for a specific AI model within the API Provider (e.g., gpt-4, claude-3-opus)
- **AIConfigPanel**: The WebView panel UI for configuring API credentials and model settings
- **VS_Code_Secrets**: The secure storage API provided by VS Code for sensitive data
- **Configuration_Validation**: The process of testing API connectivity using the IntentLens CLI
- **CLI**: The Go command-line tool (intentlens binary) that performs backend analysis and validation
- **Workspace_Configuration**: VS Code settings stored in workspace settings (settings.json)
- **Impeccable_Design**: The UI design system used throughout IntentLens featuring consistent colors, icons, and interactions

## Requirements

### Requirement 1: API Provider Selection

**User Story:** As a user setting up IntentLens, I want to select from available AI providers (OpenAI, Anthropic Claude, Other), so that I can choose the service I prefer or already have credentials for.

#### Acceptance Criteria

1. WHEN THE AIConfigPanel is opened, THE Panel SHALL display a dropdown menu showing available API providers
2. THE dropdown options SHALL include: "OpenAI", "Anthropic Claude", and "Other"
3. THE initial selected option SHALL be the currently configured provider or "OpenAI" if not yet configured
4. WHEN a user selects a different provider, THE Panel SHALL update to show provider-specific configuration options
5. THE selected provider choice SHALL be displayed visually to indicate the current selection
6. THE Panel SHALL prevent submission until a provider is selected

### Requirement 2: API Key Input and Secure Storage

**User Story:** As a user, I want to enter my API key securely in the configuration panel and have it encrypted and stored, so that my credentials are protected.

#### Acceptance Criteria

1. THE AIConfigPanel SHALL display a password-input field labeled "API Key" with placeholder text "Enter your API key..."
2. THE API Key input field SHALL render characters as dots/asterisks (password masking)
3. WHEN THE user enters an API key and clicks Save, THE VS_Code_Extension SHALL store the key in VS Code Secrets using a workspace-scoped key
4. THE VS_Code_Extension SHALL encrypt the API key before storage using VS Code Secrets API
5. WHEN THE AIConfigPanel is reopened, THE previously saved API key SHALL NOT be displayed in the input field (for security)
6. THE input field SHALL display a masked indicator (e.g., "●●●●●●●●") showing that a key is configured
7. IF a user deletes the API key value and saves, THE VS_Code_Extension SHALL clear the stored secret

### Requirement 3: Base URL Configuration (Advanced)

**User Story:** As a user with a custom or self-hosted API endpoint, I want to optionally configure a Base URL, so that I can use IntentLens with non-standard API services.

#### Acceptance Criteria

1. THE AIConfigPanel SHALL display an "Advanced Options" collapsible section
2. WITHIN the Advanced Options section, THE Panel SHALL provide a Base_URL input field with placeholder "https://api.example.com"
3. WHEN Advanced Options is collapsed, THE Base_URL field SHALL be hidden
4. THE Base_URL field SHALL be optional and default to the API Provider's official endpoint
5. WHEN a user enters a Base_URL value and saves, THE VS_Code_Extension SHALL store it in Workspace_Configuration
6. THE Panel SHALL validate that Base_URL is a valid HTTP(S) URL format
7. IF validation fails, THE Panel SHALL display a validation error message and prevent saving

### Requirement 4: Model Selection and Input

**User Story:** As a user, I want to select or input a model ID from my API provider, so that I can specify which AI model IntentLens should use for analysis.

#### Acceptance Criteria

1. THE AIConfigPanel SHALL display a "Model ID" field with a dropdown/combobox interface
2. WHEN the user clicks the dropdown, THE Panel SHALL attempt to fetch available models from the configured API Provider
3. IF model fetch succeeds, THE dropdown SHALL display the list of available models
4. IF model fetch fails, THE dropdown SHALL allow manual text input
5. THE user SHALL be able to type a custom model ID if it's not in the fetched list
6. WHEN a user selects or enters a model ID and clicks Save, THE VS_Code_Extension SHALL store it in Workspace_Configuration
7. THE Panel SHALL display the currently configured model ID
8. THE Panel SHALL validate that the model ID field is not empty before allowing save

### Requirement 5: API Connection Test

**User Story:** As a user, I want to test my API configuration before saving, so that I can verify my credentials and connection are working correctly.

#### Acceptance Criteria

1. THE AIConfigPanel SHALL provide a "Test Connection" button
2. WHEN the user clicks "Test Connection", THE VS_Code_Extension SHALL invoke the CLI with a validation command
3. THE CLI validation command SHALL attempt to authenticate with the configured API using the provided credentials
4. WHILE the test is running, THE button SHALL display a loading state with spinner and "Testing..." text
5. IF the test succeeds, THE Panel SHALL display a success message: "✓ Connection successful" with green color coding
6. IF the test fails, THE Panel SHALL display an error message: "✗ Connection failed: <error_detail>" with red color coding
7. THE test result SHALL remain visible until the user modifies any configuration field
8. IF the API Key field is empty when "Test Connection" is clicked, THE Panel SHALL display a validation error

### Requirement 6: Save and Apply Configuration

**User Story:** As a user, I want to save my configuration and apply it to IntentLens, so that the extension can use my settings for code analysis.

#### Acceptance Criteria

1. THE AIConfigPanel SHALL provide a "Save" button to persist configuration
2. WHEN all required fields (Provider, API Key, Model ID) are filled, THE Save button SHALL be enabled
3. WHEN any required field is empty, THE Save button SHALL be disabled with a disabled visual state
4. WHEN the user clicks Save, THE VS_Code_Extension SHALL validate all inputs
5. IF validation passes, THE VS_Code_Extension SHALL persist the configuration and display "✓ Configuration saved"
6. IF validation fails, THE VS_Code_Extension SHALL display error messages and prevent saving
7. AFTER successful save, THE Panel SHALL close automatically after 2 seconds OR the user can close it immediately
8. WHEN configuration is saved, THE IntentLens extension SHALL use these settings for subsequent API calls

### Requirement 7: Configuration Status Display

**User Story:** As a user, I want to see the current configuration status at a glance, so that I know whether IntentLens is properly configured for use.

#### Acceptance Criteria

1. THE AIConfigPanel SHALL display a configuration status section at the top
2. IF all required configuration is present and last validation passed, THE status SHALL show: "✓ Configured" with green indicator
3. IF required configuration is missing, THE status SHALL show: "✗ Not Configured" with red indicator
4. IF configuration exists but has not been validated, THE status SHALL show: "⚠ Needs Validation" with yellow indicator
5. THE status section SHALL display the currently configured provider and model ID
6. THE status section SHALL show the timestamp of the last successful validation
7. WHEN configuration is incomplete, THE status SHALL provide a clear message about what's missing

### Requirement 8: Panel Access and Commands

**User Story:** As a user, I want to open the API Configuration Panel using a command, so that I can access it easily from the command palette.

#### Acceptance Criteria

1. THE VS_Code_Extension SHALL register a command: `intentlens.configureAPI`
2. WHEN the user runs `intentlens.configureAPI` from the command palette, THE AIConfigPanel SHALL open in a dedicated column
3. THE command SHALL be available in the VS Code Command Palette with title: "IntentLens: Configure API"
4. IF the panel is already open, THE panel SHALL be revealed (brought to foreground)
5. IF a panel is already open for another workspace, THE new panel SHALL be created for the active workspace
6. THE Panel title bar SHALL display "IntentLens: API Configuration"

### Requirement 9: Input Validation and Error Messaging

**User Story:** As a user, I want clear error messages when configuration is invalid, so that I can quickly fix issues.

#### Acceptance Criteria

1. THE AIConfigPanel SHALL validate each field as the user types (debounced at 500ms)
2. THE Panel SHALL display inline error messages below invalid fields in red text
3. WHEN API Key field is empty and Save is clicked, THE Panel SHALL show: "API Key is required"
4. WHEN Base_URL is invalid format, THE Panel SHALL show: "Invalid URL format"
5. WHEN Model_ID field is empty and Save is clicked, THE Panel SHALL show: "Model ID is required"
6. THE Panel SHALL clear error messages when the user corrects the field value
7. ALL error messages SHALL use consistent color coding: red (#F14C4C) for errors, green (#58B368) for success

### Requirement 10: UI Design and Impeccable Consistency

**User Story:** As a user, I want the API Configuration Panel to follow IntentLens design standards, so that it feels consistent with the rest of the extension.

#### Acceptance Criteria

1. THE AIConfigPanel SHALL use VS Code color scheme variables for all UI elements
2. THE Panel SHALL use Impeccable design system colors: ✓ green (#58B368), ⚠ yellow (#F5A623), ✗ red (#F14C4C)
3. THE Panel layout SHALL be responsive and readable with proper spacing and typography
4. THE Panel buttons SHALL have hover and active states with smooth transitions
5. THE Panel input fields SHALL have focus states with visible outlines
6. THE Panel output messages SHALL use color-coded icons: ✓ for success, ⚠ for warnings, ✗ for errors
7. ALL text SHALL be centered and readable with appropriate contrast ratios (WCAG AA minimum)
