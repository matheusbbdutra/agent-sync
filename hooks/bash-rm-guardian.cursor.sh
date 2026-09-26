#!/usr/bin/env bash
# bash-rm-guardian para Cursor (beforeShellExecution).
# Emite permission=allow + agent_message warn quando o audit detecta refs.
# NÃO bloqueia (não usa deny).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CORE="$SCRIPT_DIR/bash-rm-guardian.sh"

input="$(cat)"
command_line="$(printf '%s' "$input" | grep -o '"command"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed -E 's/.*:[[:space:]]*"(.*)"/\1/' || true)"

if [ -z "$command_line" ]; then
  printf '{}'
  exit 0
fi

# Desescapa minimo
command_line="$(printf '%s' "$command_line" | sed -e 's/\\"/"/g' -e 's/\\\\/\\/g')"

result="$(AGENT_SYNC_ROOT="${AGENT_SYNC_ROOT:-$(pwd)}" bash "$CORE" analyze "$command_line" 2>/dev/null || true)"
blockers="$(printf '%s' "$result" | grep -oE '"blockers":\[[^]]*\]' | sed 's/"blockers"://' || true)"
target="$(printf '%s' "$result" | grep -oE '"target":"[^"]*"' | sed 's/"target":"//;s/"$//' || true)"

if [ -z "$blockers" ] || [ "$blockers" = "[]" ]; then
  printf '{"permission":"allow"}'
  exit 0
fi

summary="$(printf '%s' "$blockers" | grep -oE '"[^"]+"' | sed 's/"//g' | tr '\n' ';' | sed 's/;$//')"
jq -n \
  --arg summary "$summary" \
  --arg target "$target" \
  '{permission:"allow", user_message:("bash-rm-guardian: \"" + $target + "\" tem refs ativas. Revise antes."), agent_message:("bash-rm-guardian detected active references: " + $summary)}'
