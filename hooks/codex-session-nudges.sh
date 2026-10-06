#!/usr/bin/env bash
# codex-session-nudges.sh: lembretes do agent-sync em SessionStart, para Codex.
#
# POR QUE ISTO EXISTE (nao mover para PreToolUse):
# Com features.code_mode_host ligado (codex >= 0.160), a unica tool de execucao
# emite `custom_tool_call`. O Codex injeta o additionalContext de um hook como
# `message role=developer` ENTRE a tool call e o resultado dela, e o endpoint
# /v1/responses da MiniMax rejeita esse payload com:
#   400 {"code":"invalid_prompt","message":"invalid params, tool call result
#        does not follow tool call (2013)"}
# Ver internal/hooks/hooks_codex_session_nudges.go e docs/.
#
# Sem tool call em andamento (SessionStart), o additionalContext vira uma
# developer message antes de qualquer call — posicao aceita pela MiniMax
# (mesma posicao do AGENTS.md, ver rollout item 8).
#
# Cobre os 4 lembretes que hoje rodam em PreToolUse (principles-inject,
# context-guard, memory-nudge, agent-react-nudge). One-shot por sessao.
#
# NUNCA bloqueia o turno (exit 0 sempre).
set -uo pipefail

STATE_DIR="${TMPDIR:-/tmp}/agent-sync-codex-session-nudges"
mkdir -p "$STATE_DIR"

input="$(cat 2>/dev/null || true)"

# Codex popula CODEX_SESSION_ID; fallback para o payload (A-86).
sid="${CODEX_SESSION_ID:-}"
if [ -z "$sid" ] && [ -n "$input" ]; then
  sid="$(printf '%s' "$input" \
    | { grep -oE '"(session_id|conversation_id|taskId)"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
    | head -n1 \
    | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
fi
sid="${sid:-default}"

flag_file="$STATE_DIR/$sid.flag"
if [ -f "$flag_file" ]; then
  printf '{}'
  exit 0
fi
: > "$flag_file"

# Texto derivado verbatim de rules/global-rules.md (secoes 2 e 3) para evitar
# drift entre hook e doc canonico (verdade absoluta).
cat <<'EOF'
{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"[agent-sync principles] 1. Verdade absoluta: registre apenas afirmacoes verificaveis empiricamente (git, doc oficial, runtime, file:line). Sem evidencia = sem afirmacao. 'Acho que' / 'geralmente' sao proibidos. Meia-verdade = mentira. 2. Anti-overengineering: prefira editar existente a criar novo (skill, hook, agente, arquivo). Crie so quando edit >50 linhas ou for estruturalmente inviavel. [context-guard] Voce ja atualizou o STATE.md nesta sessao? Se acumulou decisoes/hipoteses/artifacts desde o ultimo update, registre (regra de memoria enxuta: max 2-3 linhas, fatos acionaveis). [memory] Esta decisao/observacao deve virar regra persistente? Se sim, registre em rules/global-rules.md ou skill canonica. Se for apenas contexto de sessao, marque como tal. [agent-react] Antes da proxima tool: voce VALIDOU sua hipotese atual? Citou evidencia (file:line, comando, teste)? Sem evidencia = sem afirmacao."}}
EOF