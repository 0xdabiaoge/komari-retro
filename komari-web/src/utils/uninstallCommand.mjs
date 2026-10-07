function quoteShellArg(value) {
  const trimmed = value.trim();
  if (trimmed === "") return "''";
  if (!/[\s"'\\$`!#&*();<>?[\]^{|}~]/.test(trimmed)) return trimmed;
  return `'${trimmed.replace(/'/g, `'\\''`)}'`;
}

function quotePowerShellArg(value) {
  return `'${value.trim().replace(/'/g, "''")}'`;
}

function linuxCommand(serviceName, installDir) {
  const directories = installDir
    ? `remove_agent_dir ${quoteShellArg(installDir)}`
    : [
        `remove_agent_dir ${quoteShellArg("/opt/komari")}`,
        `remove_agent_dir "\${XDG_DATA_HOME:-$HOME/.local/share}/komari"`,
        `remove_agent_dir "$HOME/.komari"`,
      ].join("\n");

  return `SERVICE_NAME=${quoteShellArg(serviceName)}

run_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  else
    sudo "$@"
  fi
}

# Remove both system and current-user systemd services.
run_root systemctl stop "$SERVICE_NAME.service" >/dev/null 2>&1 || true
run_root systemctl disable "$SERVICE_NAME.service" >/dev/null 2>&1 || true
run_root rm -f -- "/etc/systemd/system/$SERVICE_NAME.service"
run_root systemctl daemon-reload >/dev/null 2>&1 || true

if command -v systemctl >/dev/null 2>&1; then
  systemctl --user stop "$SERVICE_NAME.service" >/dev/null 2>&1 || true
  systemctl --user disable "$SERVICE_NAME.service" >/dev/null 2>&1 || true
  rm -f -- "\${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/$SERVICE_NAME.service"
  systemctl --user daemon-reload >/dev/null 2>&1 || true
fi

# Also handle OpenRC, OpenWrt procd, and Upstart installations.
run_root rc-service "$SERVICE_NAME" stop >/dev/null 2>&1 || true
run_root rc-update del "$SERVICE_NAME" default >/dev/null 2>&1 || true
if command -v uci >/dev/null 2>&1; then
  run_root "/etc/init.d/$SERVICE_NAME" stop >/dev/null 2>&1 || true
  run_root "/etc/init.d/$SERVICE_NAME" disable >/dev/null 2>&1 || true
fi
run_root rm -f -- "/etc/init.d/$SERVICE_NAME"
run_root initctl stop "$SERVICE_NAME" >/dev/null 2>&1 || true
run_root rm -f -- "/etc/init/$SERVICE_NAME.conf"

# Delete only Komari Agent files; remove a directory only if it is empty.
remove_agent_dir() {
  dir=$1
  [ -n "$dir" ] || return 0
  run_root rm -f -- "$dir/agent" "$dir/auto-discovery.json"
  run_root rmdir -- "$dir" 2>/dev/null || true
}
${directories}`;
}

function macOSCommand(serviceName, installDir) {
  const directories = installDir
    ? `remove_agent_dir ${quoteShellArg(installDir)}`
    : [
        `remove_agent_dir ${quoteShellArg("/usr/local/komari")}`,
        `remove_agent_dir "$HOME/.komari"`,
      ].join("\n");

  return `SERVICE_NAME=${quoteShellArg(serviceName)}
LABEL="com.komari.$SERVICE_NAME"

run_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  else
    sudo "$@"
  fi
}

SYSTEM_PLIST="/Library/LaunchDaemons/$LABEL.plist"
run_root launchctl bootout system "$SYSTEM_PLIST" >/dev/null 2>&1 || true
run_root rm -f -- "$SYSTEM_PLIST"

USER_PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
launchctl bootout "gui/$(id -u)" "$USER_PLIST" >/dev/null 2>&1 || true
rm -f -- "$USER_PLIST"

remove_agent_dir() {
  dir=$1
  [ -n "$dir" ] || return 0
  run_root rm -f -- "$dir/agent" "$dir/auto-discovery.json"
  run_root rmdir -- "$dir" 2>/dev/null || true
}
${directories}`;
}

function windowsCommand(serviceName, installDir) {
  const directory = installDir
    ? quotePowerShellArg(installDir)
    : `Join-Path $Env:ProgramFiles 'Komari'`;

  return `$ServiceName = ${quotePowerShellArg(serviceName)}
$InstallDir = ${directory}
$NssmExe = Join-Path $InstallDir 'nssm.exe'

if (-not (Test-Path -LiteralPath $NssmExe)) {
  $NssmCommand = Get-Command nssm.exe -ErrorAction SilentlyContinue
  if ($NssmCommand) { $NssmExe = $NssmCommand.Source }
}
if (Test-Path -LiteralPath $NssmExe) {
  & $NssmExe stop $ServiceName 2>$null | Out-Null
  & $NssmExe remove $ServiceName confirm 2>$null | Out-Null
} else {
  Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
  sc.exe delete $ServiceName | Out-Null
}

@('komari-agent.exe', 'nssm.exe', 'auto-discovery.json') | ForEach-Object {
  Remove-Item -LiteralPath (Join-Path $InstallDir $_) -Force -ErrorAction SilentlyContinue
}
# This removes the install directory only when no unrelated files remain.
Remove-Item -LiteralPath $InstallDir -Force -ErrorAction SilentlyContinue`;
}

function dockerCommand(containerName) {
  return `CONTAINER_NAME=${quoteShellArg(containerName)}
STATE_FILE="$(docker inspect --format ${quoteShellArg(
    '{{range .Mounts}}{{if and (eq .Type "bind") (eq .Destination "/app/auto-discovery.json")}}{{.Source}}{{end}}{{end}}',
  )} "$CONTAINER_NAME" 2>/dev/null || true)"

docker rm -f "$CONTAINER_NAME" 2>/dev/null || true
# Remove only the exact bind-mounted discovery file, never the current directory or a named volume.
if [ -n "$STATE_FILE" ] && [ -f "$STATE_FILE" ]; then
  rm -f -- "$STATE_FILE"
fi`;
}

export function createUninstallCommand(platform, options = {}) {
  const serviceName = options.serviceName?.trim() || "komari-agent";
  const installDir = options.installDir?.trim() || "";

  switch (platform) {
    case "docker":
      return dockerCommand(serviceName);
    case "windows":
      return windowsCommand(serviceName, installDir);
    case "macos":
      return macOSCommand(serviceName, installDir);
    case "linux":
    default:
      return linuxCommand(serviceName, installDir);
  }
}
