const test = require('node:test');
const assert = require('node:assert/strict');
const { createExerciseUI, renderExercise } = require('../src/exercises');

test('exercise UI keeps Go test execution closed and refuses forged run actions', async () => {
  const calls = [];
  const panels = [];
  const warnings = [];
  const view = {
    id: 'e',
    version: 'v',
    spec: { title: '<unsafe>', baseline: { 'main.go': 'package p' }, requirements: [] },
    validation: { status: 'unverified' },
    attempts: [],
    workspace: '/private/answer',
  };
  const host = {
    vscode: { window: {
      showInformationMessage() {},
      showWarningMessage(message) { warnings.push(message); },
    } },
    cli: async (_, command, input) => {
      calls.push({ command, input });
      if (command === 'exercise-view') return structuredClone(view);
      if (command === 'exercise-apply-preview') return { digest: 'diff', changes: [] };
      if (command === 'exercise-apply-confirm') return {};
      throw new Error(`unexpected CLI command ${command}`);
    },
    panel: (_, render, handler) => {
      const panel = { webview: { html: render('nonce') }, handler, dispose() { this.closed = true; } };
      panels.push(panel);
      return panel;
    },
  };
  const ui = createExerciseUI(host);
  await ui.show({}, view);

  const html = panels[0].webview.html;
  assert.match(html, /执行已关闭/);
  assert.match(html, /<button disabled>验证题目（执行关闭）<\/button>/);
  assert.match(html, /<button disabled>执行并评分（执行关闭）<\/button>/);

  await panels[0].handler({ action: 'validate' });
  await panels[0].handler({ action: 'score' });
  assert.equal(warnings.length, 2);
  assert.equal(calls.some(call => call.command === 'exercise-plan'), false);
  assert.equal(calls.some(call => call.command === 'exercise-run'), false);

  await panels[0].handler({ action: 'apply' });
  assert.equal(panels.length, 2);
  await panels[1].handler({ action: 'confirm' });
  assert.equal(calls.filter(call => call.command === 'exercise-apply-confirm').length, 1);
  assert.ok(renderExercise(view, 'nonce').includes('&lt;unsafe&gt;'));
});
