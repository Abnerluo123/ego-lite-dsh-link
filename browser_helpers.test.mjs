import fs from 'node:fs/promises';
import vm from 'node:vm';
import assert from 'node:assert/strict';
const source=await fs.readFile(new URL('./browser_helpers.js',import.meta.url),'utf8');
const context=vm.createContext({console});
vm.runInContext(source+'\nglobalThis.helper=__ego;',context);
const helper=context.helper;
assert.deepEqual(JSON.parse(JSON.stringify(helper.documentClip({x:12,y:34,width:80,height:60},{pageX:100,pageY:900},0.5,2))),{x:112,y:934,width:80,height:60,scale:0.25});
const page={evaluate:async source=>typeof source==='function'?source():vm.runInContext(source,context)};
for(const [expression,want] of [
 ['1+2',3], ['[1,2].map(x=>x+1)',[2,3]], ['"=>"','=>'],
 ['()=>4',4],['function(){return 5}',5],['async function(){return 6}',6],
 ['(()=>7)()',7],['new Promise(r=>r(8))',8],['({ok:true})',{ok:true}],['undefined',undefined]
]) assert.deepEqual(JSON.parse(JSON.stringify({v:await helper.evaluate(page,expression)})),JSON.parse(JSON.stringify({v:want})));
let invocations=0;
context.bump=()=>++invocations;
assert.equal(await helper.evaluate(page,'(()=>bump())()'),1);assert.equal(invocations,1);
const picked=await helper.pick({tabs:async()=>[{label:'p1',active:true}],page:label=>({label})},'p2');assert.equal(picked.page.label,'p1');assert.equal(picked.recoveredFrom,'p2');
assert.throws(()=>helper.resolveInPage({selector:'loc=role:button'}),/native Ego action/);
assert.throws(()=>helper.resolveInPage({selector:'#x >> nth=-2'}),/nth must/);
const receiptPage={evaluate:async()=>{throw new Error('DOM unavailable')},click:async()=>({dialog:{type:'confirm'}})};
const action=await helper.action(receiptPage,'click',[]);assert.equal(action.receipt.dialog.type,'confirm');assert.equal(action.noVisibleChange,false);
await assert.rejects(()=>helper.evaluate(page,'throw new Error("boom")'));
console.log('browser_helpers: expression, promise, IIFE, recovery and action receipt tests passed');
