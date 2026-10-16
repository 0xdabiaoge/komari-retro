import assert from 'node:assert/strict';
import {join} from 'node:path';

export async function testBillingAndPingPresets({owner,ownerContext,rpc,fixture,output}) {
  await owner.setViewportSize({width:1440,height:1100});
  await owner.goto(fixture.base+'/admin/servers');
  async function openBilling(){
    await owner.getByRole('row').filter({hasText:'Visitor Node'}).getByTitle('账单',{exact:true}).click();
    await owner.getByTestId('node-billing-dialog').waitFor();
    return owner.getByTestId('node-billing-dialog');
  }
  async function select(dialog,label,option){await dialog.getByRole('combobox',{name:label,exact:true}).click();await owner.getByRole('option',{name:option,exact:true}).click();}
  const readNode=async()=> (await rpc('admin:listClients')).find(n=>n.uuid===fixture.node);
  let bill=await openBilling();
  await select(bill,'价格模式','自定义付费金额');await bill.getByRole('spinbutton',{name:'付费金额'}).fill('1000');
  await bill.getByRole('combobox',{name:'货币',exact:true}).click();await owner.getByRole('option',{name:/日元 · JPY/}).click();
  await owner.screenshot({path:join(output,'billing-desktop.png'),fullPage:true});
  await bill.getByRole('button',{name:'保存',exact:true}).click();await bill.waitFor({state:'hidden'});
  assert.equal((await readNode()).price,1000);assert.equal((await readNode()).currency,'JPY');
  await owner.reload();bill=await openBilling();assert.ok((await bill.getByRole('combobox',{name:'货币',exact:true}).innerText()).includes('JPY'));
  await bill.getByRole('combobox',{name:'货币',exact:true}).click();const currencies=await owner.getByRole('option').allTextContents();assert.equal(currencies.length,17);await owner.keyboard.press('Escape');
  for(const label of currencies){await select(bill,'货币',label);await bill.getByRole('button',{name:'保存',exact:true}).click();await bill.waitFor({state:'hidden'});assert.equal((await readNode()).currency,label.match(/\b[A-Z]{3}\b/)[0]);bill=await openBilling();}
  await bill.getByRole('combobox',{name:'货币',exact:true}).click();await owner.getByRole('option',{name:/日元 · JPY/}).click();
  await select(bill,'计费周期','年');await bill.locator('#billing-expiry').fill('2030-12-31');await bill.getByRole('switch',{name:'自动续期',exact:true}).check();
  await bill.getByRole('button',{name:'保存',exact:true}).click();await bill.waitFor({state:'hidden'});let savedBill=await readNode();assert.equal(savedBill.billing_cycle,365);assert.equal(savedBill.auto_renewal,true);assert.ok(savedBill.expired_at.startsWith('2030-12-31'));
  await owner.reload();bill=await openBilling();assert.equal(await bill.getByRole('combobox',{name:'计费周期',exact:true}).innerText(),'年');assert.equal(await bill.locator('#billing-expiry').inputValue(),'2030-12-31');
  await bill.getByRole('button',{name:'设置为长期',exact:true}).click();assert.ok(Number((await bill.locator('#billing-expiry').inputValue()).slice(0,4))>=2200);
  await bill.getByRole('combobox',{name:'计费周期',exact:true}).click();
  assert.deepEqual(await owner.getByRole('option').allTextContents(),['天','月','季','年','免费/一次性']);await owner.keyboard.press('Escape');
  for(const [label,value] of [['天',1],['月',30],['季',92],['年',365],['免费/一次性',-1]]){
    await select(bill,'计费周期',label);await bill.getByRole('button',{name:'保存',exact:true}).click();await bill.waitFor({state:'hidden'});assert.equal((await readNode()).billing_cycle,value);await owner.reload();bill=await openBilling();assert.equal(await bill.getByRole('combobox',{name:'计费周期',exact:true}).innerText(),label);
  }
  await bill.getByRole('button',{name:'取消',exact:true}).click();await rpc('admin:editClient',{uuid:fixture.node,billing_cycle:730});await owner.reload();bill=await openBilling();
  await bill.getByRole('button',{name:'保存',exact:true}).click();await owner.getByText('请选择有效计费周期',{exact:true}).waitFor();assert.equal((await readNode()).billing_cycle,730);
  await select(bill,'计费周期','月');

  await select(bill,'价格模式','免费');await bill.getByRole('button',{name:'保存',exact:true}).click();await bill.waitFor({state:'hidden'});assert.equal((await readNode()).price,-1);
  await owner.reload();bill=await openBilling();await select(bill,'价格模式','隐藏价格');
  await bill.getByRole('button',{name:'保存',exact:true}).click();await bill.waitFor({state:'hidden'});assert.equal((await readNode()).price,0);
  await rpc('admin:editClient',{uuid:fixture.node,currency:'legacy-unit'});await owner.reload();bill=await openBilling();
  assert.ok((await bill.getByRole('combobox',{name:'货币',exact:true}).innerText()).includes('legacy-unit'));
  await bill.getByRole('button',{name:'保存',exact:true}).click();await bill.waitFor({state:'hidden'});assert.equal((await readNode()).currency,'legacy-unit');
  await owner.reload();bill=await openBilling();await select(bill,'价格模式','自定义付费金额');await bill.getByRole('spinbutton',{name:'付费金额'}).fill('19.99');
  const route='**/api/admin/client/'+fixture.node+'/edit';await owner.route(route,r=>r.fulfill({status:500,contentType:'application/json',body:JSON.stringify({message:'隔离保存失败测试'})}));
  await bill.getByRole('button',{name:'保存',exact:true}).click();await owner.getByText('隔离保存失败测试',{exact:true}).waitFor();assert.equal(await bill.isVisible(),true);assert.equal((await readNode()).price,0);await owner.unroute(route);
  await owner.setViewportSize({width:390,height:844});assert.ok(await owner.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  await owner.screenshot({path:join(output,'billing-mobile.png'),fullPage:true});
  await bill.getByRole('button',{name:'取消',exact:true}).click();await rpc('admin:editClient',{uuid:fixture.node,price:0,currency:'USD',billing_cycle:30,expired_at:null,auto_renewal:false});
  await owner.setViewportSize({width:1440,height:1100});await owner.goto(fixture.base+'/admin/ping');
  async function openPresets(){await owner.getByRole('button',{name:'批量添加预设',exact:true}).click();const d=owner.getByTestId('ping-preset-dialog');await d.waitFor();await d.getByRole('checkbox',{name:'全选当前地区目标',exact:true}).waitFor();return d;}
  let dialog=await openPresets();await dialog.getByRole('checkbox',{name:'北京电信 · V4',exact:true}).waitFor({timeout:30000});
  assert.ok((await dialog.innerText()).includes('当前地区（186）'));
  assert.equal(await dialog.getByRole('textbox',{name:'搜索探测预设'}).count(),0);assert.equal(await dialog.getByRole('combobox',{name:'IP 协议'}).count(),0);assert.equal(await dialog.getByRole('combobox',{name:'运营商',exact:true}).count(),0);
  await dialog.getByRole('checkbox',{name:'选择北京IPv6全部目标',exact:true}).check();
  assert.equal(await dialog.getByRole('checkbox',{name:'北京联通 · V6',exact:true}).isChecked(),true);
  await dialog.getByRole('checkbox',{name:'选择北京IPv6全部目标',exact:true}).uncheck();
  assert.equal(await dialog.getByRole('checkbox',{name:'北京联通 · V6',exact:true}).isChecked(),false);
  await dialog.getByRole('checkbox',{name:'北京电信 · V4',exact:true}).check();
  await dialog.getByRole('checkbox',{name:'北京电信 · V6',exact:true}).check();
  const row=dialog.getByTestId('preset-cn-bj-ct-v6');await row.getByRole('button',{name:/自定义编辑/}).click();
  await row.getByRole('textbox',{name:'探测地址',exact:true}).fill('bad-target');await row.getByRole('button',{name:'应用修改并选中'}).click();await row.getByRole('alert').waitFor();
  await row.getByRole('textbox',{name:'探测名称',exact:true}).fill('自定义北京IPv6');await row.getByRole('textbox',{name:'探测地址',exact:true}).fill('[2001:db8::1]:443');await row.getByRole('button',{name:'应用修改并选中'}).click();
  assert.equal(await row.getByRole('checkbox',{name:'自定义北京IPv6'}).isChecked(),true);
  await row.getByRole('button',{name:/自定义编辑/}).click();await row.getByRole('button',{name:'恢复默认',exact:true}).click();
  assert.equal(await row.getByRole('checkbox',{name:'北京电信 · V6',exact:true}).isChecked(),true);assert.ok((await row.innerText()).includes('bj-ct-v6.ip.zstaticcdn.com:80'));
  await row.getByRole('button',{name:/自定义编辑/}).click();await row.getByRole('textbox',{name:'探测名称',exact:true}).fill('未应用的名称');await row.getByRole('button',{name:'取消编辑',exact:true}).click();assert.equal(await row.getByRole('checkbox',{name:'北京电信 · V6',exact:true}).count(),1);
  await row.getByRole('button',{name:/自定义编辑/}).click();await row.getByRole('textbox',{name:'探测名称',exact:true}).fill('自定义北京IPv6');await row.getByRole('textbox',{name:'探测地址',exact:true}).fill('[2001:db8::1]:443');await row.getByRole('button',{name:'应用修改并选中'}).click();
  await dialog.getByRole('button',{name:'Google / Cloudflare',exact:true}).click();
  await dialog.getByRole('checkbox',{name:'Google',exact:true}).check();await dialog.getByRole('checkbox',{name:'Cloudflare',exact:true}).check();
  await dialog.getByRole('button',{name:'海外地区',exact:true}).click();await dialog.getByRole('checkbox',{name:'香港',exact:true}).check();
  const customHTTP=dialog.getByTestId('preset-hk-ipv4');await customHTTP.getByRole('button',{name:/自定义编辑/}).click();await customHTTP.getByRole('textbox',{name:'探测名称',exact:true}).fill('香港自定义HTTP');await select(customHTTP,'探测方式','HTTP');await customHTTP.getByRole('textbox',{name:'探测地址',exact:true}).fill('https://example.com/Status?view=HK');await customHTTP.getByRole('button',{name:'应用修改并选中'}).click();
  await dialog.getByRole('button',{name:'选择服务器',exact:true}).click();
  let selector=owner.locator('.km-node-selector-dialog');await selector.getByRole('row').filter({hasText:'Visitor Node'}).getByRole('checkbox').check();await selector.getByRole('button',{name:'完毕',exact:true}).click();
  await dialog.getByRole('button',{name:'中国大陆',exact:true}).click();assert.equal(await dialog.getByRole('checkbox',{name:'自定义北京IPv6'}).isChecked(),true);
  await owner.screenshot({path:join(output,'ping-presets-desktop.png'),fullPage:true});
  await dialog.getByRole('button',{name:'保存所选预设（5）',exact:true}).click();await dialog.waitFor({state:'hidden'});
  let tasks=await rpc('admin:getAllPingTasks');assert.equal(tasks.length,5);
  assert.ok(tasks.some(t=>t.type==='tcp'&&t.target==='[2001:db8::1]:443'&&t.name==='自定义北京IPv6'));
  assert.ok(tasks.some(t=>t.type==='http'&&t.target==='https://example.com/Status?view=HK'&&t.name==='香港自定义HTTP'));
  assert.ok(tasks.some(t=>t.type==='icmp'&&t.target==='1.1.1.1'));assert.ok(tasks.every(t=>t.clients.includes(fixture.node)));
  const google=tasks.find(t=>t.target==='8.8.8.8');await rpc('admin:editPingTask',{tasks:[{...google,name:'保留自定义谷歌名称',interval:120}]});
  await owner.reload();dialog=await openPresets();await dialog.getByRole('button',{name:'Google / Cloudflare',exact:true}).click();
  await dialog.getByRole('checkbox',{name:'Google',exact:true}).check();
  await dialog.getByRole('button',{name:'选择服务器',exact:true}).click();selector=owner.locator('.km-node-selector-dialog');await selector.getByRole('row').filter({hasText:'Hidden Operations Canary'}).getByRole('checkbox').check();await selector.getByRole('button',{name:'完毕',exact:true}).click();
  await dialog.locator('#preset-interval').fill('30');await dialog.getByRole('button',{name:'保存所选预设（1）',exact:true}).click();await dialog.waitFor({state:'hidden'});
  tasks=await rpc('admin:getAllPingTasks');assert.equal(tasks.length,5);const merged=tasks.find(t=>t.target==='8.8.8.8');assert.equal(merged.name,'保留自定义谷歌名称');assert.equal(merged.interval,120);assert.equal(new Set(merged.clients).size,2);
  await owner.reload();dialog=await openPresets();await dialog.getByRole('checkbox',{name:'北京联通 · V4',exact:true}).check();
  await dialog.getByRole('checkbox',{name:'新加入的服务器默认启用这些目标',exact:true}).check();
  await owner.setViewportSize({width:390,height:844});assert.ok(await owner.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  await owner.screenshot({path:join(output,'ping-presets-mobile.png'),fullPage:true});
  await dialog.getByRole('button',{name:'保存所选预设（1）',exact:true}).click();await dialog.waitFor({state:'hidden'});
  tasks=await rpc('admin:getAllPingTasks');const future=tasks.find(t=>t.target==='bj-cu-v4.ip.zstaticcdn.com:80');assert.ok(future.default_on);assert.deepEqual(future.clients,[]);
  const invalid=await ownerContext.request.post(fixture.base+'/api/rpc2',{data:{jsonrpc:'2.0',id:1,method:'admin:importPingPresets',params:{preset_ids:['cn-bj-cm-v4','invalid-fixture-id'],clients:[fixture.node],interval:60}}});assert.ok((await invalid.json()).error);assert.equal((await rpc('admin:getAllPingTasks')).length,6);
  // Closing the dialog discards edits and selections without touching saved tasks.
  dialog=await openPresets();await dialog.getByRole('button',{name:'中国大陆',exact:true}).click();const cancelledRow=dialog.getByTestId('preset-cn-bj-cm-v4');await cancelledRow.getByRole('button',{name:/自定义编辑/}).click();await cancelledRow.getByRole('textbox',{name:'探测名称',exact:true}).fill('取消整批时丢弃');await cancelledRow.getByRole('button',{name:'应用修改并选中'}).click();await dialog.getByRole('button',{name:'取消',exact:true}).click();
  dialog=await openPresets();assert.equal(await dialog.getByRole('checkbox',{name:'取消整批时丢弃',exact:true}).count(),0);assert.equal(await dialog.getByRole('checkbox',{name:'北京移动 · V4',exact:true}).isChecked(),false);await dialog.getByRole('button',{name:'取消',exact:true}).click();assert.equal((await rpc('admin:getAllPingTasks')).length,6);
  // Verify the public compact card uses the same hidden/free/paid semantics.
  const settingsURL=fixture.base+'/api/admin/theme/settings?theme=retro';
  assert.ok((await ownerContext.request.post(settingsURL,{data:{cardLayout:'compact',guestDisplay:{showPrice:true,showExpiredAt:false}}})).ok());
  await rpc('admin:editSettings',{theme:'retro'});
  await rpc('admin:editClient',{uuid:fixture.node,price:1000,currency:'JPY'});
  const guestContext=await ownerContext.browser().newContext({locale:'en-US'});
  try {
    const guest=await guestContext.newPage();await guest.goto(fixture.base+'/s/'+fixture.share);
    await guest.getByText(/JP¥1000\//).waitFor();
    await rpc('admin:editClient',{uuid:fixture.node,price:-1});await guest.reload();await guest.getByText('Free',{exact:true}).waitFor();
    await rpc('admin:editClient',{uuid:fixture.node,price:0});await guest.reload();await guest.getByText('Visitor Node',{exact:true}).first().waitFor();
    assert.equal(await guest.getByText('Free',{exact:true}).count(),0);assert.equal(await guest.getByText(/JP¥1000/).count(),0);
  } finally {await guestContext.close();}
  assert.ok((await ownerContext.request.post(settingsURL,{data:{}})).ok());
  await rpc('admin:editClient',{uuid:fixture.node,price:0,currency:'USD',billing_cycle:30,expired_at:null,auto_renewal:false});
  await owner.setViewportSize({width:1440,height:1000});
  return [{case:'billing-five-fixed-cycles-all-currencies-expiry-auto-renewal',status:'PASS',currencies:17},{case:'billing-price-modes-currency-legacy-failure-mobile',status:'PASS'},{case:'ping-presets-region-families-custom-edit-reset-cancel-atomic-merge-mobile',status:'PASS',catalog_entries:210,mainland:186,external_probes_sent:0},{case:'retro-compact-guest-price-hidden-free-paid',status:'PASS'}];
}
