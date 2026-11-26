'use strict';
// Runs inside the extension host in Restricted Mode. Must never execute the CLI.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vscode = require('vscode');

const withTimeout = (promise, ms) => Promise.race([promise, new Promise((_, reject) => setTimeout(() => reject(new Error('timeout')), ms))]);

exports.run = async function () {
  const marker = process.env.INTENTLENS_CLI_MARKER;
  const stub = process.env.INTENTLENS_STUB_PATH;
  assert.ok(marker && stub, 'marker and stub paths provided');
  assert.equal(vscode.workspace.isTrusted, false, 'workspace must start untrusted (Restricted Mode)');

  const extension = vscode.extensions.getExtension('lentenlens.intentlens');
  assert.ok(extension, 'extension discovered');
  await extension.activate();

  // Prove the detector is armed: the CLI path the extension would use is the stub.
  assert.equal(vscode.workspace.getConfiguration('intentlens').get('binaryPath'), stub, 'user-level binaryPath honored in Restricted Mode');

  const root = vscode.workspace.workspaceFolders[0].uri;
  const doc = await vscode.workspace.openTextDocument(vscode.Uri.joinPath(root, 'main.go'));
  await vscode.window.showTextDocument(doc);

  const lenses = await vscode.commands.executeCommand('vscode.executeCodeLensProvider', doc.uri);
  assert.ok(!(lenses || []).some(lens => lens.command?.command === 'intentlens.inspect'), 'no IntentLens CodeLens in Restricted Mode');

  const attempts = [
    ['intentlens.inspect', doc.uri, 2],
    ['intentlens.inspectDetails', doc.uri, 2],
    ['intentlens.status'],
    ['intentlens.refresh'],
    ['intentlens.hooksConfig'],
  ];
  const outcomes = [];
  for (const [id, ...args] of attempts) {
    try { await withTimeout(vscode.commands.executeCommand(id, ...args), 10000); outcomes.push(`${id}: returned`); }
    catch (error) { outcomes.push(`${id}: ${error?.message || error}`); }
  }
  // Allow any stray async spawn to land before checking.
  await new Promise(resolve => setTimeout(resolve, 1500));

  const invoked = fs.existsSync(marker) ? fs.readFileSync(marker, 'utf8') : '';
  assert.equal(invoked, '', `CLI executed in Restricted Mode: ${invoked}`);
  assert.equal(doc.isDirty, false, 'source unchanged');

  const result = { result: 'PASS', checks: ['workspace untrusted', 'detector armed (binaryPath=stub)', 'no CodeLens', 'commands did not spawn CLI', 'source unchanged'], outcomes, cli_invocations: 0 };
  if (process.env.INTENTLENS_TEST_RESULT) fs.writeFileSync(process.env.INTENTLENS_TEST_RESULT, JSON.stringify(result, null, 2));
};
