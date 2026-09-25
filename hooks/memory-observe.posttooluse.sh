#!/usr/bin/env bash
# memory-observe.posttooluse.sh
#
# Hook cross-CLI pós-ferramenta (PostToolUse) para observação contínua de memória (ADR-automated-memory-observation-pipeline).
# Registra eventos relevantes em background (BG=1) no buffer efêmero da sessão.
# Execução 100% não-bloqueante: exit 0 sempre.

set -uo pipefail

# A-66 patch (c) / A-71: SCOPE generaliza DISABLE=1. Aceita:
#   off         → sai cedo sem gravar
#   operational → grava ações operacionais (Edit/Write/MultiEdit/NotebookEdit/Bash + erros) (default NOVO A-71)
#   high-signal → grava só mutações + erros (compatível com A-66)
#   all         → grava tudo (desliga filtro allowlist em b)
# Back-compat: AGENT_SYNC_MEMORY_OBSERVE_DISABLE=1 ainda equivale a off.
OBSERVE_SCOPE="${AGENT_SYNC_MEMORY_OBSERVE_SCOPE:-operational}"
if [ "${AGENT_SYNC_MEMORY_OBSERVE_DISABLE:-0}" = "1" ]; then
  OBSERVE_SCOPE="off"
fi
case "$OBSERVE_SCOPE" in
  off) exit 0 ;;
  all|high-signal|operational) : ;;
  *) OBSERVE_SCOPE="operational" ;;
esac

MEM_BIN="${AGENT_SYNC_MEMORY_BIN:-}"
if [ -z "$MEM_BIN" ]; then
  MEM_BIN="$(command -v memory-mcp 2>/dev/null || true)"
fi
if [ -z "$MEM_BIN" ] && [ -x "$HOME/.local/bin/memory-mcp" ]; then
  MEM_BIN="$HOME/.local/bin/memory-mcp"
fi

if [ -z "$MEM_BIN" ] || [ ! -x "$MEM_BIN" ]; then
  exit 0
fi

input="$(cat 2>/dev/null || true)"

session_id="$(printf '%s' "$input" \
  | { grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
if [ -z "$session_id" ]; then
  session_id="$(printf '%s' "$input" \
    | { grep -o '"conversation_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
    | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
fi
session_id="${session_id:-default}"

tool_name="$(printf '%s' "$input" \
  | { grep -o '"tool_name"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
if [ -z "$tool_name" ]; then
  tool_name="$(printf '%s' "$input" \
    | { grep -o '"tool"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
    | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
fi
tool_name="${tool_name:-tool}"

status="$(printf '%s' "$input" \
  | { grep -o '"status"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
status="${status:-ok}"

# A-66 patch (b) / A-71: filtro de alto-sinal / operacional. SCOPE=all pula o filtro.
# Allowlist alinhada com nudgeIfFilters em internal/hooks/hooks_constants.go:19
# + Bash (comando shell executado é alto-sinal por intencao/mutacao).
# Excecao: status!=ok sempre grava, mesmo que tool fora da allowlist
# (erros sao alto-sinal independente da tool).
if [ "$OBSERVE_SCOPE" = "operational" ] || [ "$OBSERVE_SCOPE" = "high-signal" ]; then
  case "$tool_name" in
    Edit|Write|MultiEdit|NotebookEdit|Bash)
      : # allowlist — continua para gravar
      ;;
    *)
      if [ "$status" = "ok" ]; then
        printf '{}'
        exit 0
      fi
      : # status!=ok → alto-sinal por erro, continua para gravar
      ;;
  esac
fi

note="$(printf '%s' "$input" \
  | { grep -o '"error"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
if [ -z "$note" ]; then
  # Se não houve erro explícito, captura comando executado ou nota resumida
  note="$(printf '%s' "$input" \
    | { grep -o '"command"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
    | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/' | cut -c1-200)"
fi
if [ -z "$note" ]; then
  note="executou $tool_name"
fi

# Disparo em background sem travar o turno (ADR A-71: source=auto-hook, kind=action, retention=scratch)
("$MEM_BIN" buffer-record \
  -session "$session_id" \
  -tool "$tool_name" \
  -status "$status" \
  -note "$note" \
  -source "auto-hook" \
  -kind "action" \
  -retention "scratch" \
  -path "$PWD" >/dev/null 2>&1 || true) &

printf '{}'
exit 0
