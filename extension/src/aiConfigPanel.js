// AIConfigPanel - Webview panel for API configuration and credentials management
const vscode = require('vscode');
const { runCLI } = require('./core');
const path = require('node:path');
const fs = require('node:fs');
const crypto = require('node:crypto');

/**
 * Serialize a value as a JavaScript literal that is safe to embed in an
 * inline <script> block. JSON.stringify handles quotes and backslashes;
 * the extra replacements stop `</script>` breakouts and the U+2028/U+2029
 * line-terminator edge cases.
 * @param {*} value - Value to serialize
 * @returns {string} Safe JavaScript literal
 */
function jsValueLiteral(value) {
    return JSON.stringify(value ?? null)
        .replace(/</g, '\\u003c')
        .replace(/\u2028/g, '\\u2028')
        .replace(/\u2029/g, '\\u2029');
}

/**
 * Workspace-scoped secret key for API key storage
 * @param {string} wsPath - Workspace path
 * @returns {string} Secret key
 */
function apiKeySecretKey(wsPath) {
    return `intentlens.apiKey.${wsPath}`;
}

class AIConfigPanel {
    static currentPanel = undefined;

    /**
     * Create or show the API Configuration Panel
     * @param {vscode.ExtensionContext} context - The extension context
     * @param {vscode.WorkspaceFolder} folder - Optional workspace folder
     */
    static show(context, folder) {
        const column = vscode.ViewColumn.Two;

        if (AIConfigPanel.currentPanel) {
            AIConfigPanel.currentPanel._panel.reveal(column);
            return;
        }

        const panel = vscode.window.createWebviewPanel(
            'aiConfigPanel',
            'IntentLens: API Configuration',
            column,
            {
                enableScripts: true,
                retainContextWhenHidden: true
            }
        );

        AIConfigPanel.currentPanel = new AIConfigPanel(panel, context, folder);
    }

    /**
     * Initialize AIConfigPanel
     * @param {vscode.WebviewPanel} panel - The WebView panel
     * @param {vscode.ExtensionContext} context - The extension context
     * @param {vscode.WorkspaceFolder} folder - Workspace folder
     */
    constructor(panel, context, folder) {
        this._panel = panel;
        this._context = context;
        this._folder = folder;
        this._configurationData = {};
        this._validationInProgress = false;
        this._testInProgress = false;
        this._fieldErrors = {};

        // Initialize configuration data
        this._loadConfigurationFromSecrets();

        // Update the panel content
        this._update();

        // Handle panel disposal
        this._panel.onDidDispose(() => this.dispose(), null, context.subscriptions);

        // Handle messages from WebView
        this._panel.webview.onDidReceiveMessage(
            message => {
                this._handleMessage(message);
            },
            null,
            context.subscriptions
        );
    }

    /**
     * Update the panel content
     */
    _update() {
        this._panel.webview.html = this._getHtmlContent();
    }

    /**
     * Load configuration from VS Code Secrets and Workspace Configuration
     */
    async _loadConfigurationFromSecrets() {
        try {
            // Determine workspace folder for storage keys
            let wsPath = this._folder?.uri.fsPath || '';
            if (!wsPath) {
                const folders = vscode.workspace.workspaceFolders || [];
                if (folders.length > 0) {
                    wsPath = folders[0].uri.fsPath;
                }
            }

            // Load from workspace configuration
            const config = vscode.workspace.getConfiguration('intentlens', this._folder?.uri);
            const provider = config.get('apiProvider', 'openai');
            const modelId = config.get('apiModelId', '');
            const baseUrl = config.get('apiBaseUrl', '');
            const configStatus = config.get('configStatus', 'unconfigured');
            const lastValidation = config.get('lastValidation', null);

            // Check if API key exists in secrets
            const apiKeySecretName = apiKeySecretKey(wsPath);
            const hasApiKey = !!(await this._context.secrets.get(apiKeySecretName));

            // Update configuration data
            this._configurationData = {
                provider,
                modelId,
                baseUrl,
                hasApiKey,
                configStatus,
                lastValidation,
                wsPath
            };
        } catch (error) {
            console.error('Failed to load configuration from secrets:', error);
            this._configurationData = {
                provider: 'openai',
                modelId: '',
                baseUrl: '',
                hasApiKey: false,
                configStatus: 'unconfigured',
                lastValidation: null,
                wsPath: this._folder?.uri.fsPath || ''
            };
        }
    }

    /**
     * Validate all form fields
     * @param {Object} formData - Form data to validate
     * @returns {Object} Validation errors object
     */
    _validateFormFields(formData) {
        const errors = {};

        // Validate provider
        const validProviders = ['openai', 'anthropic', 'other'];
        if (!validProviders.includes(formData.provider)) {
            errors.provider = 'Invalid provider selected';
        }

        // Validate API key
        if (!formData.apiKey || formData.apiKey.trim() === '') {
            errors.apiKey = 'API Key is required';
        }

        // Validate model ID
        if (!formData.modelId || formData.modelId.trim() === '') {
            errors.modelId = 'Model ID is required';
        }

        // Validate base URL if provided
        if (formData.baseUrl && formData.baseUrl.trim() !== '') {
            const urlPattern = /^https?:\/\/.+/i;
            if (!urlPattern.test(formData.baseUrl)) {
                errors.baseUrl = 'Invalid URL format (must be https://...)';
            }
        }

        return errors;
    }

    /**
     * Handle messages from WebView
     * @param {Object} message - Message object from WebView
     */
    async _handleMessage(message) {
        try {
            switch (message.command) {
                case 'save':
                    await this._saveConfiguration(message);
                    break;
                case 'testConnection':
                    await this._testAPIConnection(message);
                    break;
                case 'fetchModels':
                    await this._fetchModels(message);
                    break;
                case 'updateProvider':
                    await this._updateProvider(message);
                    break;
                case 'close':
                    this._panel.dispose();
                    break;
                case 'log':
                    console.log('WebView log:', message.message);
                    break;
                default:
                    console.warn('Unknown command:', message.command);
            }
        } catch (error) {
            console.error('Error handling message:', error);
            this._postMessage({
                type: 'error',
                message: `Error: ${error.message}`
            });
        }
    }

    /**
     * Save configuration to VS Code Secrets and Workspace Configuration
     * @param {Object} message - Save message from WebView
     */
    async _saveConfiguration(message) {
        try {
            // Validate form fields
            const errors = this._validateFormFields({
                provider: message.provider,
                apiKey: message.apiKey,
                modelId: message.modelId,
                baseUrl: message.baseUrl
            });

            if (Object.keys(errors).length > 0) {
                this._postMessage({
                    type: 'validationErrors',
                    errors
                });
                return;
            }

            // Determine workspace path
            let wsPath = this._configurationData.wsPath;
            if (!wsPath) {
                const folders = vscode.workspace.workspaceFolders || [];
                if (folders.length > 0) {
                    wsPath = folders[0].uri.fsPath;
                } else {
                    throw new Error('No workspace folder available');
                }
            }

            // Store API key in VS Code Secrets (encrypted)
            const apiKeySecretName = apiKeySecretKey(wsPath);
            if (message.apiKey && message.apiKey.trim() !== '') {
                await this._context.secrets.store(apiKeySecretName, message.apiKey);
            } else {
                // Clear secret if API key is empty
                await this._context.secrets.delete(apiKeySecretName);
            }

            // Store other configuration in Workspace Configuration
            const config = vscode.workspace.getConfiguration('intentlens', this._folder?.uri);
            await config.update('apiProvider', message.provider, false);
            await config.update('apiModelId', message.modelId, false);
            await config.update('apiBaseUrl', message.baseUrl || '', false);
            await config.update('configStatus', 'configured', false);
            await config.update('lastValidation', new Date().toISOString(), false);

            // Update local config data
            this._configurationData = {
                ...this._configurationData,
                provider: message.provider,
                modelId: message.modelId,
                baseUrl: message.baseUrl,
                hasApiKey: true,
                configStatus: 'configured',
                lastValidation: new Date().toISOString()
            };

            // Send success message
            this._postMessage({
                type: 'saveSuccess',
                message: '✓ Configuration saved successfully',
                config: this._configurationData
            });

            // Auto-close panel after 2 seconds
            setTimeout(() => {
                if (AIConfigPanel.currentPanel) {
                    AIConfigPanel.currentPanel._panel.dispose();
                }
            }, 2000);
        } catch (error) {
            console.error('Failed to save configuration:', error);
            this._postMessage({
                type: 'error',
                message: `Failed to save configuration: ${error.message}`
            });
        }
    }

    /**
     * Test API connection using CLI
     * @param {Object} message - Test message from WebView
     */
    async _testAPIConnection(message) {
        try {
            if (this._testInProgress) {
                return;
            }

            // Validate required fields
            if (!message.apiKey || message.apiKey.trim() === '') {
                this._postMessage({
                    type: 'testResult',
                    success: false,
                    message: 'API Key is required to test connection'
                });
                return;
            }

            this._testInProgress = true;
            this._postMessage({
                type: 'testStarted'
            });

            // Prepare test data
            const testData = {
                provider: message.provider || 'openai',
                apiKey: message.apiKey,
                baseUrl: message.baseUrl || '',
                modelId: message.modelId || ''
            };

            // Determine binary path
            const configured = vscode.workspace.getConfiguration('intentlens', this._folder?.uri).get('binaryPath', '');
            const bundled = path.join(this._context.extensionPath, 'bin', process.platform === 'win32' ? 'intentlens.exe' : 'intentlens');
            const binary = configured || (fs.existsSync(bundled) ? bundled : 'intentlens');

            // Get workspace path
            let wsPath = this._configurationData.wsPath || (this._folder?.uri.fsPath || '');
            if (!wsPath) {
                const folders = vscode.workspace.workspaceFolders || [];
                if (folders.length > 0) {
                    wsPath = folders[0].uri.fsPath;
                }
            }

            // Call CLI for validation
            const startTime = Date.now();
            let result;
            try {
                // Create signal for timeout (5 seconds)
                const controller = new AbortController();
                const timeoutId = setTimeout(() => controller.abort(), 5000);

                result = await runCLI(
                    binary,
                    wsPath,
                    'test-api-config',
                    testData,
                    { signal: controller.signal, timeout: 5000 }
                );

                clearTimeout(timeoutId);
            } catch (cliError) {
                const duration = Date.now() - startTime;
                this._testInProgress = false;

                // Check if error is due to timeout
                if (cliError.message.includes('abort') || cliError.message.includes('timeout')) {
                    this._postMessage({
                        type: 'testResult',
                        success: false,
                        message: '✗ Connection test timed out (5s)',
                        duration: `${duration}ms`
                    });
                    return;
                }

                // Parse CLI error
                let errorMessage = cliError.message || 'Connection failed';
                if (cliError.stdout) {
                    try {
                        const parsed = JSON.parse(cliError.stdout);
                        errorMessage = parsed.error || parsed.message || errorMessage;
                    } catch (e) {
                        // Keep original error message
                    }
                }

                this._postMessage({
                    type: 'testResult',
                    success: false,
                    message: `✗ Connection failed: ${errorMessage}`,
                    duration: `${duration}ms`
                });
                return;
            }

            const duration = Date.now() - startTime;
            this._testInProgress = false;

            // Parse result
            let parsedResult;
            try {
                if (typeof result === 'string') {
                    parsedResult = JSON.parse(result);
                } else {
                    parsedResult = result;
                }
            } catch (e) {
                parsedResult = { success: true, models: [] };
            }

            if (parsedResult.success) {
                this._postMessage({
                    type: 'testResult',
                    success: true,
                    message: '✓ Connection successful',
                    models: parsedResult.models || [],
                    duration: `${duration}ms`
                });
            } else {
                this._postMessage({
                    type: 'testResult',
                    success: false,
                    message: `✗ Connection failed: ${parsedResult.error || 'Unknown error'}`,
                    duration: `${duration}ms`
                });
            }
        } catch (error) {
            this._testInProgress = false;
            console.error('Failed to test API connection:', error);
            this._postMessage({
                type: 'testResult',
                success: false,
                message: `✗ Failed to reach validation service: ${error.message}`
            });
        }
    }

    /**
     * Fetch available models from API provider
     * @param {Object} message - Fetch message from WebView
     */
    async _fetchModels(message) {
        try {
            if (this._validationInProgress) {
                return;
            }

            if (!message.apiKey || message.apiKey.trim() === '') {
                this._postMessage({
                    type: 'modelsResult',
                    success: false,
                    models: []
                });
                return;
            }

            this._validationInProgress = true;
            this._postMessage({
                type: 'modelsFetchStarted'
            });

            // Prepare fetch data
            const fetchData = {
                provider: message.provider || 'openai',
                apiKey: message.apiKey,
                baseUrl: message.baseUrl || ''
            };

            // Determine binary path
            const configured = vscode.workspace.getConfiguration('intentlens', this._folder?.uri).get('binaryPath', '');
            const bundled = path.join(this._context.extensionPath, 'bin', process.platform === 'win32' ? 'intentlens.exe' : 'intentlens');
            const binary = configured || (fs.existsSync(bundled) ? bundled : 'intentlens');

            // Get workspace path
            let wsPath = this._configurationData.wsPath || (this._folder?.uri.fsPath || '');
            if (!wsPath) {
                const folders = vscode.workspace.workspaceFolders || [];
                if (folders.length > 0) {
                    wsPath = folders[0].uri.fsPath;
                }
            }

            let result;
            try {
                // Create signal for timeout (5 seconds)
                const controller = new AbortController();
                const timeoutId = setTimeout(() => controller.abort(), 5000);

                result = await runCLI(
                    binary,
                    wsPath,
                    'list-models',
                    fetchData,
                    { signal: controller.signal, timeout: 5000 }
                );

                clearTimeout(timeoutId);
            } catch (cliError) {
                this._validationInProgress = false;
                // On error, allow manual text input (graceful degradation)
                this._postMessage({
                    type: 'modelsResult',
                    success: false,
                    models: []
                });
                return;
            }

            this._validationInProgress = false;

            // Parse result
            let parsedResult;
            try {
                if (typeof result === 'string') {
                    parsedResult = JSON.parse(result);
                } else {
                    parsedResult = result;
                }
            } catch (e) {
                parsedResult = { models: [] };
            }

            this._postMessage({
                type: 'modelsResult',
                success: true,
                models: parsedResult.models || []
            });
        } catch (error) {
            this._validationInProgress = false;
            console.error('Failed to fetch models:', error);
            this._postMessage({
                type: 'modelsResult',
                success: false,
                models: []
            });
        }
    }

    /**
     * Update provider selection
     * @param {Object} message - Update message from WebView
     */
    async _updateProvider(message) {
        try {
            const newProvider = message.provider || 'openai';
            this._configurationData.provider = newProvider;
            this._update();
        } catch (error) {
            console.error('Failed to update provider:', error);
        }
    }

    /**
     * Post message to WebView
     * @param {Object} message - Message to post
     */
    _postMessage(message) {
        if (this._panel && !this._panel.disposed) {
            this._panel.webview.postMessage(message);
        }
    }

    /**
     * Generate HTML content for the panel
     * @returns {string} HTML content
     */
    _getHtmlContent() {
        const config = this._configurationData;
        const nonce = crypto.randomBytes(16).toString('base64');
        const statusClass = config.configStatus === 'configured' ? 'status-success' : 
                           config.configStatus === 'invalid' ? 'status-error' : 'status-warning';
        const statusIcon = config.configStatus === 'configured' ? '✓' : 
                          config.configStatus === 'invalid' ? '✗' : '⚠';
        const statusText = config.configStatus === 'configured' ? 'Configured' : 
                          config.configStatus === 'invalid' ? 'Configuration Error' : 'Not Configured';

        return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'nonce-${nonce}'; script-src 'nonce-${nonce}';">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>IntentLens: API Configuration</title>
    <style nonce="${nonce}">${this._getStyles()}</style>
</head>
<body>
    <div class="container">
        <!-- Header Section -->
        <div class="header">
            <div class="header-top">
                <h1 class="header-title">⚙️ AI API Configuration</h1>
                <div class="status-badge ${statusClass}" id="statusBadge">
                    <span class="status-icon" id="statusIcon">${statusIcon}</span>
                    <span class="status-text" id="statusText">${statusText}</span>
                </div>
            </div>
            <p class="header-description">Configure your AI model provider and API credentials</p>
        </div>

        <!-- Form Container -->
        <form class="form-container" id="configForm">
            <!-- API Provider Selection -->
            <div class="form-group">
                <label for="providerSelect" class="form-label">API Provider</label>
                <select id="providerSelect" class="form-control form-select" aria-label="Select API Provider">
                    <option value="openai">OpenAI</option>
                    <option value="anthropic">Anthropic Claude</option>
                    <option value="other">Other / Custom</option>
                </select>
                <span class="helper-text">Choose your AI service provider</span>
                <span class="error-message" id="providerError"></span>
            </div>

            <!-- API Key Input -->
            <div class="form-group">
                <div class="label-row">
                    <label for="apiKeyInput" class="form-label">API Key</label>
                    <button type="button" class="visibility-toggle" id="visibilityToggle" aria-label="Toggle API Key visibility" title="Show/Hide">
                        <span class="toggle-icon">👁️</span>
                    </button>
                </div>
                <input 
                    type="password" 
                    id="apiKeyInput" 
                    class="form-control form-input" 
                    placeholder="Enter your API key..."
                    aria-label="API Key password input"
                >
                <span class="helper-text">🔒 Securely stored in VS Code Secrets</span>
                <span class="error-message" id="apiKeyError"></span>
            </div>

            <!-- Advanced Options (Collapsible) -->
            <div class="form-group">
                <button type="button" class="advanced-toggle" id="advancedToggle" aria-expanded="false" aria-controls="advancedOptions">
                    <span class="toggle-chevron">▶</span>
                    <span class="toggle-label">Advanced Options</span>
                </button>
                
                <div class="advanced-section" id="advancedOptions" aria-hidden="true">
                    <!-- Base URL Input -->
                    <div class="form-group inner-form-group">
                        <label for="baseUrlInput" class="form-label">Base URL (Optional)</label>
                        <input 
                            type="text" 
                            id="baseUrlInput" 
                            class="form-control form-input" 
                            placeholder="https://api.example.com"
                            aria-label="Custom Base URL"
                        >
                        <span class="helper-text">Leave empty to use official API endpoint</span>
                        <span class="error-message" id="baseUrlError"></span>
                    </div>
                </div>
            </div>

            <!-- Model Selection -->
            <div class="form-group">
                <label for="modelInput" class="form-label">Model ID</label>
                <div class="model-input-wrapper">
                    <input 
                        type="text"
                        id="modelInput"
                        list="modelList"
                        class="form-control form-input"
                        placeholder="Select model or enter custom ID..."
                        aria-label="Select or enter model ID"
                    >
                    <datalist id="modelList"></datalist>
                    <span class="loading-indicator" id="loadingIndicator" aria-label="Loading" style="display: none;">⏳</span>
                </div>
                <span class="helper-text">Select from list or enter a custom model ID</span>
                <span class="error-message" id="modelError"></span>
            </div>

            <!-- Action Buttons Section -->
            <div class="button-group">
                <button type="button" class="btn btn-secondary" id="testBtn" aria-label="Test connection">
                    <span class="btn-icon">🧪</span>
                    <span class="btn-text">Test Connection</span>
                </button>
                <button type="submit" class="btn btn-primary" id="saveBtn" aria-label="Save configuration">
                    <span class="btn-icon">💾</span>
                    <span class="btn-text">Save</span>
                </button>
            </div>

            <!-- Test Result Display -->
            <div class="status-section" id="testResultSection" style="display: none;">
                <div class="status-indicator" id="testStatusIndicator">
                    <span class="status-icon-result" id="testStatusIcon"></span>
                    <span class="status-message" id="testStatusMessage"></span>
                </div>
                <span class="test-duration" id="testDuration"></span>
            </div>

            <!-- Status/Error Messages -->
            <div class="alert-section" id="alertSection" style="display: none;">
                <div class="alert" id="alertBox">
                    <span class="alert-icon" id="alertIcon"></span>
                    <span class="alert-message" id="alertMessage"></span>
                </div>
            </div>
        </form>

        <!-- Footer Info -->
        <div class="footer-info">
            <span class="footer-text">Need help? Check the documentation or run <code>intentlens --help</code></span>
        </div>
    </div>

    <script nonce="${nonce}">
        const vscode = acquireVsCodeApi();

        // UI Elements
        const form = document.getElementById('configForm');
        const providerSelect = document.getElementById('providerSelect');
        const apiKeyInput = document.getElementById('apiKeyInput');
        const visibilityToggle = document.getElementById('visibilityToggle');
        const advancedToggle = document.getElementById('advancedToggle');
        const advancedOptions = document.getElementById('advancedOptions');
        const baseUrlInput = document.getElementById('baseUrlInput');
        const modelInput = document.getElementById('modelInput');
        const modelList = document.getElementById('modelList');
        const testBtn = document.getElementById('testBtn');
        const saveBtn = document.getElementById('saveBtn');
        const statusBadge = document.getElementById('statusBadge');
        const testResultSection = document.getElementById('testResultSection');
        const testStatusIcon = document.getElementById('testStatusIcon');
        const testStatusMessage = document.getElementById('testStatusMessage');
        const testDuration = document.getElementById('testDuration');
        const alertSection = document.getElementById('alertSection');
        const alertIcon = document.getElementById('alertIcon');
        const alertMessage = document.getElementById('alertMessage');
        const loadingIndicator = document.getElementById('loadingIndicator');

        // State
        let fieldErrors = {};
        let isTestInProgress = false;
        let lastTestResult = null;

        // Initialize provider (values are serialized as safe JS literals:
        // workspace settings are attacker-controlled in untrusted repos)
        const initialConfig = ${jsValueLiteral({ provider: config.provider, modelId: config.modelId, baseUrl: config.baseUrl })};
        providerSelect.value = initialConfig.provider || 'openai';
        modelInput.value = initialConfig.modelId || '';
        baseUrlInput.value = initialConfig.baseUrl || '';

        // Event: Provider selection change
        providerSelect.addEventListener('change', () => {
            clearFieldError('provider');
            clearTestResult();
            // Trigger model fetch for new provider
            if (apiKeyInput.value.trim()) {
                fetchModels();
            }
        });

        // Event: API Key input
        apiKeyInput.addEventListener('input', () => {
            clearFieldError('apiKey');
            clearTestResult();
        });

        // Event: Base URL input
        baseUrlInput.addEventListener('input', () => {
            clearFieldError('baseUrl');
            clearTestResult();
        });

        // Event: Model input
        modelInput.addEventListener('input', () => {
            clearFieldError('modelId');
            clearTestResult();
        });

        // Event: Visibility toggle
        visibilityToggle.addEventListener('click', (e) => {
            e.preventDefault();
            const type = apiKeyInput.type === 'password' ? 'text' : 'password';
            apiKeyInput.type = type;
            visibilityToggle.querySelector('.toggle-icon').textContent = type === 'password' ? '👁️' : '🙈';
        });

        // Event: Advanced toggle
        advancedToggle.addEventListener('click', (e) => {
            e.preventDefault();
            const isExpanded = advancedToggle.getAttribute('aria-expanded') === 'true';
            advancedToggle.setAttribute('aria-expanded', !isExpanded);
            advancedOptions.setAttribute('aria-hidden', isExpanded);
            advancedOptions.classList.toggle('expanded', !isExpanded);
            advancedToggle.querySelector('.toggle-chevron').style.transform = !isExpanded ? 'rotate(90deg)' : 'rotate(0deg)';
        });

        // Event: Test Connection
        testBtn.addEventListener('click', (e) => {
            e.preventDefault();
            if (isTestInProgress) return;
            testConnection();
        });

        // Event: Form Submit (Save)
        form.addEventListener('submit', (e) => {
            e.preventDefault();
            saveConfiguration();
        });

        // Handle messages from extension
        window.addEventListener('message', (event) => {
            const message = event.data;
            
            if (message.type === 'validationErrors') {
                fieldErrors = message.errors || {};
                displayFieldErrors();
            } else if (message.type === 'testResult') {
                displayTestResult(message);
                if (message.models && message.models.length > 0) {
                    populateModelList(message.models);
                }
            } else if (message.type === 'modelsFetchStarted') {
                loadingIndicator.style.display = 'inline-block';
            } else if (message.type === 'modelsResult') {
                loadingIndicator.style.display = 'none';
                if (message.success && message.models && message.models.length > 0) {
                    populateModelList(message.models);
                }
            } else if (message.type === 'saveSuccess') {
                showAlert('success', message.message);
            } else if (message.type === 'error') {
                showAlert('error', message.message);
            } else if (message.type === 'testStarted') {
                loadingIndicator.style.display = 'inline-block';
                testBtn.disabled = true;
            }
        });

        function testConnection() {
            clearTestResult();
            isTestInProgress = true;
            testBtn.disabled = true;
            
            vscode.postMessage({
                command: 'testConnection',
                provider: providerSelect.value,
                apiKey: apiKeyInput.value,
                baseUrl: baseUrlInput.value,
                modelId: modelInput.value
            });
        }

        function fetchModels() {
            vscode.postMessage({
                command: 'fetchModels',
                provider: providerSelect.value,
                apiKey: apiKeyInput.value,
                baseUrl: baseUrlInput.value
            });
        }

        function saveConfiguration() {
            fieldErrors = {};
            
            // Validate
            if (!providerSelect.value) {
                fieldErrors.provider = 'Provider is required';
            }
            if (!apiKeyInput.value || apiKeyInput.value.trim() === '') {
                fieldErrors.apiKey = 'API Key is required';
            }
            if (!modelInput.value || modelInput.value.trim() === '') {
                fieldErrors.modelId = 'Model ID is required';
            }
            if (baseUrlInput.value && baseUrlInput.value.trim() !== '') {
                const urlPattern = /^https?:\\/\\/.+/i;
                if (!urlPattern.test(baseUrlInput.value)) {
                    fieldErrors.baseUrl = 'Invalid URL format (must be https://...)';
                }
            }

            if (Object.keys(fieldErrors).length > 0) {
                displayFieldErrors();
                return;
            }

            saveBtn.disabled = true;
            vscode.postMessage({
                command: 'save',
                provider: providerSelect.value,
                apiKey: apiKeyInput.value,
                modelId: modelInput.value,
                baseUrl: baseUrlInput.value
            });
        }

        function displayFieldErrors() {
            clearAllFieldErrors();
            
            if (fieldErrors.provider) {
                showFieldError('provider', fieldErrors.provider);
            }
            if (fieldErrors.apiKey) {
                showFieldError('apiKey', fieldErrors.apiKey);
            }
            if (fieldErrors.baseUrl) {
                showFieldError('baseUrl', fieldErrors.baseUrl);
            }
            if (fieldErrors.modelId) {
                showFieldError('modelId', fieldErrors.modelId);
            }
        }

        function showFieldError(fieldName, message) {
            const errorElement = document.getElementById(fieldName + 'Error');
            if (errorElement) {
                errorElement.textContent = message;
                errorElement.classList.add('show');
            }
        }

        function clearFieldError(fieldName) {
            const errorElement = document.getElementById(fieldName + 'Error');
            if (errorElement) {
                errorElement.textContent = '';
                errorElement.classList.remove('show');
            }
            delete fieldErrors[fieldName];
        }

        function clearAllFieldErrors() {
            ['provider', 'apiKey', 'baseUrl', 'modelId'].forEach(f => {
                const errorElement = document.getElementById(f + 'Error');
                if (errorElement) {
                    errorElement.textContent = '';
                    errorElement.classList.remove('show');
                }
            });
        }

        function displayTestResult(result) {
            isTestInProgress = false;
            testBtn.disabled = false;
            loadingIndicator.style.display = 'none';
            lastTestResult = result;

            testStatusIcon.textContent = result.success ? '✓' : '✗';
            testStatusMessage.textContent = result.message;
            testStatusMessage.style.color = result.success ? '#10a981' : '#f48771';
            
            if (result.duration) {
                testDuration.textContent = '(' + result.duration + ')';
            } else {
                testDuration.textContent = '';
            }

            testResultSection.style.display = 'flex';
        }

        function clearTestResult() {
            testResultSection.style.display = 'none';
            testStatusIcon.textContent = '';
            testStatusMessage.textContent = '';
            testDuration.textContent = '';
            lastTestResult = null;
        }

        function populateModelList(models) {
            modelList.innerHTML = '';
            models.forEach(model => {
                const option = document.createElement('option');
                option.value = model;
                modelList.appendChild(option);
            });
        }

        function showAlert(type, message) {
            alertIcon.textContent = type === 'success' ? '✓' : '✗';
            alertMessage.textContent = message;
            alertSection.className = 'alert-section ' + type;
            alertSection.style.display = 'flex';
            
            if (type === 'success') {
                setTimeout(() => {
                    alertSection.style.display = 'none';
                }, 2000);
            }
        }
    </script>
</body>
</html>`;
    }

    /**
     * Generate CSS styles for the panel
     * @returns {string} CSS styles
     */
    _getStyles() {
        return `
* {
    box-sizing: border-box;
}

html, body {
    margin: 0;
    padding: 0;
    height: 100%;
}

body {
    font-family: var(--vscode-font-family);
    background: var(--vscode-editor-background);
    color: var(--vscode-editor-foreground);
    font-size: 13px;
    line-height: 1.6;
    overflow-x: hidden;
}

.container {
    max-width: 800px;
    margin: 0 auto;
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 24px;
}

/* ============ Header Section ============ */
.header {
    margin-bottom: 8px;
    padding-bottom: 20px;
    border-bottom: 1px solid var(--vscode-panel-border);
}

.header-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 12px;
}

.header-title {
    margin: 0;
    font-size: 20px;
    font-weight: 600;
    color: var(--vscode-editor-foreground);
}

.header-description {
    margin: 0;
    font-size: 12px;
    color: var(--vscode-descriptionForeground);
    font-weight: 400;
}

/* ============ Status Badge ============ */
.status-badge {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 6px 14px;
    background: rgba(245, 213, 71, 0.1);
    border: 1px solid rgba(245, 213, 71, 0.3);
    border-radius: 20px;
    font-size: 12px;
    font-weight: 500;
    color: var(--vscode-editor-foreground);
    white-space: nowrap;
    transition: all 200ms cubic-bezier(0.4, 0, 0.2, 1);
}

.status-badge.status-success {
    background: rgba(16, 169, 129, 0.1);
    border-color: rgba(16, 169, 129, 0.3);
    color: #10a981;
}

.status-badge.status-error {
    background: rgba(244, 135, 113, 0.1);
    border-color: rgba(244, 135, 113, 0.3);
    color: #f48771;
}

.status-badge.status-warning {
    background: rgba(245, 213, 71, 0.1);
    border-color: rgba(245, 213, 71, 0.3);
    color: #f5d547;
}

.status-icon {
    font-size: 14px;
}

.status-text {
    font-size: 12px;
}

/* ============ Form Container ============ */
.form-container {
    display: flex;
    flex-direction: column;
    gap: 20px;
}

/* ============ Form Group ============ */
.form-group {
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.form-group.inner-form-group {
    margin-left: 16px;
    margin-top: 12px;
    padding: 12px;
    background: var(--vscode-list-hoverBackground);
    border-radius: 4px;
    border-left: 3px solid var(--vscode-textLink-foreground);
}

/* ============ Form Labels ============ */
.form-label {
    font-size: 13px;
    font-weight: 500;
    color: var(--vscode-editor-foreground);
    display: block;
    margin-bottom: 2px;
}

.label-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
}

/* ============ Form Controls ============ */
.form-control {
    background: var(--vscode-input-background);
    color: var(--vscode-input-foreground);
    border: 1px solid var(--vscode-input-border);
    padding: 10px 12px;
    border-radius: 4px;
    font-family: var(--vscode-font-family);
    font-size: 13px;
    transition: all 200ms cubic-bezier(0.4, 0, 0.2, 1);
}

.form-input {
    padding: 10px 12px;
}

.form-input::placeholder {
    color: var(--vscode-input-placeholderForeground);
}

.form-input:focus,
.form-select:focus {
    outline: none;
    border-color: var(--vscode-focusBorder);
    box-shadow: 0 0 0 2px var(--vscode-focusBorder);
    background: var(--vscode-editor-background);
}

.form-select {
    padding: 10px 12px;
    cursor: pointer;
    background-image: url('data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><path fill="white" d="M8 11l4-4H4z"/></svg>');
    background-repeat: no-repeat;
    background-position: right 8px center;
    background-size: 12px;
    padding-right: 30px;
    appearance: none;
}

.form-select option {
    background: var(--vscode-editor-background);
    color: var(--vscode-editor-foreground);
}

.visibility-toggle {
    background: none;
    border: none;
    cursor: pointer;
    padding: 4px 8px;
    font-size: 16px;
    color: var(--vscode-editor-foreground);
    transition: transform 150ms ease;
    display: flex;
    align-items: center;
    justify-content: center;
}

.visibility-toggle:hover {
    transform: scale(1.1);
}

.visibility-toggle:active {
    transform: scale(0.95);
}

/* ============ Helper Text ============ */
.helper-text {
    font-size: 12px;
    color: var(--vscode-descriptionForeground);
    font-weight: 400;
    line-height: 1.4;
}

/* ============ Error Messages ============ */
.error-message {
    font-size: 12px;
    color: #f48771;
    display: none;
    line-height: 1.4;
    animation: slideIn 150ms cubic-bezier(0.4, 0, 0.2, 1);
}

.error-message.show {
    display: block;
}

@keyframes slideIn {
    from {
        opacity: 0;
        transform: translateY(-4px);
    }
    to {
        opacity: 1;
        transform: translateY(0);
    }
}

/* ============ Advanced Options ============ */
.advanced-toggle {
    background: none;
    border: none;
    padding: 8px 12px;
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    color: var(--vscode-editor-foreground);
    font-size: 13px;
    font-weight: 500;
    transition: all 150ms ease;
    border-radius: 4px;
    margin-left: -12px;
}

.advanced-toggle:hover {
    background: var(--vscode-list-hoverBackground);
}

.advanced-toggle:active {
    transform: scale(0.98);
}

.toggle-chevron {
    display: inline-block;
    transition: transform 200ms cubic-bezier(0.4, 0, 0.2, 1);
    font-size: 12px;
}

.advanced-section {
    max-height: 0;
    overflow: hidden;
    transition: max-height 300ms cubic-bezier(0.4, 0, 0.2, 1);
}

.advanced-section.expanded {
    max-height: 300px;
}

.toggle-label {
    font-size: 13px;
}

/* ============ Model Selection ============ */
.model-input-wrapper {
    position: relative;
    display: flex;
    align-items: center;
}

.loading-indicator {
    position: absolute;
    right: 12px;
    font-size: 16px;
    animation: spin 1s linear infinite;
}

@keyframes spin {
    from {
        transform: rotate(0deg);
    }
    to {
        transform: rotate(360deg);
    }
}

/* ============ Button Group ============ */
.button-group {
    display: flex;
    gap: 12px;
    margin-top: 8px;
    flex-wrap: wrap;
}

.btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 10px 16px;
    border: 1px solid var(--vscode-button-border, transparent);
    border-radius: 4px;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    transition: all 200ms cubic-bezier(0.4, 0, 0.2, 1);
    font-family: var(--vscode-font-family);
    white-space: nowrap;
    flex: 1;
    min-width: 120px;
}

.btn:active {
    transform: scale(0.98);
}

.btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
    transform: none;
}

.btn-primary {
    background: var(--vscode-button-background);
    color: var(--vscode-button-foreground);
}

.btn-primary:hover:not(:disabled) {
    background: var(--vscode-button-hoverBackground, var(--vscode-button-background));
    transform: translateY(-2px);
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
}

.btn-secondary {
    background: var(--vscode-button-secondaryBackground);
    color: var(--vscode-button-secondaryForeground);
    border: 1px solid var(--vscode-focusBorder);
}

.btn-secondary:hover:not(:disabled) {
    background: var(--vscode-button-secondaryHoverBackground, var(--vscode-button-secondaryBackground));
    transform: translateY(-2px);
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.btn-icon {
    font-size: 14px;
}

.btn-text {
    font-size: 13px;
    font-weight: 500;
}

/* ============ Status Section ============ */
.status-section {
    padding: 12px 16px;
    background: var(--vscode-list-hoverBackground);
    border-left: 3px solid var(--vscode-textLink-foreground);
    border-radius: 4px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    animation: slideIn 200ms cubic-bezier(0.4, 0, 0.2, 1);
}

.status-indicator {
    display: flex;
    align-items: center;
    gap: 8px;
}

.status-icon-result {
    font-size: 18px;
    min-width: 20px;
}

.status-message {
    font-size: 13px;
    color: var(--vscode-editor-foreground);
    font-weight: 500;
}

.test-duration {
    font-size: 11px;
    color: var(--vscode-descriptionForeground);
    font-style: italic;
}

/* ============ Alert Section ============ */
.alert-section {
    padding: 12px 16px;
    border-radius: 4px;
    display: flex;
    gap: 8px;
    animation: slideIn 200ms cubic-bezier(0.4, 0, 0.2, 1);
}

.alert {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
}

.alert-icon {
    font-size: 16px;
    min-width: 20px;
}

.alert-message {
    font-size: 13px;
    line-height: 1.4;
}

.alert-section.error {
    background: rgba(244, 135, 113, 0.1);
    border-left: 3px solid #f48771;
}

.alert-section.error .alert-message {
    color: #f48771;
}

.alert-section.success {
    background: rgba(16, 169, 129, 0.1);
    border-left: 3px solid #10a981;
}

.alert-section.success .alert-message {
    color: #10a981;
}

.alert-section.warning {
    background: rgba(245, 213, 71, 0.1);
    border-left: 3px solid #f5d547;
}

.alert-section.warning .alert-message {
    color: #f5d547;
}

/* ============ Footer Info ============ */
.footer-info {
    text-align: center;
    padding-top: 16px;
    border-top: 1px solid var(--vscode-panel-border);
    font-size: 11px;
    color: var(--vscode-descriptionForeground);
    line-height: 1.6;
}

.footer-text {
    display: block;
}

.footer-text code {
    background: var(--vscode-list-hoverBackground);
    padding: 2px 6px;
    border-radius: 3px;
    font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
    font-size: 10px;
    color: var(--vscode-editor-foreground);
}

/* ============ Responsive Design ============ */
@media (max-width: 600px) {
    .container {
        padding: 16px;
        gap: 16px;
    }

    .header-top {
        flex-direction: column;
        align-items: flex-start;
    }

    .status-badge {
        align-self: flex-start;
    }

    .button-group {
        flex-direction: column;
    }

    .btn {
        flex: 1;
        min-width: 100%;
    }

    .form-group.inner-form-group {
        margin-left: 0;
    }

    .header-title {
        font-size: 18px;
    }
}

/* ============ Accessibility ============ */
button:focus-visible,
input:focus-visible,
select:focus-visible {
    outline: 2px solid var(--vscode-focusBorder);
    outline-offset: 2px;
}

code {
    word-break: break-all;
}

/* ============ Print Styles ============ */
@media print {
    .button-group,
    .advanced-toggle {
        display: none;
    }
}
`;
    }

    /**
     * Dispose of the panel
     */
    dispose() {
        AIConfigPanel.currentPanel = undefined;
        this._panel.dispose();
    }
}

module.exports = { AIConfigPanel, apiKeySecretKey };
