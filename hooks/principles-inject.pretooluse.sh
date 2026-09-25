#!/usr/bin/env bash
# Hook PreToolUse: injeta as 2 premissas criticas (verdade absoluta +
# anti-overengineering) como additionalContext UMA vez por sessao.
#
# Inspirado em agent-react-nudge.stop.cursor.sh (padrao dedup via $TMPDIR).
# Wirar em Claude Code, Codex e OpenCode v2. Antigravity + Cursor ficam
# como 🟡 (matrix 5xN do ADR Trilha C).
#
# Sem estado persistido alem de $TMPDIR/agent-sync-principles-injected/<sid>.
set -uo pipefail

STATE_DIR="${TMPDIR:-/tmp}/agent-sync-principles-injected"
mkdir -p "$STATE_DIR"

input="$(cat)"

# Extrai session_id (Claude/Codex/OpenCode) ou conversation_id (Cursor).
sid="$(printf '%s' "$input" \
  | { grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 \
  | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
if [ -z "$sid" ]; then
  sid="$(printf '%s' "$input" \
    | { grep -o '"conversation_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
    | head -n1 \
    | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
fi
sid="${sid:-default}"

# One-shot por sessao.
flag_file="$STATE_DIR/$sid.flag"
if [ -f "$flag_file" ]; then
  printf '{}'
  exit 0
fi
: > "$flag_file"

# additionalContext que sera injetado pelo harness no prompt do agente.
# Texto derivado verbatim das secoes 2 e 3 de rules/global-rules.md para
# evitar drift entre hook e doc canonico (verdade absoluta).
cat <<'EOF'
{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"[agent-sync principles] 1. Verdade absoluta: registre apenas afirmacoes verificaveis empiricamente (git, doc oficial, runtime, file:line). Sem evidencia = sem afirmacao. 'Acho que' / 'geralmente' sao proibidos. Meia-verdade = mentira. 2. Anti-overengineering: prefira editar existente a criar novo (skill, hook, agente, arquivo). Crie so quando edit >50 linhas ou for estruturalmente inviavel."}}
EOF
