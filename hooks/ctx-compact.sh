#!/usr/bin/bash
# PostToolUse: a cada chamada de ferramenta, registra o tool call no working
# memory do ctx-window e dispara auto-compactacao se o budget estimado for
# atingido. Nunca bloqueia o tool call (falhas sao ignoradas via set +e).
set +e
trap '' ERR

input="$(cat)"

session_id="$(printf '%s' "$input" | { grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
session_id="${session_id:-default}"

tool_name="$(printf '%s' "$input" | { grep -o '"tool_name"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
if [ -z "$tool_name" ]; then
  tool_name="$(printf '%s' "$input" | { grep -o '"name"[[:space:]]*:[[:space:]]*"[^"]*"' || true; } | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]*)"/\1/')"
fi
tool_name="${tool_name:-unknown}"

ctx-window on-tool-call "$session_id" --tool "$tool_name" >/dev/null 2>&1
exit 0