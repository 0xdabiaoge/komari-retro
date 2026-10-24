import assert from 'node:assert/strict';
import {readFile,writeFile,mkdir} from 'node:fs/promises';
import {createRequire} from 'node:module';
const require=createRequire(import.meta.url),{chromium}=require('../komari-web/node_modules/playwright');
const f=JSON.parse(await readFile(process.argv[2],'utf8'));assert.equal(new URL(f.base).hostname,'127.0.0.1');
const out='.codex/latency-history',rows=[];await mkdir(out,{recursive:true});
const browser=await chromium.launch({headless:true,...(process.env.BROWSER_PATH?{executablePath:process.env.BROWSER_PATH}:{})});
function pass(name,v={}){rows.push({case:name,status:'PASS',...v});console.log(JSON.stringify(rows.at(-1)));}
async function wait(fn,label){const end=Date.now()+45000;while(Date.now()<end){if(await fn())return;await new Promise(r=>setTimeout(r,150));}throw new Error(label);}
function watch(page){const results=[],errors=[];page.on('pageerror',e=>errors.push(e.message));page.on('websocket',ws=>{const calls=new Map();ws.on('framesent',e=>{try{const j=JSON.parse(e.payload);calls.set(j.id,j);}catch{}});ws.on('framereceived',e=>{try{const j=JSON.parse(e.payload),c=calls.get(j.id);if(c?.method==='common:getRecords'&&c.params.type==='ping')results.push({call:c,...j});}catch{}});});page.on('response',async r=>{if(!r.url().endsWith('/rpc2')||r.request().method()!=='POST')return;try{const c=r.request().postDataJSON();if(c?.method==='common:getRecords'&&c.params.type==='ping')results.push({call:c,...await r.json()});}catch{}});return {results,errors};}
async function graph(page,within=page){
 await wait(async()=>{
  const loading=await within.getByText('Loading...',{exact:true}).all();
  if((await Promise.all(loading.map(x=>x.isVisible()))).some(Boolean))return false;
  return within.locator('.recharts-cartesian-grid').evaluateAll(es=>es.some(e=>{const b=e.getBBox();return b.width>150&&b.height>60;}));
 },'latency plot has no visible drawing area');
 assert.ok(await within.locator('path.recharts-line-curve').count()>0);
}
try{
 const owner=await browser.newContext({locale:'zh-CN',viewport:{width:1440,height:1050}}),p=await owner.newPage();
 await p.goto(f.base+f.entry);await p.locator('input[name=username]').fill(f.username);await p.locator('input[name=password]').fill(f.password);await p.locator('button[type=submit]').click();await p.waitForURL('**/admin/dashboard');
 const t=watch(p);await p.goto(f.base+f.view);await p.getByText('Full Day Latency Node',{exact:true}).first().waitFor();
 await wait(()=>t.results.some(r=>!r.error&&r.call.params.hours===24&&r.result?.count>0),'owner full-day statistics absent');
 assert.ok(t.results.every(r=>!r.error));const day=t.results.find(r=>r.call.params.hours===24);assert.equal(day.result.tasks.length,36);assert.ok(day.result.count<=4000);assert.equal(day.result.tasks[0].total,1440);assert.equal(day.result.tasks[0].loss,25);
 await p.screenshot({path:out+'/latency-owner-day.png',fullPage:true});pass('owner-24h-card-full-data-bounded-points',{records:day.result.count,tasks:36});
 await p.locator('[data-node-grid-item] button:has(svg.lucide-trending-up)').first().click();await graph(p,p.getByRole('dialog'));await p.keyboard.press('Escape');pass('owner-floating-latency-curves');
 for(const [kind,hours] of [['day',24],['hour',1]]){
  const c=await browser.newContext({locale:'zh-CN',viewport:{width:1440,height:1050}}),pg=await c.newPage(),tr=watch(pg);
  await pg.goto(f.base+f.shares[kind]);await pg.waitForURL('**/shared');await wait(()=>tr.results.some(r=>!r.error&&r.call.params.hours===hours&&r.result?.count>0),kind+' share missing history');
  assert.ok(tr.results.every(r=>!r.error&&r.call.params.hours<=hours));assert.ok((await pg.locator('body').innerText()).includes('25.0%'));pass('guest-'+kind+'-card-summary');
  await pg.getByRole('button',{name:/^(表格|Table)$/}).click();await pg.getByRole('button',{name:'Expand row',exact:true}).first().click();await graph(pg);pass('guest-'+kind+'-table-curves');
  await pg.goto(f.base+'/shared/instance/'+f.node);await pg.getByRole('tab',{name:/^(延迟|Ping)$/}).click();await graph(pg);
  if(hours===24){await pg.getByRole('tab',{name:/1\s*(天|day)/i}).click();await graph(pg);await wait(()=>tr.results.some(r=>r.call.params.hours===24&&!r.error),'24h detail failed');}
  assert.ok(tr.results.every(r=>!r.error&&r.call.params.hours<=hours));assert.deepEqual(tr.errors,[]);pass('guest-'+kind+'-detail-curves');
  await pg.setViewportSize({width:390,height:844});await graph(pg);await pg.screenshot({path:out+'/guest-'+kind+'-mobile.png',fullPage:true});pass('guest-'+kind+'-mobile-curves');await c.close();
 }
 const lc=await browser.newContext({locale:'zh-CN'}),live=await lc.newPage(),lt=watch(live);await live.goto(f.base+f.shares.live);await live.waitForURL('**/shared');await live.getByText(/未分享延迟历史|History not shared/).first().waitFor();assert.equal(lt.results.length,0);pass('realtime-share-does-not-query-latency-history');await lc.close();
 assert.deepEqual(t.errors,[]);
 const ep=await owner.newPage();let fail=true;
 await ep.addInitScript(()=>{const original=window.setInterval;window.setInterval=(fn,ms,...args)=>original(fn,ms===60000?1000:ms,...args);});
 const isPing=j=>j?.method==='common:getRecords'&&j?.params?.type==='ping';
 const failure=j=>({jsonrpc:'2.0',id:j.id,error:{code:-32603,message:'Synthetic failure',data:'PRIVATE_SQL_CANARY'}});
 await ep.route('**/api/rpc2',async route=>{let j;try{j=route.request().postDataJSON();}catch{}if(fail&&isPing(j))await route.fulfill({json:failure(j)});else await route.continue();});
 await ep.routeWebSocket('**/api/rpc2',ws=>{const server=ws.connectToServer();ws.onMessage(message=>{let j;try{j=JSON.parse(String(message));}catch{}if(fail&&isPing(j))ws.send(JSON.stringify(failure(j)));else server.send(message);});server.onMessage(message=>ws.send(message));});
 await ep.goto(f.base+f.view);await ep.getByText('延迟数据加载失败，将自动重试',{exact:true}).first().waitFor();assert.ok(!(await ep.locator('body').innerText()).includes('PRIVATE_SQL_CANARY'));pass('query-failure-shows-safe-explicit-error');
 fail=false;await wait(async()=>!(await ep.locator('body').innerText()).includes('延迟数据加载失败，将自动重试')&&(await ep.locator('body').innerText()).includes('25.0%'),'automatic history retry did not recover');pass('automatic-retry-recovers-statistics');
 await owner.close();
}catch(e){console.log(JSON.stringify({case:'latency-browser',status:'FAIL',error:e.message}));process.exitCode=1;}
finally{await browser.close();await writeFile(out+'/latency-browser-results.json',JSON.stringify(rows,null,2));}
