#!/usr/bin/env bash
# token-nudge.check.sh
#
# Hook cross-CLI wirado no evento postToolUse de Claude Code, Codex,
# Antigravity e Cursor (ADR-token-nudge-contract Decisao 3).
# Implementa o contrato de token nudge:
#   1. Le payload postToolUse do stdin (transcript_path, session_id, etc).
#   2. Chama `agent-sync budget nudge -actor <cli> -transcript <path>`.
#   3. CLI decide should_nudge baseado em utilization_pct >= threshold
#      OU heuristica legada (tool calls, summary age).
#   4. Emite injectSteps[ephemeralMessage] no payload JSON quando
#      should_nudge=true.
#
# NAO bloqueia a tool call: erros viram no-op silencioso (postToolUse
# e observacional). Exit 0 sempre.
#
# Variaveis:
#   AGENT_SYNC_AGENT_KIND=<claude|codex|agy|cursor>  CLI origem
#   AGENT_SYNC_TOKEN_NUDGE_BIN=<path>                 binario agent-sync (default: PATH)
#   AGENT_SYNC_TOKEN_NUDGE_THRESHOLD=80               threshold % para nudge
#   AGENT_SYNC_TOKEN_NUDGE_DISABLE=1                  no-op

set -uo pipefail

if [ "${AGENT_SYNC_TOKEN_NUDGE_DISABLE:-0}" = "1" ]; then
  exit 0
fi

CLI_KIND="${AGENT_SYNC_AGENT_KIND:-claude}"
BIN="${AGENT_SYNC_TOKEN_NUDGE_BIN:-}"
if [ -z "$BIN" ]; then
  BIN="$(command -v agent-sync 2>/dev/null || true)"
fi

# Sem binario: silencioso.
if [ -z "$BIN" ] || [ ! -x "$BIN" ]; then
  exit 0
fi

input="$(cat)"

# Extracao defensiva: campo pode estar ausente.
# `{ ... || true; }` neutraliza set -e quando grep nao encontra match
# (PreInvocation vem sem stdin -> grep sempre falha -> hook exit 1 ->
# Antigravity bloqueia tool call com 'hook de seguranca do terminal').
session_id="$(printf '%s' "$input" \
  | { grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
session_id="${session_id:-unknown}"

# Codex: model nativo no payload. Claude/Antigravity/Cursor: ausente.
model="$(printf '%s' "$input" \
  | { grep -o '"model"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"

transcript_path="$(printf '%s' "$input" \
  | { grep -o '"transcript_path"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"

threshold="${AGENT_SYNC_TOKEN_NUDGE_THRESHOLD:-80}"

# Constroi args para `agent-sync budget nudge`.
args=("-actor" "$CLI_KIND" "-session-id" "$session_id" "-threshold" "$threshold")
if [ -n "$model" ]; then
  args+=("-model" "$model")
fi
if [ -n "$transcript_path" ]; then
  args+=("-transcript" "$transcript_path")
fi

# Chama CLI e captura decisao.
status_json="$("$BIN" budget nudge "${args[@]}" 2>/dev/null || true)"

# Se vazio ou erro: silent no-op.
if [ -z "$status_json" ]; then
  exit 0
fi

# Decidir com base em should_nudge (extraido via grep simples).
should_nudge="$(printf '%s' "$status_json" \
  | { grep -o '"should_nudge"[[:space:]]*:[[:space:]]*true' || true; } | head -n1)"

if [ -z "$should_nudge" ]; then
  exit 0
fi

# Extrair utilization_pct e trigger para mensagem.
util_pct="$(printf '%s' "$status_json" \
  | { grep -o '"utilization_pct"[[:space:]]*:[[:space:]]*[0-9]*' || true; } \
  | head -n1 | grep -oE '[0-9]+$' || echo '?')"
trigger="$(printf '%s' "$status_json" \
  | { grep -o '"trigger"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/' || echo 'unknown')"

# Contrato por CLI (verificado em runtime 2026-09-22 no antigravity-cli):
# - Claude/Codex/Cursor: additionalContext e injetado na conversa.
# - Antigravity PostToolUse: resultado so tem overwriteResult; qualquer
#   outro campo vira erro de protojson e o hook falha. Como o nudge ja
#   foi registrado via budget + memory, emitir {} (silent) no Antigravity
#   em vez de bloquear a sessao por causa de schema.
case "$input" in
  *'"toolCall"'*)
    printf '{}'
    ;;
  *)
    cat <<EOF
{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"[agent-sync] Contexto em ${util_pct}% de utilizacao (trigger=$trigger). Considere rodar 'ctx-window summarize' para manter handoff atualizado."}}
EOF
    ;;
esac

exit 0
