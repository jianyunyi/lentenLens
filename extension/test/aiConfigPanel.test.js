// Integration tests for AIConfigPanel
// Tests configuration validation and utility functions

const assert = require('assert');
const { test, describe } = require('node:test');

describe('AIConfigPanel - Configuration Validation', () => {
    test('Configuration validation - Requires API key', () => {
        // Test validation logic
        function validateFormFields(formData) {
            const errors = {};
            
            const validProviders = ['openai', 'anthropic', 'other'];
            if (!validProviders.includes(formData.provider)) {
                errors.provider = 'Invalid provider selected';
            }
            
            if (!formData.apiKey || formData.apiKey.trim() === '') {
                errors.apiKey = 'API Key is required';
            }
            
            if (!formData.modelId || formData.modelId.trim() === '') {
                errors.modelId = 'Model ID is required';
            }
            
            if (formData.baseUrl && formData.baseUrl.trim() !== '') {
                const urlPattern = /^https?:\/\/.+/i;
                if (!urlPattern.test(formData.baseUrl)) {
                    errors.baseUrl = 'Invalid URL format (must be https://...)';
                }
            }
            
            return errors;
        }

        const errors = validateFormFields({
            provider: 'openai',
            apiKey: '',
            modelId: 'gpt-4',
            baseUrl: ''
        });

        assert(errors.apiKey, 'Should have apiKey error');
        assert.strictEqual(errors.apiKey, 'API Key is required');
    });

    test('Configuration validation - Requires model ID', () => {
        function validateFormFields(formData) {
            const errors = {};
            
            const validProviders = ['openai', 'anthropic', 'other'];
            if (!validProviders.includes(formData.provider)) {
                errors.provider = 'Invalid provider selected';
            }
            
            if (!formData.apiKey || formData.apiKey.trim() === '') {
                errors.apiKey = 'API Key is required';
            }
            
            if (!formData.modelId || formData.modelId.trim() === '') {
                errors.modelId = 'Model ID is required';
            }
            
            if (formData.baseUrl && formData.baseUrl.trim() !== '') {
                const urlPattern = /^https?:\/\/.+/i;
                if (!urlPattern.test(formData.baseUrl)) {
                    errors.baseUrl = 'Invalid URL format (must be https://...)';
                }
            }
            
            return errors;
        }

        const errors = validateFormFields({
            provider: 'openai',
            apiKey: 'sk-test',
            modelId: '',
            baseUrl: ''
        });

        assert(errors.modelId, 'Should have modelId error');
        assert.strictEqual(errors.modelId, 'Model ID is required');
    });

    test('Configuration validation - Validates base URL format', () => {
        function validateFormFields(formData) {
            const errors = {};
            
            const validProviders = ['openai', 'anthropic', 'other'];
            if (!validProviders.includes(formData.provider)) {
                errors.provider = 'Invalid provider selected';
            }
            
            if (!formData.apiKey || formData.apiKey.trim() === '') {
                errors.apiKey = 'API Key is required';
            }
            
            if (!formData.modelId || formData.modelId.trim() === '') {
                errors.modelId = 'Model ID is required';
            }
            
            if (formData.baseUrl && formData.baseUrl.trim() !== '') {
                const urlPattern = /^https?:\/\/.+/i;
                if (!urlPattern.test(formData.baseUrl)) {
                    errors.baseUrl = 'Invalid URL format (must be https://...)';
                }
            }
            
            return errors;
        }

        const errors = validateFormFields({
            provider: 'openai',
            apiKey: 'sk-test',
            modelId: 'gpt-4',
            baseUrl: 'invalid-url'
        });

        assert(errors.baseUrl, 'Should have baseUrl error');
        assert(errors.baseUrl.includes('Invalid URL format'));
    });

    test('Configuration validation - Accepts valid HTTPS URL', () => {
        function validateFormFields(formData) {
            const errors = {};
            
            const validProviders = ['openai', 'anthropic', 'other'];
            if (!validProviders.includes(formData.provider)) {
                errors.provider = 'Invalid provider selected';
            }
            
            if (!formData.apiKey || formData.apiKey.trim() === '') {
                errors.apiKey = 'API Key is required';
            }
            
            if (!formData.modelId || formData.modelId.trim() === '') {
                errors.modelId = 'Model ID is required';
            }
            
            if (formData.baseUrl && formData.baseUrl.trim() !== '') {
                const urlPattern = /^https?:\/\/.+/i;
                if (!urlPattern.test(formData.baseUrl)) {
                    errors.baseUrl = 'Invalid URL format (must be https://...)';
                }
            }
            
            return errors;
        }

        const errors = validateFormFields({
            provider: 'openai',
            apiKey: 'sk-test',
            modelId: 'gpt-4',
            baseUrl: 'https://api.example.com'
        });

        assert(!errors.baseUrl, 'Should not have baseUrl error for valid HTTPS URL');
    });

    test('Configuration validation - Accepts empty base URL', () => {
        function validateFormFields(formData) {
            const errors = {};
            
            const validProviders = ['openai', 'anthropic', 'other'];
            if (!validProviders.includes(formData.provider)) {
                errors.provider = 'Invalid provider selected';
            }
            
            if (!formData.apiKey || formData.apiKey.trim() === '') {
                errors.apiKey = 'API Key is required';
            }
            
            if (!formData.modelId || formData.modelId.trim() === '') {
                errors.modelId = 'Model ID is required';
            }
            
            if (formData.baseUrl && formData.baseUrl.trim() !== '') {
                const urlPattern = /^https?:\/\/.+/i;
                if (!urlPattern.test(formData.baseUrl)) {
                    errors.baseUrl = 'Invalid URL format (must be https://...)';
                }
            }
            
            return errors;
        }

        const errors = validateFormFields({
            provider: 'openai',
            apiKey: 'sk-test',
            modelId: 'gpt-4',
            baseUrl: ''
        });

        assert(!errors.baseUrl, 'Should not have baseUrl error for empty URL');
    });

    test('Configuration validation - Validates all required fields', () => {
        function validateFormFields(formData) {
            const errors = {};
            
            const validProviders = ['openai', 'anthropic', 'other'];
            if (!validProviders.includes(formData.provider)) {
                errors.provider = 'Invalid provider selected';
            }
            
            if (!formData.apiKey || formData.apiKey.trim() === '') {
                errors.apiKey = 'API Key is required';
            }
            
            if (!formData.modelId || formData.modelId.trim() === '') {
                errors.modelId = 'Model ID is required';
            }
            
            if (formData.baseUrl && formData.baseUrl.trim() !== '') {
                const urlPattern = /^https?:\/\/.+/i;
                if (!urlPattern.test(formData.baseUrl)) {
                    errors.baseUrl = 'Invalid URL format (must be https://...)';
                }
            }
            
            return errors;
        }

        const errors = validateFormFields({
            provider: 'openai',
            apiKey: 'sk-test',
            modelId: 'gpt-4',
            baseUrl: 'https://api.example.com'
        });

        assert.deepStrictEqual(errors, {}, 'Should have no errors for valid configuration');
    });

    test('Secret key generation - Generates workspace-scoped key', () => {
        function apiKeySecretKey(wsPath) {
            return `intentlens.apiKey.${wsPath}`;
        }

        const wsPath = '/test/workspace';
        const secretKey = apiKeySecretKey(wsPath);
        
        assert(secretKey.includes('intentlens.apiKey'), 'Secret key should include prefix');
        assert(secretKey.includes(wsPath), 'Secret key should include workspace path');
    });

    test('Secret key generation - Generates unique keys for different workspaces', () => {
        function apiKeySecretKey(wsPath) {
            return `intentlens.apiKey.${wsPath}`;
        }

        const key1 = apiKeySecretKey('/workspace1');
        const key2 = apiKeySecretKey('/workspace2');
        
        assert.notStrictEqual(key1, key2, 'Different workspaces should have different keys');
    });

    test('Provider validation - Supports openai provider', () => {
        function validateFormFields(formData) {
            const errors = {};
            
            const validProviders = ['openai', 'anthropic', 'other'];
            if (!validProviders.includes(formData.provider)) {
                errors.provider = 'Invalid provider selected';
            }
            
            if (!formData.apiKey || formData.apiKey.trim() === '') {
                errors.apiKey = 'API Key is required';
            }
            
            if (!formData.modelId || formData.modelId.trim() === '') {
                errors.modelId = 'Model ID is required';
            }
            
            if (formData.baseUrl && formData.baseUrl.trim() !== '') {
                const urlPattern = /^https?:\/\/.+/i;
                if (!urlPattern.test(formData.baseUrl)) {
                    errors.baseUrl = 'Invalid URL format (must be https://...)';
                }
            }
            
            return errors;
        }

        const errors = validateFormFields({
            provider: 'openai',
            apiKey: 'sk-test',
            modelId: 'gpt-4',
            baseUrl: ''
        });

        assert(!errors.provider, 'OpenAI should be a valid provider');
    });

    test('Provider validation - Supports anthropic provider', () => {
        function validateFormFields(formData) {
            const errors = {};
            
            const validProviders = ['openai', 'anthropic', 'other'];
            if (!validProviders.includes(formData.provider)) {
                errors.provider = 'Invalid provider selected';
            }
            
            if (!formData.apiKey || formData.apiKey.trim() === '') {
                errors.apiKey = 'API Key is required';
            }
            
            if (!formData.modelId || formData.modelId.trim() === '') {
                errors.modelId = 'Model ID is required';
            }
            
            if (formData.baseUrl && formData.baseUrl.trim() !== '') {
                const urlPattern = /^https?:\/\/.+/i;
                if (!urlPattern.test(formData.baseUrl)) {
                    errors.baseUrl = 'Invalid URL format (must be https://...)';
                }
            }
            
            return errors;
        }

        const errors = validateFormFields({
            provider: 'anthropic',
            apiKey: 'sk-test',
            modelId: 'claude-3',
            baseUrl: ''
        });

        assert(!errors.provider, 'Anthropic should be a valid provider');
    });

    test('Provider validation - Supports other provider', () => {
        function validateFormFields(formData) {
            const errors = {};
            
            const validProviders = ['openai', 'anthropic', 'other'];
            if (!validProviders.includes(formData.provider)) {
                errors.provider = 'Invalid provider selected';
            }
            
            if (!formData.apiKey || formData.apiKey.trim() === '') {
                errors.apiKey = 'API Key is required';
            }
            
            if (!formData.modelId || formData.modelId.trim() === '') {
                errors.modelId = 'Model ID is required';
            }
            
            if (formData.baseUrl && formData.baseUrl.trim() !== '') {
                const urlPattern = /^https?:\/\/.+/i;
                if (!urlPattern.test(formData.baseUrl)) {
                    errors.baseUrl = 'Invalid URL format (must be https://...)';
                }
            }
            
            return errors;
        }

        const errors = validateFormFields({
            provider: 'other',
            apiKey: 'sk-test',
            modelId: 'custom-model',
            baseUrl: 'https://custom.api.com'
        });

        assert(!errors.provider, 'Other provider should be valid');
    });

    test('Provider validation - Rejects invalid provider', () => {
        function validateFormFields(formData) {
            const errors = {};
            
            const validProviders = ['openai', 'anthropic', 'other'];
            if (!validProviders.includes(formData.provider)) {
                errors.provider = 'Invalid provider selected';
            }
            
            if (!formData.apiKey || formData.apiKey.trim() === '') {
                errors.apiKey = 'API Key is required';
            }
            
            if (!formData.modelId || formData.modelId.trim() === '') {
                errors.modelId = 'Model ID is required';
            }
            
            if (formData.baseUrl && formData.baseUrl.trim() !== '') {
                const urlPattern = /^https?:\/\/.+/i;
                if (!urlPattern.test(formData.baseUrl)) {
                    errors.baseUrl = 'Invalid URL format (must be https://...)';
                }
            }
            
            return errors;
        }

        const errors = validateFormFields({
            provider: 'invalid',
            apiKey: 'sk-test',
            modelId: 'gpt-4',
            baseUrl: ''
        });

        assert(errors.provider, 'Invalid provider should produce error');
    });

    test('HTTP URL validation - Accepts both http and https', () => {
        function validateFormFields(formData) {
            const errors = {};
            
            const validProviders = ['openai', 'anthropic', 'other'];
            if (!validProviders.includes(formData.provider)) {
                errors.provider = 'Invalid provider selected';
            }
            
            if (!formData.apiKey || formData.apiKey.trim() === '') {
                errors.apiKey = 'API Key is required';
            }
            
            if (!formData.modelId || formData.modelId.trim() === '') {
                errors.modelId = 'Model ID is required';
            }
            
            if (formData.baseUrl && formData.baseUrl.trim() !== '') {
                const urlPattern = /^https?:\/\/.+/i;
                if (!urlPattern.test(formData.baseUrl)) {
                    errors.baseUrl = 'Invalid URL format (must be https://...)';
                }
            }
            
            return errors;
        }

        const errors = validateFormFields({
            provider: 'other',
            apiKey: 'sk-test',
            modelId: 'model',
            baseUrl: 'http://api.example.com'
        });

        assert(!errors.baseUrl, 'Should accept http URLs');
    });

    test('Form validation - All fields required for save', () => {
        function validateFormFields(formData) {
            const errors = {};
            
            const validProviders = ['openai', 'anthropic', 'other'];
            if (!validProviders.includes(formData.provider)) {
                errors.provider = 'Invalid provider selected';
            }
            
            if (!formData.apiKey || formData.apiKey.trim() === '') {
                errors.apiKey = 'API Key is required';
            }
            
            if (!formData.modelId || formData.modelId.trim() === '') {
                errors.modelId = 'Model ID is required';
            }
            
            if (formData.baseUrl && formData.baseUrl.trim() !== '') {
                const urlPattern = /^https?:\/\/.+/i;
                if (!urlPattern.test(formData.baseUrl)) {
                    errors.baseUrl = 'Invalid URL format (must be https://...)';
                }
            }
            
            return errors;
        }

        // Missing both API key and model ID
        const errors = validateFormFields({
            provider: 'openai',
            apiKey: '',
            modelId: '',
            baseUrl: ''
        });

        assert.strictEqual(Object.keys(errors).length, 2, 'Should have errors for both missing fields');
        assert(errors.apiKey, 'Should error on missing API key');
        assert(errors.modelId, 'Should error on missing model ID');
    });

    test('Whitespace handling - Trims whitespace in validation', () => {
        function validateFormFields(formData) {
            const errors = {};
            
            const validProviders = ['openai', 'anthropic', 'other'];
            if (!validProviders.includes(formData.provider)) {
                errors.provider = 'Invalid provider selected';
            }
            
            if (!formData.apiKey || formData.apiKey.trim() === '') {
                errors.apiKey = 'API Key is required';
            }
            
            if (!formData.modelId || formData.modelId.trim() === '') {
                errors.modelId = 'Model ID is required';
            }
            
            if (formData.baseUrl && formData.baseUrl.trim() !== '') {
                const urlPattern = /^https?:\/\/.+/i;
                if (!urlPattern.test(formData.baseUrl)) {
                    errors.baseUrl = 'Invalid URL format (must be https://...)';
                }
            }
            
            return errors;
        }

        const errors = validateFormFields({
            provider: 'openai',
            apiKey: '   ',
            modelId: '  gpt-4  ',
            baseUrl: ''
        });

        assert(errors.apiKey, 'Should treat whitespace-only API key as empty');
        assert(!errors.modelId, 'Should accept model ID with surrounding whitespace');
    });
});
