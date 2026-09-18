#!/usr/bin/env bash
# Warm-up silencioso do cache do repo-map.
#
# Roda `repo-map --update --quiet` em background para garantir que o
# arquivo <root>/.agent-sync/cache/repomap.json esteja quente para o
# agente (ou o usuário) chamar `repo-map --focus` ou `--summary`.
#
# Este hook NAO cospe texto no prompt: sempre devolve {} e exit 0 em
# qualquer falha. Foi desenhado para ser disparado por eventos de
# inicialização de sessão (ex.: SessionStart no Claude Code, session.start
# no Antigravity, PostToolUse lazy no OpenCode).
#
# Variaveis de ambiente opcionais:
#   AGENT_SYNC_REPO_MAP_DISABLE=1   desativa o hook (sai com {} e exit 0)
#   AGENT_SYNC_REPO_MAP_ROOT=<path> raiz do repositório (default: cwd)
#   AGENT_SYNC_REPO_MAP_FOREGROUND=1 roda sincronamente em vez de background

set -euo pipefail

if [ "${AGENT_SYNC_REPO_MAP_DISABLE:-0}" = "1" ]; then
  printf '{}'
  exit 0
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="${AGENT_SYNC_REPO_MAP_BIN:-}"
if [ -z "$BIN" ]; then
  if [ -x "$SCRIPT_DIR/../bin/repo-map" ]; then
    BIN="$SCRIPT_DIR/../bin/repo-map"
  else
    BIN="$(command -v repo-map || true)"
  fi
fi

if [ -z "$BIN" ] || [ ! -x "$BIN" ]; then
  # binário ausente: nunca falha o hook por causa disso
  printf '{}'
  exit 0
fi

ROOT_FLAG=""
if [ -n "${AGENT_SYNC_REPO_MAP_ROOT:-}" ]; then
  ROOT_FLAG="--root ${AGENT_SYNC_REPO_MAP_ROOT}"
fi

if [ "${AGENT_SYNC_REPO_MAP_FOREGROUND:-0}" = "1" ]; then
  "$BIN" --update --quiet $ROOT_FLAG >/dev/null 2>&1 || true
else
  # background: redireciona tudo para /dev/null e desanexa
  ( "$BIN" --update --quiet $ROOT_FLAG >/dev/null 2>&1 || true ) &
  disown || true
fi

printf '{}'
exit 0