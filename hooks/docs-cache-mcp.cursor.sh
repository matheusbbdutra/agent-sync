#!/usr/bin/env bash
# Cursor afterMCPExecution (matcher query-docs): cacheia resultados do context7.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WRITER="$(command -v docs-cache-write || true)"
if [ -z "$WRITER" ] && [ -x "$SCRIPT_DIR/../bin/docs-cache-write" ]; then
  WRITER="$SCRIPT_DIR/../bin/docs-cache-write"
fi

if [ -z "$WRITER" ] || ! command -v python3 >/dev/null 2>&1; then
  printf '{}'
  exit 0
fi

python3 "$SCRIPT_DIR/docs-cache.cursor.py" "$WRITER" mcp || printf '{}'
