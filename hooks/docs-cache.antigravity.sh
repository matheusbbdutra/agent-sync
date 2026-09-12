#!/usr/bin/env bash
# Cacheia passivamente docs consultadas via read_url_content ou context7
# (call_mcp_tool com ServerName=context7, ToolName=query-docs) no Antigravity.
# O resultado da tool nao vem no payload do hook - le do transcriptPath
# (ver docs-cache.antigravity.py).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WRITER="$SCRIPT_DIR/../bin/docs-cache-write"
[ -x "$WRITER" ] || WRITER="$(command -v docs-cache-write || true)"

if [ -z "$WRITER" ] || ! command -v python3 >/dev/null 2>&1; then
  printf '{}'
  exit 0
fi

python3 "$SCRIPT_DIR/docs-cache.antigravity.py" "$WRITER" || printf '{}'
