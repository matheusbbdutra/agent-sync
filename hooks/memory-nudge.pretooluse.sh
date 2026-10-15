#!/usr/bin/bash
# PreToolUse unificado (fold B2 — regra §4: 1 core + deteccao de contrato):
#   - Claude PreToolUse (session_id, sem invocationNum): contador proprio em
#     $TMPDIR, nudge via additionalContext a cada THRESHOLD.
#   - Antigravity PreInvocation (invocationNum nativo): sem estado proprio,
#     nudge via stderr + '{}' no stdout (additionalContext/injectSteps aqui
#     faz o Gemini CLI interpretar como deny — verificado em runtime
#     2026-09-22; ver docs/investigations/agy-memory-nudge-loop.md).
# Custo por chamada: ~5ms.
set -euo pipefail
trap 'status=$?; "$(dirname "$0")/observe-error.sh" "PreToolUse" "hook_exit_$status" "memory pretooluse nudge falhou"; exit "$status"' ERR

THRESHOLD="${AGENT_SYNC_MEMORY_NUDGE_THRESHOLD:-25}"
STATE_DIR="${TMPDIR:-/tmp}/agent-sync-memory-nudge"
mkdir -p "$STATE_DIR"

input="$(cat)"
invocation_num="$(printf '%s' "$input" | { grep -o '"invocationNum"[[:space:]]*:[[:space:]]*[0-9]*' || true; } | head -n1 | { grep -o '[0-9]*$' || true; })"

agent="claude-code"
session_id=""
if [ -n "$invocation_num" ]; then
  # Antigravity PreInvocation: contador nativo, sem estado proprio.
  count="$invocation_num"
  agent="antigravity"
else
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
fi

if [ "$count" -le 0 ] || [ "$((count % THRESHOLD))" -ne 0 ]; then
  printf '{}'
  exit 0
fi

# Mirror best-effort para memory-mcp (record_event kind=guard_nudge).
# Falha NAO bloqueia o hook.
if command -v memory-mcp >/dev/null 2>&1; then
  if [ "$agent" = "antigravity" ]; then
    printf '%s\n' \
      '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
      '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
      "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"record_event\",\"arguments\":{\"agent\":\"antigravity\",\"kind\":\"guard_nudge\",\"note\":\"memory-nudge #$count (antigravity)\"}}}" \
      | memory-mcp -db "${AGENT_SYNC_MEMORY_DB:-${XDG_CACHE_HOME:-$HOME/.cache}/agent-sync/memory.db}" >/dev/null 2>&1 || true
  else
    printf '%s\n' \
      '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
      '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
      "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"record_event\",\"arguments\":{\"agent\":\"claude-code\",\"kind\":\"guard_nudge\",\"note\":\"memory-nudge #$count\",\"session_id\":\"$session_id\",\"source\":\"auto-hook\",\"retention\":\"scratch\",\"scratch\":true}}}" \
      | memory-mcp -db "${AGENT_SYNC_MEMORY_DB:-${XDG_CACHE_HOME:-$HOME/.cache}/agent-sync/memory.db}" >/dev/null 2>&1 || true
  fi
fi

# Antigravity PreInvocation nao aceita additionalContext: nudge via stderr.
if [ "$agent" = "antigravity" ]; then
  printf '[agent-sync] Algo nesta sessao que deveria virar memoria? (correcao do usuario, decisao de projeto, preferencia confirmada). Se sim, grave via store_memory (memory-mcp).\n' >&2
  printf '{}'
  exit 0
fi

cat <<EOF
{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"[memory #$count] Esta decisao/observacao deve virar regra persistente? Se sim, registre em rules/global-rules.md ou skill canonica. Se for apenas contexto de sessao, marque como tal."}}
EOF
