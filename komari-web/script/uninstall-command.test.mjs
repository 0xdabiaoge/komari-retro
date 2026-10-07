import assert from "node:assert/strict";
import test from "node:test";
import { createUninstallCommand } from "../src/utils/uninstallCommand.mjs";

test("Linux uninstall handles system/user service managers and preserves unrelated files", () => {
  const command = createUninstallCommand("linux");

  assert.match(command, /systemctl --user disable/);
  assert.match(command, /rc-service/);
  assert.match(command, /initctl stop/);
  assert.match(command, /remove_agent_dir \/opt\/komari/);
  assert.ok(command.includes('${XDG_DATA_HOME:-$HOME/.local/share}/komari'));
  assert.match(command, /rmdir -- "\$dir"/);
  assert.doesNotMatch(command, /rm\s+-rf/);
  assert.doesNotMatch(command, /\/etc\/komari/);
});

test("custom service names and install paths are quoted for Linux and macOS", () => {
  const options = { serviceName: "komari custom's", installDir: "/srv/komari agent's" };
  const linux = createUninstallCommand("linux", options);
  const macOS = createUninstallCommand("macos", options);

  assert.match(linux, /SERVICE_NAME='komari custom'\\''s'/);
  assert.match(linux, /remove_agent_dir '\/srv\/komari agent'\\''s'/);
  assert.match(macOS, /LABEL="com\.komari\.\$SERVICE_NAME"/);
  assert.match(macOS, /SYSTEM_PLIST="\/Library\/LaunchDaemons\/\$LABEL\.plist"/);
  assert.match(macOS, /launchctl bootout system/);
  assert.match(macOS, /remove_agent_dir '\/srv\/komari agent'\\''s'/);
});

test("Windows removes the service and known files without recursive directory deletion", () => {
  const command = createUninstallCommand("windows", {
    serviceName: "komari-agent-custom",
    installDir: "C:\\Custom Agent\\Komari",
  });

  assert.match(command, /nssm\.exe/);
  assert.match(command, /komari-agent-custom/);
  assert.match(command, /C:\\Custom Agent\\Komari/);
  assert.match(command, /sc\.exe delete/);
  assert.doesNotMatch(command, /-Recurse/);
});

test("Docker removes the selected container and only a bind-mounted discovery file", () => {
  const command = createUninstallCommand("docker", { serviceName: "my-agent" });

  assert.match(command, /CONTAINER_NAME=my-agent/);
  assert.match(command, /docker rm -f/);
  assert.match(command, /eq \.Type "bind"/);
  assert.match(command, /\/app\/auto-discovery\.json/);
  assert.match(command, /\[ -f "\$STATE_FILE" \]/);
  assert.doesNotMatch(command, /docker rmi/);
  assert.doesNotMatch(command, /\.komari-auto-discovery\.json/);
});
