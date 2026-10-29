'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const { createModelAnswerProvider } = require('../src/sidebar');

class FakeTreeItem {
  constructor(label, collapsibleState) { this.label = label; this.collapsibleState = collapsibleState; }
}

function harness() {
  const events = [];
  const vscode = { TreeItem: FakeTreeItem, TreeItemCollapsibleState: { None: 0, Collapsed: 1 } };
  const treeChanged = { event: () => {}, fire: value => events.push(value) };
  return { vscode, treeChanged, events };
}

function fakeTimers() {
  let nextID = 0;
  const scheduled = new Map();
  return {
    schedule(callback, delay) { const id = ++nextID; scheduled.set(id, { callback, delay }); return id; },
    cancel(id) { scheduled.delete(id); },
    runAll() { while (scheduled.size) { const [id, timer] = scheduled.entries().next().value; scheduled.delete(id); timer.callback(); } },
    get size() { return scheduled.size; },
    get delays() { return [...scheduled.values()].map(timer => timer.delay); },
  };
}

const folder = { name: 'fixture', uri: { fsPath: '/work/fixture', scheme: 'file' } };
const evidence = { id: 'e1', file: 'main.go', start_line: 3, end_line: 7, hash: 'hash', source: 'func run() {}' };
function target(history = []) {
  return {
    folder,
    file: 'main.go',
    line: 3,
    info: {
      function: { name: 'run', file: 'main.go', start_line: 3, end_line: 8 },
      history,
    },
  };
}

test('model-answer provider sorts saved versions and maps claims to their local evidence', async () => {
  const h = harness();
  let targetReads = 0;
  const old = { id: 'old', created: '2026-01-01T00:00:00Z', freshness: '已引用证据未变', document: { claims: [{ id: 'c0', text: '旧说法', status: 'supported', section: 'flow', evidence_ids: ['e1'] }] }, evidence: [evidence] };
  const latest = { id: 'new', created: '2026-02-01T00:00:00Z', freshness: '解释已过期：引用变化', document: { claims: [{ id: 'c1', text: '最新回答中的结论', status: 'uncertain', section: 'behavior', evidence_ids: ['e1'] }], unknowns: ['未记录的设计动机'] }, evidence: [evidence] };
  const provider = createModelAnswerProvider({ ...h, getCurrentTarget: async () => { targetReads++; return target([old, latest]); } });
  const [answerRoot] = await provider.getChildren();
  assert.match(answerRoot.label, /模型回答/);
  const versions = await provider.getChildren(answerRoot);
  assert.match(versions[0].label, /最新回答/);
  assert.match(versions[0].description, /解释已过期/);
  const answerRows = await provider.getChildren(versions[0]);
  const claim = answerRows.find(row => row.contextValue === 'intentlens.sidebar.claim');
  assert.equal(claim.label, '最新回答中的结论');
  assert.match(claim.description, /behavior · uncertain/);
  const [citation] = await provider.getChildren(claim);
  assert.equal(citation.label, 'main.go:3–7');
  assert.equal(citation.command.command, 'intentlens.openModelEvidence');
  assert.equal(citation.command.arguments[0], folder);
  assert.equal(citation.command.arguments[1], evidence);
  assert.ok(answerRows.some(row => /未记录的设计动机/.test(row.label)));
  assert.equal(targetReads, 1);
});

test('model-answer provider never asks the model to create a missing answer', async () => {
  const h = harness();
  const provider = createModelAnswerProvider({ ...h, getCurrentTarget: async () => target([]) });
  const [empty] = await provider.getChildren();
  assert.match(empty.label, /暂无本地保存的模型回答/);
  assert.match(empty.tooltip, /不会自动请求模型/);
});

test('model-answer refresh discards a late result for an earlier function', async () => {
  const h = harness();
  const clock = fakeTimers();
  let resolveOld;
  let reads = 0;
  const provider = createModelAnswerProvider({
    ...h,
    schedule: clock.schedule,
    cancelScheduled: clock.cancel,
    getCurrentTarget: () => ++reads === 1 ? new Promise(resolve => { resolveOld = resolve; }) : Promise.resolve(target()),
  });
  const staleRequest = provider.getChildren();
  clock.runAll();
  await Promise.resolve();
  provider.refresh();
  const freshRequest = provider.getChildren();
  clock.runAll();
  const [fresh] = await freshRequest;
  resolveOld(target());
  const [stale] = await staleRequest;
  assert.match(fresh.label, /模型回答/);
  assert.match(stale.label, /已变化/);
  const [stillFresh] = await provider.getChildren();
  assert.match(stillFresh.label, /模型回答/);
  assert.equal(reads, 2);
});

test('rapid selection changes debounce scans and coalesce the latest read', async () => {
  const h = harness();
  const clock = fakeTimers();
  let reads = 0;
  const provider = createModelAnswerProvider({
    ...h,
    getCurrentTarget: async () => { reads++; return target(); },
    debounceMs: 150,
    schedule: clock.schedule,
    cancelScheduled: clock.cancel,
  });

  const first = provider.getChildren();
  provider.refresh();
  const second = provider.getChildren();
  provider.refresh();
  const third = provider.getChildren();
  assert.equal(clock.size, 1);
  assert.deepEqual(clock.delays, [150]);

  clock.runAll();
  const [firstRow, secondRow, latestRow] = await Promise.all([first, second, third]);
  assert.match(firstRow[0].label, /已变化/);
  assert.match(secondRow[0].label, /已变化/);
  assert.match(latestRow[0].label, /暂无本地保存的模型回答/);
  assert.equal(reads, 1);
  provider.dispose();
});

test('refresh aborts an in-flight inspect and a late result cannot replace the latest target', async () => {
  const h = harness();
  const clock = fakeTimers();
  let oldResolve, oldSignal, reads = 0;
  const oldTarget = target([{ id: 'old' }]);
  oldTarget.info.function.name = '旧函数';
  const freshTarget = target([{ id: 'fresh' }]);
  freshTarget.info.function.name = '新函数';
  const provider = createModelAnswerProvider({
    ...h,
    getCurrentTarget: ({ signal }) => {
      reads++;
      if (reads === 1) {
        oldSignal = signal;
        return new Promise(resolve => { oldResolve = resolve; });
      }
      return Promise.resolve(freshTarget);
    },
    schedule: clock.schedule,
    cancelScheduled: clock.cancel,
  });

  const staleRequest = provider.getChildren();
  clock.runAll();
  await Promise.resolve();
  assert.equal(reads, 1);
  provider.refresh();
  assert.equal(oldSignal.aborted, true);

  const latestRequest = provider.getChildren();
  clock.runAll();
  const [latest] = await latestRequest;
  assert.match(latest.label, /新函数/);
  oldResolve(oldTarget);
  await new Promise(resolve => setImmediate(resolve));
  const [stale] = await staleRequest;
  const [stillLatest] = await provider.getChildren();
  assert.match(stale.label, /已变化/);
  assert.match(stillLatest.label, /新函数/);
  assert.equal(reads, 2);
  provider.dispose();
});

test('model-answer provider shows real errors but treats cancellation as a stale refresh', async () => {
  const h = harness();
  const clock = fakeTimers();
  let attempts = 0;
  const provider = createModelAnswerProvider({
    ...h,
    getCurrentTarget: async () => {
      if (++attempts === 1) throw new Error('scan failed');
      return target();
    },
    schedule: clock.schedule,
    cancelScheduled: clock.cancel,
  });
  const failedRequest = provider.getChildren();
  clock.runAll();
  const [failure] = await failedRequest;
  assert.match(failure.label, /无法读取本地模型回答：scan failed/);
  const retry = provider.getChildren();
  assert.equal(clock.size, 1);
  clock.runAll();
  const [recovered] = await retry;
  assert.match(recovered.label, /暂无本地保存的模型回答/);
  assert.equal(attempts, 2);
  provider.dispose();

  const cancelClock = fakeTimers();
  const cancelled = createModelAnswerProvider({
    ...h,
    getCurrentTarget: async () => { const error = new Error('cancelled'); error.name = 'AbortError'; throw error; },
    schedule: cancelClock.schedule,
    cancelScheduled: cancelClock.cancel,
  });
  const cancelledRequest = cancelled.getChildren();
  cancelClock.runAll();
  const [stale] = await cancelledRequest;
  assert.match(stale.label, /已变化/);
  assert.doesNotMatch(stale.label, /无法读取/);
  cancelled.dispose();
});

test('disposing the provider aborts its scan and suppresses pending results', async () => {
  const h = harness();
  const clock = fakeTimers();
  let signal;
  const provider = createModelAnswerProvider({
    ...h,
    getCurrentTarget: ({ signal: requestSignal }) => {
      signal = requestSignal;
      return new Promise(() => {});
    },
    schedule: clock.schedule,
    cancelScheduled: clock.cancel,
  });
  const request = provider.getChildren();
  clock.runAll();
  await Promise.resolve();
  assert.ok(signal);
  provider.dispose();
  assert.equal(signal.aborted, true);
  assert.deepEqual(await request, []);
  assert.equal(clock.size, 0);
  provider.refresh();
  assert.equal(clock.size, 0);
});
