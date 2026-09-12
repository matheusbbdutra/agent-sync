#!/usr/bin/env bash
# Lembrete pos-ferramenta: a cada N chamadas na mesma sessao, cobra o carregamento
# da skill context-guard e a atualizacao do STATE.md (reforco do enforcement por prompt).
set -euo pipefail

THRESHOLD="${AGENT_SYNC_NUDGE_THRESHOLD:-40}"
STATE_DIR="${TMPDIR:-/tmp}/agent-sync-nudge"
mkdir -p "$STATE_DIR"

input="$(cat)"
session_id="$(printf '%s' "$input" | grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
session_id="${session_id:-default}"

counter_file="$STATE_DIR/$session_id.count"
count=0
[ -f "$counter_file" ] && count="$(cat "$counter_file")"
count=$((count + 1))
printf '%s' "$count" > "$counter_file"

if [ "$((count % THRESHOLD))" -eq 0 ]; then
  cat <<EOF
{"systemMessage":"agent-sync: lembrete de context-guard (#$count)","hookSpecificOutput":{"additionalContext":"[agent-sync] Carregue context-guard e atualize STATE.md."}}
EOF
else
  printf '{}'
fi
