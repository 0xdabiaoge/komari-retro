import assert from "node:assert/strict";
import { mkdir } from "node:fs/promises";
import { chromium } from "playwright";
const base = process.env.TEST_BASE_URL || "http://127.0.0.1:5173";
const output = new URL("../../.codex/terminal-browser/", import.meta.url);
await mkdir(output, { recursive: true });
const browser = await chromium.launch({
  headless: true,
  ...(process.env.BROWSER_PATH
    ? { executablePath: process.env.BROWSER_PATH }
    : {}),
});
const errors = [],
  inputs = [],
  connections = [];
let settings = {
  terminalOptions: {
    fontSize: 14,
    fontFamily: "monospace",
    scrollback: 5000,
    cursorBlink: true,
    convertEol: false,
  },
  terminalPadding: 12,
  themeName: "dark",
  savedCommands: [],
};
const theme = {
  _komari_onboarding_v1: {
    seen: ["install", "workbench", "notifications", "markets", "terminal"],
    workbenchOpened: true,
  },
};
let currentSocket,
  nextSeq = 0;
const context = await browser.newContext({
  viewport: { width: 1440, height: 900 },
  locale: "zh-CN",
  serviceWorkers: "block",
});
await context.addInitScript(() => {
  localStorage.setItem("language", "zh-CN");
});
await context.route("**/api/**", async (route) => {
  const u = new URL(route.request().url());
  let body = { status: "success", data: [] };
  if (u.pathname === "/api/me")
    body = { logged_in: true, uuid: "fixture-admin" };
  else if (u.pathname === "/api/admin/client/list")
    body = [{ uuid: "fixture-node", name: "本地验收节点", os: "Ubuntu" }];
  else if (u.pathname === "/api/public")
    body = {
      data: { theme: "default", theme_settings: theme, sitename: "Komari" },
    };
  else if (u.pathname === "/api/admin/theme/settings")
    body = { status: "success", data: theme };
  else if (u.pathname === "/api/admin/settings/xtermjs") {
    if (route.request().method() === "POST")
      settings = route.request().postDataJSON();
    body = { status: "success", data: settings };
  } else if (u.pathname === "/api/admin/settings")
    body = { data: { eula_accepted: true } };
  else if (u.pathname === "/api/rpc2") {
    const r = route.request().postDataJSON();
    body = { jsonrpc: "2.0", id: r.id, result: {} };
  }
  await route.fulfill({ json: body });
});
await context.routeWebSocket("**/api/**", (ws) => {
  if (!ws.url().includes("/terminal")) {
    ws.onMessage((message) => {
      try {
        const r = JSON.parse(String(message));
        ws.send(JSON.stringify({ jsonrpc: "2.0", id: r.id, result: {} }));
      } catch {
        /* Non-RPC */
      }
    });
    return;
  }
  const u = new URL(ws.url());
  connections.push(u);
  currentSocket = ws;
  if (!u.searchParams.has("request_id")) nextSeq = 0;
  ws.send(
    JSON.stringify({
      type: "session",
      request_id: "retained-fixture-session",
      ready: true,
    }),
  );
  ws.send(
    JSON.stringify({
      type: "output",
      seq: ++nextSeq,
      data: Buffer.from(
        "\x1b[32mKomari Retro 本地终端\x1b[0m\r\nfixture@node:~$ ",
      ).toString("base64"),
    }),
  );
  ws.onMessage((message) => {
    if (typeof message !== "string")
      inputs.push(Buffer.from(message).toString());
  });
});
const page = await context.newPage();
page.on("pageerror", (e) => errors.push(e.message));
try {
  await page.goto(base + "/terminal?uuid=fixture-node");
  await page.locator(".km-connection-status.connected").waitFor();
  await page.locator(".rt-CalloutRoot button").click();
  await page.locator(".km-guide-content").waitFor({ state: "hidden" });
  const before = connections.length;
  await page.getByRole("button", { name: "设置", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "工作台设置" });
  await dialog.getByLabel("配色", { exact: true }).selectOption("light");
  await dialog.getByLabel("字号", { exact: true }).selectOption("18");
  await dialog.getByRole("button", { name: "保存设置", exact: true }).click();
  await dialog.waitFor({ state: "hidden" });
  assert.equal(connections.length, before, "settings recreated the shell");
  assert.equal(settings.themeName, "light");
  assert.equal(settings.terminalOptions.fontSize, 18);
  assert.equal(
    await page
      .locator(".km-terminal-session.is-active")
      .evaluate((el) => getComputedStyle(el).backgroundColor),
    "rgb(248, 250, 252)",
  );
  await page.getByRole("button", { name: "常用命令", exact: true }).click();
  const commands = page.getByRole("dialog", { name: "常用命令" });
  await commands.getByLabel("名称", { exact: true }).fill("检查负载");
  await commands.getByLabel("命令内容", { exact: true }).fill("uptime");
  await commands.getByLabel("说明", { exact: true }).fill("只填入，不自动执行");
  await commands.getByRole("button", { name: "保存命令", exact: true }).click();
  await commands.getByRole("button", { name: /检查负载/ }).click();
  const inputCount = inputs.length;
  await commands.getByRole("button", { name: "填入命令", exact: true }).click();
  await page.waitForTimeout(100);
  assert.equal(inputs.slice(inputCount).join(""), "uptime");
  assert.ok(
    !inputs.slice(inputCount).join("").includes("\r"),
    "saved command executed automatically",
  );
  await page.getByRole("button", { name: "常用命令", exact: true }).click();
  await commands.getByRole("button", { name: /检查负载/ }).click();
  await commands
    .getByLabel("命令内容", { exact: true })
    .fill("echo one\necho two");
  await commands.getByRole("button", { name: "保存命令", exact: true }).click();
  await commands.getByRole("button", { name: /检查负载/ }).click();
  await commands.getByRole("button", { name: "填入命令", exact: true }).click();
  const paste = page.getByRole("dialog", { name: "确认多行粘贴" });
  await paste.waitFor();
  const beforeCancel = inputs.length;
  await paste.getByRole("button", { name: "取消", exact: true }).click();
  assert.equal(inputs.length, beforeCancel);
  currentSocket.close({ code: 1012, reason: "isolated reconnect test" });
  await page.waitForFunction(() =>
    document.querySelector(".km-connection-status.connected"),
  );
  await page.waitForTimeout(1600);
  assert.ok(connections.length > before);
  assert.equal(
    connections.at(-1).searchParams.get("request_id"),
    "retained-fixture-session",
  );
  await page.screenshot({
    path: new URL("desktop.png", output).pathname.replace(
      /^\/([A-Za-z]:)/,
      "$1",
    ),
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.locator(".km-terminal-accessory").waitFor({ state: "visible" });
  const geometry = await page.evaluate(() => ({
    width: innerWidth,
    scroll: document.documentElement.scrollWidth,
    terminal: document
      .querySelector(".km-terminal-session.is-active")
      .getBoundingClientRect().height,
  }));
  assert.ok(geometry.scroll <= geometry.width + 1);
  assert.ok(geometry.terminal > 300);
  await page
    .locator(".km-terminal-accessory")
    .getByRole("button", { name: "Tab", exact: true })
    .click();
  assert.ok(inputs.some((s) => s.includes("\t")));
  await page.screenshot({
    path: new URL("mobile.png", output).pathname.replace(
      /^\/([A-Za-z]:)/,
      "$1",
    ),
  });
  await page.reload();
  await page.locator(".km-connection-status.connected").waitFor();
  assert.equal(settings.savedCommands.length, 1);
  assert.equal(settings.terminalOptions.fontSize, 18);
  assert.deepEqual(errors, []);
  console.log(
    "PASS: live settings without shell replacement; saved commands without execution; multiline cancel; retained-ID reconnect; mobile controls; persistence; no browser errors",
  );
} catch (error) {
  await page.screenshot({
    path: new URL("failure.png", output).pathname.replace(
      /^\/([A-Za-z]:)/,
      "$1",
    ),
  });
  console.error(
    JSON.stringify({
      errors,
      connections: connections.length,
      url: page.url(),
      text: (await page.locator("body").innerText()).slice(0, 1800),
    }),
  );
  throw error;
} finally {
  await context.close();
  await browser.close();
}
