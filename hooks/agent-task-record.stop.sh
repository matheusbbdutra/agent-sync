#!/usr/bin/env bash
# agent-task-record.stop.sh
#
# Hook cross-CLI wirado no evento Stop (Claude Code, Codex, Antigravity)
# e stop (Cursor) para popular `.agent-sync/agent_tasks.jsonl` com
# telemetria do turno que acabou (session_id, model, tokens, status).
#
# NAO bloqueia o encerramento da sessao: erros sao logados via observe-error
# e exit 0 sempre. Stop nao pode falhar por causa disso.
#
# Payloads por CLI (validados em docs oficiais 2026-09-20):
#   - Claude Code (claude.com/docs/hooks): {session_id, transcript_path,
#     last_assistant_message, hook_event_name, ...}. SEM campo model.
#   - Codex (learn.chatgpt.com/docs/hooks): {session_id, transcript_path,
#     model, last_assistant_message, hook_event_name, ...}. TEM campo model.
#   - Antigravity: payload especifico (nao verificado nesta entrega);
#     heuristica: ausencia de model + presence de conversation_id.
#   - Cursor: payload especifico (nao verificado); passa CLI=agy/cursor
#     via flag do wirar.
#
# Tokens/cost: extraidos do transcript_path se for JSONL com campo usage.
# Heuristica conservadora: campo usage.input_tokens/output_tokens OU
# usage.total_tokens. Se transcript nao existir ou nao casar, deixa null
# (gap documentado no ADR-budget-cross-cli).
#
# Wirado por: cmd/agent-sync/hooks.go -> syncAgentTaskRecordHook (novo).
# Subcommand: agent-sync budget write (le AgentTask JSON do stdin).
#
# Variaveis:
#   AGENT_SYNC_AGENT_KIND=<claude|codex|agy|cursor>  CLI origem (passado pelo wirar)
#   AGENT_SYNC_BUDGET_BIN=<path>                     binario agent-sync (default: PATH)
#   AGENT_SYNC_BUDGET_DISABLE=1                      no-op
#   AGENT_SYNC_BUDGET_BG=1                           roda em background (default)

set -uo pipefail

if [ "${AGENT_SYNC_BUDGET_DISABLE:-0}" = "1" ]; then
  exit 0
fi

CLI_KIND="${AGENT_SYNC_AGENT_KIND:-claude}"
BIN="${AGENT_SYNC_BUDGET_BIN:-}"
if [ -z "$BIN" ]; then
  BIN="$(command -v agent-sync 2>/dev/null || true)"
fi

# Sem binario: silencioso (best-effort). Stop nao pode falhar por causa disso.
if [ -z "$BIN" ] || [ ! -x "$BIN" ]; then
  exit 0
fi

input="$(cat)"

# Extracao defensiva: campo pode estar ausente ou malformado.
session_id="$(printf '%s' "$input" \
  | { grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
if [ -z "$session_id" ]; then
  session_id="$(printf '%s' "$input" \
    | { grep -o '"conversation_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
    | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
fi
session_id="${session_id:-unknown}"

# Codex-specific: campo model nativo no payload.
model="$(printf '%s' "$input" \
  | { grep -o '"model"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
model="${model:-unknown}"

transcript_path="$(printf '%s' "$input" \
  | { grep -o '"transcript_path"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"

# Extracao opcional de tokens do transcript JSONL.
tokens_in=""
tokens_out=""
if [ -n "$transcript_path" ] && [ -f "$transcript_path" ]; then
  # Claude Code/Codex gravam JSONL com estrutura vari mas tipicamente:
  # {"type":"assistant", "message":{"usage":{"input_tokens":N, "output_tokens":M}}, ...}
  # Usa ultimo evento do assistant (turno atual).
  last_assistant="$(grep -h '"type":"assistant"' "$transcript_path" 2>/dev/null | tail -n1 || true)"
  if [ -n "$last_assistant" ]; then
    if command -v jq >/dev/null 2>&1; then
      tin="$(printf '%s' "$last_assistant" | jq -r '
        .message.usage.input_tokens //
        .usage.input_tokens //
        .message.usage.prompt_tokens //
        empty
      ' 2>/dev/null || true)"
      tout="$(printf '%s' "$last_assistant" | jq -r '
        .message.usage.output_tokens //
        .usage.output_tokens //
        .message.usage.completion_tokens //
        empty
      ' 2>/dev/null || true)"
      [ -n "$tin" ] && [ "$tin" != "null" ] && tokens_in="$tin"
      [ -n "$tout" ] && [ "$tout" != "null" ] && tokens_out="$tout"
    else
      # Fallback grep quando jq ausente: regex direta.
      tin="$(printf '%s' "$last_assistant" \
        | { grep -oE '"(input|prompt)_tokens"[[:space:]]*:[[:space:]]*[0-9]+' || true; } \
        | tail -n1 | grep -oE '[0-9]+$' || true)"
      tout="$(printf '%s' "$last_assistant" \
        | { grep -oE '"(output|completion)_tokens"[[:space:]]*:[[:space:]]*[0-9]+' || true; } \
        | tail -n1 | grep -oE '[0-9]+$' || true)"
      [ -n "$tin" ] && tokens_in="$tin"
      [ -n "$tout" ] && tokens_out="$tout"
    fi
  fi
fi

# ts ISO 8601 UTC.
ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# task_id estavel: session_id (1 AgentTask por turno).
task_id="${session_id}-stop"

# Monta JSON AgentTask via jq se disponivel; senao printf (sem campos null).
if command -v jq >/dev/null 2>&1; then
  if [ -n "$tokens_in" ] && [ -n "$tokens_out" ]; then
    payload="$(jq -nc \
      --arg sv "1.0" \
      --arg tid "$task_id" \
      --arg ts "$ts" \
      --arg cli "$CLI_KIND" \
      --arg model "$model" \
      --arg sid "$session_id" \
      --argjson tin "$tokens_in" \
      --argjson tout "$tokens_out" \
      '{schema_version:$sv, task_id:$tid, ts:$ts, cli:$cli, model:$model, session_id:$sid, tokens_in:$tin, tokens_out:$tout, status:"completed"}')"
  elif [ -n "$tokens_in" ]; then
    payload="$(jq -nc \
      --arg sv "1.0" \
      --arg tid "$task_id" \
      --arg ts "$ts" \
      --arg cli "$CLI_KIND" \
      --arg model "$model" \
      --arg sid "$session_id" \
      --argjson tin "$tokens_in" \
      '{schema_version:$sv, task_id:$tid, ts:$ts, cli:$cli, model:$model, session_id:$sid, tokens_in:$tin, status:"completed"}')"
  elif [ -n "$tokens_out" ]; then
    payload="$(jq -nc \
      --arg sv "1.0" \
      --arg tid "$task_id" \
      --arg ts "$ts" \
      --arg cli "$CLI_KIND" \
      --arg model "$model" \
      --arg sid "$session_id" \
      --argjson tout "$tokens_out" \
      '{schema_version:$sv, task_id:$tid, ts:$ts, cli:$cli, model:$model, session_id:$sid, tokens_out:$tout, status:"completed"}')"
  else
    payload="$(jq -nc \
      --arg sv "1.0" \
      --arg tid "$task_id" \
      --arg ts "$ts" \
      --arg cli "$CLI_KIND" \
      --arg model "$model" \
      --arg sid "$session_id" \
      '{schema_version:$sv, task_id:$tid, ts:$ts, cli:$cli, model:$model, session_id:$sid, status:"completed"}')"
  fi
else
  # Fallback sem jq: monta via printf (tokens omitidos).
  payload="$(printf '{"schema_version":"1.0","task_id":"%s","ts":"%s","cli":"%s","model":"%s","session_id":"%s","status":"completed"}' \
    "$task_id" "$ts" "$CLI_KIND" "$model" "$session_id")"
fi

# Grava via subcommand (validacao schema enforced la).
BG="${AGENT_SYNC_BUDGET_BG:-1}"
if [ "$BG" = "1" ]; then
  printf '%s' "$payload" | "$BIN" budget write -root "$(pwd)" >/dev/null 2>&1 &
else
  printf '%s' "$payload" | "$BIN" budget write -root "$(pwd)" >/dev/null 2>&1
fi

exit 0
