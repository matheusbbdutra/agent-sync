#!/usr/bin/env bash
# Cursor stop hook: injeta next-action no fim do turno via followup_message.
# Complementa agent-stop.cursor.sh (false-success-guard): este aqui empurra
# o agente a continuar ate' a proxima acao do STATE estar feita.
# Dedup por conversation_id: nao emite o mesmo id duas vezes seguidas.
#
# Contrato Cursor (stop): {"followup_message": "..."} para orientar
# continuacao ou {} para termino normal.
set -uo pipefail

STATE_DIR="${TMPDIR:-/tmp}/agent-sync-react-nudge-cursor-stop"
mkdir -p "$STATE_DIR"

input="$(cat)"

# Extrai conversation_id (prioridade) ou session_id do payload JSON.
conversation_id="$(printf '%s' "$input" \
  | { grep -o '"conversation_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 \
  | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
if [ -z "$conversation_id" ]; then
  conversation_id="$(printf '%s' "$input" \
    | { grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
    | head -n1 \
    | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
fi
conversation_id="${conversation_id:-default}"

# Detecta root do projeto a partir do payload (cwd) ou cai para . .
root="$(printf '%s' "$input" \
  | { grep -o '"cwd"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 \
  | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
[ -n "$root" ] || root="."

# Detecta agent-sync wirado (binario instalado tem o subcommand 'state').
# O wrapper antigo retorna help em stdout com exit 0; detecta pelo primeiro char.
probe="$(agent-sync state next-action -root "$root" 2>/dev/null || true)"
if [ "${probe:0:1}" = "{" ]; then
  # Binario wirado: usa direto.
  next_json="$probe"
else
  # Fallback: detecta raiz do modulo agent-sync (go.mod) e roda via go run.
  repo_root=""
  d="$root"
  for _ in 1 2 3 4 5; do
    if [ -f "$d/go.mod" ] && grep -q "agent-sync\|token-tools" "$d/go.mod" 2>/dev/null; then
      repo_root="$d"
      break
    fi
    d="$(dirname "$d")"
    [ "$d" = "/" ] && break
  done
  if [ -z "$repo_root" ] || ! command -v go >/dev/null 2>&1; then
    printf '{}'
    exit 0
  fi
  next_json="$(cd "$repo_root" && go run ./cmd/agent-sync state next-action -root "$root" 2>/dev/null || echo "")"
fi

if [ -z "$next_json" ] || [ "${next_json:0:1}" != "{" ]; then
  printf '{}'
  exit 0
fi

# Extrai id e title; valida que ambos existem e status=pending.
action_id="$(printf '%s' "$next_json" \
  | { grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 \
  | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
action_title="$(printf '%s' "$next_json" \
  | { grep -o '"title"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 \
  | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
action_status="$(printf '%s' "$next_json" \
  | { grep -o '"status"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } \
  | head -n1 \
  | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"

# Se nada pendente (status!=pending ou campos ausentes), permite termino.
if [ -z "$action_id" ] || [ "$action_status" != "pending" ]; then
  printf '{}'
  exit 0
fi

# Dedup: nao emite o mesmo id duas vezes para a mesma conversa.
last_file="$STATE_DIR/$conversation_id.last"
last_id=""
[ -f "$last_file" ] && last_id="$(cat "$last_file")"
if [ "$last_id" = "$action_id" ]; then
  printf '{}'
  exit 0
fi
printf '%s' "$action_id" > "$last_file"

# Mirror best-effort para memory-mcp (kind=task_completed? nao — aqui ainda nao
# completou, entao kind=note com hint de proxima acao).
if command -v memory-mcp >/dev/null 2>&1; then
  printf '%s\n' \
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
    '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
    "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"record_event\",\"arguments\":{\"agent\":\"cursor\",\"kind\":\"note\",\"note\":\"stop-cursor next_action=$action_id\",\"session_id\":\"$conversation_id\",\"project_path\":\"$root\"}}}" \
    | memory-mcp -db "${AGENT_SYNC_MEMORY_DB:-${XDG_CACHE_HOME:-$HOME/.cache}/agent-sync/memory.db}" >/dev/null 2>&1 || true
fi

# Emite followup_message (escapa aspas duplas e barras invertidas).
escaped_title="${action_title//\\/\\\\}"
escaped_title="${escaped_title//\"/\\\"}"
escaped_id="${action_id//\\/\\\\}"
escaped_id="${escaped_id//\"/\\\"}"
printf '{"followup_message":"[agent-sync] Proxima acao pendente: %s (%s). Atinja-a antes de concluir o turno. Para contexto completo: agent-sync state render -root %q."}' \
  "$escaped_title" "$escaped_id" "$root"
