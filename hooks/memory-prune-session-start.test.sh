#!/usr/bin/env bash
# Test runner para hooks/memory-prune-session-start.sh (A-68).
# Sem framework externo (sem bats): bash puro + assertions em shell exit code.
#
# Estrategia: usa AGENT_SYNC_MEMORY_BIN apontando para um fake `memory-mcp`
# em tmpdir que retorna stats controlados via env vars e conta invocacoes
# de prune. O hook roda em background (`(...) &`) — test precisa esperar
# antes de contar.
#
# Casos:
#   1. AGENT_SYNC_AUTO_PRUNE=0 → no-op, nao chama memory-mcp
#   2. scratch_pct < 80% → no-op silencioso, sem alerta stderr
#   3. scratch_pct >= 80% → alerta stderr + prune em background
#   4. memory-mcp ausente → no-op silencioso, exit 0
#   5. Idempotencia 24h → 2a chamada dentro de 24h skip
#
# Rodar via: bash hooks/memory-prune-session-start.test.sh
set -uo pipefail

HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
HOOK="$HOOK_DIR/memory-prune-session-start.sh"
PASS=0
FAIL=0

# Salvar PATH original: caso 4 redefine PATH para simular memory-mcp ausente
# e nao devemos perder acesso a comandos basicos (mktemp, cat, etc) entre testes.
ORIG_PATH="$PATH"

fail() {
  printf '  FAIL: %s\n' "$1" >&2
  FAIL=$((FAIL + 1))
}

pass() {
  printf '  ok\n'
  PASS=$((PASS + 1))
}

# Setup tmpdir com fake memory-mcp. Cada teste reseta $TMP/prune.calls.
# AGENT_SYNC_MEMORY_TOTAL e AGENT_SYNC_MEMORY_SCRATCH controlam output do stats.
setup() {
  TMP="$(mktemp -d)"
  export AGENT_SYNC_MEMORY_BIN="$TMP/memory-mcp"
  cat > "$AGENT_SYNC_MEMORY_BIN" <<'EOF'
#!/usr/bin/env bash
# Detecta subcomando
case "$1" in
  stats)
    total="${AGENT_SYNC_MEMORY_TOTAL:-0}"
    scratch="${AGENT_SYNC_MEMORY_SCRATCH:-0}"
    printf '{"total":%s,"by_scratch":{"scratch":%s,"permanent":%s}}\n' \
      "$total" "$scratch" "$((total - scratch))"
    ;;
  prune)
    printf 'called\n' >> "${TMPDIR}/prune.calls"
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
EOF
  chmod +x "$AGENT_SYNC_MEMORY_BIN"
  export TMPDIR="$TMP"
  : > "$TMP/prune.calls"
  # Reset session id para isolamento
  unset CLAUDE_SESSION_ID CODEX_SESSION_ID
  unset AGENT_SYNC_AUTO_PRUNE AGENT_SYNC_AUTO_PRUNE_DAYS AGENT_SYNC_AUTO_PRUNE_STALE_PCT
  # Restaurar PATH (caso 4 redefine; sem isso mktemp/cat quebram no proximo teste)
  export PATH="$ORIG_PATH"
}

# Aguarda jobs em background terminarem (incluindo os em subshells do hook).
# Sem wait real (jobs em subshells nao sao visiveis), usa sleep best-effort.
drain_background() {
  sleep 0.3
}

count_prune_calls() {
  drain_background
  if [ -f "$TMP/prune.calls" ]; then
    wc -l < "$TMP/prune.calls" | tr -d ' '
  else
    echo 0
  fi
}

# Caso 1: AGENT_SYNC_AUTO_PRUNE=0 → no-op total
disable_total_no_op() {
  setup
  export AGENT_SYNC_AUTO_PRUNE=0
  export AGENT_SYNC_MEMORY_TOTAL=100
  export AGENT_SYNC_MEMORY_SCRATCH=95
  out="$(printf '{"session_id":"x"}' | bash "$HOOK" 2>/tmp/stderr)"
  local n
  n="$(count_prune_calls)"
  if [ "$out" = "{}" ] && [ "$n" -eq 0 ] && [ ! -s /tmp/stderr ]; then
    pass
  else
    fail "esperava stdout={} 0 prune calls 0 stderr, obtive stdout='$out' prune=$n stderr='$(cat /tmp/stderr)'"
  fi
}

# Caso 2: scratch_pct < 80% → no-op silencioso, sem alerta stderr
scratch_below_threshold_no_alert() {
  setup
  export AGENT_SYNC_AUTO_PRUNE=1
  export AGENT_SYNC_AUTO_PRUNE_STALE_PCT=80
  export AGENT_SYNC_MEMORY_TOTAL=100
  export AGENT_SYNC_MEMORY_SCRATCH=10  # 10% < 80%
  out="$(printf '{"session_id":"sess-2"}' | bash "$HOOK" 2>/tmp/stderr)"
  local n
  n="$(count_prune_calls)"
  if [ "$out" = "{}" ] && [ "$n" -eq 1 ] && [ ! -s /tmp/stderr ]; then
    pass
  else
    fail "esperava stdout={} 1 prune 0 stderr, obtive stdout='$out' prune=$n stderr='$(cat /tmp/stderr)'"
  fi
}

# Caso 3: scratch_pct >= 80% → alerta stderr + prune
scratch_above_threshold_alert_and_prune() {
  setup
  export AGENT_SYNC_AUTO_PRUNE=1
  export AGENT_SYNC_AUTO_PRUNE_STALE_PCT=80
  export AGENT_SYNC_MEMORY_TOTAL=100
  export AGENT_SYNC_MEMORY_SCRATCH=95  # 95% >= 80%
  out="$(printf '{"session_id":"sess-3"}' | bash "$HOOK" 2>/tmp/stderr)"
  local n
  n="$(count_prune_calls)"
  if [ "$out" = "{}" ] && [ "$n" -eq 1 ] \
    && grep -q "WARNING: scratch=95% (95/100)" /tmp/stderr \
    && grep -q "gc best-effort em background" /tmp/stderr; then
    pass
  else
    fail "esperava stdout={} 1 prune alerta 'scratch=95% (95/100)', obtive stdout='$out' prune=$n stderr='$(cat /tmp/stderr)'"
  fi
}

# Caso 4: memory-mcp ausente → no-op silencioso, exit 0
memory_mcp_ausente_no_op() {
  TMP="$(mktemp -d)"
  export TMPDIR="$TMP"
  unset AGENT_SYNC_MEMORY_BIN
  # HOME redirecionado para tmpdir garante que hook nao encontre
  # ~/.local/bin/memory-mcp. PATH restaurado em setup() do proximo teste.
  FAKE_HOME="$TMP/fakehome"
  mkdir -p "$FAKE_HOME/.local/bin"
  export HOME="$FAKE_HOME"
  out="$(printf '{"session_id":"sess-4"}' | bash "$HOOK" 2>/tmp/stderr)"
  rc=$?
  if [ "$rc" -eq 0 ] && [ "$out" = "{}" ] && [ ! -s /tmp/stderr ]; then
    pass
  else
    fail "esperava exit=0 stdout={} 0 stderr, obtive rc=$rc stdout='$out' stderr='$(cat /tmp/stderr)'"
  fi
  # Restaurar HOME explicitamente (setup() restaura PATH mas nao HOME)
  unset HOME
}

# Caso 5: Idempotencia 24h — 2a chamada dentro de 24h skip
idempotencia_24h_skip() {
  setup
  export AGENT_SYNC_AUTO_PRUNE=1
  export AGENT_SYNC_MEMORY_TOTAL=100
  export AGENT_SYNC_MEMORY_SCRATCH=95
  export CLAUDE_SESSION_ID="sess-5"
  # 1a chamada: deve rodar prune
  out1="$(printf '{"session_id":"sess-5"}' | bash "$HOOK" 2>/dev/null)"
  n1="$(count_prune_calls)"
  # 2a chamada (mesma sessao): deve pular (janela 24h)
  out2="$(printf '{"session_id":"sess-5"}' | bash "$HOOK" 2>/dev/null)"
  n2="$(count_prune_calls)"
  if [ "$out1" = "{}" ] && [ "$out2" = "{}" ] \
    && [ "$n1" -eq 1 ] && [ "$n2" -eq 1 ]; then
    pass
  else
    fail "esperava 2 chamadas = 1 prune total, obtive n1=$n1 n2=$n2"
  fi
}

printf '\nmemory-prune-session-start (A-68 — auto-prune + alerta staleness):\n'
disable_total_no_op
scratch_below_threshold_no_alert
scratch_above_threshold_alert_and_prune
memory_mcp_ausente_no_op
idempotencia_24h_skip

printf '\nResultado final: %d PASS, %d FAIL\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1