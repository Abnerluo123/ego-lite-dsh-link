import fs from 'node:fs/promises';
import vm from 'node:vm';
import assert from 'node:assert/strict';
// Run with source exported by ZERO_EGO_QA_SPACE=3 go test -run TestExportCompletionSDK.
const source=await fs.readFile(new URL('./.gotmp/completion-sdk/finish.mjs',import.meta.url),'utf8');
async function scenario({spaces,after,tabs=[],finishError=false,registryError=false}){
 let reads=0,finished=0,claimed=0,saved;
 const context=vm.createContext({
  listTaskSpaces:async()=>{reads++;if(registryError&&reads>1)throw Error('registry offline');return reads===1?spaces:after},
  taskSpace:async id=>{claimed++;assert.equal(id,3);return {tabs:async()=>tabs,finish:async options=>{finished++;saved=options;if(finishError)throw Error('finish failed')}}},
 });
 const result=await vm.runInContext('(async()=>{'+source+'})()',context);
 return {result:JSON.parse(JSON.stringify(result)),reads,finished,claimed,saved:saved?JSON.parse(JSON.stringify(saved)):undefined};
}
let x=await scenario({spaces:[{id:3,ownership:'agent'}],after:[{id:3,ownership:'user'},{id:9,name:'other',ownership:'agent'}],tabs:[{label:'p1',openedBy:'agent'},{label:'p2',openedBy:'unknown'},{url:'user-tab'}]});
assert.equal(x.finished,1);assert.deepEqual(x.saved,{keep:['p1','p2']});assert.equal(x.result.agentControlReleased,true);assert.equal(x.result.remainingAgentSpaces[0].id,9);
x=await scenario({spaces:[{id:3,ownership:'user'}],after:[]});assert.equal(x.finished,0);assert.equal(x.claimed,0);assert.equal(x.result.agentControlReleased,true);
x=await scenario({spaces:[{id:9,ownership:'agent'}],after:[]});assert.equal(x.finished,0);assert.equal(x.claimed,0);assert.equal(x.result.status,'already-released');
x=await scenario({spaces:[{id:3,ownership:'agent'}],after:[{id:3,ownership:'agent'}]});assert.equal(x.result.agentControlReleased,false);assert.equal(x.result.status,'release-unverified');
x=await scenario({spaces:[{id:3,ownership:'agent'}],registryError:true});assert.equal(x.result.agentControlReleased,null);assert.equal(x.result.verification,'unavailable');
await assert.rejects(()=>scenario({spaces:[{id:3,ownership:'agent'}],after:[],finishError:true}),/finish failed/);
// Default keep closes only agent-created pages; protect unknown-origin adopted pages.
const defaultSource=source.replace("const requested=\"all\";","const requested=[];");assert.notEqual(defaultSource,source);
const context=vm.createContext({listTaskSpaces:async()=>[{id:3,ownership:'agent'}],taskSpace:async()=>({tabs:async()=>[{label:'p1',openedBy:'agent'},{label:'p2',openedBy:'unknown'}],finish:async options=>assert.deepEqual(JSON.parse(JSON.stringify(options)),{keep:['p2']})})});
await vm.runInContext('(async()=>{'+defaultSource+'})()',context);
console.log('completion SDK: keep-all, protected pages, registry verification, no implicit claim, no other-space cleanup, finish errors passed');
