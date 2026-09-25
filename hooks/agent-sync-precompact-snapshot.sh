#!/usr/bin/env bash
# Wrapper para agent-sync state snapshot que resolve o project root
# corretamente em PreInvocation (sem cwd do projeto disponivel).
#
# Estrategia: sobe diretorios ate encontrar go.mod que seja do agent-sync,
# igual ao logica do agent-react-nudge.stop.cursor.sh.
set -euo pipefail

repo_root=""
d="$PWD"
for _ in 1 2 3 4 5; do
  if [ -f "$d/go.mod" ] && grep -q "agent-sync\|token-tools" "$d/go.mod" 2>/dev/null; then
    repo_root="$d"
    break
  fi
  d="$(dirname "$d")"
  [ "$d" = "/" ] && break
done

if [ -n "$repo_root" ] && command -v agent-sync >/dev/null 2>&1; then
  cd "$repo_root"
  agent-sync state snapshot -actor agy
else
  # Fallback silencioso: nao bloqueia, emite {}.
  printf '{}'
fi
