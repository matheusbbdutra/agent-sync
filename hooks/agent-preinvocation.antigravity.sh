#!/usr/bin/env bash
# Lembrete efemero just-in-time pre-invocacao do modelo (Antigravity CLI).
# Emite diretriz ativa sem poluir transcript.
set -euo pipefail

# Permite desativar seletivamente via ambiente
if [ "${AGENT_SYNC_DISABLE_PREINVOCATION_REMINDER:-0}" = "1" ]; then
  printf '{}'
  exit 0
fi

# Schema correto do Antigravity PreInvocation: emite {} (vazio).
# Schema antigo (injectSteps + ephemeralMessage) faz o Gemini CLI
# interpretar como `tool call denied by pre-tool hook` - verificado
# em runtime 2026-09-22. A diretriz PT-BR vive em GEMINI.md (system
# prompt wirado pelo syncRules em ~/.gemini/), nao precisa de nudge
# por hook.
printf '{}'
