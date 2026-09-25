#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "Uso: agent-sync-session <repo-agent-sync> <cli> [argumentos...]" >&2
  exit 2
fi

repo=$1
shift

for command_name in git agent-sync memory-sync; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "agent-sync-session: comando necessário ausente: $command_name" >&2
    exit 1
  fi
done

if ! git -C "$repo" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "agent-sync-session: diretório não é um checkout Git" >&2
  exit 1
fi

if [ -n "$(git -C "$repo" status --porcelain)" ]; then
  echo "agent-sync-session: checkout com alterações locais; sincronize ou preserve antes de iniciar" >&2
  exit 1
fi

before_sha=$(git -C "$repo" rev-parse HEAD)
if ! git -C "$repo" -c core.hooksPath=/dev/null pull --ff-only; then
  echo "agent-sync-session: atualização Git falhou; CLI não iniciada" >&2
  exit 1
fi
after_sha=$(git -C "$repo" rev-parse HEAD)
if [ "$before_sha" != "$after_sha" ]; then
  make -C "$repo" install
fi
AGENT_SYNC_HOME="$repo" agent-sync -apply
memory-sync -phase start

cli_status=0
"$@" || cli_status=$?

sync_status=0
memory-sync -phase end || sync_status=1
if [ -n "$(git -C "$repo" status --porcelain)" ]; then
  echo "agent-sync-session: há alterações sem commit; Git não foi enviado" >&2
  sync_status=1
else
  git -C "$repo" push || sync_status=1
fi

if [ "$sync_status" -ne 0 ]; then
  echo "agent-sync-session: sincronização final incompleta" >&2
  exit 1
fi
exit "$cli_status"
