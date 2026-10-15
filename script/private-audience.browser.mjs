import assert from 'node:assert/strict';
import { readFile, mkdir, writeFile } from 'node:fs/promises';
import { resolve, join } from 'node:path';
import { createRequire } from 'node:module';
const require = createRequire(import.meta.url);
const { chromium } = require('../komari-web/node_modules/playwright');
const fixture = JSON.parse(await readFile(process.env.PRIVATE_AUDIENCE_FIXTURE, 'utf8'));
assert.equal(new URL(fixture.base).hostname, '127.0.0.1');
assert.ok(Number(new URL(fixture.base).port) >= 25980 && Number(new URL(fixture.base).port) < 25990);
const output = resolve(import.meta.dirname, '../.codex/audience-browser');
await mkdir(output, { recursive: true });
const browser = await chromium.launch({ headless: true, ...(process.env.BROWSER_PATH ? { executablePath: process.env.BROWSER_PATH } : {}) });
const rows = [];
const forbidden = /\/api\/(?:login|logout|me|admin)\b|session_token|admin_entrance_token|logged_in|LoginDialog|loginRedirect|登录|登入|密码|密碼|\blog[ -]?in\b|\bsign[ -]?in\b/i;
const ownerContext = await browser.newContext({ locale: 'en-US' });
const owner = await ownerContext.newPage();
async function rpc(method, params = {}) {
  const response = await ownerContext.request.post(fixture.base + '/api/rpc2', { data: { jsonrpc: '2.0', id: 1, method, params } });
  const value = await response.json(); assert.ok(!value.error, method); return value.result;
}
try {
  // Real UI password flow, including its gated assets; no session is injected.
  await owner.goto(fixture.base + fixture.entry);
  await owner.locator('input[name="username"]').fill(fixture.username);
  await owner.locator('input[name="password"]').fill(fixture.password);
  await owner.locator('button[type="submit"]').click();
  await owner.waitForURL('**/admin/dashboard', { timeout: 30000 });
  assert.equal((await rpc('admin:listClients')).length, 2);
  rows.push({ case: 'real-owner-password-entry-and-management', status: 'PASS' });
  for (const theme of ['default', 'retro']) {
    await rpc('admin:editSettings', { theme });
    const context = await browser.newContext({ locale: 'en-US', viewport: { width: 1440, height: 1000 } });
    const page = await context.newPage();
    const failures = [], bodies = [], urls = [], sockets = [];
    page.on('websocket', socket => {
      sockets.push(socket.url());
      socket.on('framereceived', frame => {
        const text = String(frame.payload);
        assert.ok(!forbidden.test(text) && !text.includes(fixture.hidden), 'privileged websocket data reached viewer');
      });
    });
    page.on('pageerror', error => failures.push(error.message));
    page.on('response', response => {
      if (new URL(response.url()).origin !== fixture.base) return;
      urls.push(response.url());
      const contentType = response.headers()['content-type'] || '';
      if (/javascript|json|text\/html|text\/css/.test(contentType) && response.status() === 200) bodies.push(response.text().then(text => {
        assert.ok(!forbidden.test(text), `privileged content received: ${new URL(response.url()).pathname}`);
      }));
    });
    assert.equal((await context.request.get(fixture.base + '/')).status(), 404);
    assert.equal((await context.request.get(fixture.base + '/admin/login')).status(), 404);
    await page.goto(fixture.base + '/s/' + fixture.share);
    await page.getByText('Visitor Node', { exact: true }).first().waitFor({ timeout: 30000 });
    assert.ok(!(await page.locator('body').innerText()).includes('admin.'));
    await page.evaluate(async () => { const cache = await caches.open('workbox-historical-fixture'); await cache.put('/historical-fixture.js', new Response('historical fixture')); });
    await page.reload();
    await page.waitForFunction(async () => !(await caches.keys()).some(name => name.startsWith('workbox-')));
    await page.getByText('Visitor Node', { exact: true }).first().waitFor();
    assert.equal(await page.getByText('Hidden Operations Canary', { exact: true }).count(), 0);
    assert.equal(await page.locator('input[type="password"], input[autocomplete="username"]').count(), 0);
    assert.equal(await page.getByRole('button', { name: /log.?in|sign.?in|登录|登入/i }).count(), 0);
    const cookies = await context.cookies();
    assert.ok(cookies.find(cookie => cookie.name === 'temp_key')?.httpOnly);
    await page.screenshot({ path: join(output, theme + '-desktop.png'), fullPage: true });
    const search = page.getByPlaceholder(/search/i).first();
    await search.fill('unmatched-fixture-filter');
    await page.getByText('Visitor Node', { exact: true }).first().waitFor({ state:'hidden' });
    await search.fill('');
    await page.getByText('Visitor Node', { exact: true }).first().waitFor();
    for (const name of ['Table','Grid']) {
      const control = theme === 'default' ? page.locator('.view-switch-button').nth(name === 'Grid' ? 0 : 1) : page.getByRole('button', { name, exact:true });
      await control.first().click();
      await page.getByText('Visitor Node', { exact: true }).first().waitFor();
      assert.ok(!(await page.locator('body').innerText()).includes('admin.'));
    }
    // Display preferences stay local; changing them must not invoke management.
    if (theme === 'retro') {
      await page.getByRole('button', { name: /theme settings/i }).first().click();
      await page.keyboard.press('Escape');
    }
    await page.goto(fixture.base + '/instance/' + fixture.node);
    await page.getByText('Visitor Node', { exact: true }).first().waitFor({ timeout: 30000 });
    assert.ok(!(await page.locator('body').innerText()).includes('admin.'));
    await page.screenshot({ path: join(output, theme + '-details.png'), fullPage: true });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(fixture.base + '/s/' + fixture.share);
    await page.getByText('Visitor Node', { exact: true }).first().waitFor({ timeout: 30000 });
    await page.screenshot({ path: join(output, theme + '-mobile.png'), fullPage: true });
    page.removeAllListeners('response');
    await Promise.all(bodies);
    assert.ok(urls.some(url => url.includes('/share-assets/' + theme + '/')));
    assert.ok(urls.some(url => url.includes('/viewer-api/')));
    assert.ok(sockets.some(url => url.includes('/viewer-api/rpc2')));
    assert.ok(urls.every(url => !url.includes('/private-assets/') && !url.includes('/api/')));
    assert.deepEqual(failures, []);
    rows.push({ case: theme + '-real-browser-dashboard-details-mobile-resource-audit', status: 'PASS', responses: urls.length, filters:true, table_grid:true, historical_cache_retired:true });
    const temporaryContext = await browser.newContext({ locale: 'en-US' });
    const temporaryPage = await temporaryContext.newPage();
    await temporaryPage.goto(fixture.base + '/s/' + fixture.temporary);
    await temporaryPage.getByText('Visitor Node', { exact: true }).first().waitFor({ timeout: 30000 });
    const temporaryCookie = (await temporaryContext.cookies()).find(cookie => cookie.name === 'temp_key');
    assert.ok(temporaryCookie?.httpOnly && temporaryCookie.expires < Date.now()/1000 + 3601);
    assert.equal(await temporaryPage.locator('input[type="password"]').count(), 0);
    rows.push({ case: theme + '-temporary-share-real-browser', status: 'PASS' });
    await temporaryContext.close();
    await context.close();
  }
  // A browser carrying an owner session still gets a readonly data audience.
  await owner.goto(fixture.base + '/s/' + fixture.share);
  await owner.getByText('Visitor Node', { exact: true }).first().waitFor({ timeout: 30000 });
  assert.equal(await owner.getByText('Hidden Operations Canary', { exact: true }).count(), 0);
  const denied = await ownerContext.request.post(fixture.base + '/viewer-api/rpc2', { data: { jsonrpc: '2.0', id: 1, method: 'admin:getSettings', params: {} } });
  assert.ok((await denied.json()).error);
  const wsDenied = await owner.evaluate(() => new Promise((resolve, reject) => {
    const socket = new WebSocket(location.origin.replace(/^http/, 'ws') + '/viewer-api/rpc2');
    const timer = setTimeout(() => { socket.close(); reject(new Error('viewer socket timeout')); }, 5000);
    socket.onopen = () => socket.send(JSON.stringify({ jsonrpc:'2.0', id:710, method:'admin:getSettings', params:{} }));
    socket.onmessage = event => { clearTimeout(timer); socket.close(); resolve(JSON.parse(event.data).error?.code); };
    socket.onerror = () => { clearTimeout(timer); reject(new Error('viewer socket failed')); };
  }));
  assert.equal(wsDenied, -32041);
  rows.push({ case: 'owner-browser-share-tab-cannot-inherit-management', status: 'PASS' });
  const viewContext = await browser.newContext({ locale: 'en-US' });
  const viewPage = await viewContext.newPage();
  await viewPage.goto(fixture.base + fixture.view);
  const dialog = viewPage.getByRole('dialog');
  await dialog.waitFor({ timeout: 30000 });
  await dialog.locator('input:not([type="password"])').first().fill(fixture.username);
  await dialog.locator('input[type="password"]').fill(fixture.password);
  await dialog.getByRole('button', { name: /log.?in/i }).click();
  await viewPage.waitForFunction(() => !document.querySelector('[role="dialog"]'), { timeout: 30000 });
  assert.ok((await viewContext.cookies()).some(cookie => cookie.name === 'session_token'));
  const viewResult = await viewContext.request.post(fixture.base + '/api/rpc2', { data: { jsonrpc:'2.0', id:1, method:'admin:listClients', params:{} } });
  assert.equal((await viewResult.json()).result.length, 2);
  rows.push({ case: 'retro-owner-view-entry-password-dialog', status: 'PASS' });
  await viewContext.close();
  await rpc('admin:editSettings', { cors_origin_check_enabled:false });
  const crossSite = await ownerContext.request.post(fixture.base + '/api/rpc2', { headers:{ Origin:'https://foreign.invalid' }, data:{ jsonrpc:'2.0',id:1,method:'admin:getSettings',params:{} } });
  assert.equal(crossSite.status(),403);
  rows.push({ case:'cookie-authority-origin-check-independent-of-cors-toggle',status:'PASS' });
  await rpc('admin:editSettings', { tempory_share_token_expire_at: Math.floor(Date.now() / 1000) - 1 });
  assert.equal((await browser.newContext().then(context => context.request.get(fixture.base + '/s/' + fixture.temporary))).status(), 404);
  rows.push({ case: 'expired-share-real-server-rejected', status: 'PASS' });
  await writeFile(join(output, 'results.json'), JSON.stringify(rows, null, 2));
  console.log(JSON.stringify(rows));
} finally { await browser.close(); }
