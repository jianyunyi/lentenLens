const test = require('node:test');
const assert = require('node:assert/strict');
const Module = require('node:module');
const core = require('../src/core');
const { workspaceSecretKey, SECURE_API_KEY_ENV } = require('../src/providers');

// Load the real panel AND extension wiring, substituting only editor APIs, CLI
// transport, SecretStorage and the provider's network response.
let h;
const disposable = { dispose() {} };
const vscode = {
  workspace: {
    isTrusted: true,
    get workspaceFolders() { return h.folders; },
    getWorkspaceFolder: uri => h.folders.find(f => f.uri.fsPath === uri.fsPath),
    getConfiguration: () => ({ get: (key, fallback) => h.editorSettings?.[key] ?? fallback }),
    createFileSystemWatcher: () => ({ ...disposable, onDidCreate: () => disposable, onDidChange: () => disposable, onDidDelete: () => disposable }),
    onDidChangeTextDocument: () => disposable, onDidGrantWorkspaceTrust: () => disposable,
    onDidSaveTextDocument: () => disposable, onDidChangeWorkspaceFolders: () => disposable,
  },
  window: {
    createTreeView: () => disposable,
    createStatusBarItem: () => { const bar = { ...disposable, show() {}, hide() {} }; h.bars.push(bar); return bar; },
    createWebviewPanel: () => {
      const view = { webview: { html: '',
        onDidReceiveMessage(fn) { view.receive = fn; } },
        onDidDispose(fn) { view.onDispose = fn; },
        dispose() { view.onDispose?.(); }, reveal() {} };
      h.views.push(view);
      return view;
    },
    showQuickPick: async (items, { title }) => {
      if (h.cancelWizard) return undefined;
      if (title.includes('厂商')) return items.find(item => item.provider === 'openai-compatible' && !item.base_url);
      return items[0];
    },
    showInputBox: async ({ title }) => title.includes('基础地址') ? h.baseURL : title.includes('API Key') ? 'fixture-key' : '4096',
    showWarningMessage: async (_text, _options, action) => {
      if (action === '发送此模型的测试请求') {
        h.afterConfirmation?.();
        if (h.cancelTest) return undefined;
      }
      return action;
    },
    showInformationMessage: message => h.infos.push(message),
    showErrorMessage: message => h.errors.push(message),
  },
  commands: { registerCommand: (name, fn) => { h.commands.set(name, fn); return disposable; } },
  languages: { registerCodeLensProvider: () => disposable },
  EventEmitter: class { event = () => {}; fire() {} dispose() {} },
  TreeItem: class {}, TreeItemCollapsibleState: { None: 0, Collapsed: 1 },
  ViewColumn: { Two: 2 }, StatusBarAlignment: { Right: 2 },
};
const originalLoad = Module._load;
const originalCLI = core.runCLI;
let activate, AIConfigPanel;
try {
  core.runCLI = async (_binary, root, command, input, options) => {
    h.calls.push({ root, command, input, options });
    if (command === 'status') {
      if (h.statusError) throw new Error('status unavailable');
      return { settings: structuredClone(h.settings), usage: {} };
    }
    if (command === 'configure') {
      await h.onConfigure?.();
      if (h.configureUnconfirmed) return { saved: false };
      h.settings = structuredClone(input.settings);
      return { saved: true };
    }
    if (command === 'test-model') {
      if (h.testError) throw new Error('probe failed');
      return h.testResult;
    }
    throw new Error(`unexpected command ${command}`);
  };
  Module._load = function(id, ...args) { return id === 'vscode' ? vscode : originalLoad.call(this, id, ...args); };
  ({ AIConfigPanel } = require('../src/aiConfigPanel'));
  ({ activate } = require('../src/extension'));
} finally {
  Module._load = originalLoad;
  core.runCLI = originalCLI;
}

function setup(t) {
  const folder = { name: 'fixture', uri: { scheme: 'file', fsPath: '/fixture/project' } };
  h = { folder, folders: [folder], settings: { model: { token_limit: 200000 } },
    baseURL: 'https://api.fixture/v1', commands: new Map(), stored: new Map(),
    calls: [], requests: [], infos: [], errors: [], views: [], bars: [],
    testResult: { tested: true, actual_tokens: 9, fallback: false, project_data_sent: false } };
  const secrets = {
    get: async name => h.stored.get(name),
    store: async (name, value) => { await h.onStore?.(name); h.stored.set(name, value); },
    delete: async name => h.stored.delete(name),
  };
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url, options) => {
    h.requests.push({ url, options });
    return new Response(JSON.stringify({ data: [{ id: 'returned-model' }] }), { status: h.queryFails ? 401 : 200 });
  };
  t.after(() => { AIConfigPanel.currentPanel?._panel.dispose(); globalThis.fetch = originalFetch; });
  activate({ subscriptions: [], extensionPath: '/fixture/extension', secrets });
  h.open = async () => {
    const panel = await h.commands.get('intentlens.configureAPI')();
    assert.ok(panel instanceof AIConfigPanel);
    return panel;
  };
  h.send = action => h.views.at(-1).receive({ action });
  h.save = async () => { await h.open(); await h.send('save'); };
  h.name = workspaceSecretKey(folder.uri.fsPath);
  return h;
}
function deferred() {
  let resolve;
  const promise = new Promise(done => { resolve = done; });
  return { promise, resolve };
}
function panelHTML() { return h.views.at(-1).webview.html; }
function assertNoSaveSuccess() {
  assert.doesNotMatch(panelHTML(), /配置与密钥绑定已保存/);
  assert.ok(!h.infos.some(message => message.includes('安全保存')));
}

test('configureAPI and the status bar open the real panel and save through the shared wizard', async t => {
  setup(t);
  const panel = await h.open();
  assert.equal(h.bars.find(bar => bar.command === 'intentlens.configureAPI').text, '$(gear) IntentLens API');
  const manifest = require('../package.json');
  assert.ok(manifest.contributes.viewsWelcome.some(item => item.contents.includes('command:intentlens.configureAPI')));
  assert.ok(manifest.contributes.commands.some(item => item.command === 'intentlens.configureAPI'));
  for (const key of ['apiProvider', 'apiModelId', 'apiBaseUrl', 'configStatus', 'lastValidation']) {
    assert.equal(manifest.contributes.configuration.properties['intentlens.' + key], undefined);
  }
  await h.send('save');
  assert.match(panelHTML(), /配置与密钥绑定已保存/);
  assert.equal(h.calls.filter(call => call.command === 'configure').length, 1);
  assert.equal(h.stored.get(h.name), 'fixture-key');
  assert.ok(h.stored.has(h.name + '.binding'));
  assert.equal(h.settings.model.api_key_env, SECURE_API_KEY_ENV);
  assert.equal(h.settings.model.model, 'returned-model');
  assert.equal(panel._configurationData.credentialBound, true);
  assert.match(panel._panel.webview.html, /已配置并启用/);
  assert.doesNotMatch(panel._panel.webview.html, /fixture-key|<input|<select/);
});

test('save cannot report success while the backend commit is pending', async t => {
  setup(t);
  await h.open();
  const entered = deferred(), release = deferred();
  h.onConfigure = async () => { entered.resolve(); await release.promise; };
  const saving = h.send('save');
  try {
    await entered.promise;
    assertNoSaveSuccess();
    assert.equal(h.stored.size, 0);
  } finally { release.resolve(); await saving; }
  assert.match(panelHTML(), /配置与密钥绑定已保存/);
});

test('save cannot report success until the matching SecretStorage binding is committed', async t => {
  setup(t);
  await h.open();
  const entered = deferred(), release = deferred();
  h.onStore = async name => { if (name.endsWith('.binding')) { entered.resolve(); await release.promise; } };
  const saving = h.send('save');
  try {
    await entered.promise;
    assert.equal(h.settings.model.model, 'returned-model');
    assertNoSaveSuccess();
    assert.equal(h.stored.has(h.name + '.binding'), false);
  } finally { release.resolve(); await saving; }
  assert.match(panelHTML(), /配置与密钥绑定已保存/);
});

test('backend configure failure reports an error and never reports saved', async t => {
  setup(t);
  h.onConfigure = () => { throw new Error('backend commit failed'); };
  await h.save();
  assertNoSaveSuccess();
  assert.equal(h.stored.size, 0);
  assert.match(panelHTML(), /backend commit failed/);
  assert.match(h.views.at(-1).webview.html, /需要配置并绑定 API Key/);
});

test('an unconfirmed backend response does not write credentials or report saved', async t => {
  setup(t);
  h.configureUnconfirmed = true;
  await h.save();
  assertNoSaveSuccess();
  assert.equal(h.stored.size, 0);
  assert.match(panelHTML(), /后端未确认配置已保存/);
});

test('binding failure uses the existing rollback and never reports saved', async t => {
  setup(t);
  const previous = structuredClone(h.settings);
  h.onStore = name => { if (name.endsWith('.binding')) throw new Error('binding write failed'); };
  await h.save();
  assertNoSaveSuccess();
  assert.deepEqual(h.settings, previous);
  assert.match(panelHTML(), /binding write failed/);
});

test('cancelling the shared wizard makes no backend or secret write', async t => {
  setup(t);
  h.cancelWizard = true;
  await h.save();
  assertNoSaveSuccess();
  assert.ok(h.calls.every(call => call.command === 'status'));
  assert.equal(h.stored.size, 0);
});

test('querying models uses the real provider discovery inside the wizard', async t => {
  setup(t);
  await h.save();
  assert.equal(h.requests.length, 1);
  assert.equal(h.requests[0].url, h.baseURL + '/models');
  assert.equal(h.requests[0].options.headers.authorization, 'Bearer fixture-key');
  assert.equal(h.settings.model.model, 'returned-model');
  assert.ok(h.calls.every(call => ['status', 'configure'].includes(call.command)));
  assert.doesNotMatch(h.views.at(-1).webview.html, /data-action="fetchModels"/);
  const before = h.calls.length;
  await h.send('fetchModels');
  assert.equal(h.calls.length, before, 'retired query messages do not dispatch a nonexistent CLI command');
});

test('provider discovery failure prevents saving', async t => {
  setup(t);
  h.queryFails = true;
  await h.save();
  assertNoSaveSuccess();
  assert.ok(h.calls.every(call => call.command === 'status'));
  assert.match(panelHTML(), /HTTP 401/);
});

test('status reads the backend and binding, ignoring retired panel settings and secrets', async t => {
  setup(t);
  h.editorSettings = { apiProvider: 'anthropic', apiModelId: 'retired-model',
    apiBaseUrl: 'https://retired.fixture', configStatus: 'configured' };
  h.stored.set('intentlens.apiKey.' + h.folder.uri.fsPath, 'retired-key');
  const panel = await h.open();
  assert.equal(panel._configurationData.credentialBound, false);
  assert.match(panel._panel.webview.html, /需要配置并绑定 API Key/);
  assert.doesNotMatch(panelHTML(), /retired-model|retired.fixture/);
  await h.send('save');
  h.settings.model.model = 'backend-model-change';
  h.settings.model.enabled = false;
  await h.send('refresh');
  assert.match(panelHTML(), /backend-model-change/);
  assert.match(panelHTML(), /模型分析已暂停/);
  h.stored.delete(h.name + '.binding');
  await h.send('refresh');
  assert.equal(panel._configurationData.credentialBound, false);
  assert.doesNotMatch(panel._panel.webview.html, /已配置并启用/);
});

test('connection testing dispatches test-model with the saved credential and all three target fields', async t => {
  setup(t);
  await h.save();
  await h.send('testConnection');
  const probe = h.calls.find(call => call.command === 'test-model');
  assert.ok(probe);
  assert.deepEqual(probe.input, {});
  assert.equal(probe.options.env.INTENTLENS_EDITOR_API_KEY, 'fixture-key');
  assert.deepEqual(JSON.parse(probe.options.args[1]), {
    provider: h.settings.model.provider, base_url: h.settings.model.base_url, model: h.settings.model.model,
  });
  assert.match(panelHTML(), /连接测试成功.*9 tokens/);
});

for (const [field, value] of [['provider', 'openai'], ['base_url', 'https://changed.fixture/v1'], ['model', 'changed-model']]) {
  test(`connection testing refuses a changed ${field} after confirmation`, async t => {
    setup(t);
    await h.save();
    h.afterConfirmation = () => { h.settings.model[field] = value; };
    await h.send('testConnection');
    assert.ok(h.calls.every(call => call.command !== 'test-model'));
    assert.doesNotMatch(panelHTML(), /连接测试成功/);
  });
}

test('connection testing without a valid binding never sends a probe', async t => {
  setup(t);
  await h.open();
  await h.send('testConnection');
  assert.ok(h.calls.every(call => call.command === 'status'));
  assert.doesNotMatch(panelHTML(), /连接测试成功/);
  assert.match(panelHTML(), /未绑定/);
});

test('connection errors and malformed probe results never report success', async t => {
  setup(t);
  await h.save();
  h.testError = true;
  await h.send('testConnection');
  assert.doesNotMatch(panelHTML(), /连接测试成功/);
  assert.match(panelHTML(), /probe failed/);
  h.testError = false;
  h.testResult = { tested: true }; // Missing the fixed-probe assurances.
  await h.send('testConnection');
  assert.doesNotMatch(panelHTML(), /连接测试成功/);
  assert.match(panelHTML(), /未确认模型返回完整/);
});

test('cancelled connection testing never sends a probe or claims success', async t => {
  setup(t);
  await h.save();
  h.cancelTest = true;
  await h.send('testConnection');
  assert.ok(h.calls.every(call => call.command !== 'test-model'));
  assert.doesNotMatch(panelHTML(), /连接测试成功|操作失败/);
});

test('backend read failure clears the previously displayed configured state', async t => {
  setup(t);
  await h.save();
  h.statusError = true;
  await h.send('refresh');
  assert.match(h.views.at(-1).webview.html, /配置状态未知/);
  assert.doesNotMatch(h.views.at(-1).webview.html, /已配置并启用/);
  assert.match(panelHTML(), /status unavailable/);
});

test('switching workspace replaces the panel and keeps its backend target explicit', async t => {
  setup(t);
  const oldPanel = await h.open();
  const other = { name: 'other', uri: { scheme: 'file', fsPath: '/fixture/other' } };
  h.folders = [other];
  const panel = await h.open();
  assert.notEqual(panel, oldPanel);
  assert.equal(oldPanel._disposed, true);
  assert.equal(panel._folder, other);
  assert.equal(h.calls.at(-1).root, other.uri.fsPath);
});
