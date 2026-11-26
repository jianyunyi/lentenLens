'use strict';

// Logging facade over the extension's "IntentLens" output channel.
// The channel is injected so formatting and failure behaviour can be
// unit-tested without the extension host. Only event kinds, CLI command
// names, and error messages are ever logged — never CLI arguments,
// source code, or credentials.
function createLogger(channel) {
  const write = line => {
    try { channel?.appendLine(line); } catch { /* Logging must never break a command. */ }
  };
  return {
    activated() { write('IntentLens activated.'); },
    cliInvoked(command) { write(`CLI invoked: ${command}`); },
    cliFailed(command, error) { write(`CLI failed: ${command}: ${error?.message || String(error)}`); },
  };
}

// Creates the real output channel and registers it for disposal.
// Hosts whose vscode object lacks createOutputChannel (unit-test
// stubs) get a silent no-op logger instead of an activation failure.
function createOutputLogger(subscriptions) {
  let channel;
  try {
    channel = require('vscode').window.createOutputChannel('IntentLens');
  } catch { channel = undefined; }
  if (channel && subscriptions) subscriptions.push(channel);
  return createLogger(channel);
}

module.exports = { createLogger, createOutputLogger };
