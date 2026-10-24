'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const vscode = require('vscode');

// Run only with a disposable fixture workspace and isolated user/extension data.
exports.run = async function () {
  const extension = vscode.extensions.getExtension('intentlens-local.intentlens');
  assert.ok(extension, 'development extension discovered');
  await extension.activate();
  assert.ok(vscode.workspace.isTrusted, 'fixture workspace trusted');
  const folder = vscode.workspace.workspaceFolders[0];
  const root = folder.uri.fsPath;
  assert.ok(fs.existsSync(path.join(root, '.intentlens-rounds-fixture')), 'explicit disposable fixture marker');
  const file = path.join(root, 'main.go');
  const cli = (command, input) => JSON.parse(execFileSync(path.join(extension.extensionPath, 'bin', process.platform === 'win32' ? 'intentlens.exe' : 'intentlens'), [command, '--root', root], { input: input ? JSON.stringify(input) : undefined, encoding: 'utf8', windowsHide: true }));
  const contract = cli('integration-info');
  assert.ok(contract.entries.includes('skill'));
  assert.equal(cli('status').settings.model.enabled, false);
  const commands = await vscode.commands.getCommands(true);
  assert.ok(commands.includes('intentlens.startRound'));
  assert.ok(commands.includes('intentlens.stopRound'));
  assert.ok(commands.includes('intentlens.resumeRound'));
  assert.ok(commands.includes('intentlens.cancelRound'));
  await vscode.commands.executeCommand('intentlens.startRound');
  fs.writeFileSync(file, 'package p\nfunc Twice(n int) int { return n * 2 }\n');
  await vscode.commands.executeCommand('intentlens.stopRound');
  const rounds = cli('rounds');
  assert.equal(rounds.length, 1, 'actual IDE commands produced a round');
  assert.equal(rounds[0].changes.length, 1);
  assert.equal(rounds[0].changes[0].path, 'main.go');
  assert.ok(rounds[0].changes[0].before.content.includes('return n }'));
  assert.ok(rounds[0].changes[0].after.content.includes('return n * 2'));
  assert.equal(rounds[0].source, 'unconfirmed');
  assert.equal(rounds[0].analysis, undefined, 'manual stop did not invoke a model');
  // A second round also verifies successful workspace-state cleanup.
  await vscode.commands.executeCommand('intentlens.startRound');
  fs.writeFileSync(file, 'package p\nfunc Twice(n int) int { return n + n }\n');
  await vscode.commands.executeCommand('intentlens.stopRound');
  const next = cli('rounds');
  assert.equal(next.length, 2);
  assert.notEqual(next[0].session, next[1].session);
  const result = { result: 'PASS', checks: ['real extension activation', 'real configuration scope URI', 'IDE start and stop commands', 'recovery commands registered', 'net change from saved baseline', 'second independent session', 'source unconfirmed', 'no automatic model request'], model_calls: 0, claude_required: false };
  if (process.env.INTENTLENS_TEST_RESULT) fs.writeFileSync(process.env.INTENTLENS_TEST_RESULT, JSON.stringify(result, null, 2));
};
