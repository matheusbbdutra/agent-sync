#!/usr/bin/env bash
# Cursor postToolUse (matcher WebFetch): cacheia docs ja baixadas, sem rede.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Em ~/.cursor/hooks o writer fica em PATH apos make install; no repo, em bin/.
WRITER="$(command -v docs-cache-write || true)"
if [ -z "$WRITER" ] && [ -x "$SCRIPT_DIR/../bin/docs-cache-write" ]; then
  WRITER="$SCRIPT_DIR/../bin/docs-cache-write"
fi

if [ -z "$WRITER" ] || ! command -v python3 >/dev/null 2>&1; then
  printf '{}'
  exit 0
fi

python3 "$SCRIPT_DIR/docs-cache.cursor.py" "$WRITER" webfetch || printf '{}'
