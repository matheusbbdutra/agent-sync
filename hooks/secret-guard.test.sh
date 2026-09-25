#!/usr/bin/env bash
# Test runner para secret-guard.pretooluse.sh e secret-guard.posttooluse.sh.
# Sem framework externo (sem bats): bash puro + assertions em shell exit code.
#
# Cada caso monta input JSON via printf, executa o hook, valida stdout contra
# regex esperada. Falha = exit != 0 + mensagem clara.
#
# Rodar via: bash hooks/secret-guard.test.sh
# Ou via make: make test-hooks (target opcional).
set -uo pipefail

HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
PRE="$HOOK_DIR/secret-guard.pretooluse.sh"
POST="$HOOK_DIR/secret-guard.posttooluse.sh"
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

# Caso PreToolUse 1: input sem secret -> hook emite {} (permite tool call).
pre_no_secret() {
  local input='{"tool_name":"Bash","tool_input":{"command":"ls -la"}}'
  local out
  out="$(printf '%s' "$input" | bash "$PRE")"
  if [ "$out" = "{}" ]; then pass; else fail "esperava '{}', obtive: $out"; fi
}

# Caso PreToolUse 2: input com JWT literal em args -> hook emite permissionDecision deny.
# Nota: o hook NAO repete o secret raw no output (privacidade); valida pelo TIPO detectado.
pre_jwt_in_args() {
  local jwt='eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature123'
  local input
  input="$(printf '{"tool_name":"Bash","tool_input":{"command":"curl -H Authorization: Bearer %s https://api.example.com"}}' "$jwt")"
  local out
  out="$(printf '%s' "$input" | bash "$PRE")"
  if printf '%s' "$out" | grep -q '"permissionDecision":"deny"' && printf '%s' "$out" | grep -q 'detectei JWT literal'; then
    pass
  else
    fail "esperava permissionDecision deny + 'detectei JWT literal', obtive: $out"
  fi
}

# Caso PreToolUse 3: input com AWS key -> hook bloqueia.
pre_aws_key_in_args() {
  local aws='AKIAIOSFODNN7EXAMPLE'
  local input
  input="$(printf '{"tool_name":"Bash","tool_input":{"command":"aws s3 ls --access-key %s"}}' "$aws")"
  local out
  out="$(printf '%s' "$input" | bash "$PRE")"
  if printf '%s' "$out" | grep -q '"permissionDecision":"deny"' && printf '%s' "$out" | grep -q 'detectei AWS_ACCESS_KEY'; then
    pass
  else
    fail "esperava permissionDecision deny + 'detectei AWS_ACCESS_KEY', obtive: $out"
  fi
}

# Caso PostToolUse 1: output sem secret -> hook repassa output inalterado.
post_no_secret() {
  local input='{"tool_output":"total 4\ndrwxr-xr-x 2 user user 4096 sep 25 12:00 .\n"}'
  local out
  out="$(printf '%s' "$input" | bash "$POST")"
  if printf '%s' "$out" | grep -q '"tool_output":"total 4'; then pass; else fail "output deveria passar intacto, obtive: $out"; fi
}

# Caso PostToolUse 2: output com JWT -> hook redaciona para <REDACTED:JWT>.
post_jwt_in_output() {
  local jwt='eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature123'
  local input
  input="$(printf '{"tool_output":"token: %s"}' "$jwt")"
  local out
  out="$(printf '%s' "$input" | bash "$POST")"
  if printf '%s' "$out" | grep -q 'REDACTED:JWT' && ! printf '%s' "$out" | grep -q "$jwt"; then
    pass
  else
    fail "esperava REDACTED:JWT sem o JWT original, obtive: $out"
  fi
}

# Caso PostToolUse 3: output com GitHub PAT -> hook redaciona.
post_github_in_output() {
  local gh='ghp_1234567890abcdefghijklmnopqrstuvwxyzABCDE'
  local input
  input="$(printf '{"tool_output":"export TOKEN=%s"}' "$gh")"
  local out
  out="$(printf '%s' "$input" | bash "$POST")"
  if printf '%s' "$out" | grep -q 'REDACTED:GITHUB' && ! printf '%s' "$out" | grep -q "$gh"; then
    pass
  else
    fail "esperava REDACTED:GITHUB sem o PAT original, obtive: $out"
  fi
}

printf 'PreToolUse:\n'
pre_no_secret
pre_jwt_in_args
pre_aws_key_in_args
printf 'PostToolUse:\n'
post_no_secret
post_jwt_in_output
post_github_in_output

printf '\nResultado: %d PASS, %d FAIL\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1
