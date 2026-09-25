#!/usr/bin/env bash
# Guard de comandos: pede confirmacao (ask) quando o comando bate um padrao de
# risco conhecido (rm -rf, dd para device, curl|sh, force-push, etc.).
# Evento: PreToolUse. Nao bloqueia sozinho (deny) - so forca confirmacao humana.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PATTERNS_FILE="$SCRIPT_DIR/bash-guardian-patterns.txt"

input="$(cat)"
# `{ ... || true; }` neutraliza set -e quando grep nao encontra match.
# Sem isso, com `set -euo pipefail` ativo, o grep retornando 1 aborta o
# script antes de chegar no `if [ -z ... ]` (PreInvocation vem sem
# stdin -> grep sempre falha -> hook sempre exit 1 -> Antigravity
# bloqueia a tool call com mensagem 'hook de seguranca do terminal').
# Verificado em runtime 2026-09-22 no log do antigravity-cli.
command_line="$(printf '%s' "$input" | { grep -o '"CommandLine"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } | head -n1 | sed -E 's/.*:[[:space:]]*"(.*)"/\1/')"

# Contrato por evento (doc interna agy-customizations/docs/hooks.md):
# - PreToolUse: exige "decision" (allow/deny/ask). {} vazio = deny.
# - PreInvocation: so aceita injectSteps. "decision" = unknown field.
# Mesmo script wirado nos dois eventos -> decide pelo payload:
# toolCall presente = PreToolUse -> {"decision":"allow"}.
allow_pass() {
  case "$input" in
    *'"toolCall"'*) printf '{"decision":"allow"}' ;;
    *) printf '{}' ;;
  esac
}

if [ -z "$command_line" ] || [ ! -f "$PATTERNS_FILE" ]; then
  allow_pass
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

allow_pass
