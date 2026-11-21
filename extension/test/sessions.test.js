const test = require('node:test');
const assert = require('node:assert/strict');
const { createSessions, rootKey } = require('../src/sessions');

function harness(initial = {}) {
  const values = new Map(Object.entries(initial));
  const calls = [];
  let failUpdate = false;
  let failCLI = false;
  let release;
  const cli = async (root, command, input) => {
    calls.push({ root, command, input });
    if (release) await release;
    if (failCLI) throw new Error('cli down');
    if (command === 'capture-status') return null;
    if (command === 'start') return { id: `${input.session}-round`, session: input.session };
    if (command === 'stop') return { id: input.round_id, session: input.session };
    return { ok: true, command };
  };
  const workspaceState = {
    get: async key => values.get(key),
    update: async (key, value) => { if (failUpdate) throw new Error('state down'); if (value === undefined) values.delete(key); else values.set(key, value); },
  };
  return { values, calls, workspaceState, cli, set failUpdate(value) { failUpdate = value; }, set failCLI(value) { failCLI = value; }, block() { let resolve; release = new Promise(r => { resolve = r; }); return resolve; } };
}

test('starts with a random persisted session and stops without model work', async () => {
  const h = harness();
  const notices = [];
  const s = createSessions({ workspaceState: h.workspaceState, cli: h.cli, random: () => 'session-a', notify: { info: x => notices.push(x) } });
  await s.start('A');
  assert.deepEqual(JSON.parse(h.values.get(rootKey('A'))), { session: 'session-a', round_id: 'session-a-round' });
  assert.deepEqual(h.calls[0], { root: 'A', command: 'start', input: { session: 'session-a' } });
  const stopped = await s.stop('A');
  assert.equal(stopped.session, 'session-a');
  assert.equal(h.values.has(rootKey('A')), false);
  assert.deepEqual(h.calls.map(x => x.command), ['start', 'stop']);
  assert.deepEqual(h.calls[1].input, { session: 'session-a', round_id: 'session-a-round' });
  assert.equal(notices.length, 2);
});

test('CLI failures retain state for an explicit retry', async () => {
  const h = harness(); h.failCLI = true;
  const errors = [];
  const s = createSessions({ workspaceState: h.workspaceState, cli: h.cli, random: () => 'retry-me', notify: { error: x => errors.push(x) } });
  await assert.rejects(s.start('A'), /会话已保留/);
  assert.equal(await s.get('A'), 'retry-me');
  h.failCLI = false;
  await s.start('A');
  h.failCLI = true;
  await assert.rejects(s.stop('A'), /会话仍保留/);
  assert.equal(await s.get('A'), 'retry-me');
  assert.equal(errors.length, 2);
});

test('duplicate and concurrent clicks execute one operation per workspace', async () => {
  const h = harness(); const release = h.block();
  const s = createSessions({ workspaceState: h.workspaceState, cli: h.cli, random: () => 'one' });
  const first = s.start('A');
  const second = s.start('A');
  assert.equal(await second, undefined);
  release();
  await first;
  assert.equal(h.calls.filter(x => x.command === 'start').length, 1);
});

test('sessions are isolated by root and survive controller reload', async () => {
  const h = harness();
  const first = createSessions({ workspaceState: h.workspaceState, cli: h.cli, random: () => 'a' });
  await first.start('A');
  const reloaded = createSessions({ workspaceState: h.workspaceState, cli: h.cli, random: () => 'b' });
  assert.equal(await reloaded.get('A'), 'a');
  await reloaded.start('B');
  assert.equal(await reloaded.get('A'), 'a');
  assert.equal(await reloaded.get('B'), 'b');
  assert.notEqual(rootKey('A'), rootKey('B'));
});

test('persistence failure prevents CLI start and reports recovery', async () => {
  const h = harness(); h.failUpdate = true;
  const s = createSessions({ workspaceState: h.workspaceState, cli: h.cli, random: () => 'not-saved' });
  await assert.rejects(s.start('A'), /无法保存轮次会话/);
  assert.equal(h.calls.length, 0);
});


test('successful stop with failed cleanup retries cleanup without stopping twice', async () => {
  const h = harness();
  const s = createSessions({ workspaceState: h.workspaceState, cli: h.cli, random: () => 'cleanup' });
  await s.start('A');
  h.failUpdate = true;
  await assert.rejects(s.stop('A'), /成功结束.*无法清除/);
  h.failUpdate = false;
  await s.stop('A');
  assert.equal(h.calls.filter(x => x.command === 'stop').length, 1);
  assert.equal(await s.get('A'), undefined);
});

test('reload reconciles a completed round only for its own persisted identity', async () => {
  const h = harness({ [rootKey('A')]: JSON.stringify({ session: 'own-session', round_id: 'done' }) });
  const round = { id: 'done', session: 'own-session' };
  const calls = [];
  const s = createSessions({ workspaceState: h.workspaceState, cli: async (_, command) => {
    calls.push(command);
    if (command === 'stop') throw new Error('baseline already removed');
    return [{ id: 'other', session: 'someone-else' }, round];
  } });
  assert.deepEqual((await s.stop('A')).result, round);
  assert.deepEqual(calls, ['stop', 'rounds']);
  assert.equal(await s.get('A'), undefined);
});

test('starting after a stale completed session creates a new session', async () => {
  const h = harness({ [rootKey('A')]: 'old-session' });
  const calls = [];
  const s = createSessions({ workspaceState: h.workspaceState, random: () => 'new-session', cli: async (_, command, input) => {
    calls.push({ command, input });
    return command === 'rounds' ? [{ id: 'done', session: 'old-session' }] : { id: 'new' };
  } });
  await s.start('A');
  assert.equal(await s.get('A'), 'new-session');
  assert.deepEqual(calls.at(-1), { command: 'start', input: { session: 'new-session' } });
});

test('unrelated completed rounds cannot hide a stop failure', async () => {
  const h = harness({ [rootKey('A')]: 'own-session' });
  const s = createSessions({ workspaceState: h.workspaceState, cli: async (_, command) => {
    if (command === 'stop') throw new Error('mismatched baseline');
    return [{ id: 'other', session: 'someone-else' }];
  } });
  await assert.rejects(s.stop('A'), /缺少轮次 ID/);
  assert.equal(await s.get('A'), 'own-session');
});


test('explicit recovery persists a session and release cannot clear another one', async () => {
  const h = harness(); const s = createSessions({ workspaceState: h.workspaceState, cli: h.cli });
  await s.adopt('A', 'from-skill');
  assert.equal(await s.get('A'), 'from-skill');
  assert.deepEqual(await s.release('A', 'unrelated'), { released: false });
  assert.equal(await s.get('A'), 'from-skill');
  assert.deepEqual(await s.release('A', 'from-skill'), { released: true });
  assert.equal(await s.get('A'), undefined);
  assert.equal(h.calls.length, 0);
});

test('recovery and association cleanup cannot run through an in-flight capture', async () => {
  const h = harness(); const unblock = h.block();
  const s = createSessions({ workspaceState: h.workspaceState, cli: h.cli, random: () => 'running' });
  const pending = s.start('A');
  assert.equal(await s.adopt('A', 'other'), undefined);
  await assert.rejects(s.release('A', 'running'), /操作正在执行/);
  unblock(); await pending;
  assert.equal(await s.get('A'), 'running');
});

test('association save failures preserve the previous session and invalid IDs are rejected', async () => {
  const h = harness({ [rootKey('A')]: 'existing' }); h.failUpdate = true;
  const s = createSessions({ workspaceState: h.workspaceState, cli: h.cli });
  await assert.rejects(s.adopt('A', 'other'), /state down/);
  await assert.rejects(s.release('A', 'existing'), /state down/);
  assert.equal(await s.get('A'), 'existing');
  await assert.rejects(s.adopt('A', '  '), /有效会话/);
  await assert.rejects(s.release('A', ''), /有效会话/);
});
