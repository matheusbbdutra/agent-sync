#!/usr/bin/env bash
# Cursor beforeShellExecution: pede confirmacao (permission=ask) em comandos de risco.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PATTERNS_FILE="$SCRIPT_DIR/bash-guardian-patterns.txt"

input="$(cat)"
command_line="$(printf '%s' "$input" | grep -o '"command"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed -E 's/.*:[[:space:]]*"(.*)"/\1/')"

if [ -z "$command_line" ] || [ ! -f "$PATTERNS_FILE" ]; then
  printf '{}'
  exit 0
fi

# Desescapa o minimo necessario do JSON (\", \\).
command_line="$(printf '%s' "$command_line" | sed -e 's/\\"/"/g' -e 's/\\\\/\\/g')"

while IFS= read -r pattern; do
  [ -z "$pattern" ] && continue
  case "$pattern" in
    \#*) continue ;;
  esac
  # shellcheck disable=SC2254
  case "$command_line" in
    $pattern)
      printf '{"permission":"ask","user_message":"bash-guardian: comando bate o padrao de risco \\"%s\\" (agent-sync).","agent_message":"Comando marcado pelo bash-guardian; aguarde confirmacao do usuario."}' "$pattern"
      exit 0
      ;;
  esac
done < "$PATTERNS_FILE"

printf '{}'
