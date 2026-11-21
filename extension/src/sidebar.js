'use strict';

function makeItem(vscode, label, collapsible = vscode.TreeItemCollapsibleState.None) {
  return new vscode.TreeItem(label, collapsible);
}

function placeholder(vscode, message) {
  const item = makeItem(vscode, message);
  item.contextValue = 'intentlens.sidebar.placeholder';
  item.tooltip = message;
  return item;
}

const CANCELLED = Symbol('cancelled');

function isAbortError(error) {
  return error?.name === 'AbortError' || error?.code === 'ABORT_ERR';
}

function createModelAnswerProvider({ vscode, treeChanged, getCurrentTarget, issueEvidenceRecord, onInvalidate = () => {}, debounceMs = 150, schedule = setTimeout, cancelScheduled = clearTimeout }) {
  let currentTarget;
  let targetLoaded = false;
  let generation = 0;
  let pending;
  let disposed = false;
  const state = vscode.TreeItemCollapsibleState;

  function cancelPending() {
    const request = pending;
    if (!request) return;
    pending = undefined;
    if (request.timer !== undefined) cancelScheduled(request.timer);
    request.controller?.abort();
    request.resolve(CANCELLED);
  }

  function refresh() {
    if (disposed) return;
    generation++;
    onInvalidate(generation);
    targetLoaded = false;
    currentTarget = undefined;
    cancelPending();
    treeChanged.fire(undefined);
  }

  function target(requestedGeneration) {
    if (disposed || requestedGeneration !== generation) return Promise.resolve(CANCELLED);
    if (targetLoaded) return Promise.resolve(currentTarget);
    if (pending?.generation === requestedGeneration) return pending.promise;
    cancelPending();

    let resolveRequest, rejectRequest;
    const request = {
      generation: requestedGeneration,
      timer: undefined,
      controller: undefined,
      promise: new Promise((resolve, reject) => { resolveRequest = resolve; rejectRequest = reject; }),
      resolve: value => resolveRequest(value),
      reject: error => rejectRequest(error),
    };
    pending = request;
    request.timer = schedule(() => {
      request.timer = undefined;
      if (disposed || pending !== request || request.generation !== generation) {
        request.resolve(CANCELLED);
        return;
      }
      const controller = new AbortController();
      request.controller = controller;
      Promise.resolve()
        .then(() => getCurrentTarget({ signal: controller.signal }))
        .then(value => {
          if (disposed || pending !== request || request.generation !== generation || controller.signal.aborted) {
            request.resolve(CANCELLED);
            return;
          }
          currentTarget = value;
          targetLoaded = true;
          pending = undefined;
          request.resolve(value);
        }, error => {
          if (disposed || pending !== request || request.generation !== generation || controller.signal.aborted || isAbortError(error)) {
            if (pending === request) pending = undefined;
            request.resolve(CANCELLED);
            return;
          }
          pending = undefined;
          request.reject(error);
        });
    }, debounceMs);
    return request.promise;
  }

  function explanationItem(entry, index, active) {
    const label = index === 0 ? `最新回答 · ${entry.created || entry.id || '日期未知'}` : `历史回答 · ${entry.created || entry.id || '日期未知'}`;
    const item = makeItem(vscode, label, state.Collapsed);
    item.contextValue = 'intentlens.sidebar.explanation';
    item.entry = entry;
    item.active = active;
    item.generation = generation;
    item.description = entry.freshness || '本地保存的模型回答';
    item.tooltip = '只读展示本地已保存的回答与引用证据。打开此视图不会请求模型。';
    return item;
  }

  function claimItem(entry, claim, index, active) {
    const citations = Array.isArray(claim.evidence_ids) ? claim.evidence_ids : [];
    const hasEvidence = citations.some(id => (entry.evidence || []).some(ev => ev.id === id));
    const item = makeItem(vscode, String(claim.text || '(空白结论)'), hasEvidence ? state.Collapsed : state.None);
    item.contextValue = 'intentlens.sidebar.claim';
    item.claim = claim;
    item.entry = entry;
    item.active = active;
    item.generation = generation;
    item.claimIndex = index;
    item.description = [claim.section, claim.status || claim.kind].filter(Boolean).join(' · ');
    item.tooltip = `${claim.text || ''}${citations.length ? `\n引用：${citations.join(', ')}` : '\n未附证据引用'}`;
    return item;
  }

  function evidenceItem(active, entry, ev, id) {
    if (!ev) return placeholder(vscode, `未找到引用证据：${id}`);
    const item = makeItem(vscode, `${ev.file || '源码'}:${ev.start_line || '?'}–${ev.end_line || ev.start_line || '?'}`);
    item.contextValue = 'intentlens.sidebar.evidence';
    item.description = `证据 ${ev.id}`;
    item.tooltip = `打开本地保存的证据快照 · ${ev.hash || '无快照 hash'}`;
    item.generation = generation;
    const recordID = issueEvidenceRecord?.({ generation, root: active?.folder?.uri?.fsPath, file: active?.file, line: active?.line, explanationID: entry.id, evidenceID: ev.id });
    if (typeof recordID === 'string' && recordID) {
      item.command = { command: 'intentlens.openModelEvidence', title: '打开引用证据快照', arguments: [recordID] };
    }
    return item;
  }

  return {
    onDidChangeTreeData: treeChanged.event,
    getTreeItem: item => item,
    refresh,
    dispose() {
      if (disposed) return;
      disposed = true;
      generation++;
      onInvalidate(generation);
      targetLoaded = false;
      currentTarget = undefined;
      cancelPending();
    },
    async getChildren(item) {
      if (disposed) return [];
      if (item && item.generation !== generation) return [placeholder(vscode, '此侧栏节点已失效，请刷新后重新展开。')];
      try {
        if (!item) {
          const requestedGeneration = generation;
          const active = await target(requestedGeneration);
          if (disposed) return [];
          if (requestedGeneration !== generation || active === CANCELLED) return [placeholder(vscode, '当前文件或光标已变化，请重新展开侧栏。')];
          if (!active) return [placeholder(vscode, '打开已保存的 Go 文件，并将光标放在需要查看的函数中。')];
          if (active.message) return [placeholder(vscode, active.message)];
          const fn = active.info?.function;
          if (!fn) return [placeholder(vscode, '当前位置没有可解析的 Go 函数。')];
          const history = (active.info.history || []).slice().sort((a, b) => String(b.created || '').localeCompare(String(a.created || '')) || String(b.id || '').localeCompare(String(a.id || '')));
          if (!history.length) return [placeholder(vscode, `函数 ${fn.name || '当前函数'} 暂无本地保存的模型回答。此处只读显示结果，不会自动请求模型。`)];
          const root = makeItem(vscode, `模型回答 · ${fn.name || '当前函数'}`, state.Collapsed);
          root.contextValue = 'intentlens.sidebar.answerRoot';
          root.active = active;
          root.generation = generation;
          root.history = history;
          root.description = '本地已保存 · 点击展开回答与证据';
          root.tooltip = '仅显示本地保存且仍可读取的解释历史；结论需结合引用证据核对。';
          return [root];
        }
        if (item.contextValue === 'intentlens.sidebar.answerRoot') return item.history.map((entry, index) => explanationItem(entry, index, item.active));
        if (item.contextValue === 'intentlens.sidebar.explanation') {
          const entry = item.entry;
          const claims = entry.document?.claims || [];
          const rows = claims.map((claim, index) => claimItem(entry, claim, index, item.active));
          for (const unknown of entry.document?.unknowns || []) {
            const row = makeItem(vscode, `未确定 · ${String(unknown)}`);
            row.contextValue = 'intentlens.sidebar.unknown';
            rows.push(row);
          }
          return rows.length ? rows : [placeholder(vscode, '此版本没有可展示的回答条目。')];
        }
        if (item.contextValue === 'intentlens.sidebar.claim') {
          const entry = item.entry;
          const evidence = entry.evidence || [];
          const ids = Array.isArray(item.claim.evidence_ids) ? item.claim.evidence_ids : [];
          return ids.map(id => evidenceItem(item.active, entry, evidence.find(ev => ev.id === id), id));
        }
        return [];
      } catch (error) {
        if (disposed) return [];
        if (isAbortError(error)) return [placeholder(vscode, '当前文件或光标已变化，请重新展开侧栏。')];
        return [placeholder(vscode, `无法读取本地模型回答：${error.message || String(error)}`)];
      }
    },
  };
}

module.exports = { createModelAnswerProvider };
