#!/usr/bin/env bash
# codex-silent-hook.sh: executa um hook real e descarta o additionalContext.
#
# Uso: codex-silent-hook.sh <script-real> [args...]
#
# POR QUE: com features.code_mode_host ligado (codex >= 0.160), o Codex injeta
# o additionalContext de hooks PreToolUse/PostToolUse como `message
# role=developer` ENTRE a tool call e o resultado dela. O endpoint
# /v1/responses da MiniMax rejeita esse payload com 400 (2013)
# "tool call result does not follow tool call". Ver
# hooks/codex-session-nudges.sh para o ADR completo.
#
# O script real CONTINUA RODANDO (registros em memory-mcp, validacoes e
# side-effects preservados). So o stdout — que e o additionalContext — e
# descartado. nesses casos o hook devolve {}.
#
# Exit code espelha o do script real.
set -uo pipefail

script="${1:-}"
if [ -z "$script" ]; then
  echo "codex-silent-hook: script required" >&2
  exit 64
fi
shift

case "$script" in
  /*) resolved="$script" ;;
  # path relativo: usa como veio. O chamador (Go) sempre passa absoluto
  # via pathutil.HookScriptPath; o fallback por basename cobre o uso manual.
  */*) resolved="$script" ;;
  *) resolved="$(dirname "$0")/$script" ;;
esac

"$resolved" "$@" >/dev/null
exit $?