#!/usr/bin/env bash
# Persiste falhas dos hooks sem armazenar payloads de ferramentas.
set -u

stage="${1:-unknown}"
code="${2:-hook_failed}"
message="${3:-}"
log_file="${AGENT_SYNC_HOOK_LOG:-${XDG_CACHE_HOME:-$HOME/.cache}/agent-sync/hooks/errors.jsonl}"

mkdir -p "$(dirname "$log_file")" 2>/dev/null || exit 0
message="$(printf '%s' "$message" | tr '\n\r' '  ' | sed -E 's/(token|secret|password|api[_-]?key|authorization)[=:][^ ]*/\1=[REDACTED]/Ig' | cut -c1-512)"

status_arg="${HOOK_STATUS:-}"
duration_ms_json="null"
if [[ "${HOOK_DURATION_MS:-}" =~ ^[0-9]+$ ]]; then
  duration_ms_json="$HOOK_DURATION_MS"
fi

detect_cli() {
  if [ -n "${AGENT_SYNC_CLI:-}" ]; then
    printf '%s' "$AGENT_SYNC_CLI"
    return
  fi

  case "$code" in
    *antigravity*) printf '%s' "antigravity"; return ;;
    *cursor*)      printf '%s' "cursor"; return ;;
    *opencode*)    printf '%s' "opencode"; return ;;
    *claude*)      printf '%s' "claude"; return ;;
    *codex*)       printf '%s' "codex"; return ;;
  esac

  local pid="${PPID:-}"
  local depth=0
  while [ "$depth" -lt 3 ] && [ -n "$pid" ] && [ "$pid" -gt 1 ] && [ -r "/proc/$pid/cmdline" ]; do
    local pcmd
    pcmd="$(tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null || true)"
    case "$pcmd" in
      *agy*|*gemini*|*antigravity*) printf '%s' "antigravity"; return ;;
      *claude*) printf '%s' "claude"; return ;;
      *cursor*) printf '%s' "cursor"; return ;;
      *opencode*) printf '%s' "opencode"; return ;;
      *codex*) printf '%s' "codex"; return ;;
    esac
    local stat_ppid
    stat_ppid="$(awk '{print $4}' "/proc/$pid/stat" 2>/dev/null || true)"
    if [ "$stat_ppid" = "$pid" ] || [ -z "$stat_ppid" ]; then
      break
    fi
    pid="$stat_ppid"
    depth=$((depth + 1))
  done

  printf '%s' "unknown"
}

cli_name="$(detect_cli)"

exec 9>"${log_file}.lock" 2>/dev/null || exit 0
flock -w 5 9 2>/dev/null || exit 0

jq -cn --arg timestamp "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg stage "$stage" --arg code "$code" --arg message "$message" \
  --arg cli "$cli_name" --arg tool "${TOOL_NAME:-}" \
  --arg session_id "${SESSION_ID:-${session_id:-}}" \
  --arg status "$status_arg" \
  --argjson duration_ms "$duration_ms_json" \
  '{timestamp:$timestamp,event:"hook_error",version:"1",stage:$stage,code:$code,message:$message,cli:$cli,tool:$tool,session_id:$session_id}
   + (if $status != "" then {status:$status} else {} end)
   + (if $duration_ms != null then {duration_ms:$duration_ms} else {} end)' \
  >> "$log_file" 2>/dev/null || true

if [ -f "$log_file" ] && [ "$(wc -c < "$log_file" 2>/dev/null || printf 0)" -gt 5242880 ]; then
  tail -n 10000 "$log_file" > "$log_file.tmp" 2>/dev/null && mv -f "$log_file.tmp" "$log_file" || true
fi
