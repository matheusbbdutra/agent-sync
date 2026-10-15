#!/usr/bin/env bash
# Adapter fino do cmd-cache PostToolUse (regra §4: lógica vive no Go).
# Cacheia o stdout de comandos read-only do allowlist (lscpu, uname,
# /etc/os-release, git log) para o PreToolUse injetar depois — ADR-reuso-
# bash-sudo-agents §1. Opt-in: sem AGENT_SYNC_CMD_CACHE=1 é no-op silencioso.
# Fail-open: nunca muda o contrato do host (sempre '{}').
set -euo pipefail

[ "${AGENT_SYNC_CMD_CACHE:-0}" = "1" ] || { printf '{}'; exit 0; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/../bin/shell-validate"
[ -x "$BIN" ] || BIN="$(command -v shell-validate || true)"
if [ -z "$BIN" ]; then
  printf '{}'
  exit 0
fi

"$BIN" cache-put --from-hook >/dev/null 2>&1 || true
printf '{}'