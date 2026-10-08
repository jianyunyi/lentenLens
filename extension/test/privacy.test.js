const test = require('node:test');
const assert = require('node:assert/strict');
const Module = require('node:module');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const core = require('../src/core');

function harness(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'intentlens-ui-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  fs.writeFileSync(path.join(root, 'main.go'), 'package main');
  fs.writeFileSync(path.join(root, 'dep.go'), 'package main');
  const uri = file => ({ scheme: 'file', fsPath: file, toString: () => file });
  const folder = { name: 'test', uri: uri(root) }, commands = new Map(), calls = [], panels = [], watchers = [], errors = [], documents = [];
  const state = { inspection: { function: { name: 'F', source: 'safe' }, dependencies: [{ name: 'G', file: 'dep.go', start_line: 1 }], evidence: [{ id: 'e', file: 'main.go', source: 'secret', start_line: 1 }], history: [], issues: [] }, rounds: [{ id: 'r', changes: [], notes: [] }], refreshes: 0 };
  const d = { dispose() {} };
  const vscode = {
    workspace: { isTrusted: true, workspaceFolders: [folder], getWorkspaceFolder: () => folder, getConfiguration: () => ({ get: () => '' }), onDidGrantWorkspaceTrust: () => d, onDidSaveTextDocument: () => d, onDidChangeWorkspaceFolders: () => d,
      createFileSystemWatcher: pattern => { const w = { pattern, dispose() {}, onDidCreate(fn) { this.create = fn; return d; }, onDidChange(fn) { this.change = fn; return d; }, onDidDelete(fn) { this.delete = fn; return d; } }; watchers.push(w); return w; },
      openTextDocument: async value => { documents.push(value); return value; } },
    window: { createTreeView: () => d, showErrorMessage: text => errors.push(text), showTextDocument: async () => ({}), createWebviewPanel: () => { const p = { disposed: false, onDidDispose(fn) { this.onDispose = fn; return d; }, dispose() { this.disposed = true; this.onDispose?.(); }, webview: { html: '', onDidReceiveMessage(fn) { p.message = fn; return d; } } }; panels.push(p); return p; } },
    commands: { registerCommand: (name, fn) => { commands.set(name, fn); return d; } }, languages: { registerCodeLensProvider: () => d },
    EventEmitter: class { event = () => {}; fire() { state.refreshes++; } dispose() {} }, TreeItem: class {}, TreeItemCollapsibleState: { None: 0, Collapsed: 1 }, ViewColumn: { Beside: 2 }, Uri: { file: uri },
  };
  const original = Module._load;
  Module._load = function(id, ...args) {
    if (id === 'vscode') return vscode;
    if (id === './core') return { ...core, runCLI: async (_, __, command, input) => { calls.push({ command, input }); return structuredClone(command === 'inspect' ? state.inspection : command === 'rounds' ? state.rounds : {}); } };
    return original.call(this, id, ...args);
  };
  try { delete require.cache[require.resolve('../src/extension')]; require('../src/extension').activate({ subscriptions: [], extensionPath: root }); }
  finally { Module._load = original; }
  return { folder, commands, calls, panels, watchers, errors, documents, state, uri };
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
test('opening cached tree round fetches allowed current round rather than cached secret', async t => {
  const h = harness(t);
  await h.commands.get('intentlens.openRound')(h.folder, { id: 'r', changes: [{ path: 'main.go', before: { content: 'cached-secret' } }] });
  assert.equal(h.calls[0]?.command, 'rounds');
  assert.ok(!h.panels[0].webview.html.includes('cached-secret'));
});
test('evidence action re-inspects and refuses evidence removed by policy', async t => {
  const h = harness(t);
  await h.commands.get('intentlens.inspect')(h.uri(path.join(h.folder.uri.fsPath, 'main.go')), 1);
  h.state.inspection.evidence = [];
  await h.panels[0].message({ action: 'snapshot', group: 'current', index: '0' });
  assert.equal(h.calls.filter(c => c.command === 'inspect').length, 2);
  assert.equal(h.documents.length, 0);
  assert.ok(!h.panels[0].webview.html.includes('secret'));
});
test('dependency opens its own inspection directly', async t => {
  const h = harness(t);
  await h.commands.get('intentlens.inspect')(h.uri(path.join(h.folder.uri.fsPath, 'main.go')), 1);
  await h.panels[0].message({ action: 'dependency', index: '0' });
  assert.equal(h.calls.at(-1).input.file, 'dep.go');
  assert.equal(h.panels.length, 2);
});
