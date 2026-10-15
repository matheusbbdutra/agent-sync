#!/usr/bin/env bash
# Test runner do secret-guard (regra §4: lógica vive em tools/cmd/secret-scan).
# Testa o CONTRATO dos adapters + o binário Go (quando compilado).
# Sem framework externo: bash puro + assertions em exit code.
# Rodar via: bash hooks/secret-guard.test.sh
set -uo pipefail

HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
PRE="$HOOK_DIR/secret-guard.pretooluse.sh"
POST="$HOOK_DIR/secret-guard.posttooluse.sh"
BIN_CANDIDATES=("$HOOK_DIR/../bin/secret-scan" "$(command -v secret-scan || true)")
BIN=""
for c in "${BIN_CANDIDATES[@]}"; do
  if [ -n "$c" ] && [ -x "$c" ]; then BIN="$c"; break; fi
done
PASS=0
FAIL=0

fail() { printf '  FAIL: %s\n' "$1" >&2; FAIL=$((FAIL + 1)); }
pass() { printf '  ok\n'; PASS=$((PASS + 1)); }
skip() { printf '  SKIP: %s\n' "$1"; }

# Adapters sao fail-open: sem binário, stdout deve ser '{}' com exit 0.
adapter_pre_no_bin() {
  local out rc
  out="$(mkdir -p /tmp/sg_nobin/hooks && cp "$PRE" /tmp/sg_nobin/hooks/ && printf '%s' '{"tool_name":"Read","tool_input":{"file_path":"/x/.env"}}' | PATH=/usr/bin:/bin bash /tmp/sg_nobin/hooks/$(basename "$PRE"))"; rc=$?
  if [ "$rc" -eq 0 ] && [ "$out" = "{}" ]; then pass; else fail "pre fail-open: rc=$rc out=$out"; fi
}

adapter_post_no_bin() {
  local out rc
  out="$(mkdir -p /tmp/sg_nobin/hooks && cp "$POST" /tmp/sg_nobin/hooks/ && printf '%s' '{"tool_output":"x"}' | PATH=/usr/bin:/bin bash /tmp/sg_nobin/hooks/$(basename "$POST"))"; rc=$?
  if [ "$rc" -eq 0 ] && [ "$out" = "{}" ]; then pass; else fail "post fail-open: rc=$rc out=$out"; fi
}

# Com binário: deny real (Camada 1 path).
bin_pre_deny_env() {
  [ -z "$BIN" ] && { skip "secret-scan ausente"; return; }
  local out
  out="$(printf '%s' '{"tool_name":"Read","tool_input":{"file_path":"/proj/.env"}}' | "$BIN" check --format decision)"
  if printf '%s' "$out" | grep -q '"decision":"deny"'; then pass; else fail "deny .env: $out"; fi
}

bin_pre_allow_legit() {
  [ -z "$BIN" ] && { skip "secret-scan ausente"; return; }
  local out
  out="$(printf '%s' '{"tool_name":"Read","tool_input":{"file_path":"/proj/main.go"}}' | "$BIN" check --format decision)"
  if printf '%s' "$out" | grep -q '"decision":"allow"'; then pass; else fail "allow legit: $out"; fi
}

# Com binário: Camada 2 literal inline.
bin_pre_deny_jwt() {
  [ -z "$BIN" ] && { skip "secret-scan ausente"; return; }
  local jwt='eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature123'
  local out
  out="$(printf '{"tool_name":"Bash","tool_input":{"command":"curl -H X: %s"}}' "$jwt" | "$BIN" check --format decision)"
  if printf '%s' "$out" | grep -q '"decision":"deny"'; then pass; else fail "deny JWT: $out"; fi
}

bin_redact_total() {
  [ -z "$BIN" ] && { skip "secret-scan ausente"; return; }
  local out
  out="$(printf '%s' '{"tool_name":"Read","tool_input":{"file_path":"/proj/.env"},"tool_output":"X=1"}' | "$BIN" redact --format decision)"
  if printf '%s' "$out" | grep -q '"action":"redact-total"'; then pass; else fail "redact-total: $out"; fi
}

bin_redact_inline() {
  [ -z "$BIN" ] && { skip "secret-scan ausente"; return; }
  local jwt='eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature123'
  local out
  out="$(printf '{"tool_name":"Read","tool_input":{"file_path":"/proj/main.go"},"tool_output":"T=%s"}' "$jwt" | "$BIN" redact --format decision)"
  if printf '%s' "$out" | grep -q '"action":"redact-inline"' && ! printf '%s' "$out" | grep -q "$jwt"; then pass; else fail "redact-inline: $out"; fi
}

bin_redact_none() {
  [ -z "$BIN" ] && { skip "secret-scan ausente"; return; }
  local out
  out="$(printf '%s' '{"tool_output":"total 4"}' | "$BIN" redact --format decision)"
  if printf '%s' "$out" | grep -q '"action":"none"'; then pass; else fail "redact-none: $out"; fi
}

# Com binário: contrato auto por host (Claude {} / Antigravity decision).
bin_pre_auto_claude() {
  [ -z "$BIN" ] && { skip "secret-scan ausente"; return; }
  local out
  out="$(printf '%s' '{"tool_name":"Bash","tool_input":{"command":"ls"}}' | "$BIN" check --format auto)"
  if [ "$out" = "{}" ]; then pass; else fail "auto claude allow: $out"; fi
}

bin_pre_auto_antigravity() {
  [ -z "$BIN" ] && { skip "secret-scan ausente"; return; }
  local out
  out="$(printf '%s' '{"toolCall":{"name":"run_command","args":{"command":"ls"}}}' | "$BIN" check --format auto)"
  if printf '%s' "$out" | grep -q '"decision":"allow"'; then pass; else fail "auto agy allow: $out"; fi
}

printf '\nAdapters (fail-open sem binario):\n'
adapter_pre_no_bin
adapter_post_no_bin

printf '\nBinario secret-scan (contrato decision):\n'
bin_pre_deny_env
bin_pre_allow_legit
bin_pre_deny_jwt
bin_redact_total
bin_redact_inline
bin_redact_none
bin_pre_auto_claude
bin_pre_auto_antigravity

printf '\nResultado final: %d PASS, %d FAIL\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1
