#!/usr/bin/env bash
# memory-consolidate.stop.sh
#
# Hook cross-CLI de término (Stop / compaction) para consolidação automática de memória (ADR-automated-memory-observation-pipeline).
# Lê o buffer efêmero da sessão, sintetiza fatos técnicos aprendidos e grava no memory.db (FTS5).
# Exit 0 sempre para não impedir o encerramento do turno.

set -uo pipefail

if [ "${AGENT_SYNC_MEMORY_CONSOLIDATE_DISABLE:-0}" = "1" ]; then
  exit 0
fi

MEM_BIN="${AGENT_SYNC_MEMORY_BIN:-}"
if [ -z "$MEM_BIN" ]; then
  MEM_BIN="$(command -v memory-mcp 2>/dev/null || true)"
fi
if [ -z "$MEM_BIN" ] && [ -x "$HOME/.local/bin/memory-mcp" ]; then
  MEM_BIN="$HOME/.local/bin/memory-mcp"
fi

if [ -z "$MEM_BIN" ] || [ ! -x "$MEM_BIN" ]; then
  exit 0
fi

input="$(cat 2>/dev/null || true)"

session_id="$(printf '%s' "$input" \
  | { grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
if [ -z "$session_id" ]; then
  session_id="$(printf '%s' "$input" \
    | { grep -o '"conversation_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
    | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
fi
session_id="${session_id:-default}"

# Consolidação das observações da sessão
"$MEM_BIN" consolidate \
  -session "$session_id" \
  -project "$PWD" >/dev/null 2>&1 || true

printf '{}'
exit 0
