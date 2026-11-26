'use strict';
const vscode = require('vscode');
const path = require('node:path');
const fs = require('node:fs');
const crypto = require('node:crypto');
const { createExerciseUI } = require('./exercises');
const { createInline } = require('./inline');
const { createSessions } = require('./sessions');
const { configureModel, modelCredentialForCall, assertModelSaveIdle, EDITOR_API_KEY_ENV } = require('./providers');
const { createCaptureRecovery } = require('./recovery');
const { createCallPathProvider, callPathEvidenceFingerprint } = require('./callpath');
const { createModelAnswerProvider } = require('./sidebar');
const { coverageSummary, limitedCoverage } = require('./coverage');
const { runCLI, confinedPath, renderInspection, renderIndexStatus, renderRound, renderSharePreview, renderShared, renderSaved, selectedErrorDiagnostics, validateSelectionResult, renderSelectionExplanation, page, escapeHTML } = require('./core');
const { RestrictionTreeProvider } = require('./restrictionView');
const { RestrictionDetailPanel } = require('./restrictionDetailPanel');
const { AIConfigPanel } = require('./aiConfigPanel');
const { TokenUsagePanel } = require('./tokenUsagePanel');
const { createOutputLogger } = require('./log');

const modelCommands = new Set(['explain', 'explain-selection', 'analyze-round', 'revise', 'share-prepare', 'exercise-generate', 'exercise-feedback', 'test-model']);

function activate(context) {
  const log = createOutputLogger(context.subscriptions);
  log.activated();
  const treeChanged = new vscode.EventEmitter(), lensesChanged = new vscode.EventEmitter();
  const panels = new Map(), policyVersions = new Map(), roundPages = new Map();
  let callPathProvider;
  let callPathView;
  let callPathState;
  let modelAnswerProvider;
  const modelEvidenceRecords = new Map();
  let modelAnswerGeneration = 0;
  const indexDiagnostics = new Map();
  const sourcePanels = new Set();
  const indexBar = vscode.window.createStatusBarItem?.(vscode.StatusBarAlignment?.Right, 90);
  if (indexBar) context.subscriptions.push(indexBar);
  function invalidateIndex() { indexDiagnostics.clear(); indexBar?.hide(); }
  function reportIndex(folder, file, data, error) {
    const detail = { root: folder.uri.fsPath, file, coverage: data?.coverage, error: error?.message };
    indexDiagnostics.set(folder.uri.fsPath, detail);
    if (indexBar) {
      indexBar.text = error ? 'IntentLens: 索引失败' : 'IntentLens: 索引范围不完整';
      indexBar.tooltip = error ? `索引失败：${error.message}；点击查看原因。` : coverageSummary(data?.coverage);
      indexBar.command = { command: 'intentlens.indexStatus', title: '查看索引范围与遗漏原因', arguments: [folder.uri] };
      if (error || !data?.coverage || limitedCoverage(data.coverage)) indexBar.show(); else indexBar.hide();
    }
  }
  const guard = fn => async (...args) => { try { return await fn(...args); } catch (error) { vscode.window.showErrorMessage(`IntentLens: ${error.message}`); } };
  let inlineUI;
  let documentEpoch = 0;
  const openFolder = root => (vscode.workspace.workspaceFolders || []).find(folder => folder.uri.scheme === 'file' && folder.uri.fsPath === root);
  function clearInline() {
    documentEpoch++;
    callPathState = undefined;
    inlineUI?.clear();
    callPathProvider?.clear();
    modelAnswerProvider?.refresh();
    for (const view of [...sourcePanels]) view.dispose();
    invalidateIndex();
  }
  function trusted() { if (!vscode.workspace.isTrusted) throw new Error('请先信任工作区；受限模式不会执行 CLI。'); }
  async function folderFor(uri) {
    trusted();
    if (uri) {
      const folder = vscode.workspace.getWorkspaceFolder(uri);
      if (!folder || folder.uri.scheme !== 'file') throw new Error('请选择本地工作区中的文件。');
      return folder;
    }
    const folders = (vscode.workspace.workspaceFolders || []).filter(f => f.uri.scheme === 'file');
    if (!folders.length) throw new Error('请打开本地工作区。');
    if (folders.length === 1) return folders[0];
    const chosen = await vscode.window.showQuickPick(folders.map(folder => ({ label: folder.name, description: folder.uri.fsPath, folder })), { placeHolder: '选择本次操作的工作区根目录' });
    return chosen?.folder;
  }
  // VSIX archives built on Windows carry no Unix mode bits for the bundled CLI.
  function ensureExecutable(file) {
    if (process.platform === 'win32') return file;
    try { fs.accessSync(file, fs.constants.X_OK); } catch { fs.chmodSync(file, 0o755); }
    return file;
  }
  async function cli(folder, command, input, options = {}) {
    trusted();
    const configured = vscode.workspace.getConfiguration('intentlens', folder.uri).get('binaryPath', '');
    const bundled = path.join(context.extensionPath, 'bin', process.platform === 'win32' ? 'intentlens.exe' : 'intentlens');
    const binary = configured || (fs.existsSync(bundled) ? ensureExecutable(bundled) : 'intentlens');
    const version = policyVersions.get(folder.uri.fsPath) || 0;
    const runOptions = { timeout: command === 'exercise-run' ? 240000 : 180000, signal: options.signal };
    if (modelCommands.has(command)) {
      const { key, target, model } = await modelCredentialForCall(folder, cli, context.secrets);
      assertModelSaveIdle(folder);
      if (options.confirmedTarget && (target.provider !== options.confirmedTarget.provider || target.base_url !== options.confirmedTarget.base_url || model.model !== options.confirmedTarget.model)) {
        throw new Error('模型服务配置已变化，请确认配置保存后重新运行连接测试。');
      }
      runOptions.env = { ...process.env, [EDITOR_API_KEY_ENV]: key };
      // The CLI checks the target again against the exact settings used for the request.
      const editorTarget = command === 'test-model' ? { ...target, model: model.model } : target;
      runOptions.args = ['--editor-model-target', JSON.stringify(editorTarget)];
    }
    log.cliInvoked(command);
    let result;
    try {
      result = await runCLI(binary, folder.uri.fsPath, command, input, runOptions);
    } catch (error) {
      log.cliFailed(command, error);
      throw error;
    }
    if (version !== (policyVersions.get(folder.uri.fsPath) || 0)) throw new Error('排除规则已变化，请重新打开视图。');
    return result;
  }
  const sessionState = context.workspaceState || (() => {
    const values = new Map();
    return { get: key => values.get(key), update: (key, value) => { if (value === undefined) values.delete(key); else values.set(key, value); return Promise.resolve(); } };
  })();
  // Deduplication cache to prevent duplicate notifications
  const notificationCache = new Map();
  const notificationTTL = 1000; // 1 second
  function deduplicatedNotify(type, message, showFn) {
    const key = `${type}:${message}`;
    const now = Date.now();
    const cached = notificationCache.get(key);
    if (cached && (now - cached) < notificationTTL) {
      return; // Skip duplicate within TTL
    }
    notificationCache.set(key, now);
    // Cleanup old entries
    if (notificationCache.size > 50) {
      for (const [k, timestamp] of notificationCache.entries()) {
        if (now - timestamp > notificationTTL * 10) notificationCache.delete(k);
      }
    }
    showFn(message);
  }
  const sessions = createSessions({
    workspaceState: sessionState,
    cli: (root, command, input) => {
      const folder = openFolder(root);
      if (!folder) throw new Error('本地工作区已关闭，请重新打开后重试。');
      return cli(folder, command, input);
    },
    notify: {
      info: message => deduplicatedNotify('info', message, msg => vscode.window.showInformationMessage(msg)),
      error: message => deduplicatedNotify('error', message, msg => vscode.window.showErrorMessage(`IntentLens: ${msg}`)),
    },
  });
  const recovery = createCaptureRecovery({ vscode, cli, sessions });
  const refresh = () => { invalidateIndex(); roundPages.clear(); treeChanged.fire(); lensesChanged.fire(); modelAnswerProvider?.refresh(); };
  async function openCurrent(folder, file, line = 1, column = 0) {
    trusted();
    await revealCurrent(confinedPath(folder.uri.fsPath, file), line, column);
  }
  async function revealCurrent(target, line, column = 0, beforeShow = () => {}) {
    const doc = await vscode.workspace.openTextDocument(vscode.Uri.file(target));
    await beforeShow();
    const editor = await vscode.window.showTextDocument(doc);
    const lineIndex = Math.max(0, Math.min(doc.lineCount - 1, Number(line || 1) - 1));
    // Go token columns count UTF-8 bytes (1-based); VS Code uses UTF-16 code
    // units (0-based), so subtracting one alone is wrong after non-ASCII text.
    const character = column > 0 ? Buffer.from(doc.lineAt(lineIndex).text, 'utf8').subarray(0, column - 1).toString('utf8').length : 0;
    const pos = new vscode.Position(lineIndex, character);
    editor.selection = new vscode.Selection(pos, pos);
    editor.revealRange(new vscode.Range(pos, pos), vscode.TextEditorRevealType.InCenter);
  }
  async function snapshot(source, file, beforeShow = () => {}) {
    const document = await vscode.workspace.openTextDocument({ content: source || '', language: file?.endsWith('.go') ? 'go' : 'plaintext' });
    await beforeShow();
    await vscode.window.showTextDocument(document, { preview: false });
  }
  function panel(title, render, handler, folder, tracksSource = false) {
    const view = vscode.window.createWebviewPanel('intentlens.evidence', title, vscode.ViewColumn.Beside, { enableScripts: true, localResourceRoots: [] });
    view.webview.html = render(crypto.randomBytes(18).toString('hex'));
    panels.set(view, folder?.uri.fsPath);
    if (tracksSource) sourcePanels.add(view);
    view.onDidDispose(() => { panels.delete(view); sourcePanels.delete(view); }, undefined, context.subscriptions);
    view.webview.onDidReceiveMessage(guard(async message => {
      trusted();
      if (!panels.has(view)) return;
      if (!message || typeof message.action !== 'string') return;
      await handler(message);
    }), undefined, context.subscriptions);
    context.subscriptions.push(view);
    return view;
  }
  function selectedIndex(array, value) {
    if (typeof value !== 'string' || !/^\d+$/.test(value)) throw new Error('无效证据索引');
    const item = array?.[Number(value)];
    if (!item) throw new Error('证据已不存在');
    return item;
  }
  async function explanationAction(folder, entry, message) {
    if (!entry?.id) throw new Error('解释版本不存在');
    if (message.action === 'exercise') { await exerciseUI.start(folder, entry, message); return; }
    if (message.action === 'flag') {
      const claim = selectedIndex(entry.document?.claims, message.index);
      const reason = await vscode.window.showInputBox({ prompt: '说明有误或证据不支持的原因（仅保存在本地）' });
      if (!reason?.trim()) return;
      await cli(folder, 'flag', { explanation_id: entry.id, claim_id: claim.id, reason });
    } else if (message.action === 'revise') {
      return await cli(folder, 'revise', { explanation_id: entry.id });
    } else if (message.action === 'share') {
      const privateEvidence = (entry.evidence || []).filter(e => e.visibility === 'local' || (e.kind && e.kind !== 'code'));
      let selected = [];
      if (privateEvidence.length) {
        selected = await vscode.window.showQuickPick(privateEvidence.map(e => ({ label: e.id, description: e.kind, detail: e.source, evidenceID: e.id })), { canPickMany: true, placeHolder: '可选：明确允许共享的私有证据（默认不选）。所选原文将发送给模型并纳入预览。' });
        if (!selected) return;
      }
      const draft = await cli(folder, 'share-prepare', { explanation_id: entry.id, approved_evidence_ids: selected.map(e => e.evidenceID) });
      let saving = false;
      const preview = panel('IntentLens · 共享预览', nonce => renderSharePreview(draft, nonce), async action => {
        if (action.action === 'cancel') { preview.dispose(); return; }
        if (action.action !== 'confirmShare' || saving) return;
        saving = true;
        try {
          await cli(folder, 'share-confirm', { draft_id: draft.id, preview_digest: draft.digest, confirm: true });
          preview.dispose();
          vscode.window.showInformationMessage('IntentLens：共享记录已写入 .intent/shared，可通过 Git 提交。');
        } finally { saving = false; }
      }, folder);
    }
    refresh();
  }
  function showSaved(folder, record) {
    let busy = false;
    const view = panel('IntentLens · 本地纠错与修订', nonce => renderSaved(record, nonce), async message => {
      if (busy) return;
      busy = true;
      try {
        const latest = await cli(folder, 'explanation', { explanation_id: record.id });
        if (JSON.stringify(latest) !== JSON.stringify(record)) {
          record = latest;
          view.webview.html = renderSaved(record, crypto.randomBytes(18).toString('hex'));
          throw new Error('解释已变化，请重新选择操作');
        }
        if (['share', 'revise', 'flag', 'exercise'].includes(message.action)) {
          const revision = await explanationAction(folder, record, message);
          if (revision) showSaved(folder, revision);
          record = await cli(folder, 'explanation', { explanation_id: record.id });
          if (panels.has(view)) view.webview.html = renderSaved(record, crypto.randomBytes(18).toString('hex'));
        } else if (['snapshot', 'current'].includes(message.action)) {
          const ev = selectedIndex(record.evidence, message.index);
          if (message.action === 'snapshot') await snapshot(ev.source, ev.file);
          else await openCurrent(folder, ev.file, ev.start_line);
        }
      } catch (error) { if (!panels.has(view)) return; throw error; }
      finally { busy = false; }
    }, folder);
  }
  async function inspect(uri, line, inline = true) {
    uri = uri || vscode.window.activeTextEditor?.document.uri;
    if (!uri) throw new Error('请在 Go 函数中放置光标。');
    const folder = await folderFor(uri);
    if (!folder) return;
    const editor = vscode.window.activeTextEditor;
    if (editor?.document.uri.toString() === uri.toString() && editor.document.isDirty) throw new Error('请先保存文件；分析仅使用磁盘上的代码。');
    const file = path.relative(folder.uri.fsPath, uri.fsPath).split(path.sep).join('/');
    confinedPath(folder.uri.fsPath, file);
    line = line || (editor?.document.uri.toString() === uri.toString() ? editor.selection.active.line + 1 : 1);
    const epoch = documentEpoch;
    let info = await cli(folder, 'inspect', { file, line });
    if (epoch !== documentEpoch) return;
    let busy = false;
    function redraw() {
      if (inline) view.update(info);
      else view.webview.html = renderInspection(info, crypto.randomBytes(18).toString('hex'));
    }
    const handle = async message => {
      if (!['details', 'dependency', 'snapshot', 'current', 'explain', 'ask', 'share', 'revise', 'flag', 'exercise'].includes(message.action)) return;
      let latest;
      try { latest = await cli(folder, 'inspect', { file, line }); }
      catch (error) { view.dispose(); throw error; }
      if (!panels.has(view)) return;
      if (JSON.stringify(latest) !== JSON.stringify(info)) {
        info = latest;
        redraw();
        throw new Error('证据或排除规则已变化，面板已刷新，请重新选择操作。');
      }
      if (message.action === 'details') return inspect(uri, line, false);
      if (['share', 'revise', 'flag', 'exercise'].includes(message.action)) {
        if (busy) return;
        busy = true;
        try {
          await explanationAction(folder, selectedIndex(info.history, message.group), message);
          info = await cli(folder, 'inspect', { file, line });
          if (panels.has(view)) redraw();
        } finally { busy = false; }
      } else if (message.action === 'dependency') {
        const dep = selectedIndex(info.dependencies, message.index);
        if (inline) await openCurrent(folder, dep.file, dep.start_line);
        await inspect(vscode.Uri.file(confinedPath(folder.uri.fsPath, dep.file)), dep.start_line, inline);
      } else if (message.action === 'snapshot' || message.action === 'current') {
        const evidence = message.group === 'current' ? info.evidence : selectedIndex(info.history, message.group).evidence;
        const ev = selectedIndex(evidence, message.index);
        if (message.action === 'snapshot') await snapshot(ev.source, ev.file);
        else await openCurrent(folder, ev.file, ev.start_line);
      } else if (message.action === 'explain' || message.action === 'ask') {
        if (busy) return;
        let question = '';
        if (message.action === 'ask') {
          question = await vscode.window.showInputBox({ prompt: '问题将与函数及直接依赖证据发送给项目配置的模型服务；请勿包含密钥。', placeHolder: '这段代码在什么条件下返回错误？' });
          if (!question?.trim()) return;
        }
        if (!panels.has(view) || epoch !== documentEpoch) throw new Error('代码或展示范围已变化，请保存后重新打开函数解释再提问。');
        busy = true;
        try {
          await vscode.window.withProgress({ location: vscode.ProgressLocation.Notification, title: 'IntentLens：生成并检查证据支持', cancellable: false }, () => cli(folder, 'explain', { file, line, question }));
          info = await cli(folder, 'inspect', { file, line });
          if (!panels.has(view)) return;
          redraw();
          refresh();
        } finally { busy = false; }
      }
    };
    let view;
    if (inline) {
      inlineUI ||= createInline(vscode, context, guard);
      view = inlineUI.open(uri, info, guard(handle));
      panels.set(view, folder.uri.fsPath);
      view.onDidDispose(() => panels.delete(view));
      const target = await vscode.window.showTextDocument(uri, { preview: false });
      target.revealRange(view.thread.range, vscode.TextEditorRevealType.InCenterIfOutsideViewport);
    } else {
      view = panel(`IntentLens · ${info.function.name}`, nonce => renderInspection(info, nonce), handle, folder, true);
    }
    return view.thread;
  }
  async function showRound(folder, round) {
    trusted();
    const id = round.id;
    const loadRound = (cursor = '') => cli(folder, 'round-detail', { round_id: id, cursor, limit: 50 });
    const sameFirstPage = (latest, current) => {
      const { changes: latestChanges, next_cursor: _latestCursor, ...latestMetadata } = latest;
      const { changes: currentChanges, next_cursor: _currentCursor, ...currentMetadata } = current;
      return JSON.stringify(latestMetadata) === JSON.stringify(currentMetadata) &&
        JSON.stringify(latestChanges || []) === JSON.stringify((currentChanges || []).slice(0, 50));
    };
    round = await loadRound();
    let busy = false;
    const view = panel(`IntentLens · ${round.id}`, nonce => renderRound(round, nonce), async message => {
      if (busy) return;
      busy = true;
      try {
        let latest;
        try { latest = await loadRound(); }
        catch (error) { view.dispose(); throw error; }
        if (!panels.has(view)) return;
        if (!latest) { view.dispose(); throw new Error('轮次已不存在或已被排除。'); }
        if (!sameFirstPage(latest, round)) {
          round = latest;
          view.webview.html = renderRound(round, crypto.randomBytes(18).toString('hex'));
          throw new Error('轮次证据已变化，面板已刷新，请重新选择操作。');
        }
        if (message.action === 'analyzeRound') {
          const action = round.analysis ? '重新生成轮次解释' : '生成轮次解释';
          const answer = await vscode.window.showWarningMessage(`${action}？`, { modal: true, detail: '将按项目模型配置发送本轮允许的历史代码证据，并消耗模型 token 预算。不会使用 IDE 订阅凭据，不会修改源码；新解释会成为本轮展示版本，旧解释仍保存在本地历史。' }, action);
          if (answer !== action || !panels.has(view)) return;
          latest = await loadRound();
          if (!panels.has(view)) return;
          if (!latest) { view.dispose(); throw new Error('轮次已不存在或已被排除。'); }
          if (!sameFirstPage(latest, round)) {
            round = latest;
            view.webview.html = renderRound(round, crypto.randomBytes(18).toString('hex'));
            throw new Error('轮次证据已变化，面板已刷新，请重新确认生成。');
          }
          await vscode.window.withProgress({ location: vscode.ProgressLocation.Notification, title: `IntentLens：${action}并检查证据支持`, cancellable: false }, () => cli(folder, 'analyze-round', { round_id: id }));
          round = await loadRound();
          if (panels.has(view)) {
            if (!round) { view.dispose(); throw new Error('轮次已不存在或已被排除。'); }
            view.webview.html = renderRound(round, crypto.randomBytes(18).toString('hex'));
          }
          refresh();
          return;
        }
        if (['share', 'revise', 'flag', 'exercise'].includes(message.action)) {
          if (message.group !== 'analysis') return;
          await explanationAction(folder, round.analysis, message);
          round = await loadRound();
          if (panels.has(view) && round) view.webview.html = renderRound(round, crypto.randomBytes(18).toString('hex'));
          return;
        }
        if (message.action === 'snapshot' || message.action === 'current') {
          if (message.group !== 'analysis') return;
          const ev = selectedIndex(round.analysis?.evidence, message.index);
          if (message.action === 'snapshot') {
            const current = await cli(folder, 'explanation-evidence', { explanation_id: round.analysis.id, evidence_id: ev.id });
            await snapshot(current.source, current.file);
          }
          else await openCurrent(folder, ev.file, ev.start_line);
          return;
        }
        if (message.action === 'loadMoreChanges') {
          if (!round.next_cursor) return;
          const page = await loadRound(round.next_cursor);
          round = { ...round, changes: [...round.changes, ...(page.changes || [])], next_cursor: page.next_cursor };
          if (panels.has(view)) view.webview.html = renderRound(round, crypto.randomBytes(18).toString('hex'));
          return;
        }
        if (!['change', 'changeCurrent'].includes(message.action)) return;
        const change = selectedIndex(round.changes, message.index);
        if (message.action === 'changeCurrent') return openCurrent(folder, change.path);
        const beforeResult = await cli(folder, 'round-file', { round_id: id, file: change.path, side: 'before' });
        const afterResult = await cli(folder, 'round-file', { round_id: id, file: change.path, side: 'after' });
        const before = await vscode.workspace.openTextDocument({ content: beforeResult.file?.content || '', language: change.path.endsWith('.go') ? 'go' : 'plaintext' });
        const after = await vscode.workspace.openTextDocument({ content: afterResult.file?.content || '', language: change.path.endsWith('.go') ? 'go' : 'plaintext' });
        await vscode.commands.executeCommand('vscode.diff', before.uri, after.uri, `${change.path} — 历史 before / after`);
      } finally { busy = false; }
    }, folder);
  }
  const provider = {
    onDidChangeTreeData: treeChanged.event,
    getTreeItem: item => item,
    async getChildren(item) {
      if (!vscode.workspace.isTrusted) return [];
      if (!item) return (vscode.workspace.workspaceFolders || []).filter(f => f.uri.scheme === 'file').map(folder => {
        const node = new vscode.TreeItem(folder.name, vscode.TreeItemCollapsibleState.Collapsed); node.folder = folder; return node;
      });
      if (item.round) return [];
      try {
        const root = item.folder.uri.fsPath;
        let cached = roundPages.get(root);
        if (!cached) {
          const page = await cli(item.folder, 'rounds', { limit: 50 });
          cached = { items: page.items || [], nextCursor: page.next_cursor };
          roundPages.set(root, cached);
        }
        const nodes = cached.items.map(round => {
          const node = new vscode.TreeItem(`${round.started} · ${round.change_count || 0} 文件`, vscode.TreeItemCollapsibleState.None);
          node.round = round; node.folder = item.folder; node.description = round.source;
          node.command = { command: 'intentlens.openRound', title: '查看轮次', arguments: [item.folder, round] };
          return node;
        });
        if (cached.nextCursor) {
          const more = new vscode.TreeItem('加载更多轮次…', vscode.TreeItemCollapsibleState.None);
          more.command = { command: 'intentlens.loadMoreRounds', title: '加载下一页轮次', arguments: [root, cached.nextCursor] };
          nodes.push(more);
        }
        return nodes;
      } catch (error) { vscode.window.showErrorMessage(`IntentLens: ${error.message}`); return []; }
    },
  };
  command('loadMoreRounds', async (root, cursor) => {
    const folder = openFolder(root);
    if (!folder) throw new Error('轮次来源工作区已关闭，请重新打开后重试。');
    const page = await cli(folder, 'rounds', { cursor, limit: 50 });
    const cached = roundPages.get(root) || { items: [], nextCursor: undefined };
    cached.items.push(...(page.items || []));
    cached.nextCursor = page.next_cursor;
    roundPages.set(root, cached);
    treeChanged.fire(undefined);
  });
  context.subscriptions.push(treeChanged, lensesChanged, vscode.window.createTreeView('intentlens.rounds', { treeDataProvider: provider }));
  callPathProvider = createCallPathProvider(vscode);
  callPathView = vscode.window.createTreeView('intentlens.callPath', { treeDataProvider: callPathProvider });
  const modelAnswerChanged = new vscode.EventEmitter();
  modelAnswerProvider = createModelAnswerProvider({
    vscode,
    treeChanged: modelAnswerChanged,
    getCurrentTarget: getCurrentModelTarget,
    issueEvidenceRecord: record => {
      if (!record?.root || !record?.explanationID || !record?.evidenceID || record.generation !== modelAnswerGeneration) return undefined;
      const id = crypto.randomUUID();
      modelEvidenceRecords.set(id, { ...record });
      return id;
    },
    onInvalidate: generation => {
      modelAnswerGeneration = generation;
      modelEvidenceRecords.clear();
    },
  });
  const modelAnswerView = vscode.window.createTreeView('intentlens.modelAnswer', { treeDataProvider: modelAnswerProvider });
  context.subscriptions.push(callPathProvider, callPathView, modelAnswerProvider, modelAnswerChanged, modelAnswerView);

  // Parse Restrictions Unified Display
  const restrictionTreeProvider = new RestrictionTreeProvider();
  const restrictionTreeView = vscode.window.createTreeView('intentlens.restrictions', {
    treeDataProvider: restrictionTreeProvider,
    showCollapseAll: true
  });
  context.subscriptions.push(restrictionTreeProvider, restrictionTreeView);

  let restrictionResult;
  function clearRestrictions(root) {
    if (!restrictionResult || (root !== undefined && restrictionResult.root !== root)) return;
    restrictionResult = undefined;
    restrictionTreeProvider.clear();
    RestrictionDetailPanel.close();
  }

  function setRestrictions(folder, aggregated) {
    if (!aggregated || !restrictionsEnabled()) return clearRestrictions();
    if (typeof aggregated.summary?.totalRestrictions !== 'number' || !Array.isArray(aggregated.categories) ||
        !aggregated.categories.every(category => Array.isArray(category?.items) && category.items.every(item => typeof item?.message === 'string'))) {
      clearRestrictions();
      vscode.window.showErrorMessage('IntentLens: 解析限制数据格式无效');
      return;
    }

    const root = folder.uri.fsPath;
    const id = crypto.randomUUID(), records = new Map(), idsByItem = new Map();
    for (const category of aggregated.categories) {
      for (const item of category.items) {
        if (item?.type === 'summary' || typeof item?.filePath !== 'string' || !item.filePath ||
            !Number.isInteger(item.lineNumber) || item.lineNumber < 1) continue;
        const recordId = crypto.randomUUID();
        const record = { line: item.lineNumber };
        idsByItem.set(item, recordId);
        records.set(recordId, record);
        try {
          const file = item.filePath;
          if (path.isAbsolute(file) && file.split(/[/\\]/).some(part => part === '.' || part === '..')) continue;
          const relative = path.isAbsolute(file) ? path.relative(root, file) : file;
          record.target = confinedPath(root, relative);
          record.relative = relative;
        } catch { /* Keep the diagnostic visible, but refuse to open an unresolved path. */ }
      }
    }
    restrictionResult = { id, root, aggregated, records, idsByItem };
    restrictionTreeProvider.refresh(aggregated, id, idsByItem);
    RestrictionDetailPanel.update(aggregated, idsByItem);
    notifyRestrictions(id, aggregated.summary);
  }
  const restrictionsEnabled = () => vscode.workspace.getConfiguration('intentlens').get('restrictionsAggregation', true);

  function notifyRestrictions(id, { totalRestrictions, totalCategories }) {
    vscode.window.showWarningMessage(
      `IntentLens: 发现 ${totalRestrictions} 个解析限制（${totalCategories} 类）`,
      '展开详情', '忽略'
    ).then(guard(selection => {
      if (selection === '展开详情' && restrictionResult?.id === id) return showRestrictionDetails(id, true);
    }));
  }

  function restrictionFolder(result) {
    trusted();
    if (!result || restrictionResult !== result) throw new Error('解析限制记录已失效，请重新加载调用路径。');
    const folder = openFolder(result.root);
    if (!folder) {
      clearRestrictions(result.root);
      throw new Error('解析限制的来源工作区已关闭，请重新加载调用路径。');
    }
    if (!restrictionsEnabled()) {
      clearRestrictions(result.root);
      throw new Error('解析限制汇总已关闭。');
    }
    return folder;
  }

  async function validateRestrictions(result) {
    const folder = restrictionFolder(result);
    // The index only aggregates files admitted by the current CLI policy. Recheck
    // against it even if the .intentignore watcher has not fired yet.
    let current;
    try { current = await cli(folder, 'index'); }
    catch (error) {
      if (restrictionResult === result) clearRestrictions(result.root);
      throw new Error(`无法核对当前解析限制和排除规则：${error.message}`);
    }
    restrictionFolder(result);
    if (callPathEvidenceFingerprint({ aggregated: current.aggregated }) !== callPathEvidenceFingerprint({ aggregated: result.aggregated })) {
      clearRestrictions(result.root);
      throw new Error('解析限制已变化或目标已被当前排除规则排除，请重新加载调用路径。');
    }
  }

  async function showRestrictionDetails(id = restrictionResult?.id, fromNotification = false) {
    const result = restrictionResult;
    if (id === undefined) return;
    if (!result || result.id !== id) {
      if (fromNotification) return;
      throw new Error('解析限制记录已失效，请重新加载调用路径。');
    }
    try {
      await validateRestrictions(result);
      restrictionFolder(result);
      RestrictionDetailPanel.show(result.aggregated, result.idsByItem, navigateToRestriction);
    } catch (error) {
      // VS Code messages cannot be withdrawn; stale notification actions are dropped.
      if (!fromNotification) throw error;
    }
  }

  const navigateToRestriction = guard(async id => {
    const result = restrictionResult;
    const record = result?.records.get(id);
    if (!record) throw new Error('解析限制记录已失效，请重新加载调用路径。');
    const checkTarget = () => {
      try {
        if (record.target && confinedPath(result.root, record.relative) === record.target) return;
      } catch { /* Report an actionable error for both unsafe and missing files. */ }
      throw new Error('解析限制的目标不在来源工作区内、已移动或不可访问。');
    };
    restrictionFolder(result);
    checkTarget();
    await validateRestrictions(result);
    restrictionFolder(result);
    checkTarget();
    await revealCurrent(record.target, record.line, 0, async () => {
      restrictionFolder(result);
      checkTarget();
      await validateRestrictions(result);
      restrictionFolder(result);
      checkTarget();
    });
  });
  context.subscriptions.push({ dispose: () => clearRestrictions() });
  context.subscriptions.push(vscode.languages.registerCodeLensProvider({ language: 'go', scheme: 'file' }, {
    onDidChangeCodeLenses: lensesChanged.event,
    async provideCodeLenses(document, token) {
      if (!vscode.workspace.isTrusted || document.isDirty) return [];
      const folder = vscode.workspace.getWorkspaceFolder(document.uri);
      if (!folder) return [];
      const file = path.relative(folder.uri.fsPath, document.uri.fsPath).split(path.sep).join('/');
      const epoch = documentEpoch;
      const diagnosticLens = title => new vscode.CodeLens(new vscode.Range(0, 0, 0, 0), { title, command: 'intentlens.indexStatus', arguments: [document.uri] });
      try {
        const data = await cli(folder, 'index');
        if (token.isCancellationRequested || epoch !== documentEpoch) return [];
        reportIndex(folder, file, data);
        const lenses = (data.functions || []).filter(fn => fn.file === file).map(fn => new vscode.CodeLens(new vscode.Range(fn.start_line - 1, 0, fn.start_line - 1, 0), { title: 'IntentLens · 在函数下方展开解释', command: 'intentlens.inspect', arguments: [document.uri, fn.start_line] }));
        if (limitedCoverage(data.coverage)) lenses.unshift(diagnosticLens('IntentLens · 索引范围不完整（查看遗漏与排除原因）'));
        else if (!data.coverage) lenses.unshift(diagnosticLens('IntentLens · 索引覆盖范围未知（检查 CLI 版本）'));
        return lenses;
      } catch (error) {
        if (token.isCancellationRequested || epoch !== documentEpoch) return [];
        reportIndex(folder, file, undefined, error);
        return [diagnosticLens('IntentLens · 索引失败（点击查看原因）')];
      }
    },
  }));
  function command(name, fn) { context.subscriptions.push(vscode.commands.registerCommand(`intentlens.${name}`, guard(fn))); }
  async function getCurrentModelTarget({ signal } = {}) {
    if (!vscode.workspace.isTrusted) return { message: '请先信任本地工作区；受限模式不会执行 CLI。' };
    const editor = vscode.window.activeTextEditor;
    if (!editor) return { message: '打开已保存的 Go 文件，并将光标放在需要查看的函数中。' };
    if (editor.document.uri.scheme !== 'file' || editor.document.languageId !== 'go') return { message: '请切换到工作区内的 Go 源文件。' };
    if (editor.document.isDirty) return { message: '请先保存当前 Go 文件；侧栏只读取磁盘上的静态源码。' };
    const folder = vscode.workspace.getWorkspaceFolder(editor.document.uri);
    if (!folder || folder.uri.scheme !== 'file') return { message: '当前 Go 文件不属于本地工作区。' };
    const file = path.relative(folder.uri.fsPath, editor.document.uri.fsPath).split(path.sep).join('/');
    confinedPath(folder.uri.fsPath, file);
    const line = editor.selection.active.line + 1;
    const version = editor.document.version;
    const epoch = documentEpoch;
    const info = await cli(folder, 'inspect', { file, line }, { signal });
    const active = vscode.window.activeTextEditor;
    if (epoch !== documentEpoch || !active || active.document.uri.toString() !== editor.document.uri.toString() || active.selection.active.line + 1 !== line || active.document.version !== version || active.document.isDirty) {
      return { message: '当前文件或光标已变化；请重新展开侧栏查看最新本地回答。' };
    }
    return { folder, file, line, info };
  }
  async function focusCallPath() {
    if (callPathView?.show) { await callPathView.show(true); return; }
    if (vscode.commands.executeCommand) await vscode.commands.executeCommand('workbench.view.extension.intentlens');
  }
  async function loadCallPath() {
    await focusCallPath();
    const editor = vscode.window.activeTextEditor;
    if (!editor || editor.document.uri.scheme !== 'file') throw new Error('请在本地 Go 文件中放置光标。');
    if (editor.document.isDirty) throw new Error('请先保存文件；调用路径仅使用磁盘上的代码。');
    const folder = await folderFor(editor.document.uri);
    if (!folder) return;
    const file = path.relative(folder.uri.fsPath, editor.document.uri.fsPath).split(path.sep).join('/');
    confinedPath(folder.uri.fsPath, file);
    const line = editor.selection.active.line + 1;
    const epoch = documentEpoch;
    callPathProvider.clear();
    const requestEpoch = (callPathState?.requestEpoch || 0) + 1;
    callPathState = { requestEpoch, folderRoot: folder.uri.fsPath, input: { file, line } };
    const data = await cli(folder, 'call-path', { file, line });
    if (epoch !== documentEpoch || callPathState?.requestEpoch !== requestEpoch) return;
    if (!openFolder(folder.uri.fsPath)) return;
    callPathProvider.setData(data);
    setRestrictions(folder, data.aggregated);
    callPathState = { requestEpoch, folderRoot: folder.uri.fsPath, input: { file, line }, data };
    return data;
  }
  command('callPath', loadCallPath);
  command('refreshCallPath', loadCallPath);
  command('refreshModelAnswer', () => modelAnswerProvider?.refresh());

  command('showRestrictionDetails', id => showRestrictionDetails(id));
  context.subscriptions.push(vscode.commands.registerCommand('intentlens.navigateToRestriction', navigateToRestriction));
  command('refreshRestrictions', () => {
    if (restrictionResult) restrictionTreeProvider.refresh(restrictionResult.aggregated, restrictionResult.id, restrictionResult.idsByItem);
  });
  command('clearRestrictions', () => clearRestrictions());
  command('openModelEvidence', async recordID => {
    trusted();
    const record = modelEvidenceRecords.get(recordID);
    if (!record || record.generation !== modelAnswerGeneration) throw new Error('此侧栏证据已失效，请刷新后重新展开。');
    const folder = openFolder(record.root);
    if (!folder) {
      modelEvidenceRecords.delete(recordID);
      throw new Error('引用证据来源工作区已关闭，已拒绝打开。');
    }
    const assertEvidenceCurrent = () => {
      trusted();
      if (modelEvidenceRecords.get(recordID) !== record || record.generation !== modelAnswerGeneration) throw new Error('侧栏已刷新或证据记录已失效，已拒绝打开。');
      if (!openFolder(record.root)) throw new Error('引用证据来源工作区已关闭，已拒绝打开。');
    };
    const validateCurrentEvidence = async () => {
      assertEvidenceCurrent();
      const editor = vscode.window.activeTextEditor;
      const folderForEditor = editor && vscode.workspace.getWorkspaceFolder(editor.document.uri);
      const file = editor?.document?.uri?.scheme === 'file' && folderForEditor
        ? path.relative(folderForEditor.uri.fsPath, editor.document.uri.fsPath).split(path.sep).join('/')
        : undefined;
      const line = editor ? editor.selection.active.line + 1 : undefined;
      if (!editor || editor.document.isDirty || folderForEditor?.uri?.fsPath !== record.root || file !== record.file || line !== record.line) {
        throw new Error('侧栏当前函数已变化，已拒绝打开旧引用证据。');
      }
      const current = await cli(folder, 'explanation-evidence', { explanation_id: record.explanationID, evidence_id: record.evidenceID });
      assertEvidenceCurrent();
      if (!current || typeof current.source !== 'string') throw new Error('当前排除策略下无法读取此引用证据，已拒绝打开。');
      return current;
    };
    const evidence = await validateCurrentEvidence();
    await snapshot(evidence.source, evidence.file, async () => {
      const latest = await validateCurrentEvidence();
      if (latest.source !== evidence.source || latest.hash !== evidence.hash) throw new Error('引用证据在打开前已变化，已拒绝显示旧快照。');
    });
  });
  command('explainSelection', async () => {
    trusted();
    const editor = vscode.window.activeTextEditor;
    if (!editor) throw new Error('请先打开代码文件并选中一段代码。');
    const { document, selection } = editor;
    if (document.uri.scheme !== 'file') throw new Error('选区解释仅支持本地工作区文件。');
    if (document.isDirty) throw new Error('请先保存文件，再分析选区内保存后出现的错误。');
    if (!selection || selection.isEmpty) throw new Error('请先选中一段代码，再执行“解释选中代码”。');
    const language = document.languageId;
    if (!language || language === 'plaintext') throw new Error('当前语言模式不支持代码解释；请将语言模式切换为对应的代码语言后重试。');
    const savedVersion = document.version;
    const selectionRange = {
      start: { line: selection.start.line, character: selection.start.character },
      end: { line: selection.end.line, character: selection.end.character },
    };
    const source = document.getText(selection);
    if (!source.trim()) throw new Error('选区为空白，请选中包含代码的文本。');
    if (Buffer.byteLength(source, 'utf8') > 16 * 1024) throw new Error('选中代码超过 16 KiB 安全上限，请缩小选区。');
    const folder = await folderFor(document.uri);
    if (!folder) return;
    const file = path.relative(folder.uri.fsPath, document.uri.fsPath).split(path.sep).join('/');
    const samePosition = (a, b) => a.line === b.line && a.character === b.character;
    const assertSavedSnapshot = () => {
      const active = vscode.window.activeTextEditor;
      if (document.isDirty || document.version !== savedVersion || document.languageId !== language || active?.document.uri.toString() !== document.uri.toString() || !samePosition(editor.selection.start, selectionRange.start) || !samePosition(editor.selection.end, selectionRange.end)) {
        throw new Error('保存版本或选区已变化；为避免使用过期诊断，请重新保存并运行解释。');
      }
    };
    assertSavedSnapshot();
    const diagnostics = selectedErrorDiagnostics(vscode, document.uri, selectionRange);
    const startLine = selection.start.line + 1;
    const endLine = Math.max(startLine, selection.end.line + (selection.end.character > 0 ? 1 : 0));
    const input = { file, code: source, language, start_line: startLine, end_line: endLine, selection_range: selectionRange, diagnostics, saved_version: savedVersion };
    const result = await vscode.window.withProgress(
      { location: vscode.ProgressLocation.Notification, title: 'IntentLens：解释选中代码', cancellable: false },
      () => cli(folder, 'explain-selection', input),
    );
    assertSavedSnapshot();
    if (JSON.stringify(selectedErrorDiagnostics(vscode, document.uri, selectionRange)) !== JSON.stringify(diagnostics)) throw new Error('保存后的选区错误诊断已变化；请重新运行解释以使用最新诊断。');
    const combined = validateSelectionResult(result);
    panel('IntentLens · 选中代码与错误诊断', nonce => renderSelectionExplanation(combined, nonce), async () => {}, folder);
  });
  command('openCallPathSite', async call => {
    if (!call || typeof call.file !== 'string' || !Number.isInteger(call.line) || call.line < 1 || !Number.isInteger(call.column) || call.column < 0 || call.column > 100000000) throw new Error('源码调用点证据无效。');
    const state = callPathState;
    if (!state?.data || !state.folderRoot || !state.input) throw new Error('调用路径已失效，请重新加载。');
    const folder = openFolder(state.folderRoot);
    if (!folder) throw new Error('调用路径来源工作区已关闭，请重新加载。');
    const current = await cli(folder, 'call-path', state.input);
    if (state !== callPathState) throw new Error('调用路径已失效，请重新加载。');
    if (callPathEvidenceFingerprint(current) !== callPathEvidenceFingerprint(state.data)) {
      callPathState = undefined;
      callPathProvider.clear();
      throw new Error('源码证据已变化，请重新加载调用路径。');
    }
    const evidence = (current.calls || []).some(candidate => candidate && candidate.file === call.file && candidate.line === call.line && candidate.column === call.column && candidate.name === call.name && candidate.caller_id === call.caller_id && candidate.source === call.source);
    const functionEvidence = typeof call.function_id === 'string' && (current.functions || []).some(fn => fn && fn.file === call.file && fn.start_line === call.line && fn.id === call.function_id && fn.source === call.source);
    if (!evidence && !functionEvidence) throw new Error('源码证据已变化或不属于当前调用路径，请重新加载。');
    const target = confinedPath(folder.uri.fsPath, call.file);
    const dirty = (vscode.workspace.textDocuments || []).find(doc => doc.uri?.scheme === 'file' && doc.uri.fsPath === target && doc.isDirty);
    if (dirty) throw new Error('请先保存目标文件；脏文件不会被打开。');
    await openCurrent(folder, call.file, call.line, call.column || 0);
  });
  command('indexStatus', async uri => {
    uri ||= vscode.window.activeTextEditor?.document.uri;
    const folder = await folderFor(uri); if (!folder) return;
    let detail = indexDiagnostics.get(folder.uri.fsPath);
    if (!detail) {
      const file = uri ? path.relative(folder.uri.fsPath, uri.fsPath).split(path.sep).join('/') : '';
      const epoch = documentEpoch;
      try {
        const data = await cli(folder, 'index');
        if (epoch !== documentEpoch) return;
        reportIndex(folder, file, data);
      } catch (error) {
        if (epoch !== documentEpoch) return;
        reportIndex(folder, file, undefined, error);
      }
      detail = indexDiagnostics.get(folder.uri.fsPath);
    }
    panel('IntentLens · 索引范围与遗漏原因', nonce => renderIndexStatus(detail, nonce), async () => {}, folder, true);
  });
  const exerciseUI = createExerciseUI({ vscode, cli, panel, folderFor });
  command('exercises', () => exerciseUI.list());
  command('sharedExercises', () => exerciseUI.list(true));
  command('openExercise', async (folder, id) => exerciseUI.show(folder, await cli(folder, 'exercise-view', { exercise_id: id })));
  command('inspect', inspect);
  command('inspectDetails', (uri, line) => inspect(uri, line, false));
  command('openRound', showRound);
  command('refresh', refresh);
  command('startRound', async () => {
    const folder = await folderFor(); if (!folder) return;
    await sessions.start(folder.uri.fsPath);
    refresh();
  });
  command('stopRound', async () => {
    const folder = await folderFor(); if (!folder) return;
    await sessions.stop(folder.uri.fsPath);
    refresh();
  });
  command('resumeRound', async () => {
    const folder = await folderFor(); if (!folder) return;
    await recovery.resume(folder);
    refresh();
  });
  command('cancelRound', async () => {
    const folder = await folderFor(); if (!folder) return;
    await recovery.cancel(folder);
    refresh();
  });
  command('shared', async () => {
    const folder = await folderFor(); if (!folder) return;
    const records = await cli(folder, 'shared');
    if (!records.length) { vscode.window.showInformationMessage('IntentLens：没有可显示的共享记录。'); return; }
    const chosen = await vscode.window.showQuickPick(records.map(record => ({ label: record.function_id, description: `${record.created} · ${record.freshness}`, record })), { placeHolder: '选择共享解释版本' });
    if (!chosen) return;
    let record = chosen.record;
    const view = panel('IntentLens · 团队共享解释', nonce => renderShared(record, nonce), async message => {
      const latest = (await cli(folder, 'shared')).find(r => r.id === record.id);
      if (!latest) { view.dispose(); throw new Error('共享记录已移除或被排除'); }
      if (JSON.stringify(latest) !== JSON.stringify(record)) {
        record = latest;
        view.webview.html = renderShared(record, crypto.randomBytes(18).toString('hex'));
        throw new Error('共享记录已变化，请重新选择操作');
      }
      if (message.action === 'adopt') {
        showSaved(folder, await cli(folder, 'adopt-shared', { explanation_id: record.id }));
        return;
      }
      if (!['snapshot', 'current'].includes(message.action)) return;
      const ev = selectedIndex(record.evidence, message.index);
      if (message.action === 'snapshot') await snapshot(ev.source, ev.file);
      else await openCurrent(folder, ev.file, ev.start_line);
    }, folder);
  });
  async function configureForFolder(folder) {
    const status = await cli(folder, 'status');
    const saved = await configureModel(vscode, folder, status, cli, context.secrets);
    if (saved) refresh();
    return saved;
  }
  async function testModelForFolder(folder) {
    const { model, target } = await modelCredentialForCall(folder, cli, context.secrets);
    if (!model.enabled || !model.model) {
      throw new Error('请先通过“启用或调整模型分析”查询并选择 API 返回的模型 ID。');
    }
    const answer = await vscode.window.showWarningMessage(
      `将仅向以下明确配置的地址发送固定连接探针，不包含项目代码、文件路径或用户问题：\nProvider：${model.provider}\nBase URL：${model.base_url}\nAPI 返回的模型 ID：${model.model}\n\n该请求可能消耗少量 token 或产生费用。不会自动改用其他 endpoint、provider 或 model。`,
      { modal: true }, '发送此模型的测试请求');
    if (answer !== '发送此模型的测试请求') return;
    const result = await cli(folder, 'test-model', {}, { confirmedTarget: { ...target, model: model.model } });
    if (result?.tested !== true || result.fallback !== false || result.project_data_sent !== false) throw new Error('连接测试未确认模型返回完整且符合固定格式的单条文本；请检查 provider 的结束或截断提示。');
    vscode.window.showInformationMessage(`IntentLens：已确认模型返回完整且符合固定格式的单条文本。使用 ${result.actual_tokens} tokens；本次测试只发送固定探针，不读取项目代码。`);
    return result;
  }
  command('enableModel', async () => {
    const folder = await folderFor(); if (folder) return configureForFolder(folder);
  });
  command('testModel', async () => {
    const folder = await folderFor(); if (folder) return testModelForFolder(folder);
  });
  command('configureAPI', async () => {
    const folder = await folderFor(); if (!folder) return;
    return AIConfigPanel.show(context, folder, { cli, configure: configureForFolder, testConnection: testModelForFolder });
  });
  command('viewTokenUsage', async () => {
    const activeUri = vscode.window.activeTextEditor?.document?.uri;
    const folder = await folderFor(activeUri);
    if (!folder) return;
    TokenUsagePanel.show(context, () => cli(folder, 'status', {}));
  });
  command('pauseModel', async () => {
    const folder = await folderFor(); if (!folder) return;
    const status = await cli(folder, 'status');
    await cli(folder, 'configure', { settings: { ...status.settings, model: { ...status.settings.model, enabled: false } }, consent: false });
    vscode.window.showInformationMessage('IntentLens：已暂停新的模型分析；正在执行的请求可能仍完成。'); refresh();
  });
  command('status', async () => {
    const folder = await folderFor(); if (!folder) return;
    const status = await cli(folder, 'status');
    const doc = await vscode.workspace.openTextDocument({ content: JSON.stringify(status, null, 2), language: 'json' });
    await vscode.window.showTextDocument(doc);
  });
  command('hooksConfig', async () => {
    const folder = await folderFor(); if (!folder) return;
    const config = await cli(folder, 'hooks-config');
    panel('IntentLens · Claude Code hooks', nonce => page('Hook 配置预览', `<p>手动合并以下配置到 Claude Code 设置；保留已有 hooks。扩展不会覆盖设置。</p><p>确保 Claude Code 进程的 PATH 中有 intentlens 可执行文件。VS Code 的 intentlens.binaryPath 不会更改终端 PATH。配置后在新的 Claude Code 会话中验证 UserPromptSubmit / Stop。</p><pre>${escapeHTML(JSON.stringify(config, null, 2))}</pre>`, nonce), async () => {});
  });
  // 常驻入口：轮次视图一旦开始记录，viewsWelcome 会被树内容替换，
  // 那里的「配置 AI API」按钮就消失了。状态栏始终可见，不依赖视图内容。
  const apiBar = vscode.window.createStatusBarItem?.(vscode.StatusBarAlignment?.Right, 89);
  if (apiBar) {
    apiBar.text = '$(gear) IntentLens API';
    apiBar.tooltip = 'IntentLens：配置 AI API 和选择模型';
    apiBar.command = 'intentlens.configureAPI';
    apiBar.show();
    context.subscriptions.push(apiBar);
  }
  context.subscriptions.push(vscode.workspace.onDidSaveTextDocument(refresh), vscode.workspace.onDidChangeWorkspaceFolders(() => {
    if (restrictionResult && !openFolder(restrictionResult.root)) clearRestrictions();
    clearInline(); refresh();
  }), vscode.workspace.onDidGrantWorkspaceTrust(refresh));
  if (vscode.workspace.onDidChangeConfiguration) context.subscriptions.push(vscode.workspace.onDidChangeConfiguration(event => {
    if (event.affectsConfiguration('intentlens.restrictionsAggregation') && !restrictionsEnabled()) clearRestrictions();
  }));
  if (vscode.window.onDidChangeActiveTextEditor) context.subscriptions.push(vscode.window.onDidChangeActiveTextEditor(() => modelAnswerProvider?.refresh()));
  if (vscode.window.onDidChangeTextEditorSelection) context.subscriptions.push(vscode.window.onDidChangeTextEditorSelection(() => modelAnswerProvider?.refresh()));
  context.subscriptions.push(vscode.workspace.onDidChangeTextDocument(event => {
    if (event.document.uri.scheme === 'file' && event.contentChanges.length) clearInline();
  }));
  const policyWatcher = vscode.workspace.createFileSystemWatcher('**/.intentignore');
  const policyChanged = guard(async uri => {
    clearInline();
    const folder = vscode.workspace.getWorkspaceFolder(uri);
    if (!folder || folder.uri.scheme !== 'file') return;
    const root = folder.uri.fsPath;
    policyVersions.set(root, (policyVersions.get(root) || 0) + 1);
    clearRestrictions(root);
    for (const [view, panelRoot] of panels) if (panelRoot === root) view.dispose();
    refresh();
    if (vscode.workspace.isTrusted) await cli(folder, 'status');
  });
  context.subscriptions.push(policyWatcher, policyWatcher.onDidCreate(policyChanged), policyWatcher.onDidChange(policyChanged), policyWatcher.onDidDelete(policyChanged));
  const sourceWatcher = vscode.workspace.createFileSystemWatcher('**/*.go');
  context.subscriptions.push(sourceWatcher, sourceWatcher.onDidCreate(clearInline), sourceWatcher.onDidChange(clearInline), sourceWatcher.onDidDelete(clearInline));
}
module.exports = { activate };
