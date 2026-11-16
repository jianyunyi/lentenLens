'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const Module = require('node:module');

const listeners = {};
const vscode = {
  ViewColumn: { Two: 2 },
  window: {
    createWebviewPanel: () => ({
      visible: true,
      webview: {
        html: '',
        onDidReceiveMessage(fn) { listeners.message = fn; },
      },
      onDidDispose(fn) { listeners.dispose = fn; },
      onDidChangeViewState(fn) { listeners.viewState = fn; },
      reveal() {},
      dispose() {},
    }),
    showInformationMessage() {},
    showWarningMessage: async () => '取消',
  },
};
const originalLoad = Module._load;
Module._load = function(request, parent, isMain) {
  if (request === 'vscode') return vscode;
  return originalLoad.call(this, request, parent, isMain);
};
const { TokenUsagePanel } = require('../src/tokenUsagePanel');
Module._load = originalLoad;

function createPanel(statusLoader) {
  const panel = new TokenUsagePanel(
    vscode.window.createWebviewPanel(),
    { subscriptions: [] },
    statusLoader,
  );
  panel._clearAutoRefresh();
  return panel;
}

test('renders authoritative CLI usage and the selected provider and model', async () => {
  const panel = createPanel(async () => ({
    settings: { model: { provider: 'anthropic', model: 'claude-fixture', token_limit: 1000 } },
    usage: { actual_tokens: 300, uncertain_tokens: 50 },
    fees: '未配置费率，费用估算不可用',
  }));
  await panel._update();
  assert.match(panel._panel.webview.html, /anthropic/);
  assert.match(panel._panel.webview.html, /claude-fixture/);
  assert.match(panel._panel.webview.html, /累计用量按项目共享预算统计/);
  assert.match(panel._panel.webview.html, /300/);
  assert.match(panel._panel.webview.html, /50/);
  assert.match(panel._panel.webview.html, /650/);
  assert.match(panel._panel.webview.html, /费用估算不可用/);
  assert.doesNotMatch(panel._panel.webview.html, /输入 Token|输出 Token|\$0\.0000/);
});

test('shows loading and failure states instead of fabricated zero usage', async () => {
  let finish;
  const loadingPanel = createPanel(() => new Promise(resolve => { finish = resolve; }));
  assert.match(loadingPanel._panel.webview.html, /正在读取 CLI 用量状态/);
  finish({ usage: { actual_tokens: 1, uncertain_tokens: 0 }, settings: { model: { token_limit: 10 } } });
  await new Promise(resolve => setImmediate(resolve));

  const panel = createPanel(async () => { throw new Error('CLI unavailable'); });
  await panel._update();
  assert.match(panel._panel.webview.html, /CLI unavailable/);
  assert.doesNotMatch(panel._panel.webview.html, /输入 Token[\s\S]*>0</);
});

test('refresh reads usage again from the CLI status source', async () => {
  let actualTokens = 3;
  const panel = createPanel(async () => ({
    settings: { model: { provider: 'openai-compatible', model: 'fixture', token_limit: 10 } },
    usage: { actual_tokens: actualTokens, uncertain_tokens: 0 },
    fees: '未配置费率，费用估算不可用',
  }));
  await panel._update();
  actualTokens = 7;
  await panel._update();
  assert.match(panel._panel.webview.html, />7</);
});

test('clears the refresh timer and static panel reference on dispose', () => {
  const panel = createPanel(async () => ({}));
  TokenUsagePanel.currentPanel = panel;
  panel._scheduleAutoRefresh();
  assert.notEqual(panel._autoRefreshInterval, null);
  panel.dispose();
  assert.equal(panel._autoRefreshInterval, null);
  assert.equal(TokenUsagePanel.currentPanel, undefined);
});
