import { chromium } from 'playwright';
import assert from 'node:assert/strict';
let input=''; for await (const chunk of process.stdin) input+=chunk;
const f=JSON.parse(input);input='';
const browser=await chromium.launch({headless:true});
const context=await browser.newContext({locale:'zh-CN',viewport:{width:1440,height:1100}});
await context.addInitScript(()=>localStorage.setItem('language','zh-CN'));
const page=await context.newPage(),errors=[];
page.on('pageerror',e=>errors.push(e.message));
try {
 await context.request.get(f.base+f.entry);
 const login=await context.request.post(f.base+'/api/login',{data:{username:f.username,password:f.password}});
 assert.equal(login.status(),200);
 const pending=new Set(['admin:getDashboardMetrics','admin:getDashboardPing']);
 const responses=new Map();
 page.on('response',async res=>{
  if(!res.url().endsWith('/api/rpc2'))return;
  let req;try{req=res.request().postDataJSON();}catch{return;}
  if(!pending.has(req?.method))return;
  const value=await res.json();responses.set(req.method,value);pending.delete(req.method);
 });
 await page.goto(f.base+'/admin',{waitUntil:'domcontentloaded'});
 await page.waitForFunction(()=>document.querySelector('.km-page-admin-dashboard'));
 for(let i=0;i<100 && pending.size;i++)await page.waitForTimeout(100);
 assert.equal(pending.size,0);for(const value of responses.values())assert.ok(!value.error);
 const metrics=responses.get('admin:getDashboardMetrics').result,ping=responses.get('admin:getDashboardPing').result;
 assert.equal(metrics.cpu.length,100);assert.equal(metrics.traffic.points.length,120);assert.equal(ping.count,18000);
 assert.equal(ping.stable.length,100);assert.equal(ping.loss[0].loss,100);
 await page.locator('.km-dashboard-chart .recharts-surface').first().waitFor();
 assert.equal(await page.getByRole('alert').count(),0);
 console.log('PASS 500-node dashboard, compact metrics, exact latency rankings and rendered traffic chart');
 const report={cpu:{usage:25},ram:{used:512,total:1024},network:{up:100,down:200,totalUp:1000,totalDown:2000},uptime:3600,updated_at:new Date().toISOString()};
 for(let first=1;first<=500;first+=50) {
  // Deliberately retain the owner cookie: valid Agent protocol submissions
  // must use Agent capacity even in a browser-authenticated context.
  await Promise.all(Array.from({length:50},async(_,offset)=>{
   const id=first+offset;
   const response=await context.request.post(f.base+'/api/clients/v2/rpc?token=fixture-token-'+id,{data:{jsonrpc:'2.0',id:'fixture-'+id,method:'agent.report',params:{report}}});
   assert.equal(response.status(),200);assert.ok(!(await response.json()).error);
  }));
 }
 const latest=await page.evaluate(async()=>{const r=await fetch('/api/rpc2',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({jsonrpc:'2.0',id:1,method:'common:getNodesLatestStatus'})});return r.json();});
 assert.ok(!latest.error);assert.equal(Object.values(latest.result).filter(n=>n.online).length,500);
 console.log('PASS 500 authenticated v2 Agent reports at concurrency 50, all 500 nodes online');
 for(const method of ['admin:getDashboardMetrics','admin:getDashboardPing']) {
  let fail=true;
  await page.route('**/api/rpc2',async route=>{
   const req=route.request().postDataJSON();
   if(fail&&req?.method===method){fail=false;await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({jsonrpc:'2.0',id:req.id,error:{code:-32603,message:'isolated injected statistics failure'}})});return;}
   await route.continue();
  });
  await page.reload({waitUntil:'domcontentloaded'});
  await page.getByRole('alert').filter({hasText:'isolated injected statistics failure'}).waitFor();
  await page.getByRole('button',{name:'重试失败的统计',exact:true}).click();
  await page.getByRole('alert').filter({hasText:'isolated injected statistics failure'}).waitFor({state:'detached'});
  await page.locator('.km-dashboard-chart .recharts-surface').first().waitFor();
  await page.unroute('**/api/rpc2');
  console.log('PASS independent failure, finished loading state and retry: '+method);
 }
 await page.goto(f.base+'/admin/settings/site',{waitUntil:'domcontentloaded'});
 await page.getByText('数据库空间与历史保留',{exact:true}).waitFor();
 await page.getByText(/数据库文件：/).waitFor();
 assert.equal(await page.getByRole('alert').count(),0);
 assert.equal(errors.length,0,errors.join('\n'));
 console.log('PASS database allocation/reusable space/reclamation status and no browser errors');
} finally {await context.close();await browser.close();}
