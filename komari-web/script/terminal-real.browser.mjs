import assert from "node:assert/strict";
import { readFile, mkdir } from "node:fs/promises";
import { chromium } from "playwright";
const f = JSON.parse(await readFile(process.env.TERMINAL_REAL_FIXTURE, "utf8"));
const browser = await chromium.launch({ headless: true });
const context = await browser.newContext({
  serviceWorkers: "block",
  viewport: { width: 1280, height: 800 },
});
const errors = [],
  sockets = [];
const bridges = [];
await context.routeWebSocket("**/api/admin/client/*/terminal?*", (route) => {
  const upstream = route.connectToServer();
  bridges.push({ route, upstream });
});
await context.route("**/*", (route) => {
  const u = new URL(route.request().url());
  return u.hostname === "127.0.0.1" ? route.continue() : route.abort();
});
await context.addInitScript(() => localStorage.setItem("language", "zh-CN"));
const page = await context.newPage();
page.on("pageerror", (e) => errors.push(e.message));
page.on("websocket", (ws) => {
  if (ws.url().includes("/terminal")) sockets.push(ws.url());
});
const out = new URL("../../.codex/terminal-browser/", import.meta.url);
await mkdir(out, { recursive: true });
try {
  await context.request.get(f.base + f.entry);
  const response = await context.request.post(f.base + "/api/login", {
    data: { username: f.username, password: f.password },
  });
  assert.equal(response.status(), 200);
  await page.goto(f.base + "/terminal?uuid=" + f.node);
  await page
    .locator(".km-connection-status.connected")
    .waitFor({ timeout: 30000 });
  await page.locator(".rt-CalloutRoot button").click();
  const screen = page.locator(".km-terminal-session.is-active .xterm-rows");
  await page.waitForFunction(
    () =>
      /(?:PS .*?>|[$#] )/.test(
        document.querySelector(".km-terminal-session.is-active .xterm-rows")
          ?.textContent || "",
      ),
    { timeout: 30000 },
  );
  const input = page.locator(
    ".km-terminal-session.is-active .xterm-helper-textarea",
  );
  const type = async (command) => {
    await input.focus();
    await page.keyboard.type(command);
    await page.keyboard.press("Enter");
  };
  const tag = "session" + Date.now();
  await type(
    f.windows
      ? `$env:KR_TERMINAL_PROBE='${tag}'; Write-Output ('READY_' + 'PTY')`
      : `export KR_TERMINAL_PROBE='${tag}'; echo READY_"PTY"`,
  );
  await page.waitForFunction(() =>
    document
      .querySelector(".km-terminal-session.is-active .xterm-rows")
      ?.textContent.includes("READY_PTY"),
  );
  const count = sockets.length;
  const bridge = bridges.at(-1);
  bridge.route.close({ code: 1012, reason: "controlled network interruption" });
  bridge.upstream.close();
  await page.locator(".km-connection-status.reconnecting").waitFor();
  await page.locator(".km-connection-status.connected").waitFor();
  assert.ok(sockets.length > count, "browser socket did not reconnect");
  assert.ok(
    new URL(sockets.at(-1)).searchParams.get("request_id"),
    "missing retained session ID",
  );
  await type(
    f.windows
      ? "Write-Output ('KEPT_' + $env:KR_TERMINAL_PROBE)"
      : 'echo KEPT_"$KR_TERMINAL_PROBE"',
  );
  await page.waitForFunction(
    (expected) =>
      document
        .querySelector(".km-terminal-session.is-active .xterm-rows")
        ?.textContent.includes(expected),
    "KEPT_" + tag,
  );
  await page.getByRole("button", { name: "设置", exact: true }).click();
  const settings = page.getByRole("dialog", { name: "工作台设置" });
  await settings.getByLabel("字号", { exact: true }).selectOption("18");
  await settings.getByRole("button", { name: "保存设置", exact: true }).click();
  await settings.waitFor({ state: "hidden" });
  await type(
    f.windows
      ? "Write-Output ('AFTER_' + $env:KR_TERMINAL_PROBE)"
      : 'echo AFTER_"$KR_TERMINAL_PROBE"',
  );
  await page.waitForFunction(
    (expected) =>
      document
        .querySelector(".km-terminal-session.is-active .xterm-rows")
        ?.textContent.includes(expected),
    "AFTER_" + tag,
  );
  assert.ok((await screen.innerText()).includes(tag));
  await page.getByRole("button", { name: "文件", exact: true }).click();
  const changeDirectory = page.getByRole("button", { name: "填入切换目录命令", exact: true });
  await changeDirectory.waitFor();
  await page.waitForFunction(() => !document.querySelector('[aria-label="填入切换目录命令"]')?.disabled);
  await changeDirectory.click();
  await input.focus(); await page.keyboard.press("Enter");
  await type(f.windows ? "Write-Output ('DIRECTORY_' + 'OK')" : 'echo DIRECTORY_"OK"');
  await page.waitForFunction(() => document.querySelector('.km-terminal-session.is-active .xterm-rows')?.textContent.includes("DIRECTORY_OK"));
  await page.getByRole("button", { name: "文件", exact: true }).click();
  await page.screenshot({
    path: new URL("real-native-terminal.png", out).pathname.replace(
      /^\/([A-Za-z]:)/,
      "$1",
    ),
  });
  await context.request.get(f.base + "/api/logout");
  await page
    .locator(".km-connection-status.closed")
    .waitFor({ timeout: 15000 });
  assert.deepEqual(errors, []);
  console.log(
    "PASS: real PTY roundtrip; browser reconnect preserves shell environment; settings preserve shell; logout closes terminal",
  );
} catch (e) {
  await page.screenshot({
    path: new URL("real-failure.png", out).pathname.replace(
      /^\/([A-Za-z]:)/,
      "$1",
    ),
  });
  console.error(
    JSON.stringify({
      errors,
      sockets: sockets.length,
      text: (await page.locator("body").innerText()).slice(-2000),
    }),
  );
  throw e;
} finally {
  await context.close();
  await browser.close();
}
