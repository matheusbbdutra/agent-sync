#!/usr/bin/bash
# Cacheia passivamente docs ja consultadas via WebFetch ou context7 (query-docs),
# sem refazer requisicao de rede - so persiste o que a ferramenta ja trouxe.
# Compartilhado entre Claude Code e Codex (schema de PostToolUse equivalente:
# tool_name, tool_input, tool_response).
#
# Regra §4 (Python-zero): a extração de {url,text} vive em
# tools/cmd/docs-cache-write (--from-hook, Go). Este wrapper só traduz o
# contrato e repassa o stdin.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WRITER="$SCRIPT_DIR/../bin/docs-cache-write"
[ -x "$WRITER" ] || WRITER="$(command -v docs-cache-write || true)"

if [ -z "$WRITER" ]; then
  printf '{}'
  exit 0
fi

# docs-cache-write nunca quebra o hook: sem doc cacheável sai silencioso.
# O writer só loga em stderr; o contrato do hook exige '{}' no stdout.
"$WRITER" --from-hook --cli=claude || true
printf '{}'
