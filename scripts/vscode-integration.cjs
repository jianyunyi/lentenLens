'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vscode = require('vscode');

exports.run = async function () {
  const extension = vscode.extensions.getExtension('intentlens-local.intentlens');
  assert.ok(extension, 'extension discovered');
  await extension.activate();
  assert.ok(vscode.workspace.isTrusted, 'test workspace trusted');
  const root = vscode.workspace.workspaceFolders[0].uri;
  const doc = await vscode.workspace.openTextDocument(vscode.Uri.joinPath(root, 'main.go'));
  await vscode.window.showTextDocument(doc);
  const lenses = await vscode.commands.executeCommand('vscode.executeCodeLensProvider', doc.uri);
  assert.ok(lenses.some(lens => lens.command?.command === 'intentlens.inspect'), 'Go CodeLens supplied by actual extension');
  await vscode.commands.executeCommand('intentlens.inspect', doc.uri, 2);
	if (!vscode.window.tabGroups.all.some(group => group.tabs.some(tab => tab.label.includes('IntentLens')))) {
	  await new Promise(resolve => {
	    const timer = setTimeout(() => { sub.dispose(); resolve(); }, 5000);
	    const sub = vscode.window.tabGroups.onDidChangeTabs(() => {
	      if (vscode.window.tabGroups.all.some(group => group.tabs.some(tab => tab.label.includes('IntentLens')))) { clearTimeout(timer); sub.dispose(); resolve(); }
	    });
	  });
	}
  assert.ok(vscode.window.tabGroups.all.some(group => group.tabs.some(tab => tab.label.includes('IntentLens'))), 'evidence webview opened');
  const commands = await vscode.commands.getCommands(true);
  for (const id of ['intentlens.enableModel', 'intentlens.pauseModel', 'intentlens.hooksConfig', 'intentlens.refresh']) assert.ok(commands.includes(id), id);
  const result = { result: 'PASS', checks: ['extension activation', 'trusted workspace', 'real CodeLens provider', 'evidence webview', 'commands registered'], model_calls: 0 };
  if (process.env.INTENTLENS_TEST_RESULT) fs.writeFileSync(process.env.INTENTLENS_TEST_RESULT, JSON.stringify(result, null, 2));
};
