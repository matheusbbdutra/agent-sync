#!/usr/bin/env bash
# Cursor postToolUse: lembrete periodico do context-guard (additional_context).
set -euo pipefail

THRESHOLD="${AGENT_SYNC_NUDGE_THRESHOLD:-40}"
STATE_DIR="${TMPDIR:-/tmp}/agent-sync-nudge-cursor"
mkdir -p "$STATE_DIR"

input="$(cat)"
session_id="$(printf '%s' "$input" | grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
if [ -z "$session_id" ]; then
  session_id="$(printf '%s' "$input" | grep -o '"conversation_id"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
fi
session_id="${session_id:-default}"

counter_file="$STATE_DIR/$session_id.count"
count=0
[ -f "$counter_file" ] && count="$(cat "$counter_file")"
count=$((count + 1))
printf '%s' "$count" > "$counter_file"

if [ "$((count % THRESHOLD))" -eq 0 ]; then
  printf '{"additional_context":"[agent-sync] Carregue context-guard e atualize STATE.md. (lembrete #%s)"}' "$count"
else
  printf '{}'
fi
