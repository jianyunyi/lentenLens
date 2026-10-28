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
  let resolveOld;
  let reads = 0;
  const provider = createModelAnswerProvider({
    ...h,
    getCurrentTarget: () => ++reads === 1 ? new Promise(resolve => { resolveOld = resolve; }) : Promise.resolve(target()),
  });
  const staleRequest = provider.getChildren();
  provider.refresh();
  const [fresh] = await provider.getChildren();
  resolveOld(target());
  const [stale] = await staleRequest;
  assert.match(fresh.label, /模型回答/);
  assert.match(stale.label, /已变化/);
  const [stillFresh] = await provider.getChildren();
  assert.match(stillFresh.label, /模型回答/);
  assert.equal(reads, 2);
});
