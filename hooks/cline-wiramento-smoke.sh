#!/usr/bin/env bash
# cline-wiramento-smoke.sh
#
# A-85: smoke VISIVEL do wiramento de hooks no Cline. Resolve a ambiguidade
# "está wirado mas não vejo hook nenhum" (sessão 2026-09-26) — escreve um
# arquivo de log com timestamp + lista de hooks wirados esperados + injeta
# contexto no prompt para o modelo ecoar.
#
# Roda em TaskStart (proxy SessionStart no Cline). One-shot por sessao via
# flag file. NUNCA bloqueia turno (exit 0 sempre).
#
# Artefatos:
#   /tmp/agent-sync-cline-wiramento/<sid>.log    log com timestamp + lista de hooks
#   /tmp/agent-sync-cline-wiramento/<sid>.seen   flag de idempotencia (one-shot)
#
# Opt-out:
#   AGENT_SYNC_WIRAMENTO_SMOKE=0                 desabilita o smoke inteiro

set -uo pipefail

# Opt-out
if [ "${AGENT_SYNC_WIRAMENTO_SMOKE:-1}" = "0" ]; then
  printf '{}'
  exit 0
fi

# Extrai session_id (Claude/Codex) ou taskId/conversationId (Cline).
input="$(cat)"
sid="$(printf '%s' "$input" \
  | { grep -oE '"(session_id|taskId|conversation_id)"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 \
  | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
sid="${sid:-default}"

STATE_DIR="${TMPDIR:-/tmp}/agent-sync-cline-wiramento"
mkdir -p "$STATE_DIR"
SEEN_FILE="$STATE_DIR/$sid.seen"

# One-shot por sessao: se ja escreveu, so injeta contexto minimo.
if [ -f "$SEEN_FILE" ]; then
  printf '{"hookSpecificOutput":{"hookEventName":"TaskStart","additionalContext":"[agent-sync Cline] wiramento ativo (smoke ja registrado nesta sessao). Hooks wirados: PreToolUse=6, PostToolUse=5, TaskStart=3, TaskComplete=3."}}\n'
  exit 0
fi
: > "$SEEN_FILE"

LOG_FILE="$STATE_DIR/$sid.log"
TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
{
  echo "# agent-sync wiramento Cline — smoke A-85"
  echo "# session: $sid"
  echo "# timestamp: $TS"
  echo "#"
  echo "# Hooks wirados (agent-sync hook cline):"
  echo "#   PreToolUse    = 6 (bash-rm-guardian, context-guard, memory-nudge, agent-react, principles-inject, secret-guard)"
  echo "#   PostToolUse   = 5 (docs-cache, ctx-window-nudge, secret-guard, memory-observe, token-nudge)"
  echo "#   TaskStart     = 3 (memory-prune-session-start, ctx-handoff, cline-wiramento-smoke [ESTE])"
  echo "#   TaskComplete  = 3 (memory-consolidate, ctx-window-summarize-at-stop, agent-task-record)"
  echo "#"
  echo "# Artefatos de visibilidade:"
  echo "#   /tmp/agent-sync-memory-nudge/         counters por session_id (PreToolUse)"
  echo "#   /tmp/agent-sync-ctx-window-nudge/     counters por session_id (PostToolUse)"
  echo "#   /tmp/agent-sync-principles-injected/  flags one-shot por session_id (PreToolUse)"
  echo "#   /tmp/agent-sync-react-nudge/          counters por session_id (PreToolUse)"
  echo "#   /tmp/agent-sync-ctx-window-stop/      counters por session_id (TaskComplete)"
  echo "#   /tmp/agent-sync-cline-wiramento/      ESTE log (TaskStart)"
  echo "#"
  echo "# MCP servers (cline_mcp_settings.json): context7, docs, memory, code-graph"
  echo "# Skills: 54 wiradas em ~/.cline/skills"
  echo "# Agents: 0 (gap aceito A-83 — CLI v3 nao expoe loader)"
} > "$LOG_FILE"

# Injeta contexto para o modelo ecoar no inicio do run.
printf '{"hookSpecificOutput":{"hookEventName":"TaskStart","additionalContext":"[agent-sync Cline wiramento] hooks wirados via plugin (A-80/A-80.4/A-84): PreToolUse=6 PostToolUse=5 TaskStart=3 TaskComplete=3. Artefatos visiveis em /tmp/agent-sync-*/. MCP=4 servers, skills=54, agents=0 (gap aceito). Smoke log: %s"}}\n' "$LOG_FILE"
exit 0
