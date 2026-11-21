const test = require('node:test');
const assert = require('node:assert/strict');
const { createCaptureRecovery } = require('../src/recovery');

function harness() {
  const folder = { uri: { fsPath: '/fixture' } };
  const h = { folder, active: { id: 'round-a', session: 'skill-session', started: 'time-a' }, calls: [], prompts: [], notices: [], adopted: [], released: [], answer: undefined };
  h.cli = async (_, command, input) => {
    h.calls.push({ command, input });
    if (command === 'capture-status') return h.active ? { ...h.active } : null;
    if (h.abortError) throw new Error('changed in backend');
    h.active = null; return { aborted: true };
  };
  h.sessions = {
    adopt: async (root, session, roundID) => { h.adopted.push({ root, session, roundID }); h.afterAdopt?.(); return { session, roundID }; },
    release: async (root, session) => { h.released.push({ root, session }); return { released: true }; },
  };
  const vscode = { window: {
    showWarningMessage: async (text, options, action) => { h.prompts.push({ text, options, action }); await h.onConfirm?.(); return h.answer === true ? action : h.answer; },
    showInformationMessage: async text => { h.notices.push(text); },
  } };
  h.ui = createCaptureRecovery({ vscode, cli: h.cli, sessions: h.sessions });
  return h;
}

test('canceling recovery confirmation never adopts or aborts a round', async () => {
  const h = harness(); await h.ui.resume(h.folder);
  assert.equal(h.adopted.length, 0); assert.equal(h.released.length, 0);
  assert.deepEqual(h.calls.map(c => c.command), ['capture-status']);
});

test('recovery explicitly confirms and validates identity before and after association', async () => {
  const h = harness(); h.answer = true;
  await h.ui.resume(h.folder);
  assert.deepEqual(h.adopted, [{ root: '/fixture', session: 'skill-session', roundID: 'round-a' }]);
  assert.equal(h.calls.filter(c => c.command === 'capture-status').length, 3);
  assert.equal(h.prompts[0].options.modal, true);
  assert.ok(!h.calls.some(c => c.command === 'capture-abort' || c.command === 'analyze-round'));
});

test('changed baseline while confirmation is open prevents recovery and cancellation', async () => {
  for (const action of ['resume', 'cancel']) {
    const h = harness(); h.answer = true;
    h.onConfirm = () => { h.active.id = 'round-b'; };
    await assert.rejects(h.ui[action](h.folder), /已变化/);
    assert.equal(h.adopted.length, 0); assert.equal(h.released.length, 0);
    assert.ok(!h.calls.some(c => c.command === 'capture-abort'));
  }
});

test('baseline changed during association is released without claiming success', async () => {
  const h = harness(); h.answer = true;
  h.afterAdopt = () => { h.active = null; };
  await assert.rejects(h.ui.resume(h.folder), /已变化/);
  assert.deepEqual(h.released, [{ root: '/fixture', session: 'skill-session' }]);
  assert.equal(h.notices.length, 0);
});

test('cancellation uses the actual confirmed identity and requires a separate explicit choice', async () => {
  const h = harness(); await h.ui.cancel(h.folder);
  assert.ok(!h.calls.some(c => c.command === 'capture-abort'));
  h.answer = true; await h.ui.cancel(h.folder);
  assert.deepEqual(h.calls.find(c => c.command === 'capture-abort').input, { session: 'skill-session', round_id: 'round-a', confirm: true });
  assert.deepEqual(h.released, [{ root: '/fixture', session: 'skill-session' }]);
  assert.equal(h.active, null);
});

test('backend cancellation failure preserves the local association', async () => {
  const h = harness(); h.answer = true; h.abortError = true;
  await assert.rejects(h.ui.cancel(h.folder), /changed in backend/);
  assert.equal(h.released.length, 0); assert.equal(h.notices.length, 0);
});

test('absent or invalid active state cannot trigger destructive work', async () => {
  for (const action of ['resume', 'cancel']) {
    const h = harness(); h.active = null; await h.ui[action](h.folder);
    assert.equal(h.prompts.length, 0); assert.equal(h.adopted.length, 0);
    h.active = { id: 'bad' };
    await assert.rejects(h.ui[action](h.folder), /状态无效/);
    assert.equal(h.prompts.length, 0);
  }
});
