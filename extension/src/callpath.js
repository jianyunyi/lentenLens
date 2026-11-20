'use strict';

const { coverageSummary } = require('./coverage');
const MAX_NODES = 200;
const MAX_CALLS = 1000;
const MAX_RENDERED_OCCURRENCES = 2000;
const STATUSES = new Set(['resolved', 'unresolved', 'ambiguous', 'candidate', 'cycle', 'unknown']);

function text(value, fallback = '') { return typeof value === 'string' ? value : fallback; }
function positiveLine(value) { return Number.isInteger(value) && value > 0 && value <= 100000000 ? value : undefined; }
function positiveColumn(value) { return Number.isInteger(value) && value >= 0 && value <= 100000000 ? value : 0; }

function callPathEvidenceFingerprint(payload) {
  const evidence = { ...payload };
  if (evidence.aggregated && typeof evidence.aggregated === 'object' && !Array.isArray(evidence.aggregated)) {
    // Generation time is display metadata; retain every diagnostic and graph field.
    evidence.aggregated = { ...evidence.aggregated };
    delete evidence.aggregated.timestamp;
  }
  return JSON.stringify(evidence);
}

function validatePathPayload(payload) {
  if (!payload || typeof payload !== 'object' || !Array.isArray(payload.functions) || !Array.isArray(payload.calls)) {
    throw new Error('调用路径响应格式无效');
  }
  const inputFunctions = payload.functions;
  const inputCalls = payload.calls;
  const functions = inputFunctions.slice(0, MAX_NODES).filter(fn => fn && typeof fn === 'object' && typeof fn.id === 'string' && fn.id && typeof fn.name === 'string' && typeof fn.file === 'string' && positiveLine(fn.start_line));
  const ids = new Set(functions.map(fn => fn.id));
  const calls = inputCalls.slice(0, MAX_CALLS).filter(call => call && typeof call === 'object' && typeof call.caller_id === 'string' && ids.has(call.caller_id) && typeof call.name === 'string' && Array.isArray(call.targets) && typeof call.file === 'string' && positiveLine(call.line)).map(call => {
    const targets = call.targets.filter(id => typeof id === 'string' && id);
    let status = STATUSES.has(call.status) ? call.status : (targets.length ? 'candidate' : 'unknown');
    if (!targets.length) status = status === 'resolved' ? 'unresolved' : status;
    else if (status === 'unresolved' || status === 'unknown') status = targets.length > 1 ? 'ambiguous' : 'candidate';
    else if (status === 'resolved' && targets.length > 1) status = 'ambiguous';
    return { ...call, targets, status, column: positiveColumn(call.column) };
  });
  const rootIds = Array.isArray(payload.root_ids) ? payload.root_ids.filter(id => typeof id === 'string' && ids.has(id)).slice(0, MAX_NODES) : [];
  const selectedId = typeof payload.selected_id === 'string' && ids.has(payload.selected_id) ? payload.selected_id : undefined;
  const truncated = payload.truncated === true || inputFunctions.length > MAX_NODES || inputCalls.length > MAX_CALLS || (Array.isArray(payload.root_ids) && payload.root_ids.length > MAX_NODES) || (Array.isArray(payload.issues) && payload.issues.length > 100);
  return { selected_id: selectedId, root_ids: rootIds, functions, calls, coverage: payload.coverage, issues: Array.isArray(payload.issues) ? payload.issues.filter(x => typeof x === 'string').slice(0, 100) : [], truncated };
}

function makeItem(vscode, label, state = vscode.TreeItemCollapsibleState.None) {
  const item = new vscode.TreeItem(label, state);
  item.label = label;
  return item;
}

function createCallPathProvider(vscode, options = {}) {
  let payload;
  let rootRows;
  let rendered = 0;
  let capped = false;
  const changed = new vscode.EventEmitter();
  const provider = {
    onDidChangeTreeData: changed.event,
    setData(value) { payload = value ? validatePathPayload(value) : undefined; rootRows = undefined; rendered = 0; capped = false; changed.fire(); },
    clear() { payload = undefined; rootRows = undefined; rendered = 0; capped = false; changed.fire(); },
    getTreeItem(item) { return item; },
    getChildren(item) {
      if (!payload) {
        const action = makeItem(vscode, '显示当前 Go 函数的调用路径', vscode.TreeItemCollapsibleState.None);
        action.command = { command: 'intentlens.callPath', title: '显示调用路径' };
        return [action];
      }
      if (!item) {
        if (rootRows) return rootRows;
        const rows = [];
        rows.push(makeItem(vscode, '调用路径（源码证据）', vscode.TreeItemCollapsibleState.None));
        rows.push(makeItem(vscode, '不展示隐藏思考或原始推理链；不会请求模型', vscode.TreeItemCollapsibleState.None));
        if (payload.coverage) rows.push(makeItem(vscode, `覆盖范围：${coverageSummary(payload.coverage)}`));
        if (payload.truncated) rows.push(makeItem(vscode, '结果已截断：达到本地节点/调用点上限', vscode.TreeItemCollapsibleState.None));
        for (const issue of payload.issues) rows.push(makeItem(vscode, `提示：${issue}`));
        const functions = new Map(payload.functions.map(fn => [fn.id, fn]));
        const roots = payload.root_ids.length ? payload.root_ids : inferRoots(payload, functions);
        for (const id of roots) {
          const fn = functions.get(id);
          if (fn) { const row = budgetRow(() => functionItem(vscode, fn, new Set(), 'root')); if (row) rows.push(row); else break; }
        }
        if (!rows.length) rows.push(makeItem(vscode, '没有可显示的源码调用路径'));
        rootRows = rows;
        return rows;
      }
      if (item._children) return item._children;
      let rows = [];
      if (item.kind === 'function') rows = functionChildren(vscode, item);
      else if (item.kind === 'call') rows = callChildren(vscode, item);
      item._children = rows;
      return rows;
    },
    dispose() { changed.dispose(); payload = undefined; rootRows = undefined; }
  };
  function budgetRow(make) {
    if (rendered >= MAX_RENDERED_OCCURRENCES) {
      if (!capped) { capped = true; return makeItem(vscode, '展开已限制：达到源码路径展示上限'); }
      return null;
    }
    rendered++;
    return make();
  }
  function functionItem(vscode, fn, seen, relation) {
    const selected = fn.id === payload.selected_id;
    const cycle = seen.has(fn.id);
    const label = `${selected ? '● ' : ''}${fn.name}${selected ? ' [当前]' : ''}${cycle ? ' [循环]' : ''}`;
    const item = makeItem(vscode, label, cycle ? vscode.TreeItemCollapsibleState.None : vscode.TreeItemCollapsibleState.Collapsed);
    item.kind = 'function'; item.fn = fn; item.seen = new Set(seen); item.seen.add(fn.id); item.relation = relation;
    item.description = `${fn.file}:${fn.start_line}-${fn.end_line}`;
    item.tooltip = `${fn.name}\n源码证据：${fn.file}:${fn.start_line}`;
    item.command = { command: 'intentlens.openCallPathSite', title: '打开函数源码', arguments: [{ file: fn.file, line: fn.start_line, column: 0, function_id: fn.id, source: fn.source }] };
    return item;
  }
  function functionChildren(vscode, item) {
    if (item.seen.size > MAX_NODES) return [makeItem(vscode, '展开已限制：避免循环或菱形路径爆炸')];
    const rows = [];
    for (const call of payload.calls.filter(call => call.caller_id === item.fn.id)) {
      const row = budgetRow(() => callItem(vscode, call, item.seen));
      if (row) rows.push(row); else break;
    }
    return rows;
  }
  function callItem(vscode, call, seen) {
    const targets = call.targets;
    const status = call.status;
    const badge = status === 'cycle' ? ' [循环]' : status === 'ambiguous' || status === 'candidate' || targets.length > 1 ? ' [候选]' : !targets.length ? ' [未知]' : '';
    const item = makeItem(vscode, `↳ ${call.name}${badge} · ${call.file}:${call.line}`, targets.length ? vscode.TreeItemCollapsibleState.Collapsed : vscode.TreeItemCollapsibleState.None);
    item.kind = 'call'; item.call = call; item.seen = new Set(seen); item.targets = targets;
    item.description = `${call.caller_id} → ${targets.join(', ') || '未解析'}`;
    item.tooltip = `${call.caller_id} → ${call.name}\n调用点：${call.file}:${call.line}${call.column ? `:${call.column}` : ''}\n证据：${text(call.source, '源码调用点')}`;
    if (typeof call.file === 'string' && positiveLine(call.line)) item.command = { command: 'intentlens.openCallPathSite', title: '打开源码调用点', arguments: [call] };
    return item;
  }
  function callChildren(vscode, item) {
    const functions = new Map(payload.functions.map(fn => [fn.id, fn]));
    const rows = [];
    for (const id of item.targets) {
      const row = budgetRow(() => { const fn = functions.get(id); if (!fn) return makeItem(vscode, `${id} [未知/未纳入范围]`); if (item.seen.has(id)) return functionItem(vscode, fn, new Set([...item.seen, id]), 'cycle'); return functionItem(vscode, fn, item.seen, id === payload.selected_id ? 'selected' : 'downstream'); });
      if (row) rows.push(row); else break;
    }
    return rows;
  }
  return provider;
}
function inferRoots(payload, functions) {
  const targeted = new Set(payload.calls.flatMap(call => call.targets.filter(id => functions.has(id))));
  return payload.functions.filter(fn => !targeted.has(fn.id)).map(fn => fn.id).slice(0, MAX_NODES);
}
function coverageText(coverage) { return coverageSummary(coverage); }

module.exports = { createCallPathProvider, callPathEvidenceFingerprint, validatePathPayload, coverageText, MAX_NODES, MAX_CALLS, MAX_RENDERED_OCCURRENCES };
