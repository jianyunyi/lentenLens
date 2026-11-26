'use strict';
// Launches a real VS Code with the development extension against a disposable
// Go fixture and runs scripts/vscode-integration.cjs inside the extension host.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');
const testElectron = require(require.resolve('@vscode/test-electron', { paths: [path.join(__dirname, '..', 'extension')] }));
const { runTests } = testElectron;

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

  await runUntrusted(extensionDevelopmentPath);
}

// Restricted mode: launched WITHOUT --disable-workspace-trust. A user-level
// intentlens.binaryPath points at a stub that records every invocation; the
// scenario fails if the stub was executed at all.
async function runUntrusted(extensionDevelopmentPath) {
  if (process.platform === 'win32') {
    console.log('untrusted-workspace scenario skipped on win32: stub CLI is a POSIX script and the extension spawns with shell:false');
    return;
  }
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'intentlens-vscode-untrusted-'));
  const workspace = path.join(temp, 'workspace');
  const configHome = path.join(temp, 'config');
  const userData = path.join(temp, 'user-data');
  fs.mkdirSync(workspace, { recursive: true });
  fs.mkdirSync(configHome, { recursive: true });
  fs.mkdirSync(path.join(userData, 'User'), { recursive: true });
  fs.writeFileSync(path.join(workspace, 'go.mod'), 'module fixture\n\ngo 1.25\n');
  fs.writeFileSync(path.join(workspace, 'main.go'), 'package main\nfunc Twice(x int) int { return x * 2 }\n');

  const marker = path.join(temp, 'cli-invoked.log');
  const stub = path.join(temp, 'intentlens-stub');
  fs.writeFileSync(stub, `#!/bin/sh\necho "$@" >> ${JSON.stringify(marker)}\necho '{"error":"stub CLI must not run in an untrusted workspace"}'\nexit 1\n`, { mode: 0o755 });
  fs.writeFileSync(path.join(userData, 'User', 'settings.json'), JSON.stringify({
    'security.workspace.trust.enabled': true,
    'security.workspace.trust.startupPrompt': 'never',
    'security.workspace.trust.emptyWindow': false,
    'intentlens.binaryPath': stub,
  }, null, 2));

  const resultFile = path.join(temp, 'result.json');
  // @vscode/test-electron's runTests hardcodes --disable-workspace-trust in
  // its base arguments (no opt-out), which would defeat this scenario. Spawn
  // the executable directly with the same base arguments minus that flag.
  const executable = await testElectron.downloadAndUnzipVSCode(process.env.VSCODE_TEST_VERSION || 'stable');
  const args = [
    workspace,
    // Deliberately NO --disable-workspace-trust.
    '--disable-extensions',
    '--skip-welcome',
    '--skip-release-notes',
    `--user-data-dir=${userData}`,
    `--extensions-dir=${path.join(temp, 'extensions')}`,
    '--no-sandbox',
    '--disable-gpu-sandbox',
    '--disable-updates',
    `--extensionTestsPath=${path.join(__dirname, 'vscode-untrusted-integration.cjs')}`,
    `--extensionDevelopmentPath=${extensionDevelopmentPath}`,
  ];
  const env = {
    ...process.env,
    INTENTLENS_TEST_RESULT: resultFile,
    INTENTLENS_CLI_MARKER: marker,
    INTENTLENS_STUB_PATH: stub,
    XDG_CONFIG_HOME: configHome,
    APPDATA: configHome,
  };
  const code = await new Promise((resolve, reject) => {
    const child = spawn(executable, args, { env, stdio: 'inherit' });
    child.on('error', reject);
    child.on('close', resolve);
  });
  if (code !== 0) throw new Error(`untrusted-workspace scenario: VS Code exited with code ${code}`);

  const result = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
  assert.equal(result.result, 'PASS', JSON.stringify(result));
  const invoked = fs.existsSync(marker) ? fs.readFileSync(marker, 'utf8') : '';
  assert.equal(invoked, '', `CLI was invoked in an untrusted workspace: ${invoked}`);
  console.log(JSON.stringify(result, null, 2));
}

main().catch(error => {
  console.error(error);
  process.exit(1);
});
