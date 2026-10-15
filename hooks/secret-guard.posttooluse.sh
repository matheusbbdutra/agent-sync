#!/usr/bin/env bash
# Adapter fino do secret-guard PostToolUse (regra §4: lógica vive no Go).
# Repassa o stdin ao `secret-scan redact` (redação total ou inline).
# Fail-open: binário ausente = silencioso, nunca derruba o turno.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/../bin/secret-scan"
[ -x "$BIN" ] || BIN="$(command -v secret-scan || true)"

if [ -z "$BIN" ]; then
  printf '{}\n'
  exit 0
fi

"$BIN" redact --format auto || printf '{}\n'
