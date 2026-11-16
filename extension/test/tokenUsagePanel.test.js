'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const Module = require('node:module');

const listeners = {};
const saved = { value: null };
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

function createPanel() {
  const context = {
    subscriptions: [],
    globalState: {
      async get() { return saved.value; },
      async update(_key, value) { saved.value = structuredClone(value); },
    },
  };
  const panel = new TokenUsagePanel(vscode.window.createWebviewPanel(), context);
  panel._clearAutoRefresh();
  return panel;
}

test('initializes zeroed usage and renders the current panel', () => {
  const panel = createPanel();
  assert.equal(panel._usageData.inputTokens, 0);
  assert.equal(panel._usageData.outputTokens, 0);
  assert.equal(panel._usageData.monthlyBudget, 100);
  assert.match(panel._panel.webview.html, /Token 消耗追踪/);
  assert.match(panel._panel.webview.html, /月度预算/);
});

test('calculates estimated token cost from input and output counts', () => {
  const panel = createPanel();
  assert.equal(panel._calculateCost(1_000_000, 1_000_000), 2);
  assert.equal(panel._calculateCost(0, 0), 0);
});

test('aggregates daily usage for today, week, and month', () => {
  const panel = createPanel();
  const now = new Date();
  const today = now.toISOString().slice(0, 10);
  const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000).toISOString().slice(0, 10);
  panel._usageData.dailyStats = {
    [today]: { input: 10, output: 5 },
    [yesterday]: { input: 20, output: 10 },
  };
  assert.deepEqual(panel._getStatsByPeriod('today'), { input: 10, output: 5, cost: panel._calculateCost(10, 5) });
  assert.equal(panel._getStatsByPeriod('week').input, 30);
  assert.equal(panel._getStatsByPeriod('month').output, 15);
});

test('persists only positive monthly budgets', async () => {
  const panel = createPanel();
  panel._updateBudget(250);
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(panel._usageData.monthlyBudget, 250);
  assert.equal(saved.value.monthlyBudget, 250);
  panel._updateBudget(0);
  panel._updateBudget(-1);
  assert.equal(panel._usageData.monthlyBudget, 250);
});

test('exports daily usage as CSV with totals and estimated cost', () => {
  const panel = createPanel();
  panel._usageData.dailyStats = { '2026-11-16': { input: 10, output: 5 } };
  const csv = panel._generateCSV();
  assert.match(csv, /^Date,InputTokens,OutputTokens,TotalTokens,EstimatedCost/);
  assert.match(csv, /2026-11-16,10,5,15,/);
});

test('exports JSON with the persisted usage schema', async () => {
  const panel = createPanel();
  let exported;
  panel._downloadFile = async (_name, content, mime) => { exported = { content, mime }; };
  await panel._export('json');
  assert.equal(exported.mime, 'application/json');
  assert.equal(JSON.parse(exported.content).monthlyBudget, 100);
});

test('loads saved usage without discarding defaults', async () => {
  saved.value = { inputTokens: 7, monthlyBudget: 80 };
  const panel = createPanel();
  await panel._load();
  assert.equal(panel._usageData.inputTokens, 7);
  assert.equal(panel._usageData.outputTokens, 0);
  assert.equal(panel._usageData.monthlyBudget, 80);
  saved.value = null;
});

test('clears the refresh timer and static panel reference on dispose', () => {
  const panel = createPanel();
  TokenUsagePanel.currentPanel = panel;
  panel._scheduleAutoRefresh();
  assert.notEqual(panel._autoRefreshInterval, null);
  panel.dispose();
  assert.equal(panel._autoRefreshInterval, null);
  assert.equal(TokenUsagePanel.currentPanel, undefined);
});
