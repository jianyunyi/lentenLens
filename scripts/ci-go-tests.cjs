'use strict';
// Stream the Go event log instead of keeping an unbounded child-process buffer.
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const readline = require('node:readline');
const output = process.argv[2] || 'go-tests.jsonl';
const log = fs.createWriteStream(output);
const child = spawn('go', ['test', '-json', '-count=1', '-timeout=20m', './...'], { shell: false, stdio: ['ignore', 'pipe', 'inherit'] });
const requiredWindowsTests = new Set([
  'intentlens/internal/capture:TestWindowsJunctionDenied',
  'intentlens/internal/capture:TestTakeGoJunctionPreservesLaterSiblings',
  'intentlens/internal/capture:TestTakeGoJunctionPreservesLaterSiblings/root',
  'intentlens/internal/capture:TestTakeGoJunctionPreservesLaterSiblings/nested',
  'intentlens/internal/exercise:TestToolThroughWindowsJunction',
]);
const windowsPassed = new Set();
let failed = false, junctionPassed = false, passed = 0, skipped = 0;
child.on('error', error => { console.error(error.message); failed = true; });
child.stdout.pipe(log);
const lines = readline.createInterface({ input: child.stdout });
lines.on('line', line => {
  let event;
  try { event = JSON.parse(line); } catch { failed = true; return; }
  if (event.Action === 'fail') { failed = true; console.error(`FAIL ${event.Package} ${event.Test || ''}`); }
  if (event.Action === 'output' && /FAIL|panic:|fatal error:|\.go:\d+:/.test(event.Output || '')) process.stderr.write(event.Output);
  if (event.Action === 'pass' && event.Test) passed++;
  if (event.Action === 'skip') { skipped++; console.log(`SKIP ${event.Package} ${event.Test || ''}`); }
  if (event.Action === 'pass') windowsPassed.add(`${event.Package}:${event.Test}`);
  if (event.Test === 'TestWindowsJunctionDenied' && event.Action === 'pass') junctionPassed = true;
});
log.on('error', error => { failed = true; console.error(error.message); child.kill(); });
child.on('close', code => {
  lines.close();
  if (process.platform === 'win32') {
    for (const test of requiredWindowsTests) {
      if (!windowsPassed.has(test)) {
        failed = true;
        console.error(`Windows regression must actually run and pass: ${test}`);
      }
    }
  }
  console.log(JSON.stringify({ passed, skipped, exitCode: code, junctionPassed, log: output }));
  if (code !== 0 || failed || passed === 0) process.exitCode = 1;
});

