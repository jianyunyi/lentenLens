const test = require('node:test');
const assert = require('node:assert/strict');
const Module = require('node:module');
const fs = require('node:fs');
const path = require('node:path');
test('extension registers trusted local commands, Go CodeLens, and rounds view', () => {
  const file = path.join(__dirname, '../src/extension.js');
  assert.ok(fs.existsSync(file), 'extension entry exists');
  const registrations = [], commands = new Map();
  const disposable = { dispose() {} };
  const state = new Map();
  const workspaceState = { get: key => state.get(key), update: (key, value) => { if (value === undefined) state.delete(key); else state.set(key, value); return Promise.resolve(); } };
  const vscode = {
    workspace: { isTrusted: false, workspaceFolders: [], createFileSystemWatcher: () => ({ ...disposable, onDidCreate: () => disposable, onDidChange: () => disposable, onDidDelete: () => disposable }), onDidChangeTextDocument: () => ({ dispose() {} }), onDidGrantWorkspaceTrust: () => disposable, onDidSaveTextDocument: () => disposable, onDidChangeWorkspaceFolders: () => disposable, workspaceState },
    window: { createTreeView: (name) => { registrations.push(name); return disposable; }, showErrorMessage: () => {} },
    commands: { registerCommand: (name, fn) => { commands.set(name, fn); return disposable; } },
    languages: { registerCodeLensProvider: (selector) => { registrations.push(selector); return disposable; } },
    EventEmitter: class { event = () => {}; fire() {} dispose() {} },
    TreeItem: class {}, TreeItemCollapsibleState: { None: 0, Collapsed: 1 },
  };
  const original = Module._load;
  Module._load = function (id, ...args) { return id === 'vscode' ? vscode : original.call(this, id, ...args); };
  try {
    delete require.cache[require.resolve(file)];
    require(file).activate({ subscriptions: [], extensionPath: path.dirname(path.dirname(file)), workspaceState });
    assert.ok(commands.has('intentlens.inspect'));
    assert.ok(commands.has('intentlens.enableModel'));
    assert.ok(commands.has('intentlens.pauseModel'));
    assert.ok(commands.has('intentlens.hooksConfig'));
    assert.ok(commands.has('intentlens.startRound'));
    assert.ok(commands.has('intentlens.stopRound'));
    assert.ok(commands.has('intentlens.resumeRound'));
    assert.ok(commands.has('intentlens.cancelRound'));
    const manifest = require('../package.json');
    for (const name of ['startRound', 'stopRound', 'resumeRound', 'cancelRound']) {
      const id = `intentlens.${name}`;
      assert.ok(manifest.contributes.commands.some(item => item.command === id));
      assert.ok(manifest.contributes.menus['view/title'].some(item => item.command === id));
    }
    assert.ok(registrations.includes('intentlens.rounds'));
    assert.ok(registrations.some(x => x.language === 'go' && x.scheme === 'file'));
  } finally { Module._load = original; }
});
