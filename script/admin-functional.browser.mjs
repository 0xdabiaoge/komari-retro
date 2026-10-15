import assert from 'node:assert/strict';

async function waitForValue(read, expected) {
  for (let i = 0; i < 40; i++) {
    if (await read() === expected) return;
    await new Promise(resolve => setTimeout(resolve, 150));
  }
  assert.equal(await read(), expected);
}

async function dismissFixtureOnboarding({ ownerContext, rpc, fixture }) {
  const current = await rpc('public:getPublicSettings');
  assert.ok((await ownerContext.request.post(fixture.base + '/api/admin/theme/settings?theme=' + current.theme, { data: {
    ...current.theme_settings,
    _komari_onboarding_v1: { seen: ['install', 'workbench', 'notifications', 'markets'], workbenchOpened: true },
  } })).ok());
}

export async function testProfileAndTheme({ owner, ownerContext, rpc, fixture }) {
  await dismissFixtureOnboarding({ ownerContext, rpc, fixture });
  const rows = [], me = await ownerContext.request.get(fixture.base + '/api/me').then(r => r.json());
  const name = 'acceptance-renamed-owner';
  try {
    await owner.goto(fixture.base + '/admin/account');
    await owner.locator('input[name="username"]').fill(name);
    await owner.locator('.km-account-profile-form button[type="submit"]').click();
    await owner.getByText(new RegExp(name + '[，,]')).waitFor();
    rows.push({ case: 'profile-name-refreshes-without-reload', status: 'PASS' });
  } finally {
    assert.ok((await ownerContext.request.post(fixture.base + '/api/admin/update/user', { data: { uuid: me.uuid, username: me.username } })).ok());
  }
  for (const password of ['Aa1abc', 'Aa1中文', 'alllowercase2026', 'Aa1' + 'x'.repeat(4094)]) {
    assert.equal((await ownerContext.request.post(fixture.base + '/api/admin/update/user', { data: { uuid: me.uuid, password } })).status(), 400);
  }
  assert.ok((await ownerContext.request.get(fixture.base + '/api/me').then(r => r.json())).logged_in);
  rows.push({ case: 'profile-invalid-password-refusal-preserves-session', status: 'PASS' });
  const originalTheme = (await rpc('admin:getSettings')).theme;
  await rpc('admin:editSettings', { theme: 'retro' });
  const original = (await rpc('public:getPublicSettings')).theme_settings;
  try {
    const nested = { ...original, guestDisplay: { showPrice: true, showExpiredAt: true }, statusCardsVisibility: { currentTime: false }, _komari_onboarding_v1: { seen: ['install', 'workbench', 'notifications', 'markets'], workbenchOpened: true } };
    assert.ok((await ownerContext.request.post(fixture.base + '/api/admin/theme/settings?theme=retro', { data: nested })).ok());
    await owner.goto(fixture.base + '/admin/theme_managed');
    const price = owner.locator('[id="guestDisplay.showPrice"]').getByRole('switch');
    await price.waitFor();
    assert.equal(await price.getAttribute('aria-checked'), 'true');
    assert.equal(await owner.locator('[id="statusCardsVisibility.currentTime"]').getByRole('switch').getAttribute('aria-checked'), 'false');
    await price.click();
    await owner.getByRole('button', { name: /^(Save|保存)$/ }).first().click();
    await waitForValue(async () => (await rpc('public:getPublicSettings')).theme_settings.guestDisplay.showPrice, false);
    await owner.reload();
    await price.waitFor();
    assert.equal(await price.getAttribute('aria-checked'), 'false');
    rows.push({ case: 'retro-nested-settings-load-save-reload', status: 'PASS' });
  } finally {
    assert.ok((await ownerContext.request.post(fixture.base + '/api/admin/theme/settings?theme=retro', { data: original })).ok());
    await rpc('admin:editSettings', { theme: originalTheme });
  }
  return rows;
}

export async function testNodeManagement({ owner, ownerContext, rpc, fixture }) {
  await dismissFixtureOnboarding({ ownerContext, rpc, fixture });
  const rows = [], name = 'UI验收节点', updated = name + '修改';
  let uuid;
  const add = () => owner.locator('[data-guide="add-node"]');
  const dialog = () => owner.getByRole('dialog');
  const row = label => owner.locator('tr').filter({ has: owner.getByText(label, { exact: true }) });
  try {
    await owner.goto(fixture.base + '/admin/servers');
    await add().click();
    await dialog().locator('input').first().fill(name);
    await dialog().getByRole('button', { name: /^(Add|添加节点)$/ }).first().click();
    await dialog().waitFor({ state: 'hidden' });
    await row(name).waitFor();
    const node = (await rpc('admin:listClients')).find(item => item.name === name);
    assert.ok(node); uuid = node.uuid;
    await row(name).locator('button[title]').filter({ has: owner.locator('svg.lucide-pencil') }).click();
    await dialog().locator('input').nth(0).fill(updated);
    await dialog().locator('input').nth(2).fill('acceptance;中文标签');
    await dialog().locator('input').nth(3).fill('验收分组');
    await dialog().locator('textarea').nth(0).fill('private canary');
    await dialog().locator('textarea').nth(1).fill('public acceptance remark');
    await dialog().getByRole('button', { name: /^(Save|保存)$/ }).last().click();
    await dialog().waitFor({ state: 'hidden' });
    await row(updated).waitFor();
    const saved = await rpc('admin:getClient', { uuid });
    assert.equal(saved.group, '验收分组'); assert.equal(saved.remark, 'private canary');
    assert.equal(saved.public_remark, 'public acceptance remark');
    rows.push({ case: 'node-ui-add-edit-tags-group-remarks', status: 'PASS' });
    await row(updated).locator('button[title]').filter({ has: owner.locator('svg.lucide-download') }).click();
    for (const [platform, pattern] of [['Linux', /bash -s --/], ['Windows', /powershell/i], ['Docker', /docker run/]]) {
      await dialog().getByRole('radio', { name: platform, exact: true }).click();
      assert.match(await dialog().locator('textarea').first().inputValue(), pattern);
    }
    await owner.keyboard.press('Escape');
    rows.push({ case: 'node-ui-linux-windows-docker-install-commands', status: 'PASS', scope: 'command generation; no target OS installation' });
    const removePath = '**/api/admin/client/' + uuid + '/remove';
    await owner.route(removePath, route => route.fulfill({ status: 503, contentType: 'application/json', body: '{"message":"acceptance backend unavailable"}' }));
    await row(updated).locator('button[title]').filter({ has: owner.locator('svg.lucide-trash-2,svg.lucide-trash2') }).click();
    await dialog().getByRole('button', { name: /^(Confirm delete|确认删除)$/ }).click();
    await owner.getByText('Error: HTTP 503', { exact: true }).waitFor();
    assert.ok(await dialog().isVisible());
    assert.ok((await rpc('admin:listClients')).some(item => item.uuid === uuid));
    await owner.unroute(removePath);
    await dialog().getByRole('button', { name: /^(Confirm delete|确认删除)$/ }).click();
    await dialog().waitFor({ state: 'hidden' });
    await waitForValue(async () => (await rpc('admin:listClients')).some(item => item.uuid === uuid), false);
    uuid = null;
    rows.push({ case: 'node-ui-delete-rejects-http-failure-then-real-delete', status: 'PASS', injected_failure: 'HTTP 503 response only' });
    await add().click();
    await dialog().locator('input').first().fill('must retain input');
    await owner.route('**/api/admin/client/add', route => route.fulfill({ status: 503, contentType: 'application/json', body: '{}' }));
    await dialog().getByRole('button', { name: /^(Add|添加节点)$/ }).first().click();
    await owner.getByText(/HTTP 503/).last().waitFor();
    assert.ok(await dialog().isVisible());
    assert.equal(await dialog().locator('input').first().inputValue(), 'must retain input');
    rows.push({ case: 'node-ui-add-failure-preserves-dialog-input', status: 'PASS', injected_failure: 'HTTP 503 response only' });
  } finally {
    await owner.unroute('**/api/admin/client/add');
    if (uuid) { await owner.unroute('**/api/admin/client/' + uuid + '/remove'); await rpc('admin:removeClient', { uuid }); }
    await owner.keyboard.press('Escape');
  }
  return rows;
}
