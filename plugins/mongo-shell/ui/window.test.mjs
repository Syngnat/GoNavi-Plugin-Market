import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import test from 'node:test';

class Element {
  constructor() {
    this.textContent=''; this.value=''; this.hidden=true; this.disabled=true;
    this.children=[]; this.listeners={}; this.dataset={};
    this.classList={add(){},remove(){},toggle(){}};
  }
  appendChild(child) { this.children.push(child); }
  addEventListener(name,callback) {this.listeners[name]=callback;}
  focus() {}
}
const tick=()=>new Promise(resolve=>setImmediate(resolve));
const sources = ['i18n.js','bridge.js','render.js','app.js'].map(name => readFileSync(new URL(name,import.meta.url),'utf8')
  // Run the actual module bodies in an isolated browser global, stripping only
  // module linkage. No command, state, lifecycle or renderer code is replaced.
  .replace(/^import\s*\{[\s\S]*?\}\s*from\s*'[^']+';\s*/gm,'')
  .replace(/^export /gm,''));
function windowApp(id,invoke) {
  const elements={}; const events={};
  const get=id=>elements[id] ||= new Element();
  get('max-rows').value='200';
  const bridge={ready:Promise.resolve({context:{kind:'connectionAction',connectionId:id,connectionName:id,connectionType:'mongodb',database:id,config:{type:'mongodb',host:id}}}),invoke};
  const window={gonaviPlugin:bridge,addEventListener:(name,callback)=>{events[name]=callback;}};
  const context=vm.createContext({window,gonaviPlugin:bridge,document:{getElementById:get,querySelector:get,createElement:()=>new Element(),createDocumentFragment:()=>new Element()},console});
  for (const source of sources) vm.runInContext(source,context);
  return {elements,events};
}

test('two native windows route collections, run, ping, disconnect and reconnect with independent IDs',async()=>{
  const calls=[]; const counts={};
  const app=id=>windowApp(id,async(command,args)=>{
    calls.push({id,command,args});
    if(command==='connect') {
      counts[id]=(counts[id]||0)+1;
      return {ok:true,data:{sessionId:`${id}-${counts[id]}`,host:id,database:id,databases:[{name:id}]}};
    }
    assert.equal(args.sessionId,`${id}-${counts[id]}`);
    if(command==='run')return {ok:true,data:{database:id,output:[id],result:['only-'+id]}};
    return {ok:true,data:{latencyMs:1,disconnected:true}};
  });
  const a=app('A'),b=app('B'); await tick();
  assert.equal(a.elements['run-button'].disabled,false);
  assert.equal(b.elements['run-button'].disabled,false);
  a.elements.editor.value='db.getCollectionNames()';
  a.elements['run-button'].listeners.click();
  b.elements['ping-button'].listeners.click();await tick();
  assert.match(a.elements.output.textContent,/only-A/);
  assert.match(b.elements['status-text'].textContent,/1 ms/);
  a.elements['connect-button'].listeners.click();await tick();
  assert.equal(a.elements['run-button'].disabled,true);
  assert.equal(b.elements['run-button'].disabled,false);
  a.elements['connect-button'].listeners.click();await tick();
  assert.equal(calls.filter(c=>c.id==='A'&&c.command==='connect').length,2);
  a.events.pagehide();b.events.pagehide();await tick();
  assert.equal(calls.filter(c=>c.command==='disconnect').length,3);
});

test('native window failed connect envelope never enables commands',async()=>{
  const a=windowApp('A',async()=>({ok:false,error:{code:'connection_failed',message:'unreachable A'}}));
  await tick();
  assert.equal(a.elements['run-button'].disabled,true);
  assert.match(a.elements['error-message'].textContent,/unreachable A/);
});

test('native window releases a connect response arriving after pagehide',async()=>{
  let resolveConnect;const calls=[];
  const a=windowApp('A',(command,args)=>{
    calls.push({command,args});
    if(command==='connect')return new Promise(resolve=>{resolveConnect=resolve;});
    return Promise.resolve({ok:true,data:{disconnected:true}});
  });
  await tick();a.events.pagehide();
  resolveConnect({ok:true,data:{sessionId:'late-A',database:'A',databases:[]}});
  await tick();
  assert.equal(a.elements['run-button'].disabled,true);
  assert.equal(calls.at(-1).command,'disconnect');
  assert.equal(calls.at(-1).args.sessionId,'late-A');
});
