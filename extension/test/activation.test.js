const test = require('node:test');
const assert = require('node:assert/strict');
const Module = require('node:module');
const fs = require('node:fs');
const path = require('node:path');
const core = require('../src/core');

test('extension registers testModel in manifest, activates it, and dispatches to the matching CLI command', async () => {
  const file = path.join(__dirname, '../src/extension.js');
  assert.ok(fs.existsSync(file), 'extension entry exists');
  const registrations = [], commands = new Map();
  const cliCalls = [], errors = [], infos = [];
  let modelResult = { tested: true, actual_tokens: 9, fallback: false, project_data_sent: false };
  const disposable = { dispose() {} };
  const state = new Map();
  const workspaceState = { get: key => state.get(key), update: (key, value) => { if (value === undefined) state.delete(key); else state.set(key, value); return Promise.resolve(); } };
  const folder = { name: 'fixture', uri: { scheme: 'file', fsPath: '/fixture/project' } };
  const vscode = {
    workspace: { isTrusted: false, workspaceFolders: [], getConfiguration: () => ({ get: () => '' }), createFileSystemWatcher: () => ({ ...disposable, onDidCreate: () => disposable, onDidChange: () => disposable, onDidDelete: () => disposable }), onDidChangeTextDocument: () => ({ dispose() {} }), onDidGrantWorkspaceTrust: () => disposable, onDidSaveTextDocument: () => disposable, onDidChangeWorkspaceFolders: () => disposable, workspaceState },
    window: { createTreeView: name => { registrations.push(name); return disposable; }, showErrorMessage: message => errors.push(message), showWarningMessage: async () => '发送此模型的测试请求', showInformationMessage: message => infos.push(message) },
    commands: { registerCommand: (name, fn) => { commands.set(name, fn); return disposable; } },
    languages: { registerCodeLensProvider: selector => { registrations.push(selector); return disposable; } },
    EventEmitter: class { event = () => {}; fire() {} dispose() {} },
    TreeItem: class {}, TreeItemCollapsibleState: { None: 0, Collapsed: 1 },
  };
  const original = Module._load;
  const originalRunCLI = core.runCLI;
  core.runCLI = async (_binary, _root, command, input, options) => {
    cliCalls.push({ command, input, options });
    if (command === 'status') return { settings: { model: { enabled: true, provider: 'openai-compatible', base_url: 'https://api.fixture/v1', model: 'returned-model-id', api_key_env: 'INTENTLENS_API_KEY' } }, usage: {} };
    if (command === 'test-model') return modelResult;
    throw new Error(`unexpected CLI command ${command}`);
  };
  Module._load = function (id, ...args) { return id === 'vscode' ? vscode : original.call(this, id, ...args); };
  try {
    delete require.cache[require.resolve(file)];
    require(file).activate({ subscriptions: [], extensionPath: path.dirname(path.dirname(file)), workspaceState, secrets: { get: async () => 'fixture-api-key' } });
    for (const name of ['inspect', 'enableModel', 'testModel', 'pauseModel', 'hooksConfig', 'startRound', 'stopRound', 'resumeRound', 'cancelRound', 'callPath', 'refreshCallPath', 'refreshModelAnswer', 'openModelEvidence']) {
      assert.ok(commands.has(`intentlens.${name}`), `${name} command is registered`);
    }
    const manifest = require('../package.json');
    assert.ok(manifest.activationEvents.includes('onCommand:intentlens.testModel'));
    assert.ok(manifest.contributes.commands.some(item => item.command === 'intentlens.testModel'));
    assert.ok(manifest.contributes.menus['view/title'].some(item => item.command === 'intentlens.testModel'));
    for (const name of ['startRound', 'stopRound', 'resumeRound', 'cancelRound']) {
      const id = `intentlens.${name}`;
      assert.ok(manifest.contributes.commands.some(item => item.command === id));
      assert.ok(manifest.contributes.menus['view/title'].some(item => item.command === id));
    }
    for (const name of ['callPath', 'refreshCallPath', 'refreshModelAnswer']) {
      assert.ok(manifest.contributes.commands.some(item => item.command === `intentlens.${name}`));
    }
    assert.equal(manifest.contributes.commands.find(item => item.command === 'intentlens.refreshCallPath').icon, '$(refresh)');
    assert.ok(manifest.contributes.viewsWelcome.some(item => item.view === 'intentlens.callPath' && item.contents.includes('command:intentlens.callPath')));
    assert.ok(manifest.contributes.viewsWelcome.some(item => item.view === 'intentlens.modelAnswer' && item.contents.includes('不会发起模型请求')));
    assert.ok(manifest.contributes.menus['view/title'].some(item => item.command === 'intentlens.refreshModelAnswer' && item.when === 'view == intentlens.modelAnswer'));
    assert.ok(registrations.includes('intentlens.rounds'));
    assert.ok(registrations.includes('intentlens.callPath'));
    assert.ok(registrations.includes('intentlens.modelAnswer'));
    assert.ok(registrations.some(x => x.language === 'go' && x.scheme === 'file'));
    vscode.workspace.isTrusted = true;
    vscode.workspace.workspaceFolders = [folder];
    await commands.get('intentlens.testModel')();
    assert.deepEqual(cliCalls.map(call => call.command), ['status', 'test-model']);
    assert.deepEqual(cliCalls[1].input, {});
    assert.equal(cliCalls[1].options.env.INTENTLENS_API_KEY, 'fixture-api-key');
    assert.deepEqual(errors, []);
    assert.match(infos[0], /返回完整且符合固定格式/);
    modelResult = { tested: false, actual_tokens: 9, fallback: false, project_data_sent: false };
    await commands.get('intentlens.testModel')();
    assert.match(errors[0], /未确认模型返回完整/);
  } finally { Module._load = original; core.runCLI = originalRunCLI; }
});
