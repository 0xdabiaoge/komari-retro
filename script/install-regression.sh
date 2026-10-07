#!/usr/bin/env bash
# Isolated fixture: no real systemctl, network, service or production data.
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source "$ROOT/install-komari.sh"
mkdir -p "$ROOT/.codex/build"
FIXTURE=$(mktemp -d "$ROOT/.codex/build/installer-fixture.XXXXXX")
cleanup() {
  case "$FIXTURE" in "$ROOT"/.codex/build/installer-fixture.*) rm -rf -- "$FIXTURE" ;; *) exit 1 ;; esac
}
trap cleanup EXIT
INSTALL_DIR="$FIXTURE/install"
DATA_DIR="$FIXTURE/work"
BINARY_PATH="$INSTALL_DIR/komari"
mkdir -p "$INSTALL_DIR" "$DATA_DIR/data"
is_installed() { return 0; }
check_systemd() { return 0; }
# Git Bash does not ship flock. Linux CI exercises the real lock command.
if ! command -v flock >/dev/null 2>&1; then flock() { return 0; }; fi
systemctl() { printf '%s\n' "$*" >> "$FIXTURE/service.calls"; return 0; }
download_verified() {
  if [ "$SCENARIO" = download-failure ]; then return 1; fi
  [ ! -s "$FIXTURE/service.calls" ] || { echo 'Service stopped before candidate was downloaded'; return 1; }
  printf '#!/bin/bash\nexit 0\n' > "$1"
}
wait_healthy() {
  printf 'migration-change' > "$DATA_DIR/data/komari.db"
  [ "$SCENARIO" = success ]
}
for SCENARIO in download-failure health-failure success; do
  printf 'old-binary' > "$BINARY_PATH"
  printf 'original-db' > "$DATA_DIR/data/komari.db"
  : > "$FIXTURE/service.calls"
  STATUS=0
  upgrade_komari || STATUS=$?
  if [ "$SCENARIO" = success ]; then
    [ "$STATUS" = 0 ] && [ "$(cat "$DATA_DIR/data/komari.db")" = migration-change ]
  else
    [ "$STATUS" != 0 ] && [ "$(cat "$BINARY_PATH")" = old-binary ] && [ "$(cat "$DATA_DIR/data/komari.db")" = original-db ]
    if [ "$SCENARIO" = download-failure ]; then [ ! -s "$FIXTURE/service.calls" ]; fi
  fi
  printf 'PASS: %s\n' "$SCENARIO"
done
