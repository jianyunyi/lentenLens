'use strict';
const crypto = require('node:crypto');
const { coverageSummary, limitedCoverage } = require('./coverage');

function createInline(vscode, context, guard) {
  const controller = vscode.comments.createCommentController('intentlens.explanations', 'IntentLens 函数解释');
  const sessions = new Map();
  context.subscriptions.push(controller, vscode.commands.registerCommand('intentlens.inlineAction', guard(async (id, index, generation) => {
    const session = sessions.get(id);
    if (!session || !Number.isInteger(index) || generation !== session.generation) return;
    const action = session.actions[index];
    if (action?.action === 'toggle') {
      if (session.expanded.has(action.section)) session.expanded.delete(action.section);
      else session.expanded.add(action.section);
      session.view.update(session.data);
      return;
    }
    if (action) await session.handler(action);
  })));
  function open(uri, info, handler) {
    const key = `${uri.toString()}:${info.function.start_line}`;
    for (const session of sessions.values()) if (session.key === key) session.view.dispose();
    const id = crypto.randomBytes(16).toString('hex');
    const line = Math.max(0, info.function.end_line - 1);
    const thread = controller.createCommentThread(uri, new vscode.Range(line, 0, line, 0), []);
    thread.label = `IntentLens · ${info.function.name}`;
    thread.canReply = false;
    thread.collapsibleState = vscode.CommentThreadCollapsibleState.Expanded;
    const disposed = [];
    const session = { key, handler, actions: [], generation: 0, expanded: new Set() };
    const view = {
      thread,
      onDidDispose(fn) { disposed.push(fn); },
      dispose() { if (!sessions.delete(id)) return; thread.dispose(); for (const fn of disposed) fn(); },
      update(data) {
        if (!sessions.has(id)) return;
        // Revoke old links when evidence or history changes.
        session.generation++; session.actions = [];
        session.data = data;
        const body = () => { const b = new vscode.MarkdownString(); b.isTrusted = { enabledCommands: ['intentlens.inlineAction'] }; b.supportHtml = false; return b; };
        const link = (b, text, action) => {
          const index = session.actions.push(action) - 1;
          const label = new vscode.MarkdownString(); label.appendText(String(text).replace(/[\r\n]/g, ' '));
          b.appendMarkdown(`[${label.value}](command:intentlens.inlineAction?${encodeURIComponent(JSON.stringify([id, index, session.generation]))})  `);
        };
        const section = (name, b) => ({ author: { name }, body: b, mode: vscode.CommentMode.Preview });
        const history = data.history || [];
        const latest = history.map((entry, index) => ({entry,index})).sort((a,b) => String(b.entry.created).localeCompare(String(a.entry.created)))[0];
        const entry = latest?.entry;
        const current = entry?.freshness === '已引用证据未变';
        const header = body();
        header.appendText(entry ? (current ? '已引用证据未变；未检查所有间接依赖。' : '解释已过期，请重新生成；旧版本可在完整详情回看。') : '尚未生成函数解释。点击“生成解释”；未配置模型时会提示先启用。');
        if (limitedCoverage(data.coverage)) header.appendText(`\n\n${coverageSummary(data.coverage)}\n请打开“IntentLens: 查看索引范围与遗漏原因”核对预算和排除范围。`);
        header.appendMarkdown('\n\n');
        link(header, '生成解释', {action:'explain'}); link(header, '继续追问', {action:'ask'}); link(header, '完整详情与历史', {action:'details'});
        const claims = current ? entry.document?.claims || [] : [];
        const names = { purpose:'函数作用', contract:'输入输出与边界', flow:'执行流程', design:'关键设计', dependency:'依赖说明', legacy:'旧版或未分类说明' };
        const groups = Object.fromEntries(Object.keys(names).map(key => [key, []]));
        claims.forEach((claim, index) => groups[Object.hasOwn(names, claim.section) && claim.section !== 'legacy' ? claim.section : 'legacy'].push({ claim, index }));
        if (groups.legacy.length) header.appendText('\n旧版或未分类内容尚未按新结构整理；可展开回看，或主动重新生成。\n');
        for (const unknown of current ? entry.document?.unknowns || [] : []) header.appendText(`\n待确认：${unknown}\n`);
        for (const issue of data.issues || []) header.appendText(`\n解析限制：${issue}\n`);
        const missingEvidence = claim => !claim.evidence_ids?.length || claim.evidence_ids.some(id => !entry.evidence?.some(ev => ev.id === id));
        for (const claim of claims) {
          if (missingEvidence(claim)) header.appendText(`\n引用依据不可定位；待确认：${claim.text}\n`);
          if (claim.status !== 'supported') header.appendText(`\n待确认：${claim.text}\n`);
          for (const correction of (entry.corrections || []).filter(c => c.claim_id === claim.id)) header.appendText(`\n已被纠错：${claim.text}；原因：${correction.reason}\n`);
        }
        if (current && !claims.length) header.appendText('\n当前没有可展示的结论；可继续追问或重新生成。\n');
        function claimText(b, claim, index) {
          const corrections = (entry.corrections || []).filter(c => c.claim_id === claim.id);
          b.appendText(`${corrections.length ? '已标记有误 / 待复核：' : claim.status === 'supported' ? '' : '待确认：'}${claim.text}\n`);
          for (const correction of corrections) b.appendText(`纠错原因：${correction.reason}\n`);
          for (const evidenceID of claim.evidence_ids || []) {
            const i = entry.evidence?.findIndex(e => e.id === evidenceID) ?? -1;
            if (i >= 0) {
              const ev = entry.evidence[i];
              link(b, `${ev.file || '已保存证据'}:${ev.start_line || '?'}${ev.end_line > ev.start_line ? '–' + ev.end_line : ''}`, {action:'snapshot', index:String(i), group:String(latest.index)});
            } else b.appendText('引用依据不可定位；待确认。\n');
          }
          link(b, '标记有误', {action:'flag', index:String(index), group:String(latest.index)});
          if (!corrections.length && !missingEvidence(claim) && claim.kind === 'behavior' && claim.status === 'supported') link(b, '小修改练习', {action:'exercise', index:String(index), group:String(latest.index)});
          b.appendMarkdown('\n\n');
        }
        const comments = [section('IntentLens', header)];
        for (const key of ['purpose','contract','flow','design','dependency','legacy']) {
          const items = groups[key];
          if (!items.length && !(key === 'dependency' && data.dependencies?.length)) continue;
          const b = body();
          const expanded = session.expanded.has(key);
          const count = key === 'purpose' || key === 'dependency' ? items.length : key === 'design' ? 1 : 0;
          const shown = expanded ? items : items.slice(0, count);
          shown.forEach(({claim,index},i) => {
            if (key === 'flow') b.appendText(`${i+1}. `);
            claimText(b,claim,index);
          });
          if (key === 'dependency') {
            if (!items.length) b.appendText('已定位直接依赖，尚无当前版本的职责说明。\n');
            if (expanded) (data.dependencies || []).forEach((dep,i) => {
              b.appendText(`${dep.name} · ${dep.file}:${dep.start_line}\n`);
              link(b,'查看依赖解释',{action:'dependency',index:String(i)}); b.appendMarkdown('\n\n');
            });
          }
          if (items.length > count || key === 'dependency' && data.dependencies?.length) {
            const title = key === 'dependency' ? '定义位置与深入阅读' : key === 'design' ? '更多取舍' : names[key];
            link(b, `${expanded ? '收起' : '展开'}${title}`, {action:'toggle',section:key});
          }
          comments.push(section(names[key], b));
        }
        if (data.evidence?.length) {
          const b = body();
          if (session.expanded.has('evidence')) data.evidence.forEach((ev,i) => {
            b.appendText(`${ev.file}:${ev.start_line}–${ev.end_line}\n`);
            link(b,'当前检查的证据原文',{action:'snapshot',index:String(i),group:'current'});
            link(b,'跳转当前代码',{action:'current',index:String(i),group:'current'}); b.appendMarkdown('\n\n');
          });
          link(b, `${session.expanded.has('evidence') ? '收起' : '展开'}完整证据列表`, {action:'toggle',section:'evidence'});
          comments.push(section('查阅证据',b));
        }
        thread.comments = comments;
      },
    };
    session.view = view; sessions.set(id, session); view.update(info); return view;
  }
  function clear() { for (const session of sessions.values()) session.view.dispose(); }
  context.subscriptions.push({ dispose: clear });
  return { open, clear };
}
module.exports = { createInline };
