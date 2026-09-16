#!/usr/bin/bash
# Adapta o stdout do protect-mcp ao contrato de hooks do Codex.
set -uo pipefail

mode="${1:-}"
shift || true
output="$(npx protect-mcp@0.7.4 "$mode" "$@")"
status=$?

if [ "$status" -ne 0 ]; then
  "$(dirname "$0")/observe-error.sh" "${HOOK_EVENT_NAME:-PreToolUse}" "protect_mcp_exit_$status" "protect-mcp retornou exit code $status"
fi

if [ "$mode" = "evaluate" ]; then
  allowed="$(printf '%s' "$output" | jq -r '.allowed // false' 2>/dev/null || printf 'false')"
  reason="$(printf '%s' "$output" | jq -r '.reason // "policy evaluation failed"' 2>/dev/null || printf 'policy evaluation failed')"
  if ! printf '%s' "$output" | jq -e . >/dev/null 2>&1; then
    "$(dirname "$0")/observe-error.sh" "PreToolUse" "invalid_json" "protect-mcp retornou JSON inválido"
  fi
  if [ "$status" -eq 0 ] && [ "$allowed" = "true" ]; then
    printf '%s\n' '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow"}}'
  else
    jq -n --arg reason "$reason" '{hookSpecificOutput:{hookEventName:"PreToolUse",permissionDecision:"deny",permissionDecisionReason:$reason}}'
  fi
  exit 0
fi

# PostToolUse não altera o resultado da ferramenta. O recibo já foi gravado
# pelo protect-mcp; stdout vazio é a resposta válida e silenciosa do hook.
printf '{}\n'
exit 0
