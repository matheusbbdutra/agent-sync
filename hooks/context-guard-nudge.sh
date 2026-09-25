#!/usr/bin/bash
# Lembrete pos-ferramenta: a cada N chamadas na mesma sessao, cobra o carregamento
# da skill context-guard e a atualizacao do STATE.md (reforco do enforcement por prompt).
set -euo pipefail
trap 'status=$?; "$(dirname "$0")/observe-error.sh" "PostToolUse" "hook_exit_$status" "context-guard nudge falhou"; exit "$status"' ERR

THRESHOLD="${AGENT_SYNC_NUDGE_THRESHOLD:-120}"
STATE_DIR="${TMPDIR:-/tmp}/agent-sync-nudge"
mkdir -p "$STATE_DIR"

# `{ ... || true; }` neutraliza set -e quando grep nao encontra match
# (PreInvocation vem sem stdin -> grep sempre falha -> hook exit 1 ->
# Antigravity bloqueia tool call com 'hook de seguranca do terminal').
input="$(cat)"
session_id="$(printf '%s' "$input" | { grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
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
      "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"record_event\",\"arguments\":{\"agent\":\"claude-code\",\"kind\":\"guard_nudge\",\"note\":\"context-guard #$count\",\"session_id\":\"$session_id\"}}}" \
      | memory-mcp -db "${AGENT_SYNC_MEMORY_DB:-${XDG_CACHE_HOME:-$HOME/.cache}/agent-sync/memory.db}" >/dev/null 2>&1 || true
  fi
  # Schema correto do Antigravity PreInvocation: emite {} (vazio).
  # Verificado em runtime 2026-09-22: injectSteps + ephemeralMessage
  # faz o Gemini CLI interpretar como `tool call denied by pre-tool
  # hook`. Log do nudge via memory-mcp acima.
  printf '{}'
else
  printf '{}'
fi
