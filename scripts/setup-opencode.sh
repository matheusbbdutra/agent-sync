#!/usr/bin/env bash
# Garante os pre-requisitos de runtime para os plugins OpenCode v2 wirados
# pelo agent-sync (sincronizados em ~/.config/opencode/plugins/*.ts).
#
# Pre-requisitos (validados empiricamente em @opencode/plugin@2.0.11):
#   - Node >= 20.11  (alguns plugins usam import.meta.dirname, top-level await)
#   - Binario opencode (>= 2.0.0)  (Bun-compiled; Bun.Transpiler embutido)
#   - Pacote @opencode/plugin resolvivel a partir do OpenCodePluginDir
#     (default: ~/.config/opencode/node_modules/@opencode/plugin)
#
# Nao instala o binario OpenCode em si — isso fica a cargo de mise/npm/asdf.
# Este script fecha o gap de runtime TS que o -apply nao cobre.
#
# Uso:
#   scripts/setup-opencode.sh            # instala o que falta (idempotente)
#   scripts/setup-opencode.sh --check    # apenas diagnostica, exit 1 se faltar
#
# Padrao identico ao scripts/setup-go.sh (mesma estrategia, sem sudo).
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

OPENCODE_PLUGIN_DIR="${OPENCODE_PLUGIN_DIR:-$HOME/.config/opencode/plugins}"
OPENCODE_CONFIG_DIR="$(dirname "$OPENCODE_PLUGIN_DIR")"
NODE_MIN_MAJOR=20
NODE_MIN_MINOR=11
OPENCODE_MIN_MAJOR=2

mode="install"
for arg in "$@"; do
  case "$arg" in
    --check) mode="check" ;;
    -h|--help)
      sed -n '2,20p' "$0"
      exit 0
      ;;
    *)
      echo "Argumento desconhecido: $arg" >&2
      exit 2
      ;;
  esac
done

log_ok()   { echo "✅ $*"; }
log_warn() { echo "⚠️  $*"; }
log_fail() { echo "❌ $*" >&2; }
log_info() { echo "→ $*"; }

# --- 1. Node -------------------------------------------------------------
node_ok=0
if command -v node >/dev/null 2>&1; then
  node_ver=$(node --version 2>/dev/null | sed 's/^v//')
  node_major=$(printf '%s' "$node_ver" | cut -d. -f1)
  node_minor=$(printf '%s' "$node_ver" | cut -d. -f2)
  if [ "${node_major:-0}" -ge "$NODE_MIN_MAJOR" ] \
     && { [ "${node_major:-0}" -gt "$NODE_MIN_MAJOR" ] || [ "${node_minor:-0}" -ge "$NODE_MIN_MINOR" ]; }; then
    log_ok "Node ${node_ver} ja instalado (>= ${NODE_MIN_MAJOR}.${NODE_MIN_MINOR})."
    node_ok=1
  else
    log_warn "Node ${node_ver} e anterior ao minimo ${NODE_MIN_MAJOR}.${NODE_MIN_MINOR}."
  fi
else
  log_warn "Node nao encontrado no PATH."
fi

# --- 2. Binario opencode -------------------------------------------------
opencode_ok=0
if command -v opencode >/dev/null 2>&1; then
  opencode_ver=$(opencode --version 2>/dev/null | head -n1)
  opencode_major=$(printf '%s' "$opencode_ver" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -n1 | cut -d. -f1)
  opencode_major="${opencode_major:-0}"
  if [ "$opencode_major" -ge "$OPENCODE_MIN_MAJOR" ]; then
    log_ok "OpenCode ${opencode_ver} (major>=${OPENCODE_MIN_MAJOR})."
    opencode_ok=1
  else
    log_warn "OpenCode ${opencode_ver} e anterior ao minimo v${OPENCODE_MIN_MAJOR}.0.0."
  fi
else
  log_warn "Binario 'opencode' nao encontrado no PATH."
fi

# --- 3. @opencode/plugin resolvivel -------------------------------------
# O loader v2 (source.node.js:50-58) consulta localSource a partir do
# parentURL. Procura em 3 locais provaveis, na ordem:
#   (a) ~/.config/opencode/node_modules/@opencode/plugin/
#   (b) node_modules do proprio OpenCode (varia por instalacao)
#   (c) npm root -g (instalado via `npm i -g @opencode/plugin`)
plugin_ok=0
plugin_paths=(
  "$OPENCODE_CONFIG_DIR/node_modules/@opencode/plugin/package.json"
  "$HOME/.opencode/node_modules/@opencode/plugin/package.json"
)
if command -v npm >/dev/null 2>&1; then
  npm_global_root=$(npm root -g 2>/dev/null || true)
  if [ -n "${npm_global_root:-}" ]; then
    plugin_paths+=("$npm_global_root/@opencode/plugin/package.json")
  fi
fi

found_plugin_path=""
for p in "${plugin_paths[@]}"; do
  if [ -f "$p" ]; then
    found_plugin_path="$p"
    plugin_ok=1
    break
  fi
done

if [ "$plugin_ok" -eq 1 ]; then
  plugin_ver=$(grep -m1 '"version"' "$found_plugin_path" | sed -E 's/.*"version": *"([^"]+)".*/\1/')
  log_ok "@opencode/plugin ${plugin_ver} encontrado em: $found_plugin_path"
else
  log_warn "@opencode/plugin nao encontrado em nenhum node_modules pesquisado."
fi

# Instalacao orfa: presente no disco sob OPENCODE_CONFIG_DIR mas sem
# declaracao no package.json (ex.: instalada com `npm --no-save`).
# Um `npm prune` futuro removeria e o "Cannot find package" voltaria,
# como ocorrido em 2026-09-21. Forca reinstalacao com --save.
orphaned_local=0
if [ "$plugin_ok" -eq 1 ] \
   && [ "$found_plugin_path" = "$OPENCODE_CONFIG_DIR/node_modules/@opencode/plugin/package.json" ] \
   && ! grep -q '"@opencode/plugin"' "$OPENCODE_CONFIG_DIR/package.json" 2>/dev/null; then
  log_warn "@opencode/plugin presente no disco mas nao declarado em $OPENCODE_CONFIG_DIR/package.json (instalacao orfa)."
  orphaned_local=1
fi

# --- Modo --check --------------------------------------------------------
if [ "$mode" = "check" ]; then
  if [ "$node_ok" -eq 1 ] && [ "$opencode_ok" -eq 1 ] && [ "$plugin_ok" -eq 1 ] && [ "$orphaned_local" -eq 0 ]; then
    log_ok "Todos os pre-requisitos de runtime OpenCode v2 estao satisfeitos."
    exit 0
  fi
  log_fail "Pre-requisitos faltando. Rode 'scripts/setup-opencode.sh' para instalar."
  exit 1
fi

# --- Modo install --------------------------------------------------------
need_install=0
if [ "$plugin_ok" -eq 0 ] || [ "$orphaned_local" -eq 1 ]; then
  need_install=1
fi

if [ "$need_install" -eq 0 ]; then
  log_ok "Nada a fazer — pre-requisitos ja satisfeitos."
  exit 0
fi

# Instala @opencode/plugin. Tenta local primeiro (no OPENCODE_CONFIG_DIR) sem
# precisar de sudo; cai para global se o local falhar.
# Local usa --save de proposito: declara a dep no package.json para que
# `npm prune` nao remova (instalacao orfa = recorrencia do erro de 2026-09-21).
if [ "$plugin_ok" -eq 0 ] || [ "$orphaned_local" -eq 1 ]; then
  log_info "Instalando @opencode/plugin..."
  if command -v npm >/dev/null 2>&1; then
    mkdir -p "$OPENCODE_CONFIG_DIR"
    if npm install --prefix "$OPENCODE_CONFIG_DIR" --save --no-audit --no-fund \
         "@opencode/plugin" >/dev/null 2>&1; then
      log_ok "@opencode/plugin instalado em $OPENCODE_CONFIG_DIR/node_modules/."
    elif npm install -g --no-save --no-audit --no-fund \
         "@opencode/plugin" >/dev/null 2>&1; then
      log_ok "@opencode/plugin instalado globalmente."
    else
      log_fail "Falha ao instalar @opencode/plugin. Instale manualmente:"
      echo "       npm i -g @opencode/plugin" >&2
      exit 1
    fi
  else
    log_fail "npm nao encontrado. Instale Node + npm e rode novamente."
    exit 1
  fi
fi

# Node e OpenCode binario nao sao instalados por este script — escopo
# deliberado (separacao de concerns; mise/asdf resolvem isso).
if [ "$node_ok" -eq 0 ]; then
  log_warn "Node ainda faltando. Instale >= ${NODE_MIN_MAJOR}.${NODE_MIN_MINOR} (mise/asdf/npm)."
fi
if [ "$opencode_ok" -eq 0 ]; then
  log_warn "OpenCode binario ainda faltando. Instale >= v${OPENCODE_MIN_MAJOR}.0.0."
fi

log_ok "Setup concluido. Verifique com: scripts/setup-opencode.sh --check"
