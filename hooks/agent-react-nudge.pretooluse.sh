#!/usr/bin/bash
# PreToolUse: lembrete de validacao de hipotese antes da PROXIMA tool
# (agent-react). Migra de PostToolUse (mudo) -> PreToolUse (aceita
# additionalContext sem bloquear tool call, validado por principles-inject).
#
# Custo por chamada: ~5ms (cat + grep + sed + arithmetic + write).
# Emissao do nudge so quando count % THRESHOLD == 0; demais chamadas
# retornam {} em ~3ms (sem cat final).
set -euo pipefail
trap 'status=$?; "$(dirname "$0")/observe-error.sh" "PreToolUse" "hook_exit_$status" "agent-react pretooluse nudge falhou"; exit "$status"' ERR

THRESHOLD="${AGENT_SYNC_REACT_NUDGE_THRESHOLD:-15}"
STATE_DIR="${TMPDIR:-/tmp}/agent-sync-react-nudge"
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

# Caminho comum: nao eh multiplo de THRESHOLD -> sai rapido com {}
if [ "$((count % THRESHOLD))" -ne 0 ]; then
  printf '{}'
  exit 0
fi

# So agora (1 a cada THRESHOLD chamadas): emite additionalContext e mirror.
if command -v memory-mcp >/dev/null 2>&1; then
  printf '%s\n' \
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
    '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
    "{\"jsonrpc":"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"record_event\",\"arguments\":{\"agent\":\"claude-code\",\"kind\":\"guard_nudge\",\"note\":\"agent-react #$count\",\"session_id\":\"$session_id\"}}}" \
    | memory-mcp -db "${AGENT_SYNC_MEMORY_DB:-${XDG_CACHE_HOME:-$HOME/.cache}/agent-sync/memory.db}" >/dev/null 2>&1 || true
fi

cat <<EOF
{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"[agent-react #$count] Antes desta tool: voce VALIDOU sua hipotese atual? Citou evidencia (file:line, comando, teste)? Se nao, faca a leitura/comando antes ou pergunte o passo concreto ao usuario. Sem evidencia = sem afirmação."}}
EOF
