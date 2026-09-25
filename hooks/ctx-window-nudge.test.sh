#!/usr/bin/env bash
# Test runner para hooks/ctx-window-nudge.sh (A-66, patches a + b).
# Sem framework externo (sem bats): bash puro + assertions em shell exit code.
#
# Estrategia: usa AGENT_SYNC_MEMORY_BIN apontando para um fake `memory-mcp`
# em tmpdir que conta quantas vezes foi invocado (cada stdin tem 3 linhas JSON-RPC:
# initialize + notifications/initialized + tools/call). Validamos cadencia do
# mirror JSON-RPC com count%MIRROR_THRESHOLD==0.
#
# Rodar via: bash hooks/ctx-window-nudge.test.sh
set -uo pipefail

HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
HOOK="$HOOK_DIR/ctx-window-nudge.sh"
PASS=0
FAIL=0

fail() {
  printf '  FAIL: %s\n' "$1" >&2
  FAIL=$((FAIL + 1))
}

pass() {
  printf '  ok\n'
  PASS=$((PASS + 1))
}

# Setup tmpdir com fake memory-mcp + state dir limpo + PATH do fake.
setup() {
  TMP="$(mktemp -d)"
  # ctx-window-nudge usa `command -v memory-mcp` (nao respeita
  # AGENT_SYNC_MEMORY_BIN), entao fake precisa estar no PATH.
  cat > "$TMP/memory-mcp" <<'EOF'
#!/usr/bin/env bash
# Conta quantas vezes o hook chamou via "tools/call" (3a linha JSON-RPC)
n=0
while IFS= read -r line; do
  n=$((n + 1))
done
printf '%s\n' "$n" >> "$TMPDIR/fake-memory-mcp.invocations"
EOF
  chmod +x "$TMP/memory-mcp"
  export PATH="$TMP:$PATH"
  # state dir limpo para cada teste
  STATE_DIR="$TMP/state"
  mkdir -p "$STATE_DIR"
  export AGENT_SYNC_CTX_WINDOW_NUDGE_STATE_DIR="$STATE_DIR"
  export TMPDIR="$TMP"
  : > "$TMP/fake-memory-mcp.invocations"
}

# Helper: executa o hook N vezes simulando N tool calls (count incremental)
run_n_tool_calls() {
  local n="$1"
  local i
  for i in $(seq 1 "$n"); do
    printf '{"session_id":"test","tool_name":"Bash"}' | bash "$HOOK" >/dev/null 2>&1
  done
}

# Helper: conta invocacoes do fake memory-mcp (1 por tool call onde count%MIRROR==0)
count_mirror_invocations() {
  if [ -f "$TMP/fake-memory-mcp.invocations" ]; then
    wc -l < "$TMP/fake-memory-mcp.invocations" | tr -d ' '
  else
    echo 0
  fi
}

# Caso 1: count<MIN_TOOL_CALLS e summary presente recente -> no trigger, no mirror
count_below_min_no_trigger() {
  setup
  # summary.md "recente" (criado agora) com MAX_AGE_HOURS=4
  mkdir -p "$TMP/.agent-sync"
  printf 'fake' > "$TMP/.agent-sync/summary.md"
  cd "$TMP"
  AGENT_SYNC_CTX_WINDOW_NUDGE_MIN_TOOL_CALLS=5 \
  AGENT_SYNC_CTX_WINDOW_NUDGE_MAX_AGE_HOURS=4 \
  AGENT_SYNC_CTX_WINDOW_NUDGE_MIRROR_THRESHOLD=1 \
    run_n_tool_calls 4
  local inv
  inv="$(count_mirror_invocations)"
  if [ "$inv" -eq 0 ]; then pass; else fail "count=4 < MIN=5: esperava 0 invocations, obtive $inv"; fi
  cd - >/dev/null
}

# Caso 2: count=MIN_TOOL_CALLS com threshold=5 -> mirror dispara 1x (count%5==0)
count_above_min_trigger_mirror_first() {
  setup
  mkdir -p "$TMP/.agent-sync"
  printf 'fake' > "$TMP/.agent-sync/summary.md"
  cd "$TMP"
  AGENT_SYNC_CTX_WINDOW_NUDGE_MIN_TOOL_CALLS=5 \
  AGENT_SYNC_CTX_WINDOW_NUDGE_MAX_AGE_HOURS=4 \
  AGENT_SYNC_CTX_WINDOW_NUDGE_MIRROR_THRESHOLD=5 \
    run_n_tool_calls 5
  local inv
  inv="$(count_mirror_invocations)"
  if [ "$inv" -eq 1 ]; then pass; else fail "count=5, MIN=5, THR=5: esperava 1 inv, obtive $inv (count%5=0)"; fi
  cd - >/dev/null
}

# Caso 3: count=6 com threshold=5 -> 1 inv (em count=5) e a 6a NAO adiciona mais
# Roda 5 tool calls (espera 1 inv em count=5), depois 1 call extra (count=6,
# 6%5!=0) -> continua 1 inv total. Validamos que off-cycle nao espelha.
count_above_min_trigger_no_mirror_off_cycle() {
  setup
  mkdir -p "$TMP/.agent-sync"
  printf 'fake' > "$TMP/.agent-sync/summary.md"
  cd "$TMP"
  AGENT_SYNC_CTX_WINDOW_NUDGE_MIN_TOOL_CALLS=5 \
  AGENT_SYNC_CTX_WINDOW_NUDGE_MAX_AGE_HOURS=4 \
  AGENT_SYNC_CTX_WINDOW_NUDGE_MIRROR_THRESHOLD=5 \
    run_n_tool_calls 5
  local inv_after_5
  inv_after_5="$(count_mirror_invocations)"
  if [ "$inv_after_5" -ne 1 ]; then
    fail "apos count=5 (on-cycle): esperava 1 inv, obtive $inv_after_5"
    cd - >/dev/null
    return
  fi
  # mais 1 call -> count=6 (off-cycle) -> nao deve incrementar
  AGENT_SYNC_CTX_WINDOW_NUDGE_MIN_TOOL_CALLS=5 \
  AGENT_SYNC_CTX_WINDOW_NUDGE_MAX_AGE_HOURS=4 \
  AGENT_SYNC_CTX_WINDOW_NUDGE_MIRROR_THRESHOLD=5 \
    run_n_tool_calls 1
  local inv_after_6
  inv_after_6="$(count_mirror_invocations)"
  if [ "$inv_after_6" -eq 1 ]; then pass; else fail "apos count=6 (off-cycle): esperava continuar 1 inv, obtive $inv_after_6"; fi
  cd - >/dev/null
}

# Caso 4: count=50 com MIN=10 THR=25 -> mirror em count=25, 50 (NAO em 10..24, 26..49, exceto 25)
count_above_min_trigger_mirror_cadence_25() {
  setup
  mkdir -p "$TMP/.agent-sync"
  printf 'fake' > "$TMP/.agent-sync/summary.md"
  cd "$TMP"
  AGENT_SYNC_CTX_WINDOW_NUDGE_MIN_TOOL_CALLS=10 \
  AGENT_SYNC_CTX_WINDOW_NUDGE_MAX_AGE_HOURS=4 \
  AGENT_SYNC_CTX_WINDOW_NUDGE_MIRROR_THRESHOLD=25 \
    run_n_tool_calls 50
  local inv
  inv="$(count_mirror_invocations)"
  # count=25, 50 sao multiplos de 25 a partir de MIN=10 (25>=10 e 50>=10)
  if [ "$inv" -eq 2 ]; then pass; else fail "count=50, MIN=10, THR=25: esperava 2 inv (25, 50), obtive $inv"; fi
  cd - >/dev/null
}

# Caso 5: summary ausente -> trigger imediato, mirror no count=1 com THR=1
summary_missing_trigger_immediate() {
  setup
  # nao cria summary.md
  cd "$TMP"
  AGENT_SYNC_CTX_WINDOW_NUDGE_MIN_TOOL_CALLS=100 \
  AGENT_SYNC_CTX_WINDOW_NUDGE_MAX_AGE_HOURS=4 \
  AGENT_SYNC_CTX_WINDOW_NUDGE_MIRROR_THRESHOLD=1 \
    run_n_tool_calls 1
  local inv
  inv="$(count_mirror_invocations)"
  if [ "$inv" -eq 1 ]; then pass; else fail "summary missing, THR=1: esperava 1 inv, obtive $inv"; fi
  cd - >/dev/null
}

printf '\nctx-window-nudge (A-66 patch a — cadence count%%THRESHOLD==0):\n'
count_below_min_no_trigger
count_above_min_trigger_mirror_first
count_above_min_trigger_no_mirror_off_cycle
count_above_min_trigger_mirror_cadence_25
summary_missing_trigger_immediate

printf '\nResultado final: %d PASS, %d FAIL\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1