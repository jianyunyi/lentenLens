'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const projectRoot = path.resolve(__dirname, '..', '..');
const skillPath = path.join(projectRoot, '.cursor', 'skills', 'intentlens', 'SKILL.md');
const skill = fs.readFileSync(skillPath, 'utf8');

test('project Skill reports partial index coverage and unresolved calls honestly', () => {
  for (const field of ['coverage.complete', 'omitted_count', 'excluded_count', 'truncated', 'calls[].status']) {
    assert.ok(skill.includes(`\`${field}\``), `Skill must check ${field}`);
  }
  for (const status of ['resolved', 'unresolved', 'ambiguous']) {
    assert.ok(skill.includes(`\`${status}\``), `Skill must preserve ${status} call status`);
  }
  assert.match(skill, /最多 20 条样例/);
  assert.match(skill, /不要仅因文件\/函数没出现在不完整索引中就断言它不存在/);
});
