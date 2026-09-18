#!/usr/bin/env bash
# Lembrete efêmero just-in-time pré-invocação do modelo (Antigravity CLI).
# Emite diretriz ativa via injectSteps (ephemeralMessage) sem poluir transcript.
set -euo pipefail

# Permite desativar seletivamente via ambiente
if [ "${AGENT_SYNC_DISABLE_PREINVOCATION_REMINDER:-0}" = "1" ]; then
  printf '{}'
  exit 0
fi

# Contrato PreInvocation no Antigravity: injectSteps com ephemeralMessage
printf '{"injectSteps":[{"ephemeralMessage":"Diretriz ativa: responda em PT-BR, sem rodeios e finalize com o resumo de 1-2 frases do que mudou e o que falta."}]}'
