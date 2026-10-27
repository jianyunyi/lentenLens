'use strict';
// Local synthetic oracle only. No model calls, private project source, or dependencies.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { execFileSync, spawnSync } = require('node:child_process');

const binary = process.argv[2] && path.resolve(process.argv[2]);
if (!binary || !fs.existsSync(binary)) throw new Error('Usage: node scripts/callpath-acceptance.cjs /absolute/path/to/intentlens [result.json]');
const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'intentlens-callpath-oracle-'));
const root = path.join(temporary, 'fixture');
const state = path.join(temporary, 'state');
fs.mkdirSync(root);
const source = [
  'package main', '',
  'func main() { Handle() }',
  'func CLI() { Handle() }',
  'func Handle() {',
  '\tValidate()',
  '\tSave()',
  '}',
  'func Validate() { Shared() }',
  'func Save() { Shared() }',
  'func Shared() {}',
  'func Disconnected() {}', '',
].join('\n');
const id = name => `main.go:${name}`;
const expectedFunctions = ['main', 'CLI', 'Handle', 'Validate', 'Save', 'Shared'].map(id).sort();
const expectedRoots = ['main', 'CLI'].map(id).sort();
const expectedEdges = [
  ['main', 'Handle'], ['CLI', 'Handle'], ['Handle', 'Validate'],
  ['Handle', 'Save'], ['Validate', 'Shared'], ['Save', 'Shared'],
].map(([a,b]) => `${id(a)}->${id(b)}`).sort();
fs.writeFileSync(path.join(root, 'main.go'), source);
// Deliberately do not inherit API credentials or a user's project configuration.
const env = { PATH: process.env.PATH || '', HOME: temporary, TMPDIR: temporary, GOTOOLCHAIN: 'local', GOPROXY: 'off', GOSUMDB: 'off' };
function cli(command, input) {
  return JSON.parse(execFileSync(binary, [command, '--root', root, '--state', state], {
    input: input === undefined ? undefined : JSON.stringify(input), encoding: 'utf8', env,
  }));
}
function rejected(input, pattern) {
  const result = spawnSync(binary, ['call-path', '--root', root, '--state', state], {input:JSON.stringify(input),encoding:'utf8',env});
  assert.notEqual(result.status, 0);
  assert.match(JSON.parse(result.stdout).error, pattern);
}
try {
  const before = cli('status');
  assert.equal(before.settings.model.enabled, false);
  const graph = cli('call-path', { file:'main.go', line:5 });
  assert.equal(graph.selected_id, id('Handle'));
  assert.deepEqual(graph.functions.map(f=>f.id).sort(), expectedFunctions);
  assert.deepEqual([...graph.root_ids].sort(), expectedRoots);
  assert.equal(graph.truncated, false);
  const actualEdges = graph.calls.filter(c=>c.status==='resolved').flatMap(c=>c.targets.map(target=>`${c.caller_id}->${target}`)).sort();
  assert.deepEqual(actualEdges, expectedEdges);
  assert.equal(graph.calls.length, 6);
  const lines = source.split('\n');
  for (const call of graph.calls) {
    assert.equal(call.file, 'main.go');
    assert.ok(Number.isInteger(call.line) && Number.isInteger(call.column));
    assert.equal(lines[call.line-1].slice(call.column-1, call.column-1+call.source.length), call.source);
    assert.equal(call.source, `${call.name}()`);
  }
  for (const fn of graph.functions) {
    assert.equal(fn.source, lines.slice(fn.start_line-1, fn.end_line).join('\n'));
  }
  const second = cli('call-path', {file:'main.go',line:5});
  assert.deepEqual(second, graph, 'same input must have deterministic local output');
  assert.equal(fs.readFileSync(path.join(root,'main.go'),'utf8'),source);
  rejected({file:'../outside.go',line:1}, /路径|排除|不存在|允许|不安全/);
  fs.writeFileSync(path.join(root,'.intentignore'),'main.go\n');
  rejected({file:'main.go',line:5},/排除/);
  const after = cli('status');
  assert.equal(after.settings.model.enabled, false);
  assert.deepEqual(after.usage,before.usage,'source navigation must not consume model usage');
  const result = {
    result:'PASS', oracle:'one independent synthetic multi-root diamond fixture',
    selected_functions:{correct:1,total:1}, graph_function_ids:{correct:6,total:6},
    resolved_edges:{correct:6,total:6,false_positive:0,false_negative:0},
    roots:{correct:2,total:2}, exact_callsite_evidence:{correct:6,total:6},
    checks:['deterministic JSON','unrelated function excluded','source unchanged','path traversal rejected','policy exclusion rejected','model disabled','model usage unchanged'],
    model_requests:0,
    limitation:'This evaluates only deterministic source facts in a synthetic fixture, not model answers or real-world completeness.',
  };
  if (process.argv[3]) fs.writeFileSync(path.resolve(process.argv[3]),JSON.stringify(result,null,2)+'\n');
  console.log(JSON.stringify(result,null,2));
} finally {
  fs.rmSync(temporary,{recursive:true,force:true});
}
