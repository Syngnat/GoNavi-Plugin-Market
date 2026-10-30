import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import test from 'node:test';

const html = readFileSync(new URL('./page.html', import.meta.url), 'utf8');
// 页面里有两个脚本块：先注入的轻量词典，再是页面逻辑；测试按顺序依次执行。
const scripts = [...html.matchAll(/<script>([\s\S]*?)<\/script>/g)].map((match) => match[1]);
class Element {
  constructor() { this.textContent = ''; this.value = ''; this.hidden = true; this.disabled = true; this.children = []; this.listeners = {}; }
  appendChild(child) { this.children.push(child); }
  addEventListener(name, callback) { this.listeners[name] = callback; }
}
const tick = () => new Promise(resolve => setImmediate(resolve));
function page(connectionId, invoke) {
  const ids = Object.fromEntries(['conn','db','collection','editor','run','status','error','result'].map(id => [id,new Element()]));
  ids.editor.value = 'db.getCollectionNames()';
  const events = {};
  const window = {
    __gonaviLaunch: {connectionId,connectionName:connectionId},
    gonavi: {call: (method, args) => { assert.equal(method,'plugin.invoke'); return invoke(args); }},
    addEventListener: (name, callback) => { events[name] = callback; },
  };
  const sandbox = {window,document:{getElementById:id=>ids[id],createElement:()=>new Element()},console,navigator:{language:'en-US'}};
  for (const block of scripts) vm.runInNewContext(block, sandbox);
  return {ids, events};
}

test('failed backend connect envelope never makes the real page ready', async () => {
  const p = page('A', async () => ({ok:false,error:{code:'connection_failed',message:'cannot reach A'}}));
  await tick();
  assert.equal(p.ids.run.disabled,true);
  assert.equal(p.ids.db.disabled,true);
  assert.equal(p.ids.status.textContent,'not connected');
  assert.match(p.ids.error.textContent,/cannot reach A/);
});

test('two real pages unwrap data and route run and close to their own session', async () => {
  const calls = [];
  const invoke = async args => {
    calls.push(args);
    const id = args.connectionId;
    if (args.command === 'connect') return {ok:true,data:{sessionId:`secret-${id}`,host:`mongo-${id}`,database:id,databases:[{name:id}],readOnly:id==='A'}};
    assert.equal(args.arguments.sessionId, `secret-${id}`);
    if (args.command === 'run') return {ok:true,data:{output:[id],result:`result-${id}`}};
    return {ok:true,data:{disconnected:true}};
  };
  const a=page('A',invoke),b=page('B',invoke);
  await tick();
  assert.equal(a.ids.conn.textContent,'mongo-A');
  assert.equal(a.ids.db.children.length,1);
  assert.equal(a.ids.status.textContent,'read-only');
  assert.equal(b.ids.status.textContent,'ready');
  await a.ids.run.listeners.click(); await b.ids.run.listeners.click();
  assert.match(a.ids.result.children[0].textContent,/result-A/);
  assert.match(b.ids.result.children[0].textContent,/result-B/);
  a.events.pagehide(); await tick();
  assert.equal(calls.filter(call=>call.command==='disconnect').length,1);
  assert.equal(calls.at(-1).arguments.sessionId,'secret-A');
  await b.ids.run.listeners.click();
  assert.equal(calls.at(-1).arguments.sessionId,'secret-B');
});

test('run failure envelope is shown as failure, never rendered as a result', async () => {
  const p = page('A', async args => args.command==='connect'
    ? {ok:true,data:{sessionId:'secret-A',host:'A',databases:[]}}
    : {ok:false,error:{code:'query_failed',message:'bad query'}});
  await tick(); await p.ids.run.listeners.click();
  assert.equal(p.ids.status.textContent,'failed');
  assert.match(p.ids.error.textContent,/bad query/);
  assert.equal(p.ids.result.children.length,0);
});

test('closing during connect releases the late session without enabling controls', async () => {
  let resolveConnect;
  const calls = [];
  const p = page('A', args => {
    calls.push(args);
    if (args.command === 'connect') return new Promise(resolve => { resolveConnect = resolve; });
    return Promise.resolve({ok:true,data:{disconnected:true}});
  });
  p.events.pagehide();
  resolveConnect({ok:true,data:{sessionId:'late-A',host:'A',databases:[]}});
  await tick();
  assert.equal(p.ids.run.disabled,true);
  assert.equal(calls.at(-1).command,'disconnect');
  assert.equal(calls.at(-1).arguments.sessionId,'late-A');
});

test('missing sessionId and malformed backend responses fail closed', async () => {
  for (const payload of [{ok:true,data:{host:'A'}}, {host:'A'}, null, {ok:true,data:null}]) {
    const p = page('A', async () => payload);
    await tick();
    assert.equal(p.ids.run.disabled,true);
    assert.equal(p.ids.status.textContent,'not connected');
    assert.equal(p.ids.error.hidden,false);
  }
});

test('manifest inline page stays byte-for-byte synchronized', () => {
  const manifest = JSON.parse(readFileSync(new URL('../gonavi.plugin.json',import.meta.url),'utf8'));
  assert.equal(manifest.contributes.pages.find(page=>page.id==='shell').html.source,html);
});
