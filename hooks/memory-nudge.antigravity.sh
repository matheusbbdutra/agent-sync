#!/usr/bin/env bash
# Lembrete pre-invocacao do modelo (Antigravity): usa o contador nativo
# invocationNum do evento PreInvocation, sem precisar de estado proprio.
set -euo pipefail

THRESHOLD="${AGENT_SYNC_MEMORY_NUDGE_THRESHOLD:-25}"

input="$(cat)"
invocation_num="$(printf '%s' "$input" | grep -o '"invocationNum"[[:space:]]*:[[:space:]]*[0-9]*' | head -n1 | grep -o '[0-9]*$')"
invocation_num="${invocation_num:-0}"

if [ "$invocation_num" -gt 0 ] && [ "$((invocation_num % THRESHOLD))" -eq 0 ]; then
  printf '{"injectSteps":[{"ephemeralMessage":"[agent-sync] Aconteceu algo nesta sessao que deveria virar memoria (correcao do usuario, decisao de projeto, preferencia confirmada)? Se sim, grave um resumo com o porque via store_memory (memory-mcp)."}]}'
else
  printf '{}'
fi
