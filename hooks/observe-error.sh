#!/usr/bin/env bash
# Persiste falhas dos hooks sem armazenar payloads de ferramentas.
set -u

stage="${1:-unknown}"
code="${2:-hook_failed}"
message="${3:-}"
log_file="${AGENT_SYNC_HOOK_LOG:-${XDG_CACHE_HOME:-$HOME/.cache}/agent-sync/hooks/errors.jsonl}"

mkdir -p "$(dirname "$log_file")" 2>/dev/null || exit 0
message="$(printf '%s' "$message" | tr '\n\r' '  ' | sed -E 's/(token|secret|password|api[_-]?key|authorization)[=:][^ ]*/\1=[REDACTED]/Ig' | cut -c1-512)"

jq -cn --arg timestamp "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg stage "$stage" --arg code "$code" --arg message "$message" \
  --arg cli "${AGENT_SYNC_CLI:-codex}" --arg tool "${TOOL_NAME:-}" \
  --arg session_id "${SESSION_ID:-${session_id:-}}" \
  '{timestamp:$timestamp,event:"hook_error",stage:$stage,code:$code,message:$message,cli:$cli,tool:$tool,session_id:$session_id}' \
  >> "$log_file" 2>/dev/null || true

# Mantém o arquivo sob controle sem apagar o evento atual.
if [ -f "$log_file" ] && [ "$(wc -c < "$log_file" 2>/dev/null || printf 0)" -gt 5242880 ]; then
  tail -n 10000 "$log_file" > "$log_file.tmp" 2>/dev/null && mv -f "$log_file.tmp" "$log_file" || true
fi
