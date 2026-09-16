#!/usr/bin/env bash
# Cursor postToolUse: lembrete periodico de memoria compartilhada (store_memory).
set -euo pipefail

THRESHOLD="${AGENT_SYNC_MEMORY_NUDGE_THRESHOLD:-25}"
STATE_DIR="${TMPDIR:-/tmp}/agent-sync-memory-nudge-cursor"
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
  printf '{"additional_context":"[agent-sync] Aconteceu algo nesta sessao que deveria virar memoria (correcao do usuario, decisao de projeto, preferencia confirmada)? Se sim, grave um resumo com o porque via store_memory (memory-mcp), agent=cursor. (lembrete #%s)"}' "$count"
else
  printf '{}'
fi
