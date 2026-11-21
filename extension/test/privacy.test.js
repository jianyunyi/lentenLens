const test = require('node:test');
const assert = require('node:assert/strict');
const Module = require('node:module');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { createHash } = require('node:crypto');
const core = require('../src/core');
const { workspaceSecretKey, SECURE_API_KEY_ENV } = require('../src/providers');

function harness(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'intentlens-ui-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  fs.writeFileSync(path.join(root, 'main.go'), 'package main');
  fs.writeFileSync(path.join(root, 'dep.go'), 'package main');
  const uri = file => ({ scheme: 'file', fsPath: file, toString: () => file });
  const folder = { name: 'test', uri: uri(root) }, commands = new Map(), calls = [], panels = [], watchers = [], errors = [], documents = [];
  const target = { provider: 'openai-compatible', base_url: 'https://api.fixture/v1' };
  const modelStatus = { settings: { model: { ...target, api_key_env: SECURE_API_KEY_ENV } } };
  const key = 'privacy-fixture-key', name = workspaceSecretKey(root);
  const binding = JSON.stringify({ ...target, digest: createHash('sha256').update(key).digest('hex') });
  const stored = new Map([[name, key], [name + '.binding', binding]]);
  const secrets = { get: async name => stored.get(name) };
  const state = { inspection: { function: { name: 'F', source: 'safe' }, dependencies: [{ name: 'G', file: 'dep.go', start_line: 1 }], evidence: [{ id: 'e', file: 'main.go', source: 'secret', start_line: 1 }], history: [], issues: [] }, rounds: [{ id: 'r', changes: [], notes: [] }], refreshes: 0 };
  const d = { dispose() {} };
  const vscode = {
    workspace: { isTrusted: true, workspaceFolders: [folder], getWorkspaceFolder: () => folder, getConfiguration: () => ({ get: () => '' }), onDidChangeTextDocument: fn => { state.changed = fn; return d; }, onDidGrantWorkspaceTrust: () => d, onDidSaveTextDocument: () => d, onDidChangeWorkspaceFolders: () => d,
      createFileSystemWatcher: pattern => { const w = { pattern, dispose() {}, onDidCreate(fn) { this.create = fn; return d; }, onDidChange(fn) { this.change = fn; return d; }, onDidDelete(fn) { this.delete = fn; return d; } }; watchers.push(w); return w; },
      openTextDocument: async value => { documents.push(value); return value; } },
    window: { createStatusBarItem: () => { const item = { visible: false, show() { this.visible = true; }, hide() { this.visible = false; }, dispose() {} }; state.indexBar ||= item; return item; }, createTreeView: () => d, showErrorMessage: text => errors.push(text), showInformationMessage: () => {}, showWarningMessage: async () => undefined, withProgress: (_, fn) => fn(), showTextDocument: async () => ({}), createWebviewPanel: () => { const p = { disposed: false, onDidDispose(fn) { this.onDispose = fn; return d; }, dispose() { this.disposed = true; this.onDispose?.(); }, webview: { html: '', onDidReceiveMessage(fn) { p.message = fn; return d; } } }; panels.push(p); return p; } },
    commands: { registerCommand: (name, fn) => { commands.set(name, fn); return d; } }, languages: { registerCodeLensProvider: (_, provider) => { state.codeLensProvider = provider; return d; } },
    Range: class { constructor(line, character, endLine, endCharacter) { this.start = { line, character }; this.end = { line: endLine, character: endCharacter }; } },
    CodeLens: class { constructor(range, command) { this.range = range; this.command = command; } }, StatusBarAlignment: { Right: 2 },
    EventEmitter: class { event = () => {}; fire() { state.refreshes++; } dispose() {} }, TreeItem: class {}, TreeItemCollapsibleState: { None: 0, Collapsed: 1 }, ViewColumn: { Beside: 2 }, ProgressLocation: { Notification: 1 }, Uri: { file: uri },
  };
  const original = Module._load;
  Module._load = function(id, ...args) {
    if (id === 'vscode') return vscode;
    if (id === './core') return { ...core, runCLI: async (_, __, command, input) => {
      calls.push({ command, input });
      await state.onCLI?.(command, input);
      if (command === 'inspect') return structuredClone(state.inspection);
      if (command === 'rounds') return structuredClone(state.rounds);
      return structuredClone(state.responses?.[command] ?? (command === 'status' ? modelStatus : {}));
    } };
    return original.call(this, id, ...args);
  };
  try { delete require.cache[require.resolve('../src/extension')]; require('../src/extension').activate({ subscriptions: [], extensionPath: root, secrets }); }
  finally { Module._load = original; }
  return { folder, commands, calls, panels, watchers, errors, documents, state, uri, vscode };
}
test('all external ignore events dispose cached panels, refresh UI, and run privacy cleanup', async t => {
  const h = harness(t);
  assert.ok(h.watchers.length, 'ignore watcher registered');
  for (const event of ['create', 'change', 'delete']) {
    await h.commands.get('intentlens.openRound')(h.folder, { id: 'r' });
    const panel = h.panels.at(-1), before = h.state.refreshes;
    await h.watchers[0][event](h.uri(path.join(h.folder.uri.fsPath, '.intentignore')));
    assert.equal(panel.disposed, true);
    assert.ok(h.state.refreshes > before);
    assert.equal(h.calls.at(-1).command, 'status');
  }
});

test('source edit while entering a question cancels generation at the old line', async t => {
  const h = harness(t);
  await h.commands.get('intentlens.inspectDetails')(h.uri(path.join(h.folder.uri.fsPath, 'main.go')), 1);
  let resolveInput;
  h.vscode.window.showInputBox = () => new Promise(resolve => { resolveInput = resolve; });
  h.vscode.ProgressLocation = { Notification: 1 };
  h.vscode.window.withProgress = (_, fn) => fn();
  const pending = h.panels[0].message({ action: 'ask' });
  while (!resolveInput) await new Promise(resolve => setImmediate(resolve));
  h.state.changed({ document: { uri: h.uri(path.join(h.folder.uri.fsPath, 'main.go')) }, contentChanges: [{}] });
  resolveInput('Explain the old function');
  await pending;
  assert.equal(h.calls.some(c => c.command === 'explain'), false);
});
test('opening cached tree round fetches allowed current round rather than cached secret', async t => {
  const h = harness(t);
  await h.commands.get('intentlens.openRound')(h.folder, { id: 'r', changes: [{ path: 'main.go', before: { content: 'cached-secret' } }] });
  assert.equal(h.calls[0]?.command, 'rounds');
  assert.ok(!h.panels[0].webview.html.includes('cached-secret'));
});
test('evidence action re-inspects and refuses evidence removed by policy', async t => {
  const h = harness(t);
  await h.commands.get('intentlens.inspectDetails')(h.uri(path.join(h.folder.uri.fsPath, 'main.go')), 1);
  h.state.inspection.evidence = [];
  await h.panels[0].message({ action: 'snapshot', group: 'current', index: '0' });
  assert.equal(h.calls.filter(c => c.command === 'inspect').length, 2);
  assert.equal(h.documents.length, 0);
  assert.ok(!h.panels[0].webview.html.includes('secret'));
});
test('dependency opens its own inspection directly', async t => {
  const h = harness(t);
  await h.commands.get('intentlens.inspectDetails')(h.uri(path.join(h.folder.uri.fsPath, 'main.go')), 1);
  await h.panels[0].message({ action: 'dependency', index: '0' });
  assert.equal(h.calls.at(-1).input.file, 'dep.go');
  assert.equal(h.panels.length, 2);
});

test('sharing requires a separate preview confirmation and cancel never publishes', async t => {
  const h = harness(t);
  const entry = { id: 'source', document: { claims: [] }, evidence: [] };
  h.state.inspection.history = [entry];
  h.state.responses = { 'share-prepare': { id: 'draft', digest: 'preview-hash', record: { id: 'draft', document: { claims: [{ text: '<script>unsafe</script>' }] } } } };
  h.vscode.window.showInformationMessage = () => {};
  await h.commands.get('intentlens.inspectDetails')(h.uri(path.join(h.folder.uri.fsPath, 'main.go')), 1);
  const source = h.panels[0];
  await source.message({ action: 'share', group: '0', index: '' });
  assert.equal(h.calls.filter(c => c.command === 'share-confirm').length, 0);
  assert.ok(h.panels.at(-1).webview.html.includes('&lt;script&gt;unsafe'));
  await h.panels.at(-1).message({ action: 'cancel' });
  assert.equal(h.calls.filter(c => c.command === 'share-confirm').length, 0);
  await source.message({ action: 'share', group: '0', index: '' });
  await h.panels.at(-1).message({ action: 'confirmShare', draft_id: 'forged' });
  assert.deepEqual(h.calls.find(c => c.command === 'share-confirm').input, { draft_id: 'draft', preview_digest: 'preview-hash', confirm: true });
  assert.deepEqual(h.errors, []);
});

test('only explicitly selected private evidence is approved and flag uses selected claim', async t => {
  const h = harness(t);
  h.state.inspection.history = [{ id: 'source', document: { claims: [{ id: 'c1', text: 'claim' }] }, evidence: [{ id: 'private1', kind: 'prompt', source: 'private' }, { id: 'private2', kind: 'self_report', source: 'other' }] }];
  h.state.responses = { 'share-prepare': { id: 'draft', digest: 'hash', record: {} } };
  h.vscode.window.showQuickPick = async options => [options[0]];
  h.vscode.window.showInputBox = async () => 'evidence does not support claim';
  await h.commands.get('intentlens.inspectDetails')(h.uri(path.join(h.folder.uri.fsPath, 'main.go')), 1);
  await h.panels[0].message({ action: 'share', group: '0', index: '' });
  assert.deepEqual(h.calls.find(c => c.command === 'share-prepare').input.approved_evidence_ids, ['private1']);
  await h.panels[0].message({ action: 'flag', group: '0', index: '0' });
  assert.deepEqual(h.calls.find(c => c.command === 'flag').input, { explanation_id: 'source', claim_id: 'c1', reason: 'evidence does not support claim' });
  assert.deepEqual(h.errors, []);
});

test('recipient creates local review state before flagging a shared claim', async t => {
  const h = harness(t);
  const record = { id: 'shared-id', function_id: 'main.go:F', document: { claims: [{ id: 'claim', text: 'shared claim' }] }, evidence: [] };
  h.state.responses = { shared: [record], 'adopt-shared': record, explanation: record };
  h.vscode.window.showQuickPick = async items => items[0];
  h.vscode.window.showInputBox = async () => 'local reason';
  await h.commands.get('intentlens.shared')();
  await h.panels[0].message({ action: 'adopt' });
  await h.panels[1].message({ action: 'flag', index: '0', group: 'saved' });
  assert.equal(h.calls.find(c => c.command === 'adopt-shared').input.explanation_id, record.id);
  assert.equal(h.calls.find(c => c.command === 'flag').input.reason, 'local reason');
  assert.equal(h.calls.filter(c => c.command === 'share-confirm').length, 0);
  assert.deepEqual(h.errors, []);
});

// These exercise the registered extension handlers, not just the HTML renderer.
test('round explanation requires confirmation, uses server round ID, and refreshes generated analysis', async t => {
  const h = harness(t);
  await h.commands.get('intentlens.openRound')(h.folder, { id: 'r' });
  const panel = h.panels[0];
  assert.match(panel.webview.html, /生成轮次解释/);
  assert.equal(h.calls.some(c => c.command === 'analyze-round'), false);
  await panel.message({ action: 'analyzeRound', round_id: 'forged' });
  assert.equal(h.calls.some(c => c.command === 'analyze-round'), false, 'dismissed confirmation never requests model');
  h.vscode.window.showWarningMessage = async (_, options, action) => {
    assert.equal(options.modal, true);
    assert.match(options.detail, /模型 token 预算/);
    return action;
  };
  let version = 0;
  h.state.onCLI = command => {
    if (command === 'analyze-round') h.state.rounds[0].analysis = { id: `generated-${++version}`, document: { claims: [] }, evidence: [] };
  };
  await panel.message({ action: 'analyzeRound', round_id: 'forged' });
  assert.deepEqual(h.calls.find(c => c.command === 'analyze-round').input, { round_id: 'r' });
  assert.match(panel.webview.html, /generated-1/);
  assert.match(panel.webview.html, /重新生成轮次解释/);
  await panel.message({ action: 'analyzeRound' });
  assert.match(panel.webview.html, /generated-2/);
  assert.equal(h.calls.filter(c => c.command === 'analyze-round').length, 2);
  assert.deepEqual(h.errors, []);
});

test('round analysis ignores duplicate clicks during confirmation and request', async t => {
  const h = harness(t);
  await h.commands.get('intentlens.openRound')(h.folder, { id: 'r' });
  let confirm, finish;
  h.vscode.window.showWarningMessage = () => new Promise(resolve => { confirm = resolve; });
  h.state.onCLI = command => command === 'analyze-round' ? new Promise(resolve => { finish = resolve; }) : undefined;
  const panel = h.panels[0], first = panel.message({ action: 'analyzeRound' });
  while (!confirm) await new Promise(resolve => setImmediate(resolve));
  await panel.message({ action: 'analyzeRound' });
  confirm('生成轮次解释');
  while (!finish) await new Promise(resolve => setImmediate(resolve));
  await panel.message({ action: 'analyzeRound' });
  assert.equal(h.calls.filter(c => c.command === 'analyze-round').length, 1);
  finish();
  await first;
  assert.deepEqual(h.errors, []);
});

test('round analysis rejects changed evidence while confirmation is open', async t => {
  const h = harness(t);
  await h.commands.get('intentlens.openRound')(h.folder, { id: 'r' });
  h.vscode.window.showWarningMessage = async (_, __, action) => { h.state.rounds[0].notes = ['policy changed']; return action; };
  await h.panels[0].message({ action: 'analyzeRound' });
  assert.equal(h.calls.some(c => c.command === 'analyze-round'), false);
  assert.match(h.panels[0].webview.html, /policy changed/);
  assert.match(h.errors[0], /重新确认/);
});

test('round analysis never submits after panel is closed or ignore policy disposes it', async t => {
  for (const invalidate of ['close', 'policy']) {
    const h = harness(t);
    await h.commands.get('intentlens.openRound')(h.folder, { id: 'r' });
    h.vscode.window.showWarningMessage = async (_, __, action) => {
      if (invalidate === 'close') h.panels[0].dispose();
      else await h.watchers[0].change(h.uri(path.join(h.folder.uri.fsPath, '.intentignore')));
      return action;
    };
    await h.panels[0].message({ action: 'analyzeRound' });
    assert.equal(h.calls.some(c => c.command === 'analyze-round'), false);
  }
});

test('failed round model request preserves prior display and unlocks retry', async t => {
  const h = harness(t);
  h.state.rounds[0].analysis = { id: 'prior', document: { claims: [] } };
  await h.commands.get('intentlens.openRound')(h.folder, { id: 'r' });
  h.vscode.window.showWarningMessage = async (_, __, action) => action;
  h.state.onCLI = command => { if (command === 'analyze-round') throw new Error('模型未启用'); };
  const panel = h.panels[0], before = panel.webview.html;
  await panel.message({ action: 'analyzeRound' });
  assert.equal(panel.webview.html, before);
  assert.match(h.errors[0], /模型未启用/);
  h.state.onCLI = undefined;
  await panel.message({ action: 'analyzeRound' });
  assert.equal(h.calls.filter(c => c.command === 'analyze-round').length, 2);
});

test('registered recovery commands adopt CLI session and cancel only confirmed baseline', async t => {
  const h = harness(t);
  h.state.responses = { 'capture-status': { id: 'active-id', session: 'from-skill', started: '2026-10-23T00:00:00Z' }, 'capture-abort': { aborted: true } };
  // No confirmation means no adoption or deletion.
  await h.commands.get('intentlens.resumeRound')();
  assert.equal(h.calls.some(c => c.command === 'start'), false);
  h.vscode.window.showWarningMessage = async (_, __, action) => action;
  await h.commands.get('intentlens.resumeRound')();
  await h.commands.get('intentlens.stopRound')();
  assert.deepEqual(h.calls.find(c => c.command === 'stop').input, { session: 'from-skill', round_id: 'active-id' });
  await h.commands.get('intentlens.cancelRound')();
  assert.deepEqual(h.calls.find(c => c.command === 'capture-abort').input, { session: 'from-skill', round_id: 'active-id', confirm: true });
  assert.deepEqual(h.errors, []);
});

test('untrusted workspace cannot resume, cancel, or analyze rounds', async t => {
  const h = harness(t);
  h.vscode.workspace.isTrusted = false;
  await h.commands.get('intentlens.resumeRound')();
  await h.commands.get('intentlens.cancelRound')();
  await h.commands.get('intentlens.openRound')(h.folder, { id: 'r' });
  assert.equal(h.calls.length, 0);
  assert.equal(h.errors.length, 3);
});


test('partial index retains function entries and adds visible diagnostics with exact reasons', async t => {
  const h = harness(t), uri = h.uri(path.join(h.folder.uri.fsPath, 'main.go'));
  h.state.responses = { index: { functions: [{ file: 'main.go', start_line: 3 }], coverage: { complete: false, scope: 'go-only', budget_bytes: 16777216, captured_bytes: 16777216, omitted_count: 1, excluded_count: 2, omitted: [{ file: 'later.go', reason: 'snapshot_budget' }] } } };
  const lenses = await h.state.codeLensProvider.provideCodeLenses({ uri, isDirty: false }, { isCancellationRequested: false });
  assert.equal(lenses.length, 2);
  assert.equal(lenses[0].command.command, 'intentlens.indexStatus');
  assert.match(lenses[0].command.title, /索引范围不完整/);
  assert.equal(lenses[1].command.command, 'intentlens.inspect');
  assert.equal(h.state.indexBar.visible, true);
  await h.commands.get('intentlens.indexStatus')(uri);
  assert.match(h.panels[0].webview.html, /Go 源码总预算已耗尽/);
  assert.match(h.panels[0].webview.html, /later.go/);
  assert.match(h.panels[0].webview.html, /2 项文件或目录/);
  assert.equal(h.calls.filter(c => c.command === 'index').length, 1, 'cached diagnostic opens without another full scan');
  assert.ok(h.calls.every(c => c.command === 'index'), 'diagnostics never call a model or change project policy');
});

test('zero functions from intentional exclusions still shows an actionable diagnostic entry', async t => {
  const h = harness(t), uri = h.uri(path.join(h.folder.uri.fsPath, 'main.go'));
  h.state.responses = { index: { functions: [], coverage: { complete: false, omitted_count: 0, excluded_count: 1, omitted: [] } } };
  const lenses = await h.state.codeLensProvider.provideCodeLenses({ uri, isDirty: false }, { isCancellationRequested: false });
  assert.equal(lenses.length, 1);
  assert.equal(lenses[0].command.command, 'intentlens.indexStatus');
  await h.commands.get('intentlens.indexStatus')(uri);
  assert.match(h.panels[0].webview.html, /按默认规则 \/ .intentignore 排除/);
  assert.doesNotMatch(h.panels[0].webview.html, /当前位置没有可解析/);
});

test('index CLI failure is visible instead of silently returning zero CodeLens entries', async t => {
  const h = harness(t), uri = h.uri(path.join(h.folder.uri.fsPath, 'main.go'));
  h.state.onCLI = () => { throw new Error('cannot read capture path <secret>'); };
  const lenses = await h.state.codeLensProvider.provideCodeLenses({ uri, isDirty: false }, { isCancellationRequested: false });
  assert.equal(lenses.length, 1);
  assert.match(lenses[0].command.title, /索引失败/);
  assert.match(h.state.indexBar.text, /索引失败/);
  await h.commands.get('intentlens.indexStatus')(uri);
  assert.match(h.panels[0].webview.html, /cannot read capture path &lt;secret&gt;/);
  assert.doesNotMatch(h.panels[0].webview.html, /<secret>/);
  assert.equal(h.calls.length, 1);
});

test('refresh clears stale diagnostics and a complete new index hides the warning', async t => {
  const h = harness(t), uri = h.uri(path.join(h.folder.uri.fsPath, 'main.go'));
  h.state.responses = { index: { functions: [], coverage: { complete: false, omitted_count: 1, excluded_count: 0 } } };
  await h.state.codeLensProvider.provideCodeLenses({ uri, isDirty: false }, { isCancellationRequested: false });
  await h.commands.get('intentlens.refresh')();
  assert.equal(h.state.indexBar.visible, false);
  h.state.responses.index = { functions: [{ file: 'main.go', start_line: 2 }], coverage: { complete: true, omitted_count: 0, excluded_count: 0 } };
  const lenses = await h.state.codeLensProvider.provideCodeLenses({ uri, isDirty: false }, { isCancellationRequested: false });
  assert.equal(lenses.length, 1);
  assert.equal(lenses[0].command.command, 'intentlens.inspect');
  assert.equal(h.state.indexBar.visible, false);
  await h.commands.get('intentlens.indexStatus')(uri);
  assert.match(h.panels[0].webview.html, /Go 源码索引完整/);
  assert.doesNotMatch(h.panels[0].webview.html, /1 个 Go 文件未纳入/);
});

test('cancellation and source changes do not publish late index warnings', async t => {
  for (const invalidate of ['cancel', 'edit']) {
    const h = harness(t), uri = h.uri(path.join(h.folder.uri.fsPath, 'main.go'));
    let finish;
    h.state.onCLI = () => new Promise(resolve => { finish = resolve; });
    h.state.responses = { index: { functions: [], coverage: { complete: false, omitted_count: 1 } } };
    const token = { isCancellationRequested: false };
    const pending = h.state.codeLensProvider.provideCodeLenses({ uri, isDirty: false }, token);
    while (!finish) await new Promise(resolve => setImmediate(resolve));
    if (invalidate === 'cancel') token.isCancellationRequested = true;
    else h.state.changed({ document: { uri }, contentChanges: [{}] });
    finish();
    assert.deepEqual(await pending, []);
    assert.equal(h.state.indexBar.visible, false);
  }
});


test('source changes close current-source diagnostics and details without closing historical rounds', async t => {
  for (const event of ['edit', 'external']) {
    const h = harness(t), uri = h.uri(path.join(h.folder.uri.fsPath, 'main.go'));
    h.state.responses = { index: { functions: [], coverage: { complete: false, omitted_count: 1 } } };
    await h.commands.get('intentlens.inspectDetails')(uri, 1);
    const details = h.panels.at(-1);
    await h.commands.get('intentlens.indexStatus')(uri);
    const diagnostic = h.panels.at(-1);
    await h.commands.get('intentlens.openRound')(h.folder, { id: 'r' });
    const historical = h.panels.at(-1);
    if (event === 'edit') h.state.changed({ document: { uri }, contentChanges: [{}] });
    else h.watchers.find(w => w.pattern === '**/*.go').change(uri);
    assert.equal(details.disposed, true);
    assert.equal(diagnostic.disposed, true);
    assert.equal(historical.disposed, false, 'historical round evidence is not invalidated by current-source edits');
    assert.equal(h.state.indexBar.visible, false);
  }
});

test('in-flight inspection or diagnostic cannot reopen stale panels after a source edit', async t => {
  for (const command of ['intentlens.inspectDetails', 'intentlens.indexStatus']) {
    const h = harness(t), uri = h.uri(path.join(h.folder.uri.fsPath, 'main.go'));
    let finish;
    h.state.onCLI = () => new Promise(resolve => { finish = resolve; });
    const pending = h.commands.get(command)(uri, 1);
    while (!finish) await new Promise(resolve => setImmediate(resolve));
    h.state.changed({ document: { uri }, contentChanges: [{}] });
    finish();
    await pending;
    assert.equal(h.panels.length, 0);
    assert.equal(h.state.indexBar.visible, false);
  }
});
