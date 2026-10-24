const test = require('node:test');
const assert = require('node:assert/strict');
const { createInline } = require('../src/inline');
const { samples, explanation } = require('./reading-fixtures');

function setup(sample) {
  const commands = new Map(), actions = [];
  const vscode = {
    MarkdownString: class { value=''; appendText(s) { this.value += String(s).replace(/[\\`*_{}\[\]()#+\-.!<>|]/g,'\\$&'); return this; } appendMarkdown(s) { this.value += s; return this; } },
    Range: class {}, CommentMode:{Preview:0}, CommentThreadCollapsibleState:{Expanded:1},
    commands:{registerCommand(id,fn){commands.set(id,fn);return {dispose(){}};}},
    comments:{createCommentController(){return {dispose(){},createCommentThread(uri,range,comments){return {uri,range,comments,dispose(){this.disposed=true;}};}};}},
  };
  const evidence=[{id:'opaque-evidence-main',file:sample.file,start_line:2,end_line:5,source:sample.source}];
  if(sample.name==='dependency') evidence.push({id:'opaque-evidence-dependency',file:sample.file,start_line:6,end_line:9,source:sample.source});
  const info={function:{name:sample.functionName,start_line:2,end_line:5},evidence,history:[explanation(sample,evidence)],issues:[],dependencies:sample.name==='dependency'?[{name:'discountSample',file:sample.file,start_line:6}]:[]};
  const ui=createInline(vscode,{subscriptions:[]},fn=>fn);
  const view=ui.open({toString:()=>sample.file},info,async action=>actions.push(action));
  const get=name=>view.thread.comments.find(c=>c.author.name===name)?.body.value;
  async function click(name, label) {
    const links=[...get(name).matchAll(/\[([^\n]*?)\]\(command:intentlens.inlineAction\?([^)]*)\)/g)];
    const link=links.find(m=>m[1].includes(label));assert.ok(link,`${name} link ${label}`);
    const args=JSON.parse(decodeURIComponent(link[2]));await commands.get('intentlens.inlineAction')(...args);return args;
  }
  return {ui,view,info,get,click,actions,commands};
}

for(const sample of samples) test(`reading layout and citations: ${sample.name}`,async()=>{
  const h=setup(sample);
  assert.ok(h.get('函数作用').includes(sample.claims[0].text));
  assert.ok(!h.get('输入输出与边界').includes(sample.claims[1].text));
  assert.ok(!h.view.thread.comments.some(c=>c.body.value.includes('opaque-evidence')));
  const old=await h.click('输入输出与边界','展开');
  assert.ok(h.get('输入输出与边界').includes(sample.claims[1].text));
  await h.commands.get('intentlens.inlineAction')(...old);
  assert.ok(h.get('输入输出与边界').includes(sample.claims[1].text),'old generation cannot toggle');
  await h.click('输入输出与边界','收起');
  assert.ok(!h.get('输入输出与边界').includes(sample.claims[1].text));
  assert.equal(h.actions.length,0,'folding never invokes model handler');
  await h.click('函数作用',sample.file.replace('.','\\.'));
  assert.deepEqual(h.actions[0],{action:'snapshot',index:'0',group:'0'});
  if(sample.name==='simple') {assert.equal(h.get('执行流程'),undefined);assert.equal(h.get('关键设计'),undefined);}
  if(sample.name==='error') {await h.click('执行流程','展开');assert.match(h.get('执行流程'),/1\\\. /);assert.ok(h.get('关键设计').includes('提前返回'));}
  if(sample.name==='dependency') {
    assert.ok(h.get('依赖说明').includes('接收小计与数量'));
    assert.ok(!h.get('关键设计').includes('订单函数负责'));
    await h.click('关键设计','展开');assert.ok(h.get('关键设计').includes('订单函数负责'));
    await h.click('依赖说明','展开');await h.click('依赖说明','查看依赖');assert.equal(h.actions.at(-1).action,'dependency');
    assert.ok(h.get('IntentLens').includes('没有记录业务为何'));
  }
});

test('collapsed warnings, legacy and expired explanations stay explicit',async()=>{
  const h=setup(samples[1]);
  h.info.history[0].document.claims[1].status='uncertain';
  h.info.history[0].corrections=[{claim_id:'c2',reason:'分支解释不符'}];
  h.view.update(h.info);
  assert.ok(h.get('IntentLens').includes('待确认'));
  assert.ok(h.get('IntentLens').includes('分支解释不符'));
  delete h.info.history[0].document.claims[0].section;h.view.update(h.info);
  assert.equal(h.get('函数作用'),undefined);assert.ok(h.get('旧版或未分类说明').includes('展开'));
  assert.ok(h.get('IntentLens').includes('重新生成'));
  h.info.history[0].freshness='解释已过期';h.view.update(h.info);
  assert.equal(h.get('执行流程'),undefined);assert.ok(h.get('IntentLens').includes('已过期'));
  h.ui.clear();h.view.update(h.info);assert.ok(h.view.thread.disposed);
});

test('missing citations warn before expansion and cannot start practice', async()=>{
  const h=setup(samples[0]);
  h.info.history[0].document.claims[1].evidence_ids=['missing'];
  h.view.update(h.info);
  assert.ok(h.get('IntentLens').includes('引用依据不可定位'));
  await h.click('输入输出与边界','展开');
  assert.ok(!h.get('输入输出与边界').includes('小修改练习'));
});
