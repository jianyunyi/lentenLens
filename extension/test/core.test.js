const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { EventEmitter } = require('node:events');
const { PassThrough } = require('node:stream');
let core;
try { core = require('../src/core'); } catch (error) { if (error.code !== 'MODULE_NOT_FOUND') throw error; }
test('CLI bridge preserves JSON and root with spaces without a shell', async t => {
  assert.ok(core, 'core module implemented');
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'intentlens bridge '));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const script = path.join(root, 'fixture.cjs');
  fs.writeFileSync(script, `let s='';process.stdin.on('data',x=>s+=x);process.stdin.on('end',()=>{if(process.argv[2]==='fail'){console.log(JSON.stringify({error:'rejected'}));process.exitCode=1;}else{console.log(JSON.stringify({args:process.argv.slice(2),input:JSON.parse(s),injected:process.env.INTENTLENS_TEST_SENTINEL}));}});`);
  const value = await core.runCLI(process.execPath, root, 'inspect', { question: 'a " & b' }, { prefixArgs: [script], env: { ...process.env, INTENTLENS_TEST_SENTINEL: 'secret-storage-env' } });
  assert.deepEqual(value, { args: ['inspect', '--root', root], input: { question: 'a " & b' }, injected: 'secret-storage-env' });
  await assert.rejects(core.runCLI(process.execPath, root, 'fail', {}, { prefixArgs: [script] }), /rejected/);
});
test('paths reject traversal, absolute paths, ADS, and symlink escapes', t => {
  assert.ok(core, 'core module implemented');
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'intentlens-path-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  const root = path.join(dir, 'root'); fs.mkdirSync(root);
  fs.writeFileSync(path.join(root, 'ok.go'), 'package main');
  assert.equal(core.confinedPath(root, 'ok.go'), path.join(root, 'ok.go'));
  for (const name of ['../outside.go', '/etc/passwd', 'C:\\outside.go', 'ok.go:secret', '..\\outside.go']) {
    assert.throws(() => core.confinedPath(root, name), /outside|relative|unsafe/);
  }
  fs.mkdirSync(path.join(dir, 'outside'));
  fs.writeFileSync(path.join(dir, 'outside', 'x.go'), 'outside');
  fs.symlinkSync(path.join(dir, 'outside'), path.join(root, 'link'), 'junction');
  assert.throws(() => core.confinedPath(root, 'link/x.go'), /outside/);
});
test('CLI bridge preserves UTF-8 across stream chunks and rejects malformed or stalled output', async t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'intentlens-utf8-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const script = path.join(root, 'fixture.cjs');
  fs.writeFileSync(script, `if(process.argv[2]==='invalid'){console.log('not json');}else if(process.argv[2]==='wait'){setTimeout(()=>{},10000);}else{const data=Buffer.from(JSON.stringify({text:'中文证据'}));process.stdout.write(data.subarray(0,10));setTimeout(()=>process.stdout.end(data.subarray(10)),30);}`);
  const result = await core.runCLI(process.execPath, root, 'utf8', {}, { prefixArgs: [script] });
  assert.equal(result.text, '中文证据');
  await assert.rejects(core.runCLI(process.execPath, root, 'invalid', {}, { prefixArgs: [script] }), /invalid JSON/);
  await assert.rejects(core.runCLI(process.execPath, root, 'wait', {}, { prefixArgs: [script], timeout: 100 }), /timed out/);
});
test('CLI bridge terminates a running local process when its signal is aborted', async t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'intentlens-abort-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const script = path.join(root, 'wait.cjs');
  fs.writeFileSync(script, 'setTimeout(() => {}, 10000);');
  const controller = new AbortController();
  const request = core.runCLI(process.execPath, root, 'inspect', {}, { prefixArgs: [script], signal: controller.signal });
  await new Promise(resolve => setTimeout(resolve, 25));
  controller.abort();
  await assert.rejects(request, error => error.name === 'AbortError' || error.code === 'ABORT_ERR');
});
test('CLI bridge waits for child close after abort error before temp-directory cleanup', async t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'intentlens-abort-close-'));
  const heldPath = path.join(root, 'child-held.txt');
  let heldFd = fs.openSync(heldPath, 'w');
  let child, childClosed = false;
  const closeChild = () => {
    if (childClosed) return;
    if (heldFd !== undefined) { fs.closeSync(heldFd); heldFd = undefined; }
    child.stdin.destroy();
    child.stdout.destroy();
    child.stderr.destroy();
    childClosed = true;
    child.emit('close', null, 'SIGTERM');
  };
  t.after(() => {
    if (child) closeChild();
    else if (heldFd !== undefined) fs.closeSync(heldFd);
    fs.rmSync(root, { recursive: true, force: true });
  });

  const controller = new AbortController();
  const spawnProcess = (_binary, _args, { signal }) => {
    child = new EventEmitter();
    child.stdin = new PassThrough();
    child.stdout = new PassThrough();
    child.stderr = new PassThrough();
    child.kill = () => true;
    signal.addEventListener('abort', () => {
      const error = new Error('The operation was aborted');
      error.name = 'AbortError';
      error.code = 'ABORT_ERR';
      child.emit('error', error);
    }, { once: true });
    return child;
  };
  let settled = false;
  const request = core.runCLI(process.execPath, root, 'inspect', {}, {
    signal: controller.signal,
    spawnProcess,
  }).finally(() => { settled = true; });

  controller.abort();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(settled, false, 'AbortError must remain pending until child close');
  assert.notEqual(heldFd, undefined, 'the simulated child resource stays open before close');

  closeChild();
  await assert.rejects(request, error => error.name === 'AbortError' || error.code === 'ABORT_ERR');
  assert.equal(heldFd, undefined);
  assert.doesNotThrow(() => fs.rmSync(root, { recursive: true, force: false }));
});
test('rendering escapes hostile source and claims and separates historical source', () => {
  assert.ok(core, 'core module implemented');
  const hostile = '<img src=x onerror="alert(1)">';
  const html = core.renderInspection({ function: { name: hostile, source: hostile }, evidence: [], dependencies: [], history: [{ id: 'h', freshness: 'stale', document: { claims: [{ text: hostile, status: 'uncertain', evidence_ids: ['e'] }] }, evidence: [{ id: 'e', file: 'main.go', source: hostile, start_line: 2, end_line: 3 }] }] }, 'nonce123');
  assert.ok(!html.includes(hostile));
  assert.match(html, /&lt;img/);
  assert.match(html, /default-src 'none'/);
  assert.match(html, /nonce-nonce123/);
  assert.match(html, /历史证据原文/);
  assert.match(html, /当前文件/);
  assert.match(html, /uncertain/);
  assert.match(html, /位置: main\.go:2–3/);
});
