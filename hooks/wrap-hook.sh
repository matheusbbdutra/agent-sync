#!/usr/bin/env bash
# wrap-hook.sh: envolve invocação de hook para capturar exit code, stderr
# e duration_ms, registrando via observe-error.sh em caso de falha.
# Uso: wrap-hook.sh [--cli=<cli>] <stage> <hook_name> <script_real> [args...]
# Exit code: o mesmo do script_real (preserva contrato com a CLI)
set -uo pipefail

if [[ "${1:-}" =~ ^--cli=(.+)$ ]]; then
  export AGENT_SYNC_CLI="${BASH_REMATCH[1]}"
  shift
fi

stage="${1:-unknown}"
hook_name="${2:-unknown}"
shift 2
script_arg="${1:-}"
if [ -z "$script_arg" ]; then
  echo "wrap-hook: script required" >&2
  exit 64
fi
shift
case "$script_arg" in
  /*) script="$script_arg" ;;
  */*) script="$script_arg" ;;
  *) script="$(dirname "$0")/$script_arg" ;;
esac

start_ms=$(date +%s%3N)
stderr_file=$(mktemp)
output=$("$script" "$@" 2>"$stderr_file")
status=$?
duration_ms=$(($(date +%s%3N) - start_ms))

if [ "$status" -ne 0 ]; then
  HOOK_STATUS=fail \
  HOOK_DURATION_MS="$duration_ms" \
  AGENT_SYNC_CLI="${AGENT_SYNC_CLI:-}" \
  "$(dirname "$0")/observe-error.sh" "$stage" "${hook_name}_exit_${status}" "$(head -c 512 "$stderr_file")" \
    || true
fi

rm -f "$stderr_file"
printf '%s' "$output"
exit "$status"