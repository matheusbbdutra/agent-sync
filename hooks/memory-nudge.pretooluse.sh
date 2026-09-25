#!/usr/bin/bash
# PreToolUse: lembrete pos-N-edits para consolidar na memoria persistida
# (memory-mcp). Migra de PostToolUse (mudo) -> PreToolUse (additionalContext
# sem bloquear). Custo por chamada: ~5ms.
set -euo pipefail
trap 'status=$?; "$(dirname "$0")/observe-error.sh" "PreToolUse" "hook_exit_$status" "memory pretooluse nudge falhou"; exit "$status"' ERR

THRESHOLD="${AGENT_SYNC_MEMORY_NUDGE_THRESHOLD:-25}"
STATE_DIR="${TMPDIR:-/tmp}/agent-sync-memory-nudge"
mkdir -p "$STATE_DIR"

input="$(cat)"
session_id="$(printf '%s' "$input" \
  | { grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 \
  | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
session_id="${session_id:-default}"

counter_file="$STATE_DIR/$session_id.count"
count=0
[ -f "$counter_file" ] && count="$(cat "$counter_file")"
count=$((count + 1))
printf '%s' "$count" > "$counter_file"

if [ "$((count % THRESHOLD))" -ne 0 ]; then
  printf '{}'
  exit 0
fi

cat <<EOF
{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"[memory #$count] Esta decisao/observacao deve virar regra persistente? Se sim, registre em rules/global-rules.md ou skill canonica. Se for apenas contexto de sessao, marque como tal."}}EOF
