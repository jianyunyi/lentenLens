'use strict';
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

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
function explanationHTML(entry, group) {
  return `<section><h2>解释 ${escapeHTML(entry.created || entry.id)}</h2><p>${escapeHTML(entry.freshness)}</p><p class="muted">模型说法需要核对；引用存在不代表结论成立。测试源码不代表测试已执行。</p>${(entry.document?.claims || []).map(c => `<p><strong>[${escapeHTML(c.status)} · ${escapeHTML(c.kind)}]</strong> ${escapeHTML(c.text)}<br><small>证据: ${escapeHTML((c.evidence_ids || []).join(', '))}</small></p>`).join('')}<ul>${(entry.document?.unknowns || []).map(x => `<li>${escapeHTML(x)}</li>`).join('')}</ul>${evidenceHTML(entry.evidence, group)}</section>`;
}
function renderInspection(data, nonce) {
  return page(data.function?.name || '函数证据', `<p>本地静态证据 · 已保存的磁盘版本</p>${button('explain', '', '生成解释')}${button('ask', '', '自由提问')}<pre>${escapeHTML(data.function?.source)}</pre><h2>直接依赖</h2>${(data.dependencies || []).map((f, i) => button('dependency', i, `${f.name} · ${f.file}:${f.start_line}`)).join('') || '<p>没有唯一解析的直接依赖。</p>'}<h2>当前证据</h2>${evidenceHTML(data.evidence, 'current')}<h2>分析限制</h2><ul>${(data.issues || []).map(x => `<li>${escapeHTML(x)}</li>`).join('')}</ul>${(data.history || []).map((entry, i) => explanationHTML(entry, String(i))).join('')}`, nonce);
}
function renderRound(round, nonce) {
  return page(`轮次 ${round.id}`, `<p>${escapeHTML(round.started)} → ${escapeHTML(round.ended)}</p><p>来源: ${escapeHTML(round.source)} · 工作区净变化，不能据此推断作者动机。</p><ul>${(round.notes || []).map(x => `<li>${escapeHTML(x)}</li>`).join('')}</ul>${(round.changes || []).map((c, i) => `<section><h2>${escapeHTML(c.path)} · ${escapeHTML(c.kind)}</h2>${button('change', i, '查看 before / after 快照')} ${button('changeCurrent', i, '打开当前文件')}<details ${c.folded ? '' : 'open'}><summary>净变化${c.folded ? '（辅助文件已折叠）' : ''}</summary><h3>Before</h3><pre>${escapeHTML(c.before?.content ?? '(不存在)')}</pre><h3>After</h3><pre>${escapeHTML(c.after?.content ?? '(不存在)')}</pre></details></section>`).join('')}<h2>未捕获文件</h2><ul>${(round.omitted || []).map(x => `<li>${escapeHTML(x)}</li>`).join('')}</ul>${round.analysis ? explanationHTML(round.analysis, 'analysis') : '<p>本轮没有已保存的模型解释。</p>'}`, nonce);
}
module.exports = { runCLI, confinedPath, escapeHTML, renderInspection, renderRound, page };
