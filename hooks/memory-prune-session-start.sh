#!/usr/bin/env bash
# memory-prune-session-start.sh
#
# A-68: auto-prune scratch + alerta staleness no SessionStart. NUNCA bloqueia
# turno (exit 0 sempre; falhas logadas em stderr e ignoradas).
#
# Wirado como SessionStart em 4 CLIs: Claude Code, Codex, Antigravity (via
# PreInvocation como proxy) e Cursor. OpenCode fica A-69 (sem hook nativo
# SessionStart em @opencode/plugin v2.0.11).
#
# Variáveis:
#   AGENT_SYNC_AUTO_PRUNE=0           opt-out (default ON)
#   AGENT_SYNC_AUTO_PRUNE_DAYS=7      idade minima para prune (alinhado memory-mcp prune)
#   AGENT_SYNC_AUTO_PRUNE_STALE_PCT=80 alerta stderr quando scratch% >= pct
#   CLAUDE_SESSION_ID / CODEX_SESSION_ID  id da sessao (dedup 24h)

set -uo pipefail

# Opt-out
if [ "${AGENT_SYNC_AUTO_PRUNE:-1}" = "0" ]; then
  printf '{}'
  exit 0
fi

# Resolve memory-mcp (padrao identico a memory-observe.posttooluse.sh:14-24)
MEM_BIN="${AGENT_SYNC_MEMORY_BIN:-}"
if [ -z "$MEM_BIN" ]; then
  MEM_BIN="$(command -v memory-mcp 2>/dev/null || true)"
fi
if [ -z "$MEM_BIN" ] && [ -x "$HOME/.local/bin/memory-mcp" ]; then
  MEM_BIN="$HOME/.local/bin/memory-mcp"
fi

DAYS="${AGENT_SYNC_AUTO_PRUNE_DAYS:-7}"
STALE_PCT="${AGENT_SYNC_AUTO_PRUNE_STALE_PCT:-80}"

# Idempotencia: rodar no maximo 1x por sessao a cada 24h
STATE_DIR="${TMPDIR:-/tmp}/agent-sync-memory-prune-session-start"
mkdir -p "$STATE_DIR"
SESSION_ID="${CLAUDE_SESSION_ID:-${CODEX_SESSION_ID:-default}}"
LAST_FILE="$STATE_DIR/$SESSION_ID.last"
if [ -f "$LAST_FILE" ]; then
  last_ts="$(cat "$LAST_FILE" 2>/dev/null || echo 0)"
  now_ts="$(date +%s)"
  if [ $((now_ts - last_ts)) -lt 86400 ]; then
    printf '{}'
    exit 0
  fi
fi

if [ -z "$MEM_BIN" ] || [ ! -x "$MEM_BIN" ]; then
  printf '{}'
  exit 0
fi

# 1) Stats em background para detectar staleness.
# `+()` subshell + `|| true` neutraliza set -u em grep nao-casando.
STATS_JSON="$( ("$MEM_BIN" stats -json 2>/dev/null || true) )"
if [ -n "$STATS_JSON" ]; then
  total="$(printf '%s' "$STATS_JSON" \
    | { grep -o '"total"[[:space:]]*:[[:space:]]*[0-9]*' || true; } \
    | head -n1 | sed -E 's/.*:[[:space:]]*//')"
  scratch="$(printf '%s' "$STATS_JSON" \
    | { grep -o '"scratch"[[:space:]]*:[[:space:]]*[0-9]*' || true; } \
    | head -n1 | sed -E 's/.*:[[:space:]]*//')"
  if [ -n "$total" ] && [ -n "$scratch" ] && [ "$total" -gt 0 ]; then
    pct=$(( (scratch * 100) / total ))
    if [ "$pct" -ge "$STALE_PCT" ]; then
      printf '[agent-sync memory-prune] WARNING: scratch=%d%% (%d/%d) >= threshold=%d%% - gc best-effort em background\n' "$pct" "$scratch" "$total" "$STALE_PCT" >&2
      # A-71: registra alerta como evento state_render efêmero
      printf '%s\n' \
        '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
        '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
        "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"record_event\",\"arguments\":{\"agent\":\"agent-sync\",\"kind\":\"state_render\",\"note\":\"memory-prune-session-start alerta staleness: scratch=${pct}% (${scratch}/${total})\",\"session_id\":\"$SESSION_ID\",\"source\":\"auto-hook\",\"retention\":\"scratch\",\"scratch\":true}}}" \
        | "$MEM_BIN" -db "${AGENT_SYNC_MEMORY_DB:-${XDG_CACHE_HOME:-$HOME/.cache}/agent-sync/memory.db}" >/dev/null 2>&1 || true
    fi
  fi
fi

# 2) Prune em background (real delete, A-67 default safer requires --confirm).
# `(...) &` isola falhas; `>/dev/null 2>&1` evita poluir stdout do turno.
( "$MEM_BIN" prune -older-than "${DAYS}h" -confirm -json >/dev/null 2>&1 || true ) &

# 3) Atualizar last timestamp (depois de disparar prune, antes de sair)
date +%s > "$LAST_FILE"

printf '{}'
exit 0