'use strict';
// Synthetic local-only acceptance run. Never opens a supplied project, enables a
// model, or leaves fixture source behind unless the process is forcibly killed.
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const crypto = require('node:crypto');
const { spawnSync } = require('node:child_process');
const { performance } = require('node:perf_hooks');

function run(binary, iterations) {
  if (!Number.isInteger(iterations) || iterations < 1 || iterations > 10000) throw new Error('iterations must be 1..10000');
  binary = fs.realpathSync(binary);
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'intentlens-soak-'));
  const root = path.join(directory, 'repo'), state = path.join(directory, 'state');
  fs.mkdirSync(root); fs.mkdirSync(state);
  const samples = {};
  const started = performance.now();
  function cli(command, input) {
    const before = performance.now();
    const result = spawnSync(binary, [command, '--root', root, '--state', state], {
      input: input === undefined ? undefined : JSON.stringify(input), encoding: 'utf8', shell: false,
      timeout: 30000, maxBuffer: 32 * 1024 * 1024,
    });
    (samples[command] ||= []).push(performance.now() - before);
    if (result.error || result.status !== 0) throw new Error(`${command} failed: ${result.error?.message || result.stdout || result.stderr}`);
    return JSON.parse(result.stdout);
  }
  function stateBytes(dir) {
    return fs.readdirSync(dir, { withFileTypes: true }).reduce((n, entry) => {
      const p = path.join(dir, entry.name);
      if (entry.isDirectory()) return n + stateBytes(p);
      const bytes = fs.readFileSync(p);
      if (bytes.includes(Buffer.from('PRIVATE_SOAK_SENTINEL'))) throw new Error('excluded source leaked');
      return n + bytes.length;
    }, 0);
  }
  try {
    fs.writeFileSync(path.join(root, 'main.go'), 'package p\nfunc F() int { return 0 }\n');
    fs.writeFileSync(path.join(root, '.env'), 'PRIVATE_SOAK_SENTINEL');
    for (let i = 0; i < iterations; i++) {
      const session = `soak-${i}`;
      cli('start', { session });
      fs.writeFileSync(path.join(root, 'main.go'), `package p\nfunc F() int { return ${i + 1} }\n`);
      const round = cli('stop', { session });
      if (round.changes.length !== 1 || round.changes[0].path !== 'main.go') throw new Error('incorrect round changes');
      const index = cli('index');
      if (index.functions.length !== 1) throw new Error('incorrect index');
      const inspection = cli('inspect', { file: 'main.go', line: 2 });
      if (inspection.function.name !== 'F') throw new Error('incorrect inspection');
      const rounds = cli('rounds');
      if (rounds.length !== i + 1) throw new Error('history lost or duplicated');
    }
    const latency = {};
    for (const [command, values] of Object.entries(samples)) {
      values.sort((a, b) => a - b);
      const percentile = p => values[Math.max(0, Math.ceil(values.length * p) - 1)];
      latency[command] = { samples: values.length, p50Ms: percentile(.5), p95Ms: percentile(.95), maxMs: values.at(-1) };
    }
    return { result: 'PASS', iterations, durationMs: performance.now() - started,
      environment: { platform: process.platform, arch: process.arch, node: process.version },
      binarySha256: crypto.createHash('sha256').update(fs.readFileSync(binary)).digest('hex'),
      modelCalls: 0, stateBytes: stateBytes(state), harnessRssBytes: process.memoryUsage().rss, latency,
      limits: 'Synthetic fixture; harness RSS is not CLI peak memory; this is not long-term or real-user acceptance.' };
  } finally { fs.rmSync(directory, { recursive: true, force: true }); }
}

if (require.main === module) {
  try {
    const [, , binary, iterations = '100'] = process.argv;
    if (!binary || process.argv.length > 4) throw new Error('Usage: node scripts/soak.cjs /absolute/intentlens[.exe] [iterations]');
    process.stdout.write(JSON.stringify(run(binary, Number(iterations)), null, 2) + '\n');
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
module.exports = { run };
