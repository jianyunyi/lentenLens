'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const { createCallPathProvider, validatePathPayload, MAX_NODES, MAX_RENDERED_OCCURRENCES } = require('../src/callpath');

function vscodeMock() {
  return {
    TreeItem: class { constructor(label, collapsibleState) { this.label = label; this.collapsibleState = collapsibleState; } },
    TreeItemCollapsibleState: { None: 0, Collapsed: 1 },
    EventEmitter: class { event = () => {}; fire() {} dispose() {} },
  };
}
const fn = (id, name, line) => ({ id, name, file: 'main.go', start_line: line, end_line: line + 2, source: `func ${name}()` });

test('call path provider renders source evidence, selected node, and unresolved/candidate badges', () => {
  const vscode = vscodeMock();
  const provider = createCallPathProvider(vscode);
  provider.setData({ selected_id: 'b', root_ids: ['a'], functions: [fn('a', 'Root', 1), fn('b', 'Selected', 10)], calls: [
    { caller_id: 'a', name: 'maybe', status: 'ambiguous', targets: ['b', 'missing'], file: 'main.go', line: 5, source: 'maybe()' },
  ], coverage: { files: 1 }, issues: ['静态图不代表运行时完整'], truncated: false });
  const root = provider.getChildren();
  assert.match(root[0].label, /调用路径/);
  assert.match(root.find(item => item.kind === 'function').label, /Root/);
  const call = provider.getChildren(root.find(item => item.kind === 'function'))[0];
  assert.match(call.label, /候选/);
  assert.equal(call.command.arguments[0].file, 'main.go');
  const targets = provider.getChildren(call);
  assert.match(targets[0].label, /Selected/);
  assert.match(targets[1].label, /未知/);
});

test('call path payload rejects malformed data and filters unsafe shape without commands', () => {
  assert.throws(() => validatePathPayload({}), /格式无效/);
  const result = validatePathPayload({ selected_id: '../run', root_ids: ['ok', '../bad'], functions: [fn('ok', 'Safe', 1), { id: 'bad', name: 'NoLine', file: 'x.go' }], calls: [
    { caller_id: 'ok', name: 'x', targets: ['ok'], file: '../evil', line: 0 },
  ] });
  assert.equal(result.selected_id, undefined);
  assert.deepEqual(result.root_ids, ['ok']);
  assert.equal(result.functions.length, 1);
  assert.equal(result.calls.length, 0);
});

test('payload caps are visible and statuses cannot claim resolved targets incorrectly', () => {
  const functions = Array.from({ length: MAX_NODES + 1 }, (_, i) => fn(`f${i}`, `F${i}`, i + 1));
  const result = validatePathPayload({ functions, calls: [
    { caller_id: 'f0', name: 'none', status: 'resolved', targets: [], file: 'main.go', line: 4 },
    { caller_id: 'f0', name: 'one', status: 'unresolved', targets: ['f1'], file: 'main.go', line: 5 },
  ] });
  assert.equal(result.functions.length, MAX_NODES);
  assert.equal(result.truncated, true);
  assert.equal(result.calls[0].status, 'unresolved');
  assert.equal(result.calls[1].status, 'candidate');
});

test('provider caches repeated expansion and disposes its event emitter', () => {
  let disposed = false;
  const vscode = vscodeMock();
  vscode.EventEmitter = class { event = () => {}; fire() {} dispose() { disposed = true; } };
  const provider = createCallPathProvider(vscode);
  provider.setData({ root_ids: ['a'], functions: [fn('a', 'Root', 1)], calls: Array.from({ length: 3 }, (_, i) => ({ caller_id: 'a', name: `x${i}`, targets: ['a'], file: 'main.go', line: i + 2 })), issues: [] });
  const root = provider.getChildren();
  const item = root.find(row => row.kind === 'function');
  const first = provider.getChildren(item);
  assert.strictEqual(provider.getChildren(item), first);
  for (let i = 0; i < MAX_RENDERED_OCCURRENCES + 20; i++) provider.getChildren(item);
  provider.dispose();
  assert.equal(disposed, true);
});
