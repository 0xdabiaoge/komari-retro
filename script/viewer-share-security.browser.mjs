import assert from 'node:assert/strict';

export async function testViewerShares({owner,ownerContext,rpc,fixture,browser,output}) {
  await owner.goto(fixture.base+'/admin/settings/site');
  await owner.getByText('独立授权分享',{exact:true}).click();
  await owner.getByLabel('分享名称').fill('浏览器范围验收');
  await owner.getByText('Visitor Node',{exact:true}).last().click();
  await owner.getByRole('button',{name:'生成独立分享链接',exact:true}).click();
  const link=owner.getByRole('textbox',{name:'新分享链接'});
  await link.waitFor();
  const url=await link.inputValue();
  assert.ok(url.startsWith(fixture.base+'/s/'));
  const grants=await rpc('admin:listViewerShares');
  const grant=grants.find(item=>item.name==='浏览器范围验收');
  assert.ok(grant && grant.nodes.length===1 && grant.nodes[0]===fixture.node);
  assert.ok(!JSON.stringify(grants).includes('token_hash'));
  const guestContext=await browser.newContext({locale:'zh-CN'});
  const guest=await guestContext.newPage();
  await guest.goto(url);
  await guest.waitForURL(fixture.base+'/shared');
  await guest.getByText('Visitor Node',{exact:true}).first().waitFor();
  assert.ok(!guest.url().includes('/s/'));
  assert.equal(await guest.locator('input[type=password]').count(),0);
  const cookie=(await guestContext.cookies()).find(c=>c.name==='temp_key');
  assert.ok(cookie?.httpOnly);
  const data=await guestContext.request.post(fixture.base+'/viewer-api/rpc2',{data:{jsonrpc:'2.0',id:1,method:'common:getNodes',params:{}}}).then(r=>r.json());
  assert.ok(data.result[fixture.node]);
  assert.equal(Object.keys(data.result).length,1);
  assert.ok(!('price' in data.result[fixture.node]) && !('expired_at' in data.result[fixture.node]));
  await owner.screenshot({path:output+'/independent-share-management.png',fullPage:true});
  await owner.getByRole('button',{name:'撤销',exact:true}).last().click();
  await owner.getByText('已撤销',{exact:true}).first().waitFor();
  assert.equal((await guestContext.request.get(fixture.base+'/shared')).status(),404);
  const revoked=await guestContext.request.post(fixture.base+'/viewer-api/rpc2',{data:{jsonrpc:'2.0',id:1,method:'common:getNodes',params:{}}}).then(r=>r.json());
  assert.ok(revoked.error);
  await guestContext.close();
  return [{case:'independent-share-real-ui-create-scope-cookie-clean-url-revoke',status:'PASS'}];
}
