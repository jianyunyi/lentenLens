'use strict';
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const { coverageLines } = require('./coverage');

function runCLI(binary, root, command, input = {}, options = {}) {
  return new Promise((resolve, reject) => {
    const args = [...(options.prefixArgs || []), command, '--root', root, ...(options.args || [])];
    const child = (options.spawnProcess || spawn)(binary, args, { cwd: root, shell: false, windowsHide: true, env: options.env || process.env, signal: options.signal });
    let stdout = '', stderr = '', size = 0, settled = false, terminationError, childError, killTimer;
    const finish = (error, value) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      clearTimeout(killTimer);
      if (error) reject(error);
      else resolve(value);
    };
    const kill = signal => {
      try { child.kill(signal); } catch { /* The child may already have exited. */ }
    };
    const terminate = (error, sendSignal = true) => {
      if (settled || terminationError) return;
      terminationError = error;
      clearTimeout(timer);
      killTimer = setTimeout(() => {
        kill('SIGKILL');
        finish(childError || terminationError);
        // Descendants can keep these pipes open even after the child exits.
        child.stdin.destroy();
        child.stdout.destroy();
        child.stderr.destroy();
      }, 1000);
      if (sendSignal) kill();
    };
    const timer = setTimeout(() => terminate(new Error('IntentLens CLI timed out')), options.timeout || 180000);
    child.on('error', error => {
      if (settled) return;
      childError ||= error;
      // Spawn errors and AbortSignal cancellation already come from Node.
      terminate(error, false);
    });
    child.stdout.setEncoding('utf8');
    child.stderr.setEncoding('utf8');
    child.stdout.on('data', data => {
      if (settled || terminationError) return;
      size += Buffer.byteLength(data);
      if (size > 32 * 1024 * 1024) terminate(new Error('CLI response exceeds 32 MiB'));
      else stdout += data;
    });
    child.stderr.on('data', data => { if (stderr.length < 8192) stderr += data.toString(); });
    child.on('close', code => {
      if (settled) return;
      if (childError) return finish(childError);
      if (terminationError) return finish(terminationError);
      let value;
      try { value = JSON.parse(stdout); } catch { return finish(new Error(`CLI returned invalid JSON (exit ${code}): ${stderr.slice(0, 500)}`)); }
      if (code !== 0 || value?.error) return finish(new Error(value?.error || `CLI exited ${code}`));
      finish(null, value);
    });
    child.stdin.on('error', error => {
      terminate(error);
      child.stdin.destroy();
    });
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
function compareEditorPosition(a, b) { return a.line - b.line || a.character - b.character; }
function validEditorPosition(value) { return value && Number.isInteger(value.line) && value.line >= 0 && value.line <= 100000000 && Number.isInteger(value.character) && value.character >= 0 && value.character <= 100000000; }
function rangeOverlapsSelection(range, selection) {
  if (!validEditorPosition(range?.start) || !validEditorPosition(range?.end) || !validEditorPosition(selection?.start) || !validEditorPosition(selection?.end)) return false;
  const startToEnd = compareEditorPosition(range.start, range.end);
  if (startToEnd > 0 || compareEditorPosition(selection.start, selection.end) >= 0) return false;
  if (startToEnd === 0) return compareEditorPosition(range.start, selection.start) >= 0 && compareEditorPosition(range.start, selection.end) < 0;
  return compareEditorPosition(range.start, selection.end) < 0 && compareEditorPosition(selection.start, range.end) < 0;
}
function selectedErrorDiagnostics(vscode, uri, selection) {
  const errorSeverity = vscode.DiagnosticSeverity?.Error;
  if (!Number.isInteger(errorSeverity) || typeof vscode.languages?.getDiagnostics !== 'function') throw new Error('VS Code 错误诊断接口不可用。');
  const selected = (vscode.languages.getDiagnostics(uri) || []).filter(item => item?.severity === errorSeverity && typeof item.message === 'string' && rangeOverlapsSelection(item.range, selection)).map(item => ({
    severity: 'error',
    message: item.message,
    range: {
      start: { line: item.range.start.line, character: item.range.start.character },
      end: { line: item.range.end.line, character: item.range.end.character },
    },
  }));
  return selected.sort((a, b) => compareEditorPosition(a.range.start, b.range.start) || compareEditorPosition(a.range.end, b.range.end) || (a.message < b.message ? -1 : a.message > b.message ? 1 : 0));
}
function compareSelectionPosition(a, b) { return a.line - b.line || a.column - b.column; }
function validSelectionRange(range) {
  const valid = point => point && Number.isInteger(point.line) && point.line >= 1 && Number.isInteger(point.column) && point.column >= 1;
  return range && range.end_exclusive === true && valid(range.start) && valid(range.end) && compareSelectionPosition(range.start, range.end) <= 0;
}
function validateSelectionResult(data) {
  if (!data || !Array.isArray(data.claims) || !Array.isArray(data.diagnostics) || !Array.isArray(data.unknowns) || !Array.isArray(data.evidence) || !Number.isInteger(data.saved_version) || data.saved_version < 1) throw new Error('选区解释结果不符合统一结构。');
  if (!['errors_found', 'no_errors'].includes(data.diagnostics_status) || (data.diagnostics_status === 'errors_found') !== (data.diagnostics.length > 0)) throw new Error('选区错误诊断状态与结果内容不一致。');
  const evidenceIDs = new Set();
  for (const item of data.evidence) {
    if (!item || !['selected-code', 'selected-diagnostics'].includes(item.id) || evidenceIDs.has(item.id) || typeof item.source !== 'string' || item.file !== '') throw new Error('选区证据结构无效或包含文件路径。');
    if ((item.id === 'selected-code' && (item.kind !== 'selection' || typeof item.language !== 'string' || !Number.isInteger(item.start_line) || !Number.isInteger(item.end_line))) || (item.id === 'selected-diagnostics' && item.kind !== 'diagnostics')) throw new Error('选区证据类型无效。');
    evidenceIDs.add(item.id);
  }
  if (!evidenceIDs.has('selected-code')) throw new Error('统一结果缺少所选代码证据。');
  if ((data.diagnostics_status === 'errors_found') !== evidenceIDs.has('selected-diagnostics')) throw new Error('选区诊断与证据集合不一致。');
  const claimIDs = new Set();
  for (const claim of data.claims) {
    if (!claim || typeof claim.id !== 'string' || !claim.id || claimIDs.has(claim.id) || typeof claim.text !== 'string' || !claim.text || !['supported', 'uncertain'].includes(claim.status) || !Array.isArray(claim.evidence_ids) || claim.evidence_ids.length === 0 || claim.evidence_ids.some(id => !evidenceIDs.has(id))) throw new Error('选区解释条目缺少有效证据引用。');
    claimIDs.add(claim.id);
  }
  for (const diagnostic of data.diagnostics) {
    if (!diagnostic || diagnostic.severity !== 'error' || typeof diagnostic.message !== 'string' || !validSelectionRange(diagnostic.absolute_range) || !validSelectionRange(diagnostic.relative_range)) throw new Error('选区诊断结构无效或包含非错误级诊断。');
  }
  if (data.unknowns.some(item => typeof item !== 'string')) throw new Error('选区未知项结构无效。');
  return data;
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
function renderSelectionExplanation(data, nonce) {
  const labels = { purpose: '意义 / 可能用途', contract: '可见的输入、输出与副作用', flow: '执行流程', design: '可能的设计考虑', dependency: '依赖' };
  const claims = data.claims || [];
  const evidence = new Map((data.evidence || []).map(item => [item.id, item]));
  const selectedCode = evidence.get('selected-code');
  const formatRange = range => `${range.start.line}:${range.start.column}–${range.end.line}:${range.end.column}（末端不含）`;
  const sections = ['purpose', 'contract', 'flow', 'design', 'dependency'].map(section => {
    const items = claims.filter(claim => claim.section === section);
    if (!items.length) return '';
    return `<section><h2>${labels[section]}</h2>${items.map(claim => `<p><strong>${escapeHTML(claim.status || '待核对')}</strong> ${escapeHTML(claim.text)}<br><small>依据：${escapeHTML((claim.evidence_ids || []).map(id => id === 'selected-code' ? '所选代码' : id === 'selected-diagnostics' ? '所选范围内错误诊断' : id).join('、'))}</small></p>`).join('')}</section>`;
  }).join('');
  const unclassified = claims.filter(claim => !labels[claim.section]).map(claim => `<p><strong>${escapeHTML(claim.status || '待核对')}</strong> ${escapeHTML(claim.text)}</p>`).join('');
  const unknowns = (data.unknowns || []).map(item => `<li>${escapeHTML(item)}</li>`).join('');
  const diagnostics = data.diagnostics_status === 'no_errors'
    ? '<p>保存后读取的选区诊断中没有 VS Code Error 级错误。</p>'
    : `<ul>${data.diagnostics.map(item => `<li><strong>绝对位置：</strong>${escapeHTML(formatRange(item.absolute_range))}；<strong>选区内：</strong>${escapeHTML(formatRange(item.relative_range))}<pre>${escapeHTML(item.message)}</pre></li>`).join('')}</ul>`;
  const code = selectedCode ? `<details><summary>查看本次选区证据</summary><pre>${escapeHTML(selectedCode.source)}</pre></details>` : '';
  return page('选中代码解释与错误诊断', `<p>保存版本：${escapeHTML(data.saved_version)} · 选区行 ${escapeHTML(selectedCode?.start_line)}–${escapeHTML(selectedCode?.end_line)}${selectedCode?.language ? ` · 语言 <code>${escapeHTML(selectedCode.language)}</code>` : ''}</p><p class="muted">只读取保存后的 VS Code Error 级诊断；只传送所选代码及与选区相交的错误消息，不发送文件路径或其他项目上下文。诊断消息作为不可信数据处理。此结果不写入解释历史。</p><section><h2>选区内的保存后错误</h2>${diagnostics}<p class="muted">位置按 1 起始行列显示；诊断区间的末端不包含在范围内。</p></section>${code}${sections}${unclassified}${claims.length ? '' : '<p>当前没有可展示的解释条目；若上方有错误诊断，仍可按原始消息和位置核对。</p>'}<section><h2>未知项与缺少上下文</h2><ul>${unknowns || '<li>仅凭选区无法验证完整调用上下文、运行效果或测试结果。</li>'}</ul></section>`, nonce);
}
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
  const changes = (round.changes || []).map((c, i) => `<section><h2>${escapeHTML(c.path)} · ${escapeHTML(c.kind)}</h2>${button('change', i, '按需读取 before / after 快照')} ${button('changeCurrent', i, '打开当前文件')}<p class="muted">Before ${escapeHTML(c.before?.bytes ?? 0)} 字节 · After ${escapeHTML(c.after?.bytes ?? 0)} 字节</p></section>`).join('');
  const analysisEntry = round.analysis ? { ...round.analysis, evidence: (round.analysis.evidence || []).map(ev => ({ ...ev, source: '点击后按证据 ID 重新读取' })) } : undefined;
  const moreChanges = round.next_cursor ? button('loadMoreChanges', '', '加载更多文件变化') : '';
  return page(`轮次 ${round.id}`, `<p>${escapeHTML(round.started)} → ${escapeHTML(round.ended)}</p><p>来源: ${escapeHTML(round.source)} · 工作区净变化，不能据此推断作者动机。</p>${analysis}<ul>${(round.notes || []).map(x => `<li>${escapeHTML(x)}</li>`).join('')}</ul>${changes || '<p>本轮没有可查看的文件变化。</p>'}${moreChanges}<h2>未捕获文件</h2><p>${escapeHTML(round.omitted_count || 0)} 个文件未完整捕获。</p>${analysisEntry ? explanationHTML(analysisEntry, 'analysis') : '<p>本轮没有已保存的模型解释。</p>'}`, nonce);
}
module.exports = { runCLI, confinedPath, escapeHTML, compareEditorPosition, rangeOverlapsSelection, selectedErrorDiagnostics, validateSelectionResult, renderInspection, renderIndexStatus, renderRound, renderSharePreview, renderShared, renderSaved, renderSelectionExplanation, page };
