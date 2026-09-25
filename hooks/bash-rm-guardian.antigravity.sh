#!/usr/bin/env bash
# bash-rm-guardian para Antigravity (PreToolUse).
# Emite injectSteps warn quando o audit detecta refs ativas. NÃO bloqueia.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CORE="$SCRIPT_DIR/bash-rm-guardian.sh"

input="$(cat)"
command_line="$(printf '%s' "$input" | { grep -o '"CommandLine"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } | head -n1 | sed -E 's/.*:[[:space:]]*"(.*)"/\1/' || true)"

if [ -z "$command_line" ]; then
  printf '{"decision":"allow"}'
  exit 0
fi

# PreInvocation sem toolCall → emite {} (sem decision)
case "$input" in
  *'"toolCall"'*) ;;
  *) printf '{}'; exit 0 ;;
esac

# Roda core para extrair blockers
result="$(AGENT_SYNC_ROOT="${AGENT_SYNC_ROOT:-$(pwd)}" bash "$CORE" analyze "$command_line" 2>/dev/null || true)"
blockers="$(printf '%s' "$result" | grep -oE '"blockers":\[[^]]*\]' | sed 's/"blockers"://' || true)"
target="$(printf '%s' "$result" | grep -oE '"target":"[^"]*"' | sed 's/"target":"//;s/"$//' || true)"

# Sem target destrutivo ou sem blockers → allow silencioso
if [ -z "$blockers" ] || [ "$blockers" = "[]" ]; then
  printf '{"decision":"allow"}'
  exit 0
fi

# Com blockers → allow + injectSteps com resumo
# Monta mensagem legível a partir dos blockers
summary="$(printf '%s' "$blockers" | grep -oE '"[^"]+"' | sed 's/"//g' | tr '\n' ';' | sed 's/;$//')"

# injectSteps: shape aceita por Antigravity
jq -n \
  --arg decision "allow" \
  --arg summary "$summary" \
  --arg target "$target" \
  '{decision:$decision, injectSteps:[{label:"bash-rm-guardian", message:("bash-rm-guardian: \"" + $target + "\" tem refs ativas no repo. Revise antes de prosseguir: " + $summary)}]}'
