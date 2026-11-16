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

# Mensagem vem do Go (fonte unica, -message): vazia = sem nudge.
# stderr vai para arquivo temporario: `2>/dev/null` engolia stderr E o exit
# code, entao falha do budget nudge nunca chegava em hooks/errors.jsonl
# (A-94). O hook segue exit 0 — postToolUse e observacional, nunca bloqueia.
budget_err="$(mktemp)"
message="$("$BIN" budget nudge "${args[@]}" -message 2>"$budget_err")"
budget_status=$?
if [ "$budget_status" -ne 0 ]; then
  AGENT_SYNC_CLI="$CLI_KIND" HOOK_STATUS=fail \
    "$(dirname "$0")/observe-error.sh" "PostToolUse" "budget_nudge_failed" \
    "$(head -c 512 "$budget_err")" 2>/dev/null || true
  message=""
fi
rm -f "$budget_err"

if [ -z "$message" ]; then
  exit 0
fi

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
{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"${message}"}}
EOF
    ;;
esac

exit 0
