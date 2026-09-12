#!/usr/bin/env bash
# Guard de comandos: pede confirmacao (ask) quando o comando bate um padrao de
# risco conhecido (rm -rf, dd para device, curl|sh, force-push, etc.).
# Evento: PreToolUse. Nao bloqueia sozinho (deny) - so forca confirmacao humana.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PATTERNS_FILE="$SCRIPT_DIR/bash-guardian-patterns.txt"

input="$(cat)"
command_line="$(printf '%s' "$input" | grep -o '"CommandLine"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed -E 's/.*:[[:space:]]*"(.*)"/\1/')"

if [ -z "$command_line" ] || [ ! -f "$PATTERNS_FILE" ]; then
  printf '{}'
  exit 0
fi

while IFS= read -r pattern; do
  [ -z "$pattern" ] && continue
  case "$pattern" in
    \#*) continue ;;
  esac
  # shellcheck disable=SC2254
  case "$command_line" in
    $pattern)
      printf '{"decision":"ask","reason":"bash-guardian: comando bate o padrão de risco \\"%s\\" (agent-sync)."}' "$pattern"
      exit 0
      ;;
  esac
done < "$PATTERNS_FILE"

printf '{}'
