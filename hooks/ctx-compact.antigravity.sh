#!/usr/bin/bash
# Antigravity pre-invocation: a cada chamada, registra o tool call e dispara
# auto-compactacao via ctx-window. Usa sessionId do Antigravity quando
# presente; cai para invocationNum-based counter caso nao tenha.
set +e
trap '' ERR

input="$(cat)"

session_id="$(printf '%s' "$input" | { grep -o '"sessionId"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
if [ -z "$session_id" ]; then
  session_id="$(printf '%s' "$input" | { grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
fi
session_id="${session_id:-default}"

tool_name="$(printf '%s' "$input" | { grep -o '"toolName"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
if [ -z "$tool_name" ]; then
  tool_name="$(printf '%s' "$input" | { grep -o '"tool_name"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
fi
tool_name="${tool_name:-unknown}"

ctx-window on-tool-call "$session_id" --tool "$tool_name" >/dev/null 2>&1
exit 0