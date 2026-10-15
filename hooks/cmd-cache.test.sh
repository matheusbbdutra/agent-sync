#!/usr/bin/env bash
# Test runner do cmd-cache PostToolUse (ADR-reuso-bash-sudo-agents §1).
# Testa o CONTRATO do adapter: opt-in, allowlist, fail-open.
# Rodar via: bash hooks/cmd-cache.test.sh
set -uo pipefail

HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
ADAPTER="$HOOK_DIR/cmd-cache.posttooluse.sh"
BIN="$HOOK_DIR/../bin/shell-validate"
PASS=0
FAIL=0

fail() { printf '  FAIL: %s\n' "$1" >&2; FAIL=$((FAIL + 1)); }
pass() { printf '  ok\n'; PASS=$((PASS + 1)); }

if [ ! -x "$BIN" ]; then
  echo "SKIP: $BIN ausente (rode 'make build')"
  exit 0
fi

CACHE_DIR="$(mktemp -d)/cmd-cache"
export AGENT_SYNC_CMD_CACHE_DIR="$CACHE_DIR"

# 1) Desabilitado = no-op silencioso, nada em disco.
out="$(printf '%s' '{"tool_name":"Bash","tool_input":{"command":"uname -a"},"tool_response":{"stdout":"Linux"}}' \
  | env -u AGENT_SYNC_CMD_CACHE AGENT_SYNC_CMD_CACHE_DIR="$CACHE_DIR" bash "$ADAPTER")"
if [ "$out" = "{}" ] && [ ! -d "$CACHE_DIR" ]; then pass; else fail "desabilitado: out=$out dir=$(ls -d "$CACHE_DIR" 2>/dev/null)"; fi

# 2) Habilitado + allowlist = grava e devolve '{}'.
out="$(printf '%s' '{"tool_name":"Bash","tool_input":{"command":"uname -a"},"tool_response":{"stdout":"Linux host"}}' \
  | AGENT_SYNC_CMD_CACHE=1 bash "$ADAPTER")"
count="$(ls "$CACHE_DIR" 2>/dev/null | wc -l)"
if [ "$out" = "{}" ] && [ "$count" = "1" ]; then pass; else fail "habilitado: out=$out entradas=$count"; fi

# 3) Habilitado + fora do allowlist = não grava.
out="$(printf '%s' '{"tool_name":"Bash","tool_input":{"command":"rm -rf /tmp/x"},"tool_response":{"stdout":"x"}}' \
  | AGENT_SYNC_CMD_CACHE=1 bash "$ADAPTER")"
count="$(ls "$CACHE_DIR" 2>/dev/null | wc -l)"
if [ "$out" = "{}" ] && [ "$count" = "1" ]; then pass; else fail "fora do allowlist: out=$out entradas=$count"; fi

# 4) Payload inválido = fail-open ('{}', nada gravado).
out="$(printf '%s' 'not-json' | AGENT_SYNC_CMD_CACHE=1 bash "$ADAPTER")"
count="$(ls "$CACHE_DIR" 2>/dev/null | wc -l)"
if [ "$out" = "{}" ] && [ "$count" = "1" ]; then pass; else fail "payload invalido: out=$out entradas=$count"; fi

# 5) Binário ausente = fail-open ('{}').
mkdir -p /tmp/cc-nobin-dir && cp "$ADAPTER" /tmp/cc-nobin-dir/adapter.sh
out="$(printf '%s' '{"tool_name":"Bash","tool_input":{"command":"uname -a"},"tool_response":{"stdout":"x"}}' \
  | AGENT_SYNC_CMD_CACHE=1 PATH="/usr/bin:/bin" "$BASH" /tmp/cc-nobin-dir/adapter.sh)"
if [ "$out" = "{}" ]; then pass; else fail "binario ausente: out=$out"; fi
rm -rf /tmp/cc-nobin-dir

printf '\ncmd-cache: %d PASS, %d FAIL\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
