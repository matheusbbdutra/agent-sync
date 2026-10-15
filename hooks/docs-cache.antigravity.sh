#!/usr/bin/env bash
# Cacheia passivamente docs consultadas via read_url_content ou context7
# (call_mcp_tool com ServerName=context7, ToolName=query-docs) no Antigravity.
# O resultado da tool nao vem no payload do hook - o writer Go le do
# transcriptPath (tools/cmd/docs-cache-write/hook_extract.go).
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

# O writer só loga em stderr; o contrato do hook exige '{}' no stdout.
"$WRITER" --from-hook --cli=antigravity || true
printf '{}'
