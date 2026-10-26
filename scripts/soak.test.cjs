const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { run } = require('./soak.cjs');

test('real CLI survives repeated local rounds without loss or excluded material', () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'intentlens-soak-test-'));
  try {
    const binary = path.join(directory, process.platform === 'win32' ? 'intentlens.exe' : 'intentlens');
    const built = spawnSync('go', ['build', '-o', binary, './cmd/intentlens'], { cwd: path.join(__dirname, '..'), encoding: 'utf8', timeout: 120000 });
    assert.equal(built.status, 0, built.stderr || built.error?.message);
    const report = run(binary, 3);
    assert.equal(report.result, 'PASS');
    assert.equal(report.iterations, 3);
    assert.equal(report.modelCalls, 0);
    assert.equal(report.latency.rounds.samples, 3);
    assert.ok(report.stateBytes > 0);
    assert.throws(() => run(binary, 0), /iterations/);
  } finally { fs.rmSync(directory, { recursive: true, force: true }); }
});
