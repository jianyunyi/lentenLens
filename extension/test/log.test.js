const test = require('node:test');
const assert = require('node:assert/strict');
const Module = require('node:module');
const { createLogger, createOutputLogger } = require('../src/log');

function fakeChannel() {
  const lines = [];
  return { lines, appendLine: line => lines.push(line) };
}

test('logger writes activation, invocation, and failure lines', () => {
  const channel = fakeChannel();
  const log = createLogger(channel);
  log.activated();
  log.cliInvoked('status');
  log.cliFailed('status', new Error('CLI exited 1'));
  assert.deepEqual(channel.lines, [
    'IntentLens activated.',
    'CLI invoked: status',
    'CLI failed: status: CLI exited 1',
  ]);
});

test('logger stringifies non-Error failures', () => {
  const channel = fakeChannel();
  const log = createLogger(channel);
  log.cliFailed('inspect', 'plain failure');
  log.cliFailed('inspect', undefined);
  assert.deepEqual(channel.lines, [
    'CLI failed: inspect: plain failure',
    'CLI failed: inspect: undefined',
  ]);
});

test('logger tolerates a missing or throwing channel', () => {
  const silent = createLogger(undefined);
  assert.doesNotThrow(() => {
    silent.activated();
    silent.cliInvoked('status');
    silent.cliFailed('status', new Error('x'));
  });
  const throwing = createLogger({ appendLine() { throw new Error('channel closed'); } });
  assert.doesNotThrow(() => throwing.activated());
});

test('createOutputLogger builds a channel-backed logger and registers it for disposal', () => {
  const channel = fakeChannel();
  const subscriptions = [];
  const original = Module._load;
  Module._load = function (id, ...args) {
    return id === 'vscode'
      ? { window: { createOutputChannel: name => { assert.equal(name, 'IntentLens'); return channel; } } }
      : original.call(this, id, ...args);
  };
  try {
    const log = createOutputLogger(subscriptions);
    log.cliInvoked('status');
    assert.deepEqual(channel.lines, ['CLI invoked: status']);
    assert.deepEqual(subscriptions, [channel]);
    const unregistered = createOutputLogger();
    unregistered.activated();
    assert.deepEqual(channel.lines, ['CLI invoked: status', 'IntentLens activated.']);
  } finally { Module._load = original; }
});

test('createOutputLogger degrades to a silent logger when the channel cannot be created', () => {
  const original = Module._load;
  Module._load = function (id, ...args) {
    return id === 'vscode'
      ? { window: { createOutputChannel() { throw new Error('no host'); } } }
      : original.call(this, id, ...args);
  };
  try {
    const subscriptions = [];
    const log = createOutputLogger(subscriptions);
    assert.doesNotThrow(() => {
      log.activated();
      log.cliFailed('status', new Error('x'));
    });
    assert.deepEqual(subscriptions, []);
  } finally { Module._load = original; }
});
