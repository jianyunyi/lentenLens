'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const Module = require('node:module');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

function setup() {
  const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'intentlens-ui-oracle-'));
  const commands = new Map(), providers = new Map(), errors = [], calls = [], opened = [], callbacks = {};
  const disposable = () => ({ dispose() {} });
  const uri = file => ({ scheme:'file', fsPath:file, toString:()=>`file://${file}` });
  const folders = ['A','B'].map(name => { const root=path.join(temporary,name); fs.mkdirSync(root); return {name,uri:uri(root)}; });
  const documents = folders.map((folder,i) => {
    const name=i?'Beta':'Alpha';
    const source=`package p\nfunc ${name}(){ /*中文*/ Leaf() }\nfunc Leaf(){}\n`;
    const file=path.join(folder.uri.fsPath,'main.go');fs.writeFileSync(file,source);
    const lines=source.split('\n');
    return {uri:uri(file),languageId:'go',isDirty:false,lineCount:lines.length,lineAt:line=>({text:lines[line]}),source,name};
  });
  const graph = index => {
    const doc=documents[index], line=doc.lineAt(1).text;
    return {selected_id:`main.go:${doc.name}`,root_ids:[`main.go:${doc.name}`],functions:[
      {id:`main.go:${doc.name}`,name:doc.name,file:'main.go',start_line:2,end_line:2,source:line},
      {id:'main.go:Leaf',name:'Leaf',file:'main.go',start_line:3,end_line:3,source:'func Leaf(){}'},
    ],calls:[{caller_id:`main.go:${doc.name}`,name:'Leaf',status:'resolved',targets:['main.go:Leaf'],file:'main.go',line:2,column:Buffer.byteLength(line.slice(0,line.indexOf('Leaf')))+1,source:'Leaf()'}],coverage:{complete:true,scope:'local',omitted_count:0,excluded_count:0},issues:[],truncated:false};
  };
  let respond = root => structuredClone(graph(root===folders[0].uri.fsPath?0:1));
  const editor = document => ({document,selection:{active:{line:1}},revealRange(){}});
  const vscode = {
    workspace:{isTrusted:true,workspaceFolders:folders,textDocuments:documents,
      getWorkspaceFolder:value=>folders.find(f=>value.fsPath.startsWith(f.uri.fsPath+path.sep)),
      getConfiguration:()=>({get:()=>''}),
      openTextDocument:async value=>{opened.push(value.fsPath);return documents.find(d=>d.uri.fsPath===value.fsPath);},
      createFileSystemWatcher:glob=>({dispose(){},onDidCreate:fn=>{callbacks[glob+'create']=fn;return disposable();},onDidChange:fn=>{callbacks[glob+'change']=fn;return disposable();},onDidDelete:fn=>{callbacks[glob+'delete']=fn;return disposable();}}),
      onDidChangeTextDocument:fn=>{callbacks.text=fn;return disposable();},onDidGrantWorkspaceTrust:()=>disposable(),onDidSaveTextDocument:()=>disposable(),onDidChangeWorkspaceFolders:()=>disposable(),
    },
    window:{activeTextEditor:editor(documents[0]),createTreeView:(id,options)=>{providers.set(id,options.treeDataProvider);return disposable();},showErrorMessage:message=>errors.push(message),showTextDocument:async doc=>{const e=editor(doc);vscode.window.activeTextEditor=e;return e;}},
    commands:{registerCommand:(id,fn)=>{commands.set(id,fn);return disposable();},executeCommand:async()=>{}},
    languages:{registerCodeLensProvider:()=>disposable()},
    EventEmitter:class{event=()=>{};fire(){};dispose(){}},TreeItem:class{constructor(label,collapsibleState){this.label=label;this.collapsibleState=collapsibleState;}},TreeItemCollapsibleState:{None:0,Collapsed:1},
    Uri:{file:uri},Position:class{constructor(line,character){this.line=line;this.character=character;}},Selection:class{constructor(start,end){this.start=start;this.end=end;}},Range:class{},TextEditorRevealType:{InCenter:0},
  };
  const file=path.join(__dirname,'../src/extension.js');
  const core=require('../src/core');
  const original=Module._load;
  Module._load=function(id,...args){if(id==='vscode')return vscode;if(id==='./core'&&args[0]?.filename===file)return {...core,runCLI:async(binary,root,command,input)=>{calls.push({root,command,input});return respond(root,command,input);}};return original.call(this,id,...args);};
  try{delete require.cache[require.resolve(file)];require(file).activate({subscriptions:[],extensionPath:path.dirname(path.dirname(file))});}finally{Module._load=original;}
  const provider=providers.get('intentlens.callPath');
  function site(){const root=provider.getChildren().find(item=>item.kind==='function');return provider.getChildren(root)[0].command.arguments[0];}
  return {vscode,commands,provider,errors,calls,opened,callbacks,documents,folders,graph,editor,site,setResponse:fn=>{respond=fn;},cleanup:()=>fs.rmSync(temporary,{recursive:true,force:true})};
}

test('call path icon action is usable; navigation retains original workspace and converts UTF-8 source columns',async()=>{
  const h=setup();try{
    assert.equal(h.provider.getChildren()[0].command.command,'intentlens.callPath');
    await h.commands.get('intentlens.callPath')();const site=h.site();
    h.vscode.window.activeTextEditor=h.editor(h.documents[1]);
    await h.commands.get('intentlens.openCallPathSite')(site);
    assert.equal(h.calls.at(-1).root,h.folders[0].uri.fsPath);
    assert.equal(h.opened.at(-1),h.documents[0].uri.fsPath);
    const position=h.vscode.window.activeTextEditor.selection.start;
    assert.equal(position.line,1);
    assert.equal(position.character,h.documents[0].lineAt(1).text.indexOf('Leaf'));
    assert.deepEqual(h.errors,[]);
  }finally{h.cleanup();}
});

test('call path refuses trust, dirty hidden targets, stale source and fabricated evidence',async()=>{
  const h=setup();try{
    h.vscode.workspace.isTrusted=false;await h.commands.get('intentlens.callPath')();assert.equal(h.calls.length,0);
    h.vscode.workspace.isTrusted=true;await h.commands.get('intentlens.callPath')();const site=h.site();
    h.vscode.window.activeTextEditor=h.editor(h.documents[1]);h.documents[0].isDirty=true;
    await h.commands.get('intentlens.openCallPathSite')(site);assert.equal(h.opened.length,0);assert.match(h.errors.at(-1),/保存/);
    h.documents[0].isDirty=false;
    await h.commands.get('intentlens.openCallPathSite')({...site,source:'Leaf(secret)'});assert.equal(h.opened.length,0);assert.match(h.errors.at(-1),/证据/);
    const changed=h.graph(0);changed.functions[0].source+=' // changed at same coordinates';h.setResponse(()=>changed);
    await h.commands.get('intentlens.openCallPathSite')(site);assert.equal(h.opened.length,0);assert.match(h.errors.at(-1),/证据已变化/);
    assert.equal(h.provider.getChildren()[0].command.command,'intentlens.callPath');
  }finally{h.cleanup();}
});

test('source invalidation during navigation cannot reopen stale evidence',async()=>{
  const h=setup();try{
    await h.commands.get('intentlens.callPath')();const site=h.site();let release;
    h.setResponse(()=>new Promise(resolve=>{release=resolve;}));
    const pending=h.commands.get('intentlens.openCallPathSite')(site);
    await new Promise(resolve=>setImmediate(resolve));
    h.callbacks.text({document:h.documents[0],contentChanges:[{text:'changed'}]});
    release(h.graph(0));await pending;
    assert.equal(h.opened.length,0);assert.match(h.errors.at(-1),/失效/);
  }finally{h.cleanup();}
});

test('concurrent local path loads keep only the most recent cursor request',async()=>{
  const h=setup();try{
    const pending=new Map();h.setResponse(root=>new Promise(resolve=>pending.set(root,resolve)));
    const first=h.commands.get('intentlens.callPath')();await new Promise(resolve=>setImmediate(resolve));
    h.vscode.window.activeTextEditor=h.editor(h.documents[1]);
    const second=h.commands.get('intentlens.callPath')();await new Promise(resolve=>setImmediate(resolve));
    pending.get(h.folders[1].uri.fsPath)(h.graph(1));await second;
    pending.get(h.folders[0].uri.fsPath)(h.graph(0));await first;
    assert.match(h.provider.getChildren().find(item=>item.kind==='function').label,/Beta/);
    assert.deepEqual(h.errors,[]);
  }finally{h.cleanup();}
});
