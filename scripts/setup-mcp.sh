#!/usr/bin/env bash
# Configura MCPs nos CLIs instalados:
#   - context7 : docs atualizadas de bibliotecas (remoto por padrão; local/stdio com CONTEXT7_LOCAL=1)
#   - docs     : MCP local offline sobre o cache do docs-fetch (~/.cache/agent-sync/docs)
#   - memory   : memória compartilhada entre CLIs via libSQL local (~/.cache/agent-sync/memory.db)
#   - sentry   : erros/performance do Sentry (opcional; defina SENTRY_MCP_URL)
#
# Uso:
#   bash scripts/setup-mcp.sh
#   CONTEXT7_LOCAL=1 bash scripts/setup-mcp.sh        # Context7 via npx (stdio, ainda precisa de internet)
#   CONTEXT7_API_KEY=xxx bash scripts/setup-mcp.sh    # limites maiores (não fica no repo)
#   SENTRY_MCP_URL=https://mcp.sentry.dev/mcp/org/proj bash scripts/setup-mcp.sh
set -euo pipefail

CONTEXT7_NAME="context7"
CONTEXT7_URL="https://mcp.context7.com/mcp"
DOCS_NAME="docs"
DOCS_BIN="$(command -v docs-mcp 2>/dev/null || true)"
[ -n "$DOCS_BIN" ] || DOCS_BIN="$HOME/.local/bin/docs-mcp"
MEMORY_NAME="memory"
MEMORY_BIN="$(command -v memory-mcp 2>/dev/null || true)"
[ -n "$MEMORY_BIN" ] || MEMORY_BIN="$HOME/.local/bin/memory-mcp"

remove_server() {
  local cli="$1" name="$2"
  case "$cli" in
    claude) claude mcp remove "$name" -s user >/dev/null 2>&1 || true ;;
    codex) codex mcp remove "$name" >/dev/null 2>&1 || true ;;
    agy) agy mcp remove "$name" >/dev/null 2>&1 || true ;;
  esac
}

upsert_opencode() {
  local name="$1" spec="$2"
  command -v python3 >/dev/null 2>&1 || { echo "⚠️  python3 ausente; pulando opencode"; return 0; }
  python3 - "$name" "$spec" <<'PY'
import json, os, sys
name, spec = sys.argv[1], json.loads(sys.argv[2])
path = os.path.expanduser("~/.config/opencode/opencode.json")
data = {}
if os.path.exists(path):
    with open(path) as fh:
        data = json.load(fh)
data.setdefault("mcp", {})[name] = spec
with open(path, "w") as fh:
    json.dump(data, fh, indent=2, ensure_ascii=False)
    fh.write("\n")
PY
}

# ── Context7 ────────────────────────────────────────────────────────────────
setup_context7() {
  local local_mode="${CONTEXT7_LOCAL:-0}"
  if [ "$local_mode" = "1" ]; then
    echo "→ Context7 em modo local (stdio via npx)"
    if command -v claude >/dev/null 2>&1; then
      remove_server claude "$CONTEXT7_NAME"
      claude mcp add -s user "$CONTEXT7_NAME" -- npx -y @upstash/context7-mcp >/dev/null
    fi
    if command -v codex >/dev/null 2>&1; then
      remove_server codex "$CONTEXT7_NAME"
      codex mcp add "$CONTEXT7_NAME" -- npx -y @upstash/context7-mcp >/dev/null
    fi
    if command -v agy >/dev/null 2>&1; then
      remove_server agy "$CONTEXT7_NAME"
      agy mcp add "$CONTEXT7_NAME" npx -y @upstash/context7-mcp >/dev/null
    fi
    if command -v opencode >/dev/null 2>&1; then
      upsert_opencode "$CONTEXT7_NAME" '{"type":"local","command":["npx","-y","@upstash/context7-mcp"],"enabled":true}'
    fi
  else
    echo "→ Context7 em modo remoto ($CONTEXT7_URL)"
    if command -v claude >/dev/null 2>&1; then
      remove_server claude "$CONTEXT7_NAME"
      if [ -n "${CONTEXT7_API_KEY:-}" ]; then
        claude mcp add --transport http -s user "$CONTEXT7_NAME" "$CONTEXT7_URL" \
          --header "Authorization: Bearer ${CONTEXT7_API_KEY}" >/dev/null
      else
        claude mcp add --transport http -s user "$CONTEXT7_NAME" "$CONTEXT7_URL" >/dev/null
      fi
    fi
    if command -v codex >/dev/null 2>&1; then
      remove_server codex "$CONTEXT7_NAME"
      if [ -n "${CONTEXT7_API_KEY:-}" ]; then
        codex mcp add "$CONTEXT7_NAME" --url "$CONTEXT7_URL" --bearer-token-env-var CONTEXT7_API_KEY >/dev/null
      else
        codex mcp add "$CONTEXT7_NAME" --url "$CONTEXT7_URL" >/dev/null
      fi
    fi
    if command -v agy >/dev/null 2>&1; then
      remove_server agy "$CONTEXT7_NAME"
      if [ -n "${CONTEXT7_API_KEY:-}" ]; then
        agy mcp add --type http --header "Authorization: Bearer ${CONTEXT7_API_KEY}" "$CONTEXT7_NAME" "$CONTEXT7_URL" >/dev/null
      else
        agy mcp add --type http "$CONTEXT7_NAME" "$CONTEXT7_URL" >/dev/null
      fi
    fi
    if command -v opencode >/dev/null 2>&1; then
      local spec='{"type":"remote","url":"'"$CONTEXT7_URL"'","enabled":true}'
      upsert_opencode "$CONTEXT7_NAME" "$spec"
    fi
  fi
  echo "✅ context7 configurado"
}

# ── MCP local de docs (offline) ─────────────────────────────────────────────
setup_docs() {
  if [ ! -x "$DOCS_BIN" ]; then
    echo "⚠️  docs-mcp não encontrado ($DOCS_BIN). Rode 'make install' primeiro."
    return 0
  fi
  echo "→ MCP local de docs (offline): $DOCS_BIN"
  if command -v claude >/dev/null 2>&1; then
    remove_server claude "$DOCS_NAME"
    claude mcp add -s user "$DOCS_NAME" -- "$DOCS_BIN" >/dev/null
  fi
  if command -v codex >/dev/null 2>&1; then
    remove_server codex "$DOCS_NAME"
    codex mcp add "$DOCS_NAME" -- "$DOCS_BIN" >/dev/null
  fi
  if command -v agy >/dev/null 2>&1; then
    remove_server agy "$DOCS_NAME"
    agy mcp add "$DOCS_NAME" "$DOCS_BIN" >/dev/null
  fi
  if command -v opencode >/dev/null 2>&1; then
    upsert_opencode "$DOCS_NAME" '{"type":"local","command":["'"$DOCS_BIN"'"],"enabled":true}'
  fi
  echo "✅ docs configurado (offline)"
}

# ── MCP local de memória compartilhada (entre Claude Code, Codex, agy, OpenCode) ─
setup_memory() {
  if [ ! -x "$MEMORY_BIN" ]; then
    echo "⚠️  memory-mcp não encontrado ($MEMORY_BIN). Rode 'make install' primeiro."
    return 0
  fi
  echo "→ MCP local de memória compartilhada: $MEMORY_BIN"
  if command -v claude >/dev/null 2>&1; then
    remove_server claude "$MEMORY_NAME"
    claude mcp add -s user "$MEMORY_NAME" -- "$MEMORY_BIN" >/dev/null
  fi
  if command -v codex >/dev/null 2>&1; then
    remove_server codex "$MEMORY_NAME"
    codex mcp add "$MEMORY_NAME" -- "$MEMORY_BIN" >/dev/null
  fi
  if command -v agy >/dev/null 2>&1; then
    remove_server agy "$MEMORY_NAME"
    agy mcp add "$MEMORY_NAME" "$MEMORY_BIN" >/dev/null
  fi
  if command -v opencode >/dev/null 2>&1; then
    upsert_opencode "$MEMORY_NAME" '{"type":"local","command":["'"$MEMORY_BIN"'"],"enabled":true}'
  fi
  echo "✅ memory configurado (compartilhado entre CLIs)"
}

# ── Sentry (opcional) ───────────────────────────────────────────────────────
setup_sentry() {
  local url="${SENTRY_MCP_URL:-}"
  if [ -z "$url" ]; then
    echo "ℹ️  SENTRY_MCP_URL não definido; pulando Sentry MCP (ex.: https://mcp.sentry.dev/mcp/<org>/<proj>)."
    return 0
  fi
  local name="sentry"
  echo "→ Sentry MCP: $url"
  if command -v claude >/dev/null 2>&1; then
    remove_server claude "$name"
    claude mcp add --transport http -s user "$name" "$url" >/dev/null
  fi
  if command -v codex >/dev/null 2>&1; then
    remove_server codex "$name"
    codex mcp add "$name" --url "$url" >/dev/null
  fi
  if command -v agy >/dev/null 2>&1; then
    remove_server agy "$name"
    agy mcp add --type http "$name" "$url" >/dev/null
  fi
  if command -v opencode >/dev/null 2>&1; then
    upsert_opencode "$name" '{"type":"remote","url":"'"$url"'","enabled":true}'
  fi
  echo "✅ sentry configurado (OAuth no primeiro uso)"
}

setup_context7
setup_docs
setup_memory
setup_sentry
echo "✨ MCPs configurados. Verifique com 'mcp list' de cada CLI."
