#!/usr/bin/env bash
# Adapter fino do secret-guard PreToolUse (regra §4: lógica vive no Go).
# Repassa o stdin ao `secret-scan check` e devolve o envelope do host
# (Claude/Codex `{}`+hookSpecificOutput, Antigravity `decision`).
# Fail-open: binário ausente = allow, nunca derruba o turno.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/../bin/secret-scan"
[ -x "$BIN" ] || BIN="$(command -v secret-scan || true)"

if [ -z "$BIN" ]; then
  printf '{}\n'
  exit 0
fi

"$BIN" check --format auto || printf '{}\n'
