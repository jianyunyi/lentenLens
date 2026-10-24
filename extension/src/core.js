'use strict';
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const { coverageLines } = require('./coverage');

function runCLI(binary, root, command, input = {}, options = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(binary, [...(options.prefixArgs || []), command, '--root', root], { cwd: root, shell: false, windowsHide: true, env: process.env });
    let stdout = '', stderr = '', size = 0, settled = false, terminationError;
    const finish = (error, value) => { if (settled) return; settled = true; clearTimeout(timer); error ? reject(error) : resolve(value); };
    const timer = setTimeout(() => { terminationError = new Error('IntentLens CLI timed out'); child.kill(); }, options.timeout || 180000);
    child.on('error', error => finish(error));
    child.stdout.setEncoding('utf8');
    child.stderr.setEncoding('utf8');
    child.stdout.on('data', data => { size += Buffer.byteLength(data); if (size > 32 * 1024 * 1024) { terminationError = new Error('CLI response exceeds 32 MiB'); child.kill(); } else stdout += data; });
    child.stderr.on('data', data => { if (stderr.length < 8192) stderr += data.toString(); });
    child.on('close', code => {
      if (settled) return;
      if (terminationError) return finish(terminationError);
      let value;
      try { value = JSON.parse(stdout); } catch { return finish(new Error(`CLI returned invalid JSON (exit ${code}): ${stderr.slice(0, 500)}`)); }
      if (code !== 0 || value?.error) return finish(new Error(value?.error || `CLI exited ${code}`));
      finish(null, value);
    });
    child.stdin.on('error', error => finish(error));
    child.stdin.end(JSON.stringify(input));
  });
}

function confinedPath(root, relative) {
  if (typeof relative !== 'string' || !relative || /[:\x00]/.test(relative) || path.isAbsolute(relative) || path.win32.isAbsolute(relative)) throw new Error('Expected safe relative path');
  const pieces = relative.replace(/\\/g, '/').split('/');
  if (pieces.some(piece => piece === '..' || piece === '.' || !piece)) throw new Error('Path outside workspace or unsafe');
  const base = fs.realpathSync(root);
  const target = fs.realpathSync(path.join(base, ...pieces));
  const rel = path.relative(base, target);
  if (!rel || rel.startsWith('..' + path.sep) || rel === '..' || path.isAbsolute(rel)) throw new Error('Path outside workspace');
  return target;
}
function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
}
function button(action, index, text, group = '') {
  return `<button data-action="${escapeHTML(action)}" data-index="${escapeHTML(index)}" data-group="${escapeHTML(group)}">${escapeHTML(text)}</button>`;
}
function page(title, body, nonce) {
  return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'nonce-${nonce}'; script-src 'nonce-${nonce}';"><meta name="viewport" content="width=device-width, initial-scale=1"><title>${escapeHTML(title)}</title><style nonce="${nonce}">body{font-family:var(--vscode-font-family);color:var(--vscode-foreground);background:var(--vscode-editor-background);padding:20px;max-width:1000px;line-height:1.6}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:var(--vscode-textCodeBlock-background);padding:12px}button{background:var(--vscode-button-background);color:var(--vscode-button-foreground);border:0;padding:7px 11px;margin:4px;cursor:pointer}section{border-top:1px solid var(--vscode-panel-border);padding-top:12px;margin-top:20px}.muted,small{opacity:.75}summary{cursor:pointer}code{overflow-wrap:anywhere}</style></head><body><h1>${escapeHTML(title)}</h1>${body}<script nonce="${nonce}">const vscode=acquireVsCodeApi();document.addEventListener('click',event=>{const b=event.target.closest('button[data-action]');if(b)vscode.postMessage({action:b.dataset.action,index:b.dataset.index,group:b.dataset.group});});</script></body></html>`;
}
function evidenceHTML(evidence, group) {
  return (evidence || []).map((ev, i) => `<details><summary>${escapeHTML(ev.id)} · ${escapeHTML(ev.file)}:${escapeHTML(ev.start_line)}–${escapeHTML(ev.end_line)}</summary><p>快照 hash: <code>${escapeHTML(ev.hash)}</code></p>${button('snapshot', i, '历史证据原文', group)}${button('current', i, '当前文件（可能已变化）', group)}<pre>${escapeHTML(ev.source)}</pre></details>`).join('');
}
function evidenceLocations(entry, ids) {
  const byId = new Map((entry.evidence || []).map(item => [item.id, item]));
  return (ids || []).map(id => {
    const item = byId.get(id);
    if (!item) return `${id}（位置未知）`;
    const lines = item.start_line === item.end_line ? String(item.start_line) : `${item.start_line}–${item.end_line}`;
    return `${item.file}:${lines}${item.role ? `（${item.role}）` : ''}`;
  }).join('、');
}
function explanationHTML(entry, group, editable = true) {
  return `<section><h2>解释 ${escapeHTML(entry.created || entry.id)}</h2><p>版本 ${escapeHTML(entry.id)} ${entry.supersedes ? `· 修订自 ${escapeHTML(entry.supersedes)}` : ''}</p><p>${escapeHTML(entry.freshness)}</p><p class="muted">模型说法需要核对；引用存在不代表结论成立。测试源码不代表测试已执行。</p>${editable ? button('share', '', '准备共享', group) + button('revise', '', '生成修订版本', group) : ''}${(entry.document?.claims || []).map((c, i) => `<p><strong>[${escapeHTML(c.status)} · ${escapeHTML(c.kind)}]</strong> ${escapeHTML(c.text)}<br><small>证据: ${escapeHTML((c.evidence_ids || []).join(', '))}<br>位置: ${escapeHTML(evidenceLocations(entry, c.evidence_ids))}</small>${editable ? button('flag', i, '有误 / 证据不支持', group) : ''}${editable && c.kind === 'behavior' && c.status === 'supported' ? button('exercise', i, '针对此行为生成练习', group) : ''}</p>`).join('')}<ul>${(entry.corrections || []).map(c => `<li>本地纠错 ${escapeHTML(c.claim_id)}：${escapeHTML(c.reason)}</li>`).join('')}${(entry.document?.unknowns || []).map(x => `<li>${escapeHTML(x)}</li>`).join('')}</ul>${evidenceHTML(entry.evidence, group)}</section>`;
}
function renderSharePreview(draft, nonce) {
  return page('共享预览', `<p>以下是将写入 .intent/shared/${escapeHTML(draft.id)}.json 的全部内容。确认后可由你通过 Git 提交；关闭面板不会写入仓库。</p><p>已仅根据获准证据重新生成。请核对解释及证据原文。</p><pre>${escapeHTML(JSON.stringify(draft.record, null, 2))}</pre>${button('confirmShare', '', '确认写入共享记录')}${button('cancel', '', '取消')}`, nonce);
}
function renderShared(record, nonce) {
  return page('团队共享解释', `${button('adopt', '', '本地纠错与修订')}<ul>${(record.issues || []).map(x => `<li>${escapeHTML(x)}</li>`).join('')}</ul>${explanationHTML(record, 'shared', false)}`, nonce);
}
function renderSaved(record, nonce) { return page('本地解释版本', explanationHTML(record, 'saved'), nonce); }
function coverageHTML(coverage) {
  return `<section aria-label="索引覆盖范围"><h2>索引覆盖范围</h2><ul>${coverageLines(coverage).map(line => `<li>${escapeHTML(line)}</li>`).join('')}</ul></section>`;
}
function renderIndexStatus(data, nonce) {
  const error = data.error ? `<p role="alert">索引失败：${escapeHTML(data.error)}</p><p>请检查工作区信任、CLI 路径和文件访问权限，再执行“刷新轮次与 CodeLens”。失败不会被解释为文件没有函数。</p>` : '';
  return page('索引范围与遗漏原因', `<p>工作区：${escapeHTML(data.root || '')}</p>${data.file ? `<p>当前文件：${escapeHTML(data.file)}</p>` : ''}${error}${data.error ? '' : coverageHTML(data.coverage)}<p>此页面只显示本地索引诊断，不发送代码、不请求模型、不更改排除规则。</p>`, nonce);
}
function renderInspection(data, nonce) {
  return page(data.function?.name || '函数证据', `<p>本地静态证据 · 已保存的磁盘版本</p>${coverageHTML(data.coverage)}${button('explain', '', '生成解释')}${button('ask', '', '自由提问')}<pre>${escapeHTML(data.function?.source)}</pre><h2>直接依赖</h2>${(data.dependencies || []).map((f, i) => button('dependency', i, `${f.name} · ${f.file}:${f.start_line}`)).join('') || '<p>没有唯一解析的直接依赖。</p>'}<h2>当前证据</h2>${evidenceHTML(data.evidence, 'current')}<h2>分析限制</h2><ul>${(data.issues || []).map(x => `<li>${escapeHTML(x)}</li>`).join('')}</ul>${(data.history || []).map((entry, i) => explanationHTML(entry, String(i))).join('')}`, nonce);
}
function renderRound(round, nonce) {
  const analysis = `${button('analyzeRound', '', round.analysis ? '重新生成轮次解释' : '生成轮次解释')}<p class="muted">此手动入口仅在点击并确认后请求项目配置的模型；查看和折叠不会调用模型。可选 Claude Stop hook 在启用模型后仍可自动分析。解释依据本轮历史快照，不代表当前代码。</p>`;
  return page(`轮次 ${round.id}`, `<p>${escapeHTML(round.started)} → ${escapeHTML(round.ended)}</p><p>来源: ${escapeHTML(round.source)} · 工作区净变化，不能据此推断作者动机。</p>${analysis}<ul>${(round.notes || []).map(x => `<li>${escapeHTML(x)}</li>`).join('')}</ul>${(round.changes || []).map((c, i) => `<section><h2>${escapeHTML(c.path)} · ${escapeHTML(c.kind)}</h2>${button('change', i, '查看 before / after 快照')} ${button('changeCurrent', i, '打开当前文件')}<details ${c.folded ? '' : 'open'}><summary>净变化${c.folded ? '（辅助文件已折叠）' : ''}</summary><h3>Before</h3><pre>${escapeHTML(c.before?.content ?? '(不存在)')}</pre><h3>After</h3><pre>${escapeHTML(c.after?.content ?? '(不存在)')}</pre></details></section>`).join('')}<h2>未捕获文件</h2><ul>${(round.omitted || []).map(x => `<li>${escapeHTML(x)}</li>`).join('')}</ul>${round.analysis ? explanationHTML(round.analysis, 'analysis') : '<p>本轮没有已保存的模型解释。</p>'}`, nonce);
}
module.exports = { runCLI, confinedPath, escapeHTML, renderInspection, renderIndexStatus, renderRound, renderSharePreview, renderShared, renderSaved, page };
