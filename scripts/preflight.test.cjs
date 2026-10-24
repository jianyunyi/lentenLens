const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { collect, parseArgs } = require('./preflight.cjs');

function fixture(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'intentlens-preflight-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  for (const dir of ['extension/bin', 'bin', 'tools']) fs.mkdirSync(path.join(root, dir), { recursive: true });
  fs.writeFileSync(path.join(root, 'extension/package.json'), JSON.stringify({ version: '0.3.0' }));
  return root;
}
function fullIde(root) {
  for (const file of ['code.cmd', 'claude.cmd', 'go.exe']) fs.writeFileSync(path.join(root, 'tools', file), 'not executable');
  for (const file of ['bin/intentlens.exe', 'extension/bin/intentlens.exe', 'intentlens-0.3.0-win32-x64.vsix']) fs.writeFileSync(path.join(root, file), 'fixture');
}

 test('default offline IDE reports requirements without claiming integration', t => {
  const root = fixture(t);
  const result = collect(root, { PATH: '', ANTHROPIC_API_KEY: '  ' }, 'win32');
  assert.equal(result.entry, 'ide');
  assert.equal(result.model, false);
  assert.deepEqual(result.requirements.required, ['code', 'cli', 'bundled_cli', 'vsix', 'matching_cli', 'windows_x64_package_platform']);
  assert.equal(result.key_present, false);
  assert.ok(result.missing.includes('code'));
  assert.ok(!result.missing.includes('api_key'));
  assert.equal(result.live_integration, 'NOT RUN');
});

test('IDE prerequisites pass and candidates are never executed', t => {
  const root = fixture(t); fullIde(root);
  const result = collect(root, { Path: `;"${path.join(root, 'tools')}";`, ANTHROPIC_API_KEY: 'DO_NOT_PRINT_THIS' }, 'win32', 'x64', 'ANTHROPIC_API_KEY', { entry: 'ide' });
  assert.deepEqual(result.missing, []);
  assert.equal(result.bundled_cli_matches, true);
  assert.match(result.artifacts.cli.sha256, /^[a-f0-9]{64}$/);
  assert.equal(JSON.stringify(result).includes('DO_NOT_PRINT_THIS'), false);
});

test('skill requires only local CLI and ignores IDE, Claude, VSIX and model key', t => {
  const root = fixture(t);
  fs.writeFileSync(path.join(root, 'bin/intentlens.exe'), 'fixture');
  const result = collect(root, { PATH: '', ANTHROPIC_API_KEY: '' }, 'linux', 'arm64', 'ANTHROPIC_API_KEY', { entry: 'skill' });
  assert.deepEqual(result.missing, []);
  assert.deepEqual(result.requirements.required, ['cli']);
  assert.equal(result.tools.claude, null);
});

test('claude entry requires Claude and local CLI, not IDE artifacts', t => {
  const root = fixture(t);
  fs.writeFileSync(path.join(root, 'bin/intentlens.exe'), 'fixture');
  fs.writeFileSync(path.join(root, 'tools', 'claude.cmd'), 'not executable');
  const result = collect(root, { PATH: path.join(root, 'tools') }, 'win32', 'x64', 'ANTHROPIC_API_KEY', { entry: 'claude' });
  assert.deepEqual(result.missing, []);
  assert.equal(result.requirements.required.includes('vsix'), false);
  assert.equal(result.requirements.required.includes('windows_x64_package_platform'), false);
});

test('model is explicit opt-in and only then requires selected key variable', t => {
  const root = fixture(t); fs.writeFileSync(path.join(root, 'bin/intentlens.exe'), 'fixture');
  const env = { ANTHROPIC_API_KEY: 'wrong-vendor', DEEPSEEK_API_KEY: 'private-value' };
  const result = collect(root, env, 'win32', 'x64', 'DEEPSEEK_API_KEY', { entry: 'skill', model: true });
  assert.equal(result.key_present, true);
  assert.equal(result.key_env, 'DEEPSEEK_API_KEY');
  assert.ok(result.requirements.required.includes('api_key'));
  assert.ok(!JSON.stringify(result).includes('private-value'));
  assert.equal(collect(root, {}, 'win32', 'x64', 'MISSING_KEY', { entry: 'skill', model: true }).missing.includes('api_key'), true);
});

test('strict CLI argument parsing supports help and rejects bad, duplicate, missing and unknown args', () => {
  assert.deepEqual(parseArgs([]), { entry: 'ide', model: false, keyEnv: 'ANTHROPIC_API_KEY', help: false });
  assert.deepEqual(parseArgs(['--entry', 'skill', '--model', '--key-env', 'DEEPSEEK_API_KEY']), { entry: 'skill', model: true, keyEnv: 'DEEPSEEK_API_KEY', help: false });
  assert.deepEqual(parseArgs(['--help']), { entry: 'ide', model: false, keyEnv: 'ANTHROPIC_API_KEY', help: true });
  for (const args of [
    ['--wat'], ['--entry'], ['--entry', 'bad'], ['--entry', 'skill', '--entry', 'claude'],
    ['--model', '--model'], ['--key-env'], ['--key-env', 'BAD-NAME'], ['--help', '--model'],
  ]) assert.throws(() => parseArgs(args));
});

test('mismatched CLI and native Claude fallback remain diagnostic-only', t => {
  const root = fixture(t);
  fs.writeFileSync(path.join(root, 'bin/intentlens.exe'), 'new');
  fs.writeFileSync(path.join(root, 'extension/bin/intentlens.exe'), 'old');
  fs.mkdirSync(path.join(root, '.local/bin'), { recursive: true });
  fs.writeFileSync(path.join(root, '.local/bin/claude.exe'), 'fixture');
  const result = collect(root, { USERPROFILE: root }, 'win32', 'x64', 'ANTHROPIC_API_KEY', { entry: 'claude' });
  assert.ok(result.tools.claude);
  assert.equal(result.bundled_cli_matches, false);
  assert.deepEqual(result.missing, []);
});
