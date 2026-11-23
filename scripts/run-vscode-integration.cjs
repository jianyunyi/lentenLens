'use strict';
// Launches a real VS Code with the development extension against a disposable
// Go fixture and runs scripts/vscode-integration.cjs inside the extension host.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { runTests } = require(require.resolve('@vscode/test-electron', { paths: [path.join(__dirname, '..', 'extension')] }));

async function main() {
  const repo = path.resolve(__dirname, '..');
  const extensionDevelopmentPath = path.join(repo, 'extension');
  const extensionTestsPath = path.join(__dirname, 'vscode-integration.cjs');
  const binary = path.join(extensionDevelopmentPath, 'bin', process.platform === 'win32' ? 'intentlens.exe' : 'intentlens');
  if (!fs.existsSync(binary)) throw new Error(`CLI not built: ${binary}`);

  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'intentlens-vscode-it-'));
  const workspace = path.join(temp, 'workspace');
  const configHome = path.join(temp, 'config'); // isolates IntentLens private state (os.UserConfigDir)
  fs.mkdirSync(workspace, { recursive: true });
  fs.mkdirSync(configHome, { recursive: true });
  fs.writeFileSync(path.join(workspace, 'go.mod'), 'module fixture\n\ngo 1.25\n');
  fs.writeFileSync(path.join(workspace, 'main.go'), 'package main\nfunc Twice(x int) int { return x * 2 }\n');

  const resultFile = path.join(temp, 'result.json');
  await runTests({
    version: process.env.VSCODE_TEST_VERSION || 'stable',
    extensionDevelopmentPath,
    extensionTestsPath,
    extensionTestsEnv: {
      INTENTLENS_TEST_RESULT: resultFile,
      XDG_CONFIG_HOME: configHome,
      APPDATA: configHome,
    },
    launchArgs: [
      workspace,
      '--disable-workspace-trust',
      '--disable-extensions',
      '--skip-welcome',
      '--skip-release-notes',
      `--user-data-dir=${path.join(temp, 'user-data')}`,
      `--extensions-dir=${path.join(temp, 'extensions')}`,
    ],
  });

  const result = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
  assert.equal(result.result, 'PASS', JSON.stringify(result));
  assert.equal(result.model_calls, 0);
  assert.equal(result.exercise_execution, false);
  console.log(JSON.stringify(result, null, 2));
}

main().catch(error => {
  console.error(error);
  process.exit(1);
});
