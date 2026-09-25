#!/usr/bin/env bash
# Variante do repo-map-warmup.sh para o Cursor.
# Mesmo contrato: silencioso, sempre retorna {} em qualquer falha.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WARMUP="$SCRIPT_DIR/repo-map-warmup.sh"

if [ ! -x "$WARMUP" ]; then
  printf '{}'
  exit 0
fi

AGENT_SYNC_REPO_MAP_FOREGROUND="${AGENT_SYNC_REPO_MAP_FOREGROUND:-1}" \
  bash "$WARMUP"