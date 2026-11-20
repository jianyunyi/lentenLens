'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const Module = require('node:module');

function createPanel(data) {
  const copies = [], navigations = [];
  const vscode = {
    env: { clipboard: { writeText: async text => { copies.push(text); } } },
    window: { showInformationMessage() {} },
  };
  const webview = { html: '', onDidReceiveMessage(fn) { this.receive = fn; } };
  const panel = { webview, onDidDispose() {} };
  const load = Module._load;
  Module._load = function(id, ...args) { return id === 'vscode' ? vscode : load.call(this, id, ...args); };
  try {
    const modulePath = require.resolve('../src/restrictionDetailPanel');
    delete require.cache[modulePath];
    const { RestrictionDetailPanel } = require(modulePath);
    new RestrictionDetailPanel(panel, data, new Map([[categories[0].items[0], 'host-record']]), id => navigations.push(id));
  } finally {
    Module._load = load;
  }
  return { webview, copies, navigations };
}

const message = "expected <T> & \"quoted\" 'name'";
const filePath = '/fixture/workspace/odd<&>.go';
const categories = [
  { name: 'Types <T>', count: 2, icon: '&', items: [
    { message, filePath, lineNumber: 7 },
    { type: 'summary', message: 'Summary <T>', fullList: ['a<b', 'c&d', 'e', 'f', 'g', 'sixth-item'] },
  ] },
  { name: 'Other', count: 1, icon: '•', items: [{ message: 'another diagnostic' }] },
];

test('detail panel escapes all diagnostic text and uses only nonce-authorized styles and scripts', () => {
  const { webview } = createPanel({ categories });
  const { html } = webview;
  const nonce = html.match(/<script nonce="([a-f0-9]+)">/)[1];
  assert.ok(html.includes("default-src 'none'; script-src 'nonce-" + nonce + "'; style-src 'nonce-" + nonce + "';"));
  assert.deepEqual([...html.matchAll(/<(?:style|script)\b[^>]*>/g)].map(match => match[0]), [
    '<style nonce="' + nonce + '">', '<script nonce="' + nonce + '">',
  ]);
  assert.doesNotMatch(html, /<[^>]+\s+on[a-z]+\s*=/i);
  assert.doesNotMatch(html, /<T>|sixth-item/);
  assert.match(html, /addEventListener\('click'/);
  assert.match(html, /&amp; Types &lt;T&gt;/);
  assert.match(html, /expected &lt;T&gt; &amp; &quot;quoted&quot; &#39;name&#39;/);
  assert.match(html, /odd&lt;&amp;&gt;\.go:7/);
  assert.match(html, /Summary &lt;T&gt;.*a&lt;b.*c&amp;d/);

  const empty = createPanel(undefined).webview.html;
  assert.match(empty, /default-src 'none'.*style-src 'nonce-([a-f0-9]+)';">\s*<style nonce="\1">/);
});

test('detail panel resolves copy and navigation through host records and invalidates old IDs on filter changes', async () => {
  const { webview, copies, navigations } = createPanel({ categories });
  const id = webview.html.match(/data-command="navigate" data-id="([^"]+)"/)[1];
  const [, summaryId] = [...webview.html.matchAll(/data-command="copy" data-id="([^"]+)"/g)].map(match => match[1]);

  webview.receive({ command: 'copy', text: 'untrusted text' });
  webview.receive({ command: 'navigate', id: 'unknown', filePath: '/other.go', line: 1 });
  webview.receive({ command: 'navigate', id: summaryId, filePath: '/other.go', line: 1 });
  assert.deepEqual(copies, []);
  assert.deepEqual(navigations, []);

  webview.receive({ command: 'copy', id, text: 'untrusted text' });
  webview.receive({ command: 'copy', id: summaryId });
  webview.receive({ command: 'navigate', id, filePath: '/other.go', line: 1 });
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(copies, [message, 'a<b, c&d, e, f, g, sixth-item']);
  assert.deepEqual(navigations, ['host-record']);

  const filter = webview.html.match(/data-filter="([^"]+)">• Other<\/button>/)[1];
  webview.receive({ command: 'filter', id: filter });
  assert.match(webview.html, /another diagnostic/);
  assert.doesNotMatch(webview.html, /quoted/);
  webview.receive({ command: 'copy', id });
  webview.receive({ command: 'navigate', id });
  assert.equal(copies.length, 2);
  assert.equal(navigations.length, 1);
});
