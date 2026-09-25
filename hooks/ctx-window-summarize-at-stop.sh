#!/usr/bin/env bash
# Auto-summarize em Stop/agent-stop: dispara ctx-window summarize em background
# quando (a) o contexto tem >= MIN_TOOL_CALLS tool calls desde o último summary
# ou (b) o summary.md tem mais de MAX_AGE_HOURS horas.
#
# NAO bloqueia o encerramento da sessão: roda em background e exit 0 sempre.
# Wirado em Claude Code (Stop), Codex (Stop), Antigravity (Stop), Cursor (stop)
# e OpenCode (plugin TS).
#
# Variáveis:
#   AGENT_SYNC_CTX_SUMMARIZE_AT_STOP_DISABLE=1   desativa o hook (no-op)
#   AGENT_SYNC_CTX_SUMMARIZE_MIN_TOOL_CALLS=100 threshold de tool calls
#   AGENT_SYNC_CTX_SUMMARIZE_MAX_AGE_HOURS=4     idade máxima do summary.md
#   AGENT_SYNC_CTX_WINDOW_BIN=<path>             binário ctx-window (default: PATH)
#   AGENT_SYNC_CTX_SUMMARIZE_AT_STOP_BG=1        roda em background (default)

set -uo pipefail

if [ "${AGENT_SYNC_CTX_SUMMARIZE_AT_STOP_DISABLE:-0}" = "1" ]; then
  exit 0
fi

MIN_TOOL_CALLS="${AGENT_SYNC_CTX_SUMMARIZE_MIN_TOOL_CALLS:-100}"
MAX_AGE_HOURS="${AGENT_SYNC_CTX_SUMMARIZE_MAX_AGE_HOURS:-4}"

BIN="${AGENT_SYNC_CTX_WINDOW_BIN:-}"
if [ -z "$BIN" ]; then
  BIN="$(command -v ctx-window 2>/dev/null || true)"
fi

# Sem binário: silencioso (best-effort). Stop não pode falhar por causa disso.
if [ -z "$BIN" ] || [ ! -x "$BIN" ]; then
  exit 0
fi

# Decide se precisa summarizar: tool calls OU idade do summary.
# Tool calls: contamos entradas no transcript se disponível (Claude Code passa
# transcript_path no payload); fallback para contador local estilo memory-nudge.
input="$(cat)"
session_id="$(printf '%s' "$input" \
  | { grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
if [ -z "$session_id" ]; then
  session_id="$(printf '%s' "$input" \
    | { grep -o '"conversation_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
    | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
fi
session_id="${session_id:-default}"

# Idade do summary.md (se existir) em horas.
root="$(pwd)"
summary="$root/.agent-sync/summary.md"
needs=0
if [ -f "$summary" ]; then
  age_secs=$(( $(date +%s) - $(stat -c %Y "$summary" 2>/dev/null || stat -f %m "$summary" 2>/dev/null || echo 0) ))
  age_hours=$(( age_secs / 3600 ))
  if [ "$age_hours" -ge "$MAX_AGE_HOURS" ]; then
    needs=1
  fi
else
  # summary.md ausente = precisa criar.
  needs=1
fi

# Tool calls desde o último summary: contador local simples (state dir partilhado).
STATE_DIR="${TMPDIR:-/tmp}/agent-sync-ctx-window-stop"
mkdir -p "$STATE_DIR"
counter_file="$STATE_DIR/$session_id.count"
count=0
[ -f "$counter_file" ] && count="$(cat "$counter_file")"
count=$((count + 1))
printf '%s' "$count" > "$counter_file"

if [ "$count" -ge "$MIN_TOOL_CALLS" ]; then
  needs=1
fi

if [ "$needs" -ne 1 ]; then
  exit 0
fi

# Reset contador + dispara summarize em background.
printf '0' > "$counter_file"

run_summarize() {
  if AGENT_SYNC_HOME="${AGENT_SYNC_HOME:-}" "$BIN" summarize -root "$root" >/dev/null 2>&1; then
    # Mirror best-effort para memory-mcp.
    if command -v memory-mcp >/dev/null 2>&1; then
      printf '%s\n' \
        '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
        '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
        "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"record_event\",\"arguments\":{\"agent\":\"agent-sync\",\"kind\":\"task_completed\",\"note\":\"ctx-window summarize auto (stop)\",\"project_path\":\"$root\"}}}" \
        | memory-mcp -db "${AGENT_SYNC_MEMORY_DB:-${XDG_CACHE_HOME:-$HOME/.cache}/agent-sync/memory.db}" >/dev/null 2>&1 || true
    fi
  fi
}

if [ "${AGENT_SYNC_CTX_SUMMARIZE_AT_STOP_BG:-1}" = "1" ]; then
  ( run_summarize ) &
  disown || true
else
  run_summarize
fi

exit 0