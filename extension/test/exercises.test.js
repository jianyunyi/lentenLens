const test = require('node:test');
const assert = require('node:assert/strict');
const { createExerciseUI, renderExercise } = require('../src/exercises');
const os = require('node:os');
const path = require('node:path');

test('dirty answer files use path boundaries and Windows case-insensitive comparison', async () => {
 const workspace = path.join(os.tmpdir(), 'intentlens-answer-boundary');
 async function runWithDirtyFile(dirtyPath) {
  const calls=[],panels=[];
  const view={id:'e',version:'v',spec:{title:'t',baseline:{'main.go':'package p'},requirements:[]},validation:{status:'unverified'},attempts:[],workspace};
  const host={vscode:{workspace:{textDocuments:[{isDirty:true,uri:{fsPath:dirtyPath}}]},window:{showInformationMessage(){}}},cli:async(_,command)=>{calls.push(command);if(command==='exercise-view')return structuredClone(view);if(command==='exercise-plan')return {digest:'scope',workspace,tool:{path:'go'},directories:[],args:[],scope:[]};throw new Error(`unexpected ${command}`);},panel:(_,render,handler)=>{const p={webview:{html:render('nonce')},handler,dispose(){}};panels.push(p);return p;}};
  const ui=createExerciseUI(host);await ui.show({},view);
  let failure;try{await panels[0].handler({action:'score'});}catch(error){failure=error;}
  return {failure,calls};
 }
 const inside = path.join(workspace, 'main.go');
 const caseVariant = process.platform === 'win32' ? inside.toUpperCase() : inside;
 const blocked = await runWithDirtyFile(caseVariant);
 assert.match(blocked.failure?.message || '', /请先保存作答目录/);
 assert.equal(blocked.calls.includes('exercise-plan'), false);
 const sibling = path.join(`${workspace}-copy`, 'main.go');
 const allowed = await runWithDirtyFile(sibling);
 assert.equal(allowed.failure, undefined);
 assert.equal(allowed.calls.includes('exercise-plan'), true);
});

test('exercise UI requires execution and application previews; closing does not confirm', async () => {
 const calls=[], panels=[];
 const view={id:'e',version:'v',spec:{title:'<unsafe>',baseline:{'main.go':'package p'},requirements:[]},validation:{status:'unverified'},attempts:[],workspace:'/private/answer'};
 const host={vscode:{window:{showInformationMessage(){}}},cli:async(_,command,input)=>{calls.push({command,input});if(command==='exercise-view'||command==='exercise-run')return structuredClone(view);if(command==='exercise-plan')return {digest:'scope',workspace:view.workspace,tool:{path:'/go'},directories:[],args:[],scope:[]};if(command==='exercise-apply-preview')return {digest:'diff',changes:[]};},panel:(_,render,handler)=>{const p={webview:{html:render('nonce')},handler,dispose(){this.closed=true}};panels.push(p);return p;}};
 const ui=createExerciseUI(host);await ui.show({},view);
 await panels[0].handler({action:'validate'});
 assert.equal(calls.filter(c=>c.command==='exercise-run').length,0);
 await panels[1].handler({action:'cancel'});
 assert.equal(calls.filter(c=>c.command==='exercise-run').length,0);
 await panels[0].handler({action:'validate'});
 await panels.at(-1).handler({action:'confirm'});
 assert.equal(calls.find(c=>c.command==='exercise-run').input.confirm,true);
 await panels[0].handler({action:'score'});
 assert.equal(calls.filter(c=>c.command==='exercise-run').at(-1).input.confirm,false);
 await panels[0].handler({action:'apply'});
 assert.equal(calls.filter(c=>c.command==='exercise-apply-confirm').length,0);
 await panels.at(-1).handler({action:'confirm'});
 assert.equal(calls.find(c=>c.command==='exercise-apply-confirm').input.preview_digest,'diff');
 assert.ok(renderExercise(view,'nonce').includes('&lt;unsafe&gt;'));
});
