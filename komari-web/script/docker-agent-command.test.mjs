import assert from "node:assert/strict";
import test from "node:test";
import { createDockerAgentCommand } from "../src/utils/dockerAgentCommand.mjs";

const quote = args => args.map(arg => `'${arg.replace(/'/g, `'\\''`)}'`).join(" ");

test("Docker agent honors the selected release and excludes installer-only options", () => {
  const command = createDockerAgentCommand([
    "-e", "https://monitor.example", "-t", "test token", "--install-version", "v1.2.13",
    "--install-dir", "/custom path", "--install-service-name", "custom", "--install-ghproxy", "https://proxy.example",
    "--disable-web-ssh",
  ], quote);
  assert.ok(command.includes("komari-retro-agent:v1.2.13"));
  assert.ok(command.includes("'test token'"));
  assert.ok(command.includes("--disable-web-ssh"));
  assert.ok(!command.includes("--install-"));
  assert.ok(!command.includes("/custom path"));
});

test("Docker auto discovery uses a private absolute bind mount", () => {
  const command = createDockerAgentCommand(["--auto-discovery", "fixture-key"], quote, true);
  assert.ok(command.startsWith("umask 077 && touch "));
  assert.ok(command.includes("chmod 600 .komari-auto-discovery.json"));
  assert.ok(command.includes('-v "$(pwd)/.komari-auto-discovery.json:/app/auto-discovery.json"'));
  assert.ok(command.includes("komari-retro-agent:latest"));
  assert.ok(command.includes("'--auto-discovery' 'fixture-key'"));
});
