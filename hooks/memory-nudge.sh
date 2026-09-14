#!/usr/bin/env bash
# Lembrete pos-ferramenta: a cada N chamadas na mesma sessao, cobra a
# checagem/gravacao de memoria via memory-mcp (store_memory), ja que hoje
# isso depende so da disciplina do modelo seguindo o CLAUDE.md.
set -euo pipefail

THRESHOLD="${AGENT_SYNC_MEMORY_NUDGE_THRESHOLD:-25}"
STATE_DIR="${TMPDIR:-/tmp}/agent-sync-memory-nudge"
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
{"systemMessage":"agent-sync: lembrete de memoria (#$count)","hookSpecificOutput":{"additionalContext":"[agent-sync] Aconteceu algo nesta sessao que deveria virar memoria (correcao do usuario, decisao de projeto, preferencia confirmada)? Se sim, grave um resumo com o porque via store_memory (memory-mcp)."}}
EOF
else
  printf '{}'
fi
