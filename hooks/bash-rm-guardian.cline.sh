#!/usr/bin/env bash
# bash-rm-guardian para Cline v3 (PreToolUse).
# Cline invoca hooks em ~/.cline/hooks/PreToolUse via stdin (payload JSON)
# e stdout (resposta JSON com `cancel` opcional).
#
# Formato payload Cline (validado empiricamente): {"tool":"Bash","input":{...},"context":{...}}
# Formato output: {"cancel":false,"context":"<texto>","error":""}
#
# Comportamento (A-74): NUNCA cancela (decisão do user). Emite context com
# blockers do audit_removal quando rm/rmdir/mv destrutivo em dir com refs.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CORE="$SCRIPT_DIR/bash-rm-guardian.sh"

input="$(cat)"
# Cline v3 format: tool + input.command (tool_input-like em outras CLIs)
command_line="$(printf '%s' "$input" | grep -oE '"command"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed -E 's/.*:[[:space:]]*"(.*)"/\1/' || true)"

if [ -z "$command_line" ]; then
  # tenta outros patterns comuns (tool_input.command)
  command_line="$(printf '%s' "$input" | grep -oE '"tool_input"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed 's/.*"tool_input".*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' || true)"
fi

if [ -z "$command_line" ]; then
  printf '{"cancel":false,"context":"","error":""}'
  exit 0
fi

# Desescapar minimo
command_line="$(printf '%s' "$command_line" | sed -e 's/\\"/"/g' -e 's/\\\\/\\/g')"

# Roda core para extrair blockers
result="$(AGENT_SYNC_ROOT="${AGENT_SYNC_ROOT:-$(pwd)}" bash "$CORE" analyze "$command_line" 2>/dev/null || true)"
blockers="$(printf '%s' "$result" | grep -oE '"blockers":\[[^]]*\]' | sed 's/"blockers"://' || true)"
target="$(printf '%s' "$result" | grep -oE '"target":"[^"]*"' | sed 's/"target":"//;s/"$//' || true)"

if [ -z "$blockers" ] || [ "$blockers" = "[]" ]; then
  printf '{"cancel":false,"context":"","error":""}'
  exit 0
fi

summary="$(printf '%s' "$blockers" | grep -oE '"[^"]+"' | sed 's/"//g' | tr '\n' ';' | sed 's/;$//')"
message="bash-rm-guardian: \\\"$target\\\" tem refs ativas no repo. Revise antes de prosseguir: $summary"

# Cline v3 output schema (validado empiricamente): {cancel, context, error}
# context vira mensagem visível para o modelo
jq -n \
  --arg msg "$message" \
  '{cancel:false, context:$msg, error:""}'
