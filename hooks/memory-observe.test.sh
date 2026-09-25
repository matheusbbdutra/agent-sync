#!/usr/bin/env bash
# Test runner para hooks/memory-observe.posttooluse.sh (A-66, patches b + c).
# Sem framework externo (sem bats): bash puro + assertions em shell exit code.
#
# Estrategia: usa AGENT_SYNC_MEMORY_BIN apontando para um fake `memory-mcp`
# em tmpdir. O hook chama `memory-mcp buffer-record ...` em background;
# o fake apenas conta invocacoes em um arquivo de log.
#
# Validamos:
#   - patch (b): filtro allowlist high-signal (Edit/Write/MultiEdit/
#     NotebookEdit/Bash gravam; Read/Grep/Glob/NotebookRead dao skip)
#   - patch (b) excecao: status!=ok sempre grava (erros sao alto-sinal)
#   - patch (c): SCOPE=high-signal|all|off + back-compat DISABLE=1
#
# Rodar via: bash hooks/memory-observe.test.sh
set -uo pipefail

HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
HOOK="$HOOK_DIR/memory-observe.posttooluse.sh"
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

# Setup tmpdir com fake memory-mcp. Cada teste reseta $TMP/calls.
setup() {
  TMP="$(mktemp -d)"
  export AGENT_SYNC_MEMORY_BIN="$TMP/memory-mcp"
  cat > "$AGENT_SYNC_MEMORY_BIN" <<'EOF'
#!/usr/bin/env bash
printf 'called\n' >> "${TMPDIR}/calls"
exit 0
EOF
  chmod +x "$AGENT_SYNC_MEMORY_BIN"
  export TMPDIR="$TMP"
  : > "$TMP/calls"
}

count_calls() {
  # Hook dispara `memory-mcp buffer-record` em background (`(...) &`);
  # test runner precisa aguardar o subshell terminar antes de contar.
  sleep 0.2
  if [ -f "$TMP/calls" ]; then
    wc -l < "$TMP/calls" | tr -d ' '
  else
    echo 0
  fi
}

# Helper: roda o hook com input JSON via stdin
run_hook() {
  local input="$1"
  printf '%s' "$input" | bash "$HOOK" >/dev/null 2>&1
}

# --- patch (b): filtro allowlist ---

# Caso 1: Read puro status=ok -> skip (Read nao eh mutacao)
read_pure_skip() {
  setup
  run_hook '{"session_id":"s","tool_name":"Read","status":"ok"}'
  local n
  n="$(count_calls)"
  if [ "$n" -eq 0 ]; then pass; else fail "Read puro: esperava 0 calls, obtive $n"; fi
}

# Caso 2: Edit status=ok -> grava
edit_grava() {
  setup
  run_hook '{"session_id":"s","tool_name":"Edit","status":"ok"}'
  local n
  n="$(count_calls)"
  if [ "$n" -eq 1 ]; then pass; else fail "Edit ok: esperava 1 call, obtive $n"; fi
}

# Caso 3: Write status=ok -> grava
write_grava() {
  setup
  run_hook '{"session_id":"s","tool_name":"Write","status":"ok"}'
  local n
  n="$(count_calls)"
  if [ "$n" -eq 1 ]; then pass; else fail "Write ok: esperava 1 call, obtive $n"; fi
}

# Caso 4: MultiEdit status=ok -> grava
multiedit_grava() {
  setup
  run_hook '{"session_id":"s","tool_name":"MultiEdit","status":"ok"}'
  local n
  n="$(count_calls)"
  if [ "$n" -eq 1 ]; then pass; else fail "MultiEdit ok: esperava 1 call, obtive $n"; fi
}

# Caso 5: NotebookEdit status=ok -> grava
notebookedit_grava() {
  setup
  run_hook '{"session_id":"s","tool_name":"NotebookEdit","status":"ok"}'
  local n
  n="$(count_calls)"
  if [ "$n" -eq 1 ]; then pass; else fail "NotebookEdit ok: esperava 1 call, obtive $n"; fi
}

# Caso 6: Bash status=ok -> grava (comando shell eh alto-sinal)
bash_grava() {
  setup
  run_hook '{"session_id":"s","tool_name":"Bash","status":"ok","command":"ls -la"}'
  local n
  n="$(count_calls)"
  if [ "$n" -eq 1 ]; then pass; else fail "Bash ok: esperava 1 call, obtive $n"; fi
}

# Caso 7: Grep status=ok -> skip (read-only)
grep_skip() {
  setup
  run_hook '{"session_id":"s","tool_name":"Grep","status":"ok"}'
  local n
  n="$(count_calls)"
  if [ "$n" -eq 0 ]; then pass; else fail "Grep ok: esperava 0 calls, obtive $n"; fi
}

# Caso 8: Glob status=ok -> skip (read-only)
glob_skip() {
  setup
  run_hook '{"session_id":"s","tool_name":"Glob","status":"ok"}'
  local n
  n="$(count_calls)"
  if [ "$n" -eq 0 ]; then pass; else fail "Glob ok: esperava 0 calls, obtive $n"; fi
}

# Caso 9: NotebookRead status=ok -> skip (read-only)
notebookread_skip() {
  setup
  run_hook '{"session_id":"s","tool_name":"NotebookRead","status":"ok"}'
  local n
  n="$(count_calls)"
  if [ "$n" -eq 0 ]; then pass; else fail "NotebookRead ok: esperava 0 calls, obtive $n"; fi
}

# Caso 10: Read com status=error -> grava (excecao: erro eh alto-sinal)
status_error_grava_mesmo_read() {
  setup
  run_hook '{"session_id":"s","tool_name":"Read","status":"error","error":"file not found"}'
  local n
  n="$(count_calls)"
  if [ "$n" -eq 1 ]; then pass; else fail "Read error: esperava 1 call (erro alto-sinal), obtive $n"; fi
}

# --- patch (c): SCOPE + back-compat ---

# Caso 11: SCOPE=off -> skip total (independente de tool)
scope_off_skip_total() {
  setup
  export AGENT_SYNC_MEMORY_OBSERVE_SCOPE=off
  run_hook '{"session_id":"s","tool_name":"Edit","status":"ok"}'
  run_hook '{"session_id":"s","tool_name":"Bash","status":"ok"}'
  run_hook '{"session_id":"s","tool_name":"Read","status":"ok"}'
  local n
  n="$(count_calls)"
  if [ "$n" -eq 0 ]; then pass; else fail "SCOPE=off: esperava 0 calls, obtive $n"; fi
  unset AGENT_SYNC_MEMORY_OBSERVE_SCOPE
}

# Caso 12: SCOPE=all + Read -> grava (filtro desativado, mesmo Read passa)
scope_all_grava_read() {
  setup
  export AGENT_SYNC_MEMORY_OBSERVE_SCOPE=all
  run_hook '{"session_id":"s","tool_name":"Read","status":"ok"}'
  local n
  n="$(count_calls)"
  if [ "$n" -eq 1 ]; then pass; else fail "SCOPE=all + Read: esperava 1 call, obtive $n"; fi
  unset AGENT_SYNC_MEMORY_OBSERVE_SCOPE
}

# Caso 13: SCOPE=high-signal explicito + Read -> skip (filtro ativo)
scope_high_signal_explicito_filtra() {
  setup
  export AGENT_SYNC_MEMORY_OBSERVE_SCOPE=high-signal
  run_hook '{"session_id":"s","tool_name":"Read","status":"ok"}'
  local n
  n="$(count_calls)"
  if [ "$n" -eq 0 ]; then pass; else fail "SCOPE=high-signal + Read: esperava 0 calls, obtive $n"; fi
  unset AGENT_SYNC_MEMORY_OBSERVE_SCOPE
}

# Caso 14: SCOPE=invalid -> default silencioso para operational
scope_invalid_default_operational() {
  setup
  export AGENT_SYNC_MEMORY_OBSERVE_SCOPE=foobar
  run_hook '{"session_id":"s","tool_name":"Read","status":"ok"}'
  run_hook '{"session_id":"s","tool_name":"Edit","status":"ok"}'
  local n
  n="$(count_calls)"
  # Read skipado (default operational), Edit gravado
  if [ "$n" -eq 1 ]; then pass; else fail "SCOPE=foobar (default operational): esperava 1 call (Edit only), obtive $n"; fi
  unset AGENT_SYNC_MEMORY_OBSERVE_SCOPE
}

# Caso 15: back-compat AGENT_SYNC_MEMORY_OBSERVE_DISABLE=1 -> equivale a off
disable_back_compat_equiv_off() {
  setup
  export AGENT_SYNC_MEMORY_OBSERVE_DISABLE=1
  run_hook '{"session_id":"s","tool_name":"Edit","status":"ok"}'
  run_hook '{"session_id":"s","tool_name":"Bash","status":"ok"}'
  local n
  n="$(count_calls)"
  if [ "$n" -eq 0 ]; then pass; else fail "DISABLE=1 (back-compat): esperava 0 calls, obtive $n"; fi
  unset AGENT_SYNC_MEMORY_OBSERVE_DISABLE
}

# Caso 16: A-71 - buffer-record recebe flags -source auto-hook -kind action -retention scratch
buffer_record_recebe_flags_a71() {
  TMP="$(mktemp -d)"
  export AGENT_SYNC_MEMORY_BIN="$TMP/memory-mcp"
  cat > "$AGENT_SYNC_MEMORY_BIN" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${TMPDIR}/args.log"
exit 0
EOF
  chmod +x "$AGENT_SYNC_MEMORY_BIN"
  export TMPDIR="$TMP"
  : > "$TMP/args.log"
  run_hook '{"session_id":"sess-a71","tool_name":"Edit","status":"ok","command":"test"}'
  sleep 0.2
  local args
  args="$(cat "$TMP/args.log" 2>/dev/null || true)"
  if echo "$args" | grep -q -- "-source auto-hook" \
    && echo "$args" | grep -q -- "-kind action" \
    && echo "$args" | grep -q -- "-retention scratch"; then
    pass
  else
    fail "buffer-record nao recebeu flags A-71 esperadas: $args"
  fi
}

printf '\nmemory-observe (A-66 patch b — filtro allowlist high-signal):\n'
read_pure_skip
edit_grava
write_grava
multiedit_grava
notebookedit_grava
bash_grava
grep_skip
glob_skip
notebookread_skip
status_error_grava_mesmo_read

printf '\nmemory-observe (A-66 patch c / A-71 — SCOPE + back-compat):\n'
scope_off_skip_total
scope_all_grava_read
scope_high_signal_explicito_filtra
scope_invalid_default_operational
disable_back_compat_equiv_off
buffer_record_recebe_flags_a71

printf '\nResultado final: %d PASS, %d FAIL\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1