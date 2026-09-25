#!/usr/bin/env bash
# Garante o Go exigido pelo projeto (mínimo lido dos go.mod).
# Instala via mise (se disponível) ou pelo tarball oficial em ~/.local, sem sudo.
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

required_minor="1.24"
required_exact=""

read_requirements() {
  local gomod="$1"
  [ -f "$gomod" ] || return 0
  local go_line toolchain_line
  go_line=$(awk '$1=="go"{print $2; exit}' "$gomod")
  toolchain_line=$(awk '$1=="toolchain"{print $2; exit}' "$gomod")
  if [ -n "${go_line:-}" ]; then
    required_minor="${go_line%.*}"
  fi
  if [ -n "${toolchain_line:-}" ]; then
    required_exact="${toolchain_line#go}"
    required_minor="${required_exact%.*}"
  fi
}

read_requirements "$REPO_ROOT/go.mod"
read_requirements "$REPO_ROOT/tools/go.mod"

minor_of() { printf '%s' "$1" | cut -d. -f2; }

if command -v go >/dev/null 2>&1; then
  current=$(go version 2>/dev/null | awk '{print $3}')
  current_ver="${current#go}"
  if [ "$(minor_of "$current_ver")" -ge "$(minor_of "$required_minor")" ] 2>/dev/null; then
    echo "✅ Go ${current_ver} já instalado (>= ${required_minor})."
    exit 0
  fi
  echo "⚠️  Go ${current_ver} é anterior ao mínimo ${required_minor}; instalando..."
fi

install_version="${required_exact:-${required_minor}.0}"

make_wrapper() {
  local name="$1" target="$2"
  mkdir -p "$HOME/.local/bin"
  cat > "$HOME/.local/bin/$name" <<EOF
#!/bin/sh
exec "$target" "\$@"
EOF
  chmod +x "$HOME/.local/bin/$name"
  echo "   → wrapper ~/.local/bin/$name"
}

install_via_mise() {
  command -v mise >/dev/null 2>&1 || return 1
  echo "→ Instalando Go ${install_version} via mise..."
  mise install "go@${install_version}" >/dev/null 2>&1 || return 1
  local root
  root=$(mise where "go@${install_version}" 2>/dev/null) || return 1
  [ -x "$root/bin/go" ] || return 1
  make_wrapper go "$root/bin/go"
  make_wrapper gofmt "$root/bin/gofmt"
  return 0
}

download() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$1"
  else
    echo "❌ Nem curl nem wget disponíveis para baixar o Go." >&2
    return 1
  fi
}

WORK_TMP=""
cleanup() { [ -n "${WORK_TMP:-}" ] && rm -rf "$WORK_TMP"; }
trap cleanup EXIT

install_via_tarball() {
  case "$(uname -s)" in
    Linux) goos="linux" ;;
    Darwin) goos="darwin" ;;
    *) echo "❌ SO não suportado: $(uname -s)" >&2; return 1 ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) goarch="amd64" ;;
    arm64|aarch64) goarch="arm64" ;;
    *) echo "❌ Arquitetura não suportada: $(uname -m)" >&2; return 1 ;;
  esac

  local file="go${install_version}.${goos}-${goarch}.tar.gz"
  local base="https://go.dev/dl/${file}"
  local dest="$HOME/.local/share/agent-sync/go/${install_version}"

  if [ -d "$dest" ]; then
    echo "→ Reutilizando toolchain existente em $dest"
  else
    WORK_TMP=$(mktemp -d)
    echo "→ Baixando ${file}..."
    if ! download "$base" > "$WORK_TMP/$file"; then
      echo "❌ Falha ao baixar ${file}." >&2
      return 1
    fi
    if command -v sha256sum >/dev/null 2>&1; then
      local expected actual
      expected=$(download "${base}.sha256" | awk '{print $1}' || true)
      actual=$(sha256sum "$WORK_TMP/$file" | awk '{print $1}')
      if [ -z "$expected" ]; then
        echo "❌ Não foi possível obter o checksum de ${file}." >&2
        return 1
      fi
      if [ "$expected" != "$actual" ]; then
        echo "❌ Checksum inválido para ${file}." >&2
        return 1
      fi
      echo "   → checksum verificado"
    fi
    mkdir -p "$(dirname "$dest")"
    tar -C "$WORK_TMP" -xzf "$WORK_TMP/$file" || return 1
    mv "$WORK_TMP/go" "$dest" || return 1
  fi

  [ -x "$dest/bin/go" ] || return 1
  make_wrapper go "$dest/bin/go"
  make_wrapper gofmt "$dest/bin/gofmt"
  return 0
}

if install_via_mise || install_via_tarball; then
  echo "✅ Go ${install_version} disponível. Garanta que ~/.local/bin esteja no PATH."
  "$HOME/.local/bin/go" version
else
  echo "❌ Não foi possível instalar o Go automaticamente." >&2
  echo "   Instale manualmente (>= ${required_minor}): https://go.dev/dl/" >&2
  exit 1
fi
