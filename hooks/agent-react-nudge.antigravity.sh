#!/usr/bin/env bash
# Lembrete pre-invocacao do modelo (Antigravity): validacao de hipoteses
# (agent-react). Usa invocationNum nativo do PreInvocation.
set -euo pipefail

THRESHOLD="${AGENT_SYNC_REACT_NUDGE_THRESHOLD:-15}"

input="$(cat)"
invocation_num="$(printf '%s' "$input" | grep -o '"invocationNum"[[:space:]]*:[[:space:]]*[0-9]*' | head -n1 | grep -o '[0-9]*$')"
invocation_num="${invocation_num:-0}"

if [ "$invocation_num" -gt 0 ] && [ "$((invocation_num % THRESHOLD))" -eq 0 ]; then
  printf '{"injectSteps":[{"ephemeralMessage":"[agent-sync] Hipotese ativa sem validacao? Nao conclua/implemente como fato. Valide com tool/leitura ou peca o passo concreto ao usuario. Skill: agent-react."}]}'
else
  printf '{}'
fi
