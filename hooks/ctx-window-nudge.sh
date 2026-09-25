#!/usr/bin/env bash
# Nudge combinado: dispara quando (a) >= MIN_TOOL_CALLS tool calls desde o
# início da sessão OU (b) summary.md tem > MAX_AGE_HOURS horas. Injeta
# lembrete pra considerar `ctx-window summarize`.
#
# Wirado como PostToolUse nos 5 alvos (low-fadiga: threshold alto, default 80).
#
# Variáveis:
#   AGENT_SYNC_CTX_WINDOW_NUDGE_DISABLE=1   desativa
#   AGENT_SYNC_CTX_WINDOW_NUDGE_MIN_TOOL_CALLS=80
#   AGENT_SYNC_CTX_WINDOW_NUDGE_MAX_AGE_HOURS=4
#   AGENT_SYNC_CTX_WINDOW_NUDGE_STATE_DIR=  override do state dir

set -euo pipefail
trap 'status=$?; "$(dirname "$0")/observe-error.sh" "PostToolUse" "hook_exit_$status" "ctx-window nudge falhou"; exit "$status"' ERR

if [ "${AGENT_SYNC_CTX_WINDOW_NUDGE_DISABLE:-0}" = "1" ]; then
  printf '{}'
  exit 0
fi

MIN_TOOL_CALLS="${AGENT_SYNC_CTX_WINDOW_NUDGE_MIN_TOOL_CALLS:-80}"
MAX_AGE_HOURS="${AGENT_SYNC_CTX_WINDOW_NUDGE_MAX_AGE_HOURS:-4}"
STATE_DIR="${AGENT_SYNC_CTX_WINDOW_NUDGE_STATE_DIR:-${TMPDIR:-/tmp}/agent-sync-ctx-window-nudge}"
mkdir -p "$STATE_DIR"

# `{ ... || true; }` neutraliza set -e quando grep nao encontra match
# (PreInvocation vem sem stdin -> grep sempre falha -> hook exit 1 ->
# Antigravity bloqueia tool call com 'hook de seguranca do terminal').
input="$(cat)"
session_id="$(printf '%s' "$input" \
  | { grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
session_id="${session_id:-default}"

counter_file="$STATE_DIR/$session_id.count"
count=0
[ -f "$counter_file" ] && count="$(cat "$counter_file")"
count=$((count + 1))
printf '%s' "$count" > "$counter_file"

trigger=0
reason=""

# Regra 1: tool calls >= MIN_TOOL_CALLS
if [ "$count" -ge "$MIN_TOOL_CALLS" ]; then
  trigger=1
  reason="tool_calls>=${MIN_TOOL_CALLS}"
fi

# Regra 2: summary.md velho ou ausente
root="$(pwd)"
summary="$root/.agent-sync/summary.md"
if [ -f "$summary" ]; then
  age_secs=$(( $(date +%s) - $(stat -c %Y "$summary" 2>/dev/null || stat -f %m "$summary" 2>/dev/null || echo 0) ))
  age_hours=$(( age_secs / 3600 ))
  if [ "$age_hours" -ge "$MAX_AGE_HOURS" ]; then
    trigger=1
    reason="${reason:+$reason; }summary_age=${age_hours}h>=${MAX_AGE_HOURS}h"
  fi
else
  trigger=1
  reason="${reason:+$reason; }summary_missing"
fi

if [ "$trigger" -ne 1 ]; then
  printf '{}'
  exit 0
fi

# Mirror best-effort para memory-mcp.
if command -v memory-mcp >/dev/null 2>&1; then
  printf '%s\n' \
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
    '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
    "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"record_event\",\"arguments\":{\"agent\":\"claude-code\",\"kind\":\"guard_nudge\",\"note\":\"ctx-window-nudge #$count ($reason)\",\"session_id\":\"$session_id\"}}}" \
    | memory-mcp -db "${AGENT_SYNC_MEMORY_DB:-${XDG_CACHE_HOME:-$HOME/.cache}/agent-sync/memory.db}" >/dev/null 2>&1 || true
fi

# Schema correto do Antigravity PreInvocation/PostToolUse: {"decision":"allow"}
# vazio NAO bloqueia a tool call. Schema anterior (injectSteps +
# ephemeralMessage) faz o Gemini CLI interpretar a saida como block
# com `tool call denied by pre-tool hook` (verificado em runtime
# 2026-09-22). Por enquanto emitimos {} (silent nudge via logging
# best-effort no memory-mcp) ate descobrir schema que injetaria
# contexto adicional sem bloquear.
printf '{}'
