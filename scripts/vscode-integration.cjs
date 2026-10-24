'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { execFileSync } = require('node:child_process');
const vscode = require('vscode');

exports.run = async function () {
  const extension = vscode.extensions.getExtension('intentlens-local.intentlens');
  assert.ok(extension, 'extension discovered');
  await extension.activate();
  assert.ok(vscode.workspace.isTrusted, 'test workspace trusted');
  const root = vscode.workspace.workspaceFolders[0].uri;
  const doc = await vscode.workspace.openTextDocument(vscode.Uri.joinPath(root, 'main.go'));
  await vscode.window.showTextDocument(doc);
  const lenses = await vscode.commands.executeCommand('vscode.executeCodeLensProvider', doc.uri);
  assert.ok(lenses.some(lens => lens.command?.command === 'intentlens.inspect'), 'Go CodeLens supplied by actual extension');
  const thread = await vscode.commands.executeCommand('intentlens.inspect', doc.uri, 2);
  assert.ok(thread, 'native inline comment thread returned');
  assert.equal(thread.range.end.line, 1, 'thread at function closing line');
  assert.equal(thread.canReply, false);
  assert.equal(thread.collapsibleState, vscode.CommentThreadCollapsibleState.Expanded);
  assert.deepEqual(thread.comments.slice(1).map(c => c.author.name), ['查阅证据']);
  thread.collapsibleState = vscode.CommentThreadCollapsibleState.Collapsed;
  assert.equal(thread.collapsibleState, vscode.CommentThreadCollapsibleState.Collapsed);
  thread.collapsibleState = vscode.CommentThreadCollapsibleState.Expanded;
  assert.ok(thread.comments[0].body.value.includes('尚未生成'));
  assert.equal(doc.isDirty, false, 'inline explanation does not modify source');
  const cli = (command, input={}) => JSON.parse(execFileSync(path.join(extension.extensionPath,'bin',process.platform==='win32'?'intentlens.exe':'intentlens'),[command,'--root',root.fsPath],{input:JSON.stringify(input),windowsHide:true,encoding:'utf8'}));
  const {samples,explanation} = require('../extension/test/reading-fixtures');
  for (const [i,sample] of samples.entries()) {
    fs.writeFileSync(path.join(root.fsPath,sample.file),sample.source);
    const line=sample.source.split('\n').findIndex(s=>s.startsWith('func '+sample.functionName))+1;
    const info=cli('inspect',{file:sample.file,line});
    const record={...explanation(sample,info.evidence),id:String(i+1).repeat(24),file:sample.file,function_id:info.function.id};
    fs.writeFileSync(path.join(cli('status').state_directory,'explanation-'+record.id+'.json'),JSON.stringify(record));
  }
  // Let source watchers settle before opening the fixture threads.
  await new Promise(resolve=>setTimeout(resolve,1000));
  for (const sample of samples) {
    const sampleDoc=await vscode.workspace.openTextDocument(vscode.Uri.joinPath(root,sample.file));
    await vscode.window.showTextDocument(sampleDoc);
    const line=sample.source.split('\n').findIndex(s=>s.startsWith('func '+sample.functionName))+1;
    const sampleThread=await vscode.commands.executeCommand('intentlens.inspect',sampleDoc.uri,line);
    const body=name=>sampleThread.comments.find(c=>c.author.name===name)?.body.value;
    const escaped=text=>{const value=new vscode.MarkdownString();value.appendText(text);return value.value;};
    assert.ok(body('函数作用').includes(escaped(sample.claims[0].text)),sample.name+' purpose');
    assert.ok(!body('输入输出与边界').includes(escaped(sample.claims[1].text)));
    const toggle=body('输入输出与边界').match(/command:intentlens.inlineAction\?([^)]*)/);
    await vscode.commands.executeCommand('intentlens.inlineAction',...JSON.parse(decodeURIComponent(toggle[1])));
    const expected=new vscode.MarkdownString(); expected.appendText(sample.claims[1].text);
    assert.ok(body('输入输出与边界').includes(expected.value), sample.name+': '+body('输入输出与边界'));
    const collapse=body('输入输出与边界').match(/\[收起[^\]]*\]\(command:intentlens.inlineAction\?([^)]*)/);
    await vscode.commands.executeCommand('intentlens.inlineAction',...JSON.parse(decodeURIComponent(collapse[1])));
    assert.ok(!body('输入输出与边界').includes(expected.value));
    assert.equal(sampleDoc.isDirty,false);
    assert.equal(fs.readFileSync(sampleDoc.uri.fsPath,'utf8'),sample.source);
  }
  await vscode.commands.executeCommand('intentlens.inspectDetails', doc.uri, 2);
	if (!vscode.window.tabGroups.all.some(group => group.tabs.some(tab => tab.label.includes('IntentLens')))) {
	  await new Promise(resolve => {
	    const timer = setTimeout(() => { sub.dispose(); resolve(); }, 5000);
	    const sub = vscode.window.tabGroups.onDidChangeTabs(() => {
	      if (vscode.window.tabGroups.all.some(group => group.tabs.some(tab => tab.label.includes('IntentLens')))) { clearTimeout(timer); sub.dispose(); resolve(); }
	    });
	  });
	}
  assert.ok(vscode.window.tabGroups.all.some(group => group.tabs.some(tab => tab.label.includes('IntentLens'))), 'evidence webview opened');
  const commands = await vscode.commands.getCommands(true);
  for (const id of ['intentlens.enableModel', 'intentlens.pauseModel', 'intentlens.hooksConfig', 'intentlens.refresh', 'intentlens.shared', 'intentlens.exercises', 'intentlens.sharedExercises']) assert.ok(commands.includes(id), id);
  const spec = { title: 'Fixture exercise', requirements: [{id:'r1',text:'Twice(1) returns 3',test:'TestIntentLensThree'}], baseline: {'main.go':fs.readFileSync(path.join(root.fsPath,'main.go'),'utf8')}, acceptance: 'package main\nimport "testing"\nfunc TestIntentLensThree(t *testing.T) { if Twice(1)!=3 { t.Fatal("INTENTLENS_EXPECTED_MISSING") } }\n' };
  const exerciseID = 'bbbbbbbbbbbbbbbbbbbbbbbb';
  const shared = {schema:1,id:exerciseID,version:crypto.createHash('sha256').update(JSON.stringify(spec)).digest('hex'),package_dir:'.',spec,author_validation:'fixture: not executed'};
  fs.mkdirSync(path.join(root.fsPath,'.intent','exercises'),{recursive:true});
  fs.writeFileSync(path.join(root.fsPath,'.intent','exercises',exerciseID+'.json'),JSON.stringify(shared));
  execFileSync(path.join(extension.extensionPath,'bin',process.platform==='win32'?'intentlens.exe':'intentlens'),['exercise-import','--root',root.fsPath],{input:JSON.stringify({exercise_id:exerciseID}),windowsHide:true});
  await vscode.commands.executeCommand('intentlens.openExercise',vscode.workspace.workspaceFolders[0],exerciseID);
  await new Promise(resolve=>setTimeout(resolve,500));
  assert.ok(vscode.window.tabGroups.all.some(group=>group.tabs.some(tab=>tab.label.includes('小修改练习'))),'exercise webview opened');
  const result = { result: 'PASS', checks: ['extension activation', 'trusted workspace', 'real CodeLens provider', 'native inline thread at function end', 'three Go fixtures with purpose and collapsed contract', 'section expansion in actual host', 'source unchanged', 'evidence webview', 'exercise imported locally', 'exercise webview', 'commands registered'], model_calls: 0, exercise_execution: false };
  if (process.env.INTENTLENS_TEST_RESULT) fs.writeFileSync(process.env.INTENTLENS_TEST_RESULT, JSON.stringify(result, null, 2));
};
