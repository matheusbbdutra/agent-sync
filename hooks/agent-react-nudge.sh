#!/usr/bin/bash
# Lembrete pos-ferramenta: a cada N chamadas na mesma sessao, cobra validacao
# de hipoteses (agent-react) — hipotese != fato; validar ou pedir passo ao user.
set -euo pipefail
trap 'status=$?; "$(dirname "$0")/observe-error.sh" "PostToolUse" "hook_exit_$status" "agent-react nudge falhou"; exit "$status"' ERR

THRESHOLD="${AGENT_SYNC_REACT_NUDGE_THRESHOLD:-15}"
STATE_DIR="${TMPDIR:-/tmp}/agent-sync-react-nudge"
mkdir -p "$STATE_DIR"

input="$(cat)"
session_id="$(printf '%s' "$input" | { grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
session_id="${session_id:-default}"

counter_file="$STATE_DIR/$session_id.count"
count=0
[ -f "$counter_file" ] && count="$(cat "$counter_file")"
count=$((count + 1))
printf '%s' "$count" > "$counter_file"

if [ "$((count % THRESHOLD))" -eq 0 ]; then
  cat <<EOF
{"systemMessage":"agent-sync: lembrete de agent-react (#$count)","hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"[agent-sync] Hipotese ativa sem validacao? Nao conclua/implemente como fato. Valide com tool/leitura ou peca o passo concreto ao usuario. Skill: agent-react."}}
EOF
else
  printf '{}'
fi
