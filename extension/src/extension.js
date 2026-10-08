'use strict';
const vscode = require('vscode');
const path = require('node:path');
const fs = require('node:fs');
const crypto = require('node:crypto');
const { runCLI, confinedPath, renderInspection, renderRound, page, escapeHTML } = require('./core');

function activate(context) {
  const treeChanged = new vscode.EventEmitter(), lensesChanged = new vscode.EventEmitter();
  const panels = new Map(), policyVersions = new Map();
  const guard = fn => async (...args) => { try { return await fn(...args); } catch (error) { vscode.window.showErrorMessage(`IntentLens: ${error.message}`); } };
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
  async function cli(folder, command, input) {
    trusted();
    const configured = vscode.workspace.getConfiguration('intentlens', folder.uri).get('binaryPath', '');
    const bundled = path.join(context.extensionPath, 'bin', process.platform === 'win32' ? 'intentlens.exe' : 'intentlens');
    const binary = configured || (fs.existsSync(bundled) ? bundled : 'intentlens');
    const version = policyVersions.get(folder.uri.fsPath) || 0;
    const result = await runCLI(binary, folder.uri.fsPath, command, input);
    if (version !== (policyVersions.get(folder.uri.fsPath) || 0)) throw new Error('排除规则已变化，请重新打开视图。');
    return result;
  }
  const refresh = () => { treeChanged.fire(); lensesChanged.fire(); };
  async function openCurrent(folder, file, line = 1) {
    trusted();
    const target = confinedPath(folder.uri.fsPath, file);
    const doc = await vscode.workspace.openTextDocument(vscode.Uri.file(target));
    const editor = await vscode.window.showTextDocument(doc);
    const pos = new vscode.Position(Math.max(0, Math.min(doc.lineCount - 1, Number(line || 1) - 1)), 0);
    editor.selection = new vscode.Selection(pos, pos);
    editor.revealRange(new vscode.Range(pos, pos), vscode.TextEditorRevealType.InCenter);
  }
  async function snapshot(source, file) {
    const document = await vscode.workspace.openTextDocument({ content: source || '', language: file?.endsWith('.go') ? 'go' : 'plaintext' });
    await vscode.window.showTextDocument(document, { preview: false });
  }
  function panel(title, render, handler, folder) {
    const view = vscode.window.createWebviewPanel('intentlens.evidence', title, vscode.ViewColumn.Beside, { enableScripts: true, localResourceRoots: [] });
    view.webview.html = render(crypto.randomBytes(18).toString('hex'));
    panels.set(view, folder?.uri.fsPath);
    view.onDidDispose(() => panels.delete(view), undefined, context.subscriptions);
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
  async function inspect(uri, line) {
    uri = uri || vscode.window.activeTextEditor?.document.uri;
    if (!uri) throw new Error('请在 Go 函数中放置光标。');
    const folder = await folderFor(uri);
    if (!folder) return;
    const editor = vscode.window.activeTextEditor;
    if (editor?.document.uri.toString() === uri.toString() && editor.document.isDirty) throw new Error('请先保存文件；分析仅使用磁盘上的代码。');
    const file = path.relative(folder.uri.fsPath, uri.fsPath).split(path.sep).join('/');
    confinedPath(folder.uri.fsPath, file);
    line = line || (editor?.document.uri.toString() === uri.toString() ? editor.selection.active.line + 1 : 1);
    let info = await cli(folder, 'inspect', { file, line });
    let busy = false;
    const view = panel(`IntentLens · ${info.function.name}`, nonce => renderInspection(info, nonce), async message => {
      if (!['dependency', 'snapshot', 'current', 'explain', 'ask'].includes(message.action)) return;
      let latest;
      try { latest = await cli(folder, 'inspect', { file, line }); }
      catch (error) { view.dispose(); throw error; }
      if (!panels.has(view)) return;
      if (JSON.stringify(latest) !== JSON.stringify(info)) {
        info = latest;
        view.webview.html = renderInspection(info, crypto.randomBytes(18).toString('hex'));
        throw new Error('证据或排除规则已变化，面板已刷新，请重新选择操作。');
      }
      if (message.action === 'dependency') {
        const dep = selectedIndex(info.dependencies, message.index);
        await inspect(vscode.Uri.file(confinedPath(folder.uri.fsPath, dep.file)), dep.start_line);
      } else if (message.action === 'snapshot' || message.action === 'current') {
        const evidence = message.group === 'current' ? info.evidence : selectedIndex(info.history, message.group).evidence;
        const ev = selectedIndex(evidence, message.index);
        if (message.action === 'snapshot') await snapshot(ev.source, ev.file);
        else await openCurrent(folder, ev.file, ev.start_line);
      } else if (message.action === 'explain' || message.action === 'ask') {
        if (busy) return;
        let question = '';
        if (message.action === 'ask') {
          question = await vscode.window.showInputBox({ prompt: '问题将与函数及直接依赖证据发送给 Anthropic；请勿包含密钥。', placeHolder: '这段代码在什么条件下返回错误？' });
          if (!question?.trim()) return;
        }
        busy = true;
        try {
          await vscode.window.withProgress({ location: vscode.ProgressLocation.Notification, title: 'IntentLens：生成并检查证据支持', cancellable: false }, () => cli(folder, 'explain', { file, line, question }));
          info = await cli(folder, 'inspect', { file, line });
          if (!panels.has(view)) return;
          view.webview.html = renderInspection(info, crypto.randomBytes(18).toString('hex'));
          refresh();
        } finally { busy = false; }
      }
    }, folder);
  }
  async function showRound(folder, round) {
    trusted();
    const id = round.id;
    round = (await cli(folder, 'rounds')).find(entry => entry.id === id);
    if (!round) throw new Error('轮次已不存在或已被排除。');
    const view = panel(`IntentLens · ${round.id}`, nonce => renderRound(round, nonce), async message => {
      let latest;
      try { latest = (await cli(folder, 'rounds')).find(entry => entry.id === id); }
      catch (error) { view.dispose(); throw error; }
      if (!panels.has(view)) return;
      if (!latest) { view.dispose(); throw new Error('轮次已不存在或已被排除。'); }
      if (JSON.stringify(latest) !== JSON.stringify(round)) {
        round = latest;
        view.webview.html = renderRound(round, crypto.randomBytes(18).toString('hex'));
        throw new Error('轮次证据已变化，面板已刷新，请重新选择操作。');
      }
      if (message.action === 'snapshot' || message.action === 'current') {
        if (message.group !== 'analysis') return;
        const ev = selectedIndex(round.analysis?.evidence, message.index);
        if (message.action === 'snapshot') await snapshot(ev.source, ev.file);
        else await openCurrent(folder, ev.file, ev.start_line);
        return;
      }
      if (!['change', 'changeCurrent'].includes(message.action)) return;
      const change = selectedIndex(round.changes, message.index);
      if (message.action === 'changeCurrent') return openCurrent(folder, change.path);
      const before = await vscode.workspace.openTextDocument({ content: change.before?.content || '', language: change.path.endsWith('.go') ? 'go' : 'plaintext' });
      const after = await vscode.workspace.openTextDocument({ content: change.after?.content || '', language: change.path.endsWith('.go') ? 'go' : 'plaintext' });
      await vscode.commands.executeCommand('vscode.diff', before.uri, after.uri, `${change.path} — 历史 before / after`);
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
        const rounds = await cli(item.folder, 'rounds');
        return rounds.map(round => {
          const node = new vscode.TreeItem(`${round.started} · ${round.changes?.length || 0} 文件`, vscode.TreeItemCollapsibleState.None);
          node.round = round; node.folder = item.folder; node.description = round.source;
          node.command = { command: 'intentlens.openRound', title: '查看轮次', arguments: [item.folder, round] };
          return node;
        });
      } catch (error) { vscode.window.showErrorMessage(`IntentLens: ${error.message}`); return []; }
    },
  };
  context.subscriptions.push(treeChanged, lensesChanged, vscode.window.createTreeView('intentlens.rounds', { treeDataProvider: provider }));
  context.subscriptions.push(vscode.languages.registerCodeLensProvider({ language: 'go', scheme: 'file' }, {
    onDidChangeCodeLenses: lensesChanged.event,
    async provideCodeLenses(document, token) {
      if (!vscode.workspace.isTrusted || document.isDirty) return [];
      const folder = vscode.workspace.getWorkspaceFolder(document.uri);
      if (!folder) return [];
      try {
        const data = await cli(folder, 'index');
        if (token.isCancellationRequested) return [];
        const file = path.relative(folder.uri.fsPath, document.uri.fsPath).split(path.sep).join('/');
        return (data.functions || []).filter(fn => fn.file === file).map(fn => new vscode.CodeLens(new vscode.Range(fn.start_line - 1, 0, fn.start_line - 1, 0), { title: 'IntentLens · 解释与证据', command: 'intentlens.inspect', arguments: [document.uri, fn.start_line] }));
      } catch { return []; }
    },
  }));
  function command(name, fn) { context.subscriptions.push(vscode.commands.registerCommand(`intentlens.${name}`, guard(fn))); }
  command('inspect', inspect);
  command('openRound', showRound);
  command('refresh', refresh);
  command('enableModel', async () => {
    const folder = await folderFor(); if (!folder) return;
    const status = await cli(folder, 'status');
    const model = await vscode.window.showInputBox({ title: 'Anthropic 模型', prompt: '输入账户可用的准确模型 ID。', value: status.settings?.model?.model || '', validateInput: value => value.trim() ? null : '模型 ID 不能为空' });
    if (!model) return;
    const limit = await vscode.window.showInputBox({ title: '项目累计 token 上限', value: String(status.settings?.model?.token_limit || 200000), validateInput: value => /^\d+$/.test(value) && Number.isSafeInteger(Number(value)) && Number(value) > 0 ? null : '输入正整数' });
    if (!limit) return;
    const consent = await vscode.window.showWarningMessage(`启用 ${folder.name} 的 Anthropic 分析？函数/依赖源码证据、轮次 before/after 及你的问题会发送到 Anthropic；停止 hook 可自动分析轮次。模型 ${model}，项目累计预算 ${limit} tokens（不是费用上限）。密钥仅从 ANTHROPIC_API_KEY 环境变量读取。`, { modal: true }, '同意并启用');
    if (consent !== '同意并启用') return;
    await cli(folder, 'configure', { settings: { model: { enabled: true, model: model.trim(), token_limit: Number(limit) } }, consent: true });
    vscode.window.showInformationMessage('IntentLens：模型分析已启用。'); refresh();
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
  context.subscriptions.push(vscode.workspace.onDidSaveTextDocument(refresh), vscode.workspace.onDidChangeWorkspaceFolders(refresh), vscode.workspace.onDidGrantWorkspaceTrust(refresh));
  const policyWatcher = vscode.workspace.createFileSystemWatcher('**/.intentignore');
  const policyChanged = guard(async uri => {
    const folder = vscode.workspace.getWorkspaceFolder(uri);
    if (!folder || folder.uri.scheme !== 'file') return;
    const root = folder.uri.fsPath;
    policyVersions.set(root, (policyVersions.get(root) || 0) + 1);
    for (const [view, panelRoot] of panels) if (panelRoot === root) view.dispose();
    refresh();
    if (vscode.workspace.isTrusted) await cli(folder, 'status');
  });
  context.subscriptions.push(policyWatcher, policyWatcher.onDidCreate(policyChanged), policyWatcher.onDidChange(policyChanged), policyWatcher.onDidDelete(policyChanged));
}
module.exports = { activate };
