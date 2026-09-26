#!/usr/bin/env bash
# bash-rm-guardian para Claude Code / Codex PreToolUse.
# Emite additionalContext warn (NÃO bloqueia) quando o audit detecta refs.
# Output no formato Claude Code: hookSpecificOutput.additionalContext.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CORE="$SCRIPT_DIR/bash-rm-guardian.sh"

input="$(cat)"
# Claude Code / Codex formatam tool_input.command no payload PreToolUse.
# Tenta múltiplos patterns por compatibilidade.
command_line="$(printf '%s' "$input" | grep -oE '"command"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed -E 's/.*:[[:space:]]*"(.*)"/\1/' || true)"

# Fallback: pode vir como "CommandLine" em outros adaptadores
if [ -z "$command_line" ]; then
  command_line="$(printf '%s' "$input" | grep -oE '"CommandLine"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed -E 's/.*:[[:space:]]*"(.*)"/\1/' || true)"
fi

if [ -z "$command_line" ]; then
  printf '{}'
  exit 0
fi

# Desescapar minimo
command_line="$(printf '%s' "$command_line" | sed -e 's/\\"/"/g' -e 's/\\\\/\\/g')"

# Roda core para extrair blockers
result="$(AGENT_SYNC_ROOT="${AGENT_SYNC_ROOT:-$(pwd)}" bash "$CORE" analyze "$command_line" 2>/dev/null || true)"
blockers="$(printf '%s' "$result" | grep -oE '"blockers":\[[^]]*\]' | sed 's/"blockers"://' || true)"
target="$(printf '%s' "$result" | grep -oE '"target":"[^"]*"' | sed 's/"target":"//;s/"$//' || true)"

if [ -z "$blockers" ] || [ "$blockers" = "[]" ]; then
  printf '{}'
  exit 0
fi

summary="$(printf '%s' "$blockers" | grep -oE '"[^"]+"' | sed 's/"//g' | tr '\n' ';' | sed 's/;$//')"
message="bash-rm-guardian: \\\"$target\\\" tem refs ativas no repo. Revise antes de prosseguir: $summary"

# Formato Claude Code PreToolUse
jq -n \
  --arg msg "$message" \
  '{hookSpecificOutput:{hookEventName:"PreToolUse",additionalContext:$msg}}'
