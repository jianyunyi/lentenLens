'use strict';
const crypto = require('node:crypto');
const { page, escapeHTML, confinedPath } = require('./core');
const path = require('node:path');
function isPathWithin(root, candidate) {
 if (typeof root !== 'string' || !root || typeof candidate !== 'string' || !candidate) return false;
 const relative = path.relative(path.resolve(root), path.resolve(candidate));
 return relative === '' || (relative !== '..' && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative));
}
const button = (action, text, index = '') => `<button data-action="${escapeHTML(action)}" data-index="${escapeHTML(index)}">${escapeHTML(text)}</button>`;
const json = value => `<pre>${escapeHTML(JSON.stringify(value, null, 2))}</pre>`;

function renderExercise(view, nonce) {
 const spec=view.spec || {};
 return page(spec.title || '小修改练习', `<p>题目 ${escapeHTML(view.id)} · 版本 ${escapeHTML(view.version)}</p>${view.supersedes ? `<p>新版本源自 ${escapeHTML(view.supersedes)}；旧作答不迁移。</p>` : ''}<p>题目验证：${escapeHTML(view.validation?.status)}${view.needs_review ? ' · 依据需要复核' : ''}</p>${view.imported ? '<p>共享题目不含参考解，本地无法完成参考修改验证；评分保留暂定标记。</p>' : ''}<ul>${(spec.requirements || []).map(r=>`<li>${escapeHTML(r.text)} <small>固定测试 ${escapeHTML(r.test)}</small></li>`).join('')}</ul>${button('validate','验证题目')}${button('score','执行并评分')}${button('shareExercise','准备共享题目')}${button('apply','预览应用到原项目')}${view.source_id ? button('newVersion','重新生成题目版本') : ''}<h2>独立作答目录</h2><p>${escapeHTML(view.workspace)}</p><p>先确认执行范围并验证题目。只修改作答目录的源码，保存后再评分；这里不是 OS 沙箱。</p>${Object.keys(spec.baseline || {}).filter(p=>!p.endsWith('_test.go')).map(p=>button('openAnswer',p,p)).join('')}<details><summary>固定验收测试（修改后不能计原题通过）</summary><pre>${escapeHTML(spec.acceptance)}</pre></details><details><summary>题目验证执行记录：原测试 → 预期失败 → 参考修改</summary>${json(view.validation)}</details><h2>作答记录</h2>${(view.attempts || []).map((a,i)=>`<section><h3>${escapeHTML(a.created)} · ${escapeHTML(a.version)}</h3><p>${a.provisional ? '暂定反馈：题目验证未完整通过。' : '题目通过有限验证，仍需核对题意与测试质量。'}</p><h4>行为要求通过情况</h4>${json(a.behavior)}<h4>测试执行情况</h4>${json(a.result)}<details><summary>实际命令和范围</summary>${json(a.execution)}</details><h4>理由解释反馈（补充，不改变测试结果）</h4><p>${escapeHTML(a.feedback)}</p>${button('feedback','提交理由并获取模型反馈',String(i))}</section>`).join('')}<p>有限测试通过不能证明所有情形正确，也不代表已经完全理解。</p>`, nonce);
}

function createExerciseUI({vscode, cli, panel, folderFor}) {
 const approved = new Map();
 const nonce = () => crypto.randomBytes(18).toString('hex');
 function preview(folder,title,body,confirm) {
  let busy=false;
  const view=panel(title,n=>page(title,body+button('confirm','确认')+button('cancel','取消'),n),async message=>{
   if(message.action==='cancel'){view.dispose();return;}
   if(message.action!=='confirm'||busy)return;busy=true;
   try{await confirm();view.dispose();}finally{busy=false;}
  },folder);
 }
 async function show(folder, initial) {
  let record=initial, busy=false;
  const refresh=async()=>{record=await cli(folder,'exercise-view',{exercise_id:record.id});view.webview.html=renderExercise(record,nonce());};
  const view=panel('IntentLens · 小修改练习',n=>renderExercise(record,n),async message=>{
   if(busy)return;busy=true;
   try {
    await refresh();
    const input={exercise_id:record.id};
    if(message.action==='validate'||message.action==='score') {
     if(vscode.workspace?.textDocuments?.some(doc=>doc.isDirty&&isPathWithin(record.workspace,doc.uri?.fsPath)))throw new Error('请先保存作答目录中的文件。');
     const plan=await cli(folder,'exercise-plan',input);
     const run=async confirm=>{await cli(folder,'exercise-run',{...input,mode:message.action==='validate'?'validate':'score',preview_digest:plan.digest,confirm});approved.set(record.id,plan.digest);await refresh();};
     if(approved.get(record.id)===plan.digest)await run(false);
     else preview(folder,'确认练习执行范围',`<p>确认后，本题相同命令及范围可重复执行；新增文件、依赖、规则或工具变化须重新确认。</p>${json(plan)}`,()=>run(true));
    } else if(message.action==='openAnswer') {
     if(!Object.hasOwn(record.spec.baseline,message.index)||message.index.endsWith('_test.go'))return;
     await cli(folder,'exercise-plan',input);
     const path=confinedPath(record.workspace,message.index);
     const doc=await vscode.workspace.openTextDocument(vscode.Uri.file(path));await vscode.window.showTextDocument(doc,{preview:false});
    } else if(message.action==='apply') {
     const plan=await cli(folder,'exercise-apply-preview',input);
     preview(folder,'应用修改预览',`<p>以下为将替换的原项目文件。停止并发编辑后确认；冲突会拒绝。应用后不继承原项目测试通过状态。</p>${json(plan.changes)}`,async()=>{await cli(folder,'exercise-apply-confirm',{...input,preview_digest:plan.digest,confirm:true});vscode.window.showInformationMessage('IntentLens：练习源码修改已应用，请在原项目核对。');});
    } else if(message.action==='shareExercise') {
     const draft=await cli(folder,'exercise-share-prepare',input);
     preview(folder,'共享题目预览',`<p>只写入下面的题目记录，不含参考修改、个人答案、评分和授权。</p>${json(draft.record)}`,async()=>{await cli(folder,'exercise-share-confirm',{...input,preview_digest:draft.digest,confirm:true});vscode.window.showInformationMessage('IntentLens：题目已写入 .intent/exercises。');});
    } else if(message.action==='feedback') {
     if(!/^\d+$/.test(message.index))return;const attempt=record.attempts[Number(message.index)];if(!attempt)return;
     const reason=await vscode.window.showInputBox({prompt:'说明为什么这样修改。将与本次答案、题目要求和测试结果发送给项目配置的模型；反馈不改变测试结果。'});
     if(reason?.trim()){await cli(folder,'exercise-feedback',{...input,attempt_id:attempt.id,reason});await refresh();}
    } else if(message.action==='newVersion'&&record.source_id) {
     const generated=await cli(folder,'exercise-generate',{explanation_id:record.source_id,claim_id:record.claim_id,file:record.target_file,parent_id:record.id});await show(folder,generated);
    }
   } finally {busy=false;}
  },folder);
  return view;
 }
 async function start(folder,entry,message) {
  if(!/^\d+$/.test(message.index))throw new Error('无效行为索引');
  const claim=entry.document?.claims?.[Number(message.index)];if(!claim||claim.kind!=='behavior'||claim.status!=='supported')throw new Error('请选择有证据支持的行为');
  const files=[...new Set((entry.evidence||[]).filter(e=>claim.evidence_ids?.includes(e.id)&&e.file?.endsWith('.go')&&e.role!=='before').map(e=>e.file))];
  if(!files.length)throw new Error('该行为没有可独立练习的 Go 文件依据');
  const file=files.length===1?files[0]:await vscode.window.showQuickPick(files,{placeHolder:'选择本次练习的 Go 文件/包'});if(!file)return;
  const result=await cli(folder,'exercise-generate',{explanation_id:entry.id,claim_id:claim.id,file});return show(folder,result);
 }
 async function list(shared=false) {
  const folder=await folderFor();if(!folder)return;const records=await cli(folder,shared?'exercise-shared':'exercises');
  if(!records.length){vscode.window.showInformationMessage('IntentLens：没有可显示的题目。');return;}
  const chosen=await vscode.window.showQuickPick(records.map(record=>({label:record.spec.title,description:record.version,record})),{placeHolder:shared?'选择共享题目（个人作答另存本地）':'选择题目版本'});if(!chosen)return;
  const record=shared?await cli(folder,'exercise-import',{exercise_id:chosen.record.id}):chosen.record;return show(folder,record);
 }
 return {show,start,list};
}
module.exports={createExerciseUI,renderExercise};
