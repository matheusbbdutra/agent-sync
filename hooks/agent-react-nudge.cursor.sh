#!/usr/bin/env bash
# Cursor postToolUse: lembrete periodico de validacao de hipoteses (agent-react).
set -euo pipefail

THRESHOLD="${AGENT_SYNC_REACT_NUDGE_THRESHOLD:-15}"
STATE_DIR="${TMPDIR:-/tmp}/agent-sync-react-nudge-cursor"
mkdir -p "$STATE_DIR"

input="$(cat)"
session_id="$(printf '%s' "$input" | { grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
if [ -z "$session_id" ]; then
  session_id="$(printf '%s' "$input" | { grep -o '"conversation_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
fi
session_id="${session_id:-default}"

counter_file="$STATE_DIR/$session_id.count"
count=0
[ -f "$counter_file" ] && count="$(cat "$counter_file")"
count=$((count + 1))
printf '%s' "$count" > "$counter_file"

if [ "$((count % THRESHOLD))" -eq 0 ]; then
  # Mirror best-effort para memory-mcp.
  if command -v memory-mcp >/dev/null 2>&1; then
    printf '%s\n' \
      '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
      '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
      "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"record_event\",\"arguments\":{\"agent\":\"cursor\",\"kind\":\"guard_nudge\",\"note\":\"agent-react #$count (cursor)\",\"session_id\":\"$session_id\"}}}" \
      | memory-mcp -db "${AGENT_SYNC_MEMORY_DB:-${XDG_CACHE_HOME:-$HOME/.cache}/agent-sync/memory.db}" >/dev/null 2>&1 || true
  fi
  printf '{"additional_context":"[agent-sync] Hipotese ativa sem validacao? Nao conclua/implemente como fato. Valide com tool/leitura ou peca o passo concreto ao usuario. Skill: agent-react. (lembrete #%s)"}' "$count"
else
  printf '{}'
fi
