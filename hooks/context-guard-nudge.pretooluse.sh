#!/usr/bin/bash
# PreToolUse: lembrete pos-N-edits para atualizar STATE.md (context-guard).
# Migra de PostToolUse (mudo) -> PreToolUse (additionalContext sem bloquear,
# validado por principles-inject). Custo por chamada: ~5ms.
set -euo pipefail
trap 'status=$?; "$(dirname "$0")/observe-error.sh" "PreToolUse" "hook_exit_$status" "context-guard pretooluse nudge falhou"; exit "$status"' ERR

THRESHOLD="${AGENT_SYNC_NUDGE_THRESHOLD:-120}"
STATE_DIR="${TMPDIR:-/tmp}/agent-sync-nudge"
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

if command -v memory-mcp >/dev/null 2>&1; then
  printf '%s\n' \
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
    '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
    "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"record_event\",\"arguments\":{\"agent\":\"claude-code\",\"kind\":\"guard_nudge\",\"note\":\"context-guard #$count\",\"session_id\":\"$session_id\"}}}" \
    | memory-mcp -db "${AGENT_SYNC_MEMORY_DB:-${XDG_CACHE_HOME:-$HOME/.cache}/agent-sync/memory.db}" >/dev/null 2>&1 || true
fi

cat <<EOF
{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"[context-guard #$count] Voce ja atualizou o STATE.md nesta sessao? Se acumulou decisoes/hipoteses/artifacts desde o ultimo update, registre antes da proxima tool (regra de memoria enxuta: max 2-3 linhas, fatos acionaveis)."}}EOF
