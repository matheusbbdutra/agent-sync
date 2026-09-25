#!/usr/bin/env bash
# Cursor stop hook: Proteção contra falso sucesso e validação de término (HarnessFix arXiv:2606.06324v2).
# Disparado quando o agente tenta parar no Cursor.
# Contrato Cursor: {"followup_message": "..."} para orientar continuação ou {} para término normal.
set -euo pipefail

input="$(cat)"

# Extrai transcript_path se fornecido pelo Cursor
transcript_path="$(printf '%s' "$input" | { grep -o '"transcript_path"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"

if [ -n "$transcript_path" ] && [ -f "$transcript_path" ]; then
  if command -v false-success-guard >/dev/null 2>&1; then
    verdict="$(false-success-guard check --transcript "$transcript_path" 2>/dev/null || echo "{}")"
    if printf '%s' "$verdict" | grep -q '"flagged":true'; then
      reason="$(printf '%s' "$verdict" | grep -o '"reason":*"[^"]*"' | head -n1 | sed -E 's/.*:"(.*)"/\1/' || echo "Alegação de conclusão sem evidência anexada.")"
      printf '{"followup_message":"[agent-sync] %s. Confirme com evidência real antes de concluir a resposta."}' "$reason"
      exit 0
    fi
  fi
fi

# Parada normal permitida
printf '{}'
