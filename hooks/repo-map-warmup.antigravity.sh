#!/usr/bin/env bash
# Variante do repo-map-warmup.sh para o Antigravity CLI.
# Mesmo contrato: silencioso, sempre retorna {} em qualquer falha.
# Disparado tipicamente em session.start (PreInvocation equivalente).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WARMUP="$SCRIPT_DIR/repo-map-warmup.sh"

if [ ! -x "$WARMUP" ]; then
  printf '{}'
  exit 0
fi

bash "$WARMUP"