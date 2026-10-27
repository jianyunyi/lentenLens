'use strict';

const { execFileSync } = require('node:child_process');

const files = execFileSync('git', ['ls-files', '--', '*.vsix'], { encoding: 'utf8' })
  .split(/\r?\n/)
  .filter(Boolean);

if (files.length) {
  console.error(`Tracked VSIX artifacts are forbidden:\n${files.join('\n')}`);
  process.exitCode = 1;
} else {
  console.log('PASS: no tracked VSIX artifacts.');
}
