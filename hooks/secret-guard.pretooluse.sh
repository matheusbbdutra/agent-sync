#!/usr/bin/env bash
# Hook PreToolUse: bloqueia tool call se args contiver secret literal (JWT,
# AWS access key, GitHub PAT). Origem: ses_f2a16545bffeeCcxIFwZ04VVDa
# (2026-09-25) - user aprovou Pre+Post guard apos token Turso vazar em
# tool-outputs persistidos. Comportamental nao basta.
#
# Padrao JSON-RPC: emite {"hookSpecificOutput":{"hookEventName":"PreToolUse",
# "permissionDecision":"deny","permissionDecisionReason":"..."}} quando detecta
# secret; emite {} quando nao detecta (tool call segue normal).
#
# Regex (sem lookbehind, compativel com grep -E / RE2):
# - JWT: eyJ[A-Za-z0-9_=]+\.eyJ[A-Za-z0-9_=]+\.[A-Za-z0-9_-]+
# - AWS access key: AKIA[0-9A-Z]{16}
# - GitHub PAT: gh[pousr]_[A-Za-z0-9]{36,255}
set -uo pipefail

input="$(cat)"

# Detecta secret em todo o input (args vem em JSON).
match="$(printf '%s' "$input" | grep -oE 'eyJ[A-Za-z0-9_=]+\.eyJ[A-Za-z0-9_=]+\.[A-Za-z0-9_-]+|AKIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9]{36,255}' | head -1 || true)"

if [ -z "$match" ]; then
  # Sem secret detectado: permite tool call normalmente.
  printf '{}'
  exit 0
fi

# Identifica tipo para mensagem clara.
case "$match" in
  eyJ*) kind="JWT" ;;
  AKIA*) kind="AWS_ACCESS_KEY" ;;
  gh*) kind="GITHUB_PAT" ;;
  *) kind="SECRET" ;;
esac

# Bloqueia tool call com motivo claro. Encode do motivo em JSON via printf + sed.
reason="$(printf 'detectei %s literal nos args; use env var (ex.: AGENT_SYNC_TURSO_TOKEN) ou arquivo chmod 600 - nunca passe secrets inline' "$kind" | sed 's/"/\\"/g')"
printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"%s"}}' "$reason"
exit 0
