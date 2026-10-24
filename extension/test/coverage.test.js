const test = require('node:test');
const assert = require('node:assert/strict');
const { coverageSummary, coverageLines, limitedCoverage } = require('../src/coverage');
const { renderIndexStatus, renderInspection } = require('../src/core');

test('coverage distinguishes missing old CLI data, complete scope, and exclusions', () => {
  assert.match(coverageSummary(), /覆盖范围未知/);
  assert.equal(limitedCoverage(), false);
  assert.match(coverageSummary({ complete: true }), /索引完整/);
  assert.equal(limitedCoverage({ complete: true }), false);
  assert.match(coverageSummary({ complete: false, excluded_count: 4 }), /4 项文件或目录/);
  assert.match(coverageSummary({ complete: false, omitted_count: 3 }), /3 个 Go 文件未纳入/);
  assert.ok(limitedCoverage({ complete: true, omitted_count: 1 }), 'counts cannot falsely claim completeness');
});

test('coverage diagnostics give budget, single-file size, parse and exclusion reasons', () => {
  const c = { complete: false, captured_bytes: 16777216, budget_bytes: 16777216, excluded_count: 1, omitted_count: 4, omitted: [
    { file: 'a.go', reason: 'snapshot_budget' }, { file: 'b.go', reason: 'file_too_large' },
    { file: 'c.go', reason: 'parse_error' }, { file: 'd.go', reason: 'unsafe_path' },
  ] };
  const text = coverageLines(c).join('\n');
  assert.match(text, /16.00 \/ 16.00 MiB/);
  assert.match(text, /总预算已耗尽/);
  assert.match(text, /单文件大小上限/);
  assert.match(text, /Go 语法/);
  assert.match(text, /不安全路径/);
  assert.match(text, /不列出被排除路径/);
});

test('both full inspection and diagnostic view escape paths and cap detail samples', () => {
  const coverage = { complete: false, omitted_count: 100, truncated: true, omitted: Array.from({ length: 100 }, (_, i) => ({ file: `file-${i}-<img>.go`, reason: 'snapshot_budget' })) };
  for (const html of [renderIndexStatus({ coverage }, 'nonce'), renderInspection({ coverage, function: {}, evidence: [] }, 'nonce')]) {
    assert.match(html, /索引范围不完整/);
    assert.match(html, /&lt;img&gt;/);
    assert.doesNotMatch(html, /<img>/);
    assert.match(html, /示例已截断/);
    assert.doesNotMatch(html, /file-20-/);
    assert.match(html, /100 个 Go 文件/);
  }
});
