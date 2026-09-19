#!/usr/bin/env bash
# Proteção de fim de turno (Stop) no Antigravity CLI.
# Verifica se o modelo está tentando parar prematuramente (tarefas em segundo
# plano ativas, pendência registrada na sessão ou alegações sem evidência).
# Contrato: {"decision": "continue", "reason": "..."} para pendência ou {} para parada normal.
set -euo pipefail

input="$(cat)"

# 1. Limite de execuções para evitar loops infinitos (default: 3)
execution_num="$(printf '%s' "$input" | grep -o '"executionNum"[[:space:]]*:[[:space:]]*[0-9]*' | head -n1 | grep -o '[0-9]*$' || echo "1")"
execution_num="${execution_num:-1}"
max_retries="${AGENT_SYNC_STOP_MAX_RETRIES:-3}"

if [ "$execution_num" -ge "$max_retries" ]; then
  printf '{}'
  exit 0
fi

# 2. Tarefas em background ainda em execução (fullyIdle=false)
fully_idle="$(printf '%s' "$input" | grep -o '"fullyIdle"[[:space:]]*:[[:space:]]*[a-zA-Z]*' | head -n1 | sed -E 's/.*:[[:space:]]*([a-zA-Z]*)/\1/' || echo "true")"
if [ "$fully_idle" = "false" ]; then
  printf '{"decision":"continue","reason":"[agent-sync] Existem tarefas em segundo plano ainda em execução (fullyIdle=false). Aguarde a conclusão antes de finalizar o turno."}'
  exit 0
fi

# 3. Forçar continuação via variável de ambiente (automações e testes)
if [ "${AGENT_SYNC_STOP_FORCE_CONTINUE:-0}" = "1" ]; then
  printf '{"decision":"continue","reason":"[agent-sync] Parada prematura interceptada (AGENT_SYNC_STOP_FORCE_CONTINUE=1)."}'
  exit 0
fi

# 4. Arquivo de pendência ativa da sessão (~/.cache/agent-sync/pending/<conv_id>)
conv_id="$(printf '%s' "$input" | grep -o '"conversationId"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/' || echo "")"
if [ -n "$conv_id" ] && [ -f "$HOME/.cache/agent-sync/pending/$conv_id" ]; then
  pending_reason="$(cat "$HOME/.cache/agent-sync/pending/$conv_id" 2>/dev/null || echo "Pendência ativa registrada na sessão.")"
  rm -f "$HOME/.cache/agent-sync/pending/$conv_id"
  printf '{"decision":"continue","reason":"[agent-sync] Pendência detectada: %s"}' "$pending_reason"
  exit 0
fi

# 5. Arquivo de pendência local do projeto (.agent-sync/pending)
for ws in $(printf '%s' "$input" | grep -o '"workspacePaths"[[:space:]]*:[[:space:]]*\[[^]]*\]' | grep -o '"[^"]*"' | tr -d '"'); do
  if [ -f "$ws/.agent-sync/pending" ]; then
    pending_reason="$(cat "$ws/.agent-sync/pending" 2>/dev/null || echo "Pendência registrada no projeto.")"
    rm -f "$ws/.agent-sync/pending"
    printf '{"decision":"continue","reason":"[agent-sync] Pendência do projeto não resolvida: %s"}' "$pending_reason"
    exit 0
  fi
done

# 6. Inspeção de transcript opcional via false-success-guard quando habilitado
transcript_path="$(printf '%s' "$input" | grep -o '"transcriptPath"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/' || echo "")"
if [ -n "$transcript_path" ] && [ -f "$transcript_path" ] && [ "${AGENT_SYNC_STOP_CHECK_TRANSCRIPT:-0}" = "1" ]; then
  if command -v false-success-guard >/dev/null 2>&1; then
    verdict="$(false-success-guard check --transcript "$transcript_path" 2>/dev/null || echo "{}")"
    if printf '%s' "$verdict" | grep -q '"flagged":true'; then
      reason="$(printf '%s' "$verdict" | grep -o '"reason":*"[^"]*"' | head -n1 | sed -E 's/.*:"(.*)"/\1/' || echo "Alegação de conclusão sem evidência anexada.")"
      printf '{"decision":"continue","reason":"[agent-sync] %s. Valide com testes/leitura antes de concluir."}' "$reason"
      exit 0
    fi
  fi
fi

# Sem pendências: permite parada normal
printf '{}'
