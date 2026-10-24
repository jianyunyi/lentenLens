const test = require('node:test');
const assert = require('node:assert/strict');
test('inline explanations anchor below function, escape data, replace and invalidate threads', async () => {
  const { createInline } = require('../src/inline');
  const threads = [], commands = new Map();
  const vscode = {
    MarkdownString: class { value = ''; appendText(s) { this.value += String(s).replace(/\[/g, '\\['); return this; } appendMarkdown(s) { this.value += s; return this; } },
    Range: class { constructor(a,b,c,d) { this.start={line:a}; this.end={line:c}; } },
    CommentMode: { Preview: 0 }, CommentThreadCollapsibleState: { Expanded: 1 },
    commands: { registerCommand(id, fn) { commands.set(id, fn); return { dispose() {} }; } },
    comments: { createCommentController: () => ({ dispose() {}, createCommentThread(uri, range, comments) {
      const t = { uri, range, comments, dispose() { this.disposed = true; } }; threads.push(t); return t;
    } }) },
  };
  const ui = createInline(vscode, { subscriptions: [] }, fn => fn);
  const info = { function: { name: 'F', start_line: 2, end_line: 8 }, history: [], evidence: [], dependencies: [], issues: [] };
  const uri = { toString: () => 'file:///main.go' };
  let called = 0;
  const first = ui.open(uri, info, async () => { called++; });
  assert.equal(first.thread.range.end.line, 7);
  assert.equal(first.thread.canReply, false);
  assert.equal(first.thread.collapsibleState, 1);
  assert.match(first.thread.comments[0].body.value, /尚未生成/);
  assert.deepEqual(first.thread.comments.slice(1), [], 'do not show empty sections');
  const link = /command:intentlens.inlineAction\?([^)]*)/.exec(first.thread.comments[0].body.value);
  const args = JSON.parse(decodeURIComponent(link[1]));
  await commands.get('intentlens.inlineAction')(...args);
  assert.equal(called, 1);
  const second = ui.open(uri, info, async () => {});
  assert.equal(first.thread.disposed, true);
  info.coverage = { complete: false, omitted_count: 2, excluded_count: 1 };
  second.update(info);
  assert.match(second.thread.comments[0].body.value, /索引范围不完整/);
  assert.match(second.thread.comments[0].body.value, /2 个 Go 文件未纳入/);
  assert.match(second.thread.comments[0].body.value, /1 项文件或目录/);
  await commands.get('intentlens.inlineAction')(...args);
  assert.equal(called, 1, 'old token cannot execute');
  info.history = [{ id:'x', created:'2026', freshness:'已引用证据未变', document: { claims:[{section:'purpose',kind:'behavior',status:'supported',text:'[evil](command:evil)',evidence_ids:['e']}], unknowns:[] }, evidence:[] }];
  second.update(info);
  assert.match(second.thread.comments[1].body.value, /\\\[evil/);
  assert.deepEqual(second.thread.comments[1].body.isTrusted, { enabledCommands: ['intentlens.inlineAction'] });
  info.history[0].document.claims[0].id = 'c1';
  info.history[0].corrections = [{claim_id:'c1',reason:'行为说明不符'}];
  second.update(info);
  assert.match(second.thread.comments[1].body.value, /已标记有误/);
  assert.match(second.thread.comments[1].body.value, /行为说明不符/);
  assert.ok(!second.thread.comments[1].body.value.includes('小修改练习'));
  ui.clear();
  second.update(info);
  assert.equal(second.thread.disposed, true, 'late result cannot recreate disposed thread');
});
