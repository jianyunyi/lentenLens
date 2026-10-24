const { test } = require('node:test');
const assert = require('node:assert/strict');
const { configureModel, presets, verificationModes, validateBase } = require('../src/providers');
test('provider configuration confirms exact destination and cancellation never saves', async () => {
  for (const choice of presets) {
    const inputs = [choice.base_url || 'https://custom.example/v1', 'fixture-model', choice.api_key_env, '123456'];
    let index = 0, pickIndex = 0, saved, confirmation;
    const vscode = { window: {
      showQuickPick: async items => pickIndex++ === 0 ? choice : items.find(item => item.value === 'light'), showInputBox: async () => inputs[index++],
      showWarningMessage: async text => { confirmation = text; return 'cancel'; }, showInformationMessage() {},
    } };
    const cli = async (_, command, request) => { saved = { command, request }; };
    const status = { settings: { model: { token_limit: 200000 } }, usage: { actual_tokens: 42, uncertain_tokens: 10 } };
    assert.equal(await configureModel(vscode, { name: 'fixture' }, status, cli), false);
    assert.equal(saved, undefined);
    assert.ok(confirmation.includes(inputs[0])); assert.ok(confirmation.includes('42'));
    index = 0; pickIndex = 0; vscode.window.showWarningMessage = async text => { confirmation = text; return '同意并启用'; };
    assert.equal(await configureModel(vscode, { name: 'fixture' }, status, cli), true);
    assert.equal(saved.request.settings.model.provider, choice.provider);
    assert.equal(saved.request.settings.model.api_key_env, choice.api_key_env);
    assert.equal(saved.request.settings.model.verification_mode, 'light');
    assert.equal(saved.request.consent, true);
    assert.ok(!('api_key' in saved.request.settings.model));
    assert.match(confirmation, /轻量复核/);
  }
});
test('fast and strict choices are persisted and accurately described', async () => {
  for (const selected of ['fast', 'strict']) {
    let picks = 0, index = 0, saved, confirmation, modeItems;
    const inputs = ['https://deepseek.example/v1', 'fixture-model', 'DEEPSEEK_API_KEY', '200000'];
    const vscode = { window: {
      showQuickPick: async items => { if (picks++ === 0) return { ...items.find(item => item.provider === 'openai-compatible'), provider: 'openai-compatible', label: 'DeepSeek' }; modeItems = items; return items.find(item => item.value === selected); },
      showInputBox: async () => inputs[index++],
      showWarningMessage: async text => { confirmation = text; return '同意并启用'; }, showInformationMessage() {},
    } };
    const cli = async (_, command, request) => { saved = { command, request }; };
    assert.equal(await configureModel(vscode, { name: 'fixture' }, { settings: { model: { verification_mode: 'strict' } }, usage: {} }, cli), true);
    assert.equal(saved.request.settings.model.verification_mode, selected);
    assert.match(confirmation, selected === 'fast' ? /不做独立复核/ : /逐条检查所有说法/);
    assert.equal(modeItems[0].value, 'strict');
    assert.match(modeItems[0].label, /当前/);
  }
  assert.deepEqual(verificationModes.map(mode => mode.value), ['light', 'fast', 'strict']);
});
test('base URL checks reject credentials, query and remote plaintext', () => {
  for (const value of ['http://remote.test/v1','https://a:b@host/v1','https://host/v1?key=abc','https://host/v1#x','https://host/v1/messages']) assert.ok(validateBase(value));
  for (const value of ['https://host/v1','http://127.0.0.1:8080/v1','http://[::1]:8080/v1']) assert.equal(validateBase(value), null);
});
