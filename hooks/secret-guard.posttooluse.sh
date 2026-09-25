#!/usr/bin/env bash
# Hook PostToolUse: redaciona output se contiver secret literal. Origem:
# ses_f2a16545bffeeCcxIFwZ04VVDa (2026-09-25) - PreToolUse nao pega caso
# 'cat ~/.zshrc' (args=path, output=conteudo do zshrc). Sem este hook, output
# ainda vaza em ~/.local/share/opencode/shell/.../*.out.
#
# Padrao JSON-RPC: recebe JSON com tool_output via stdin, retorna mesmo JSON
# com tool_output redacted (substitui matches por <REDACTED:TIPO>).
#
# IMPORTANTE: este hook roda DEPOIS da tool. Output persistido em disco fica
# redacted, mas agente JA viu o secret em memoria de trabalho. Por isso a
# defesa em profundidade com PreToolUse (que impede o tool call antes).
set -uo pipefail

input="$(cat)"

# Extrai o tool_output (JSON value escapado pode ter \n literais; tratamos simples).
tool_output="$(printf '%s' "$input" | grep -oE '"tool_output"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed -E 's/^"tool_output"[[:space:]]*:[[:space:]]*"(.*)"$/\1/')"

if [ -z "$tool_output" ]; then
  # Sem tool_output (caso edge: hook chamado sem output de tool); repassa intacto.
  printf '%s' "$input"
  exit 0
fi

# Detecta e redaciona cada tipo de secret.
redacted="$tool_output"
redacted="$(printf '%s' "$redacted" | sed -E 's/eyJ[A-Za-z0-9_=]+\.eyJ[A-Za-z0-9_=]+\.[A-Za-z0-9_-]+/<REDACTED:JWT>/g')"
redacted="$(printf '%s' "$redacted" | sed -E 's/AKIA[0-9A-Z]{16}/<REDACTED:AWS_ACCESS_KEY>/g')"
redacted="$(printf '%s' "$redacted" | sed -E 's/gh[pousr]_[A-Za-z0-9]{36,255}/<REDACTED:GITHUB_PAT>/g')"

# Se nada mudou, repassa input intacto (evita hook mudo em tool normal).
if [ "$redacted" = "$tool_output" ]; then
  printf '%s' "$input"
  exit 0
fi

# Substitui o tool_output no JSON. Para simplicidade, mantemos apenas
# tool_output (descarta outros campos do input; hook PostToolUse de Claude/
# Codex aceita tool_output como unico campo).
printf '%s' "$redacted"
exit 0
