#!/usr/bin/bash
# Cacheia passivamente docs ja consultadas via WebFetch ou context7 (query-docs),
# sem refazer requisicao de rede - so persiste o que a ferramenta ja trouxe.
# Compartilhado entre Claude Code e Codex (schema de PostToolUse equivalente:
# tool_name, tool_input, tool_response).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WRITER="$SCRIPT_DIR/../bin/docs-cache-write"
[ -x "$WRITER" ] || WRITER="$(command -v docs-cache-write || true)"

if [ -z "$WRITER" ] || ! command -v python3 >/dev/null 2>&1; then
  printf '{}'
  exit 0
fi

python3 "$SCRIPT_DIR/docs-cache.py" "$WRITER" || printf '{}'
