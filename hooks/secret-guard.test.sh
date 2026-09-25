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

# (relatorio final movido para depois dos casos novos de Camada 1)

# === Camada 1: deny-list de path (A-63) ===
# Adicionados em 2026-09-25 quando principios foram reposicionados:
# deny-list de path e defesa primaria; regex de literal e cinto+suspensorio.

# Caso PreToolUse 4: Read de .env -> deny (path na deny-list).
pre_read_dotenv() {
  local input='{"tool_name":"Read","tool_input":{"file_path":"/home/u/proj/.env"}}'
  local out
  out="$(printf '%s' "$input" | bash "$PRE")"
  if printf '%s' "$out" | grep -q '"permissionDecision":"deny"' && printf '%s' "$out" | grep -q 'path denylist'; then
    pass
  else
    fail "esperava permissionDecision deny + 'path denylist', obtive: $out"
  fi
}

# Caso PreToolUse 5: Read de ~/.aws/credentials -> deny.
pre_read_aws_credentials() {
  local input='{"tool_name":"Read","tool_input":{"file_path":"/home/u/.aws/credentials"}}'
  local out
  out="$(printf '%s' "$input" | bash "$PRE")"
  if printf '%s' "$out" | grep -q '"permissionDecision":"deny"' && printf '%s' "$out" | grep -q '/.aws'; then
    pass
  else
    fail "esperava deny + match '/.aws', obtive: $out"
  fi
}

# Caso PreToolUse 6: Read de ~/.ssh/id_rsa -> deny.
pre_read_ssh_id() {
  local input='{"tool_name":"Read","tool_input":{"file_path":"/home/u/.ssh/id_rsa"}}'
  local out
  out="$(printf '%s' "$input" | bash "$PRE")"
  if printf '%s' "$out" | grep -q '"permissionDecision":"deny"' && printf '%s' "$out" | grep -q '/.ssh'; then
    pass
  else
    fail "esperava deny + match '/.ssh', obtive: $out"
  fi
}

# Caso PreToolUse 7: Read de ~/.aws (diretorio sem trailing /) -> deny.
pre_read_aws_dir() {
  local input='{"tool_name":"Read","tool_input":{"file_path":"/home/u/.aws"}}'
  local out
  out="$(printf '%s' "$input" | bash "$PRE")"
  if printf '%s' "$out" | grep -q '"permissionDecision":"deny"'; then
    pass
  else
    fail "esperava deny para ~/.aws (dir sem /), obtive: $out"
  fi
}

# Caso PreToolUse 8: Bash cat ~/.zshrc -> deny.
pre_bash_cat_zshrc() {
  local input='{"tool_name":"Bash","tool_input":{"command":"cat ~/.zshrc"}}'
  local out
  out="$(printf '%s' "$input" | bash "$PRE")"
  if printf '%s' "$out" | grep -q '"permissionDecision":"deny"' && printf '%s' "$out" | grep -q '/.zshrc'; then
    pass
  else
    fail "esperava deny + match '/.zshrc', obtive: $out"
  fi
}

# Caso PreToolUse 9: Edit de .pem -> deny.
pre_edit_pem() {
  local input='{"tool_name":"Edit","tool_input":{"file_path":"/tmp/server.pem"}}'
  local out
  out="$(printf '%s' "$input" | bash "$PRE")"
  if printf '%s' "$out" | grep -q '"permissionDecision":"deny"' && printf '%s' "$out" | grep -q '\.pem'; then
    pass
  else
    fail "esperava deny + match '.pem', obtive: $out"
  fi
}

# Caso PreToolUse 10: Grep cwd=~/.aws -> deny.
pre_grep_aws_dir() {
  local input='{"tool_name":"Grep","tool_input":{"path":"/home/u/.aws","pattern":"x"}}'
  local out
  out="$(printf '%s' "$input" | bash "$PRE")"
  if printf '%s' "$out" | grep -q '"permissionDecision":"deny"'; then
    pass
  else
    fail "esperava deny para Grep cwd=~/.aws, obtive: $out"
  fi
}

# Caso PreToolUse 11: Glob cwd=/proj/secrets -> deny.
pre_glob_secrets() {
  local input='{"tool_name":"Glob","tool_input":{"path":"/proj/secrets","pattern":"*"}}'
  local out
  out="$(printf '%s' "$input" | bash "$PRE")"
  if printf '%s' "$out" | grep -q '"permissionDecision":"deny"'; then
    pass
  else
    fail "esperava deny para Glob cwd=/proj/secrets, obtive: $out"
  fi
}

# Caso PreToolUse 12: Read /tmp/file.txt (legit) -> allow.
pre_read_legit() {
  local input='{"tool_name":"Read","tool_input":{"file_path":"/tmp/file.txt"}}'
  local out
  out="$(printf '%s' "$input" | bash "$PRE")"
  if [ "$out" = "{}" ]; then pass; else fail "esperava '{}' para path legit, obtive: $out"; fi
}

# Caso PreToolUse 13: Read ~/.aws/config (outro arquivo da deny-list) -> deny.
pre_read_aws_config() {
  local input='{"tool_name":"Read","tool_input":{"file_path":"/home/u/.aws/config"}}'
  local out
  out="$(printf '%s' "$input" | bash "$PRE")"
  if printf '%s' "$out" | grep -q '"permissionDecision":"deny"'; then
    pass
  else
    fail "esperava deny para ~/.aws/config, obtive: $out"
  fi
}

# Caso PreToolUse 14: STDOUT termina com \n (anti 'invalid JSON output').
# Harness de Claude/Codex parseia stdout como JSON linha-unica;
# falta de \n no final gera 'hook returned invalid pre-tool-use JSON output'.
# Validacao: captura stdout em arquivo temporario (command substitution
# strip-newline-oficial), depois compara bytes com/sem \n via wc -c.
pre_newline_check() {
  local input='{"tool_name":"Bash","tool_input":{"command":"ls"}}'
  local tmpf="$(mktemp)"
  printf '%s' "$input" | bash "$PRE" > "$tmpf"
  local with_nl without_nl
  with_nl="$(wc -c < "$tmpf" | tr -d ' ')"
  without_nl="$(tr -d '\n' < "$tmpf" | wc -c | tr -d ' ')"
  rm -f "$tmpf"
  if [ "$with_nl" -gt "$without_nl" ]; then
    pass
  else
    fail "STDOUT nao termina com newline (got bytes=$with_nl, no-newline=$without_nl)"
  fi
}

# Caso PostToolUse 4: Read de arquivo na deny-list -> redacao total.
post_read_denied_redact_total() {
  local input='{"tool_name":"Read","tool_input":{"file_path":"/proj/.env"},"tool_output":"DATABASE_URL=postgres://u:p@h/db"}'
  local out
  out="$(printf '%s' "$input" | bash "$POST")"
  if printf '%s' "$out" | grep -q 'REDACTED:FILE_IN_DENYLIST' && ! printf '%s' "$out" | grep -q 'postgres://'; then
    pass
  else
    fail "esperava redacao total + sem postgres://, obtive: $out"
  fi
}

# Caso PostToolUse 5: Read de arquivo legit com JWT -> redacao JWT (Camada 2).
post_read_legit_jwt() {
  local jwt='eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature123'
  local input
  input="$(printf '{"tool_name":"Read","tool_input":{"file_path":"/proj/main.go"},"tool_output":"const T = %s"}' "$jwt")"
  local out
  out="$(printf '%s' "$input" | bash "$POST")"
  if printf '%s' "$out" | grep -q 'REDACTED:JWT' && ! printf '%s' "$out" | grep -q "$jwt"; then
    pass
  else
    fail "esperava REDACTED:JWT sem JWT original, obtive: $out"
  fi
}

printf '\nPreToolUse (Camada 1 path-deny):\n'
pre_read_dotenv
pre_read_aws_credentials
pre_read_ssh_id
pre_read_aws_dir
pre_bash_cat_zshrc
pre_edit_pem
pre_grep_aws_dir
pre_glob_secrets
pre_read_legit
pre_read_aws_config
pre_newline_check

printf '\nPostToolUse (Camada 1 redacao total + Camada 2):\n'
post_read_denied_redact_total
post_read_legit_jwt

printf '\nResultado final: %d PASS, %d FAIL\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1
