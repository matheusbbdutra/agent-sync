#!/usr/bin/env bash
# Hook PreToolUse: bloqueia tool call que possa ler ou expor segredo.
# Defesa em 2 camadas:
#   Camada 1 (PRIMARIO): deny-list de path. Bloqueia Read/Edit/Write/
#     MultiEdit/Grep/Glob/Bash quando o path casa qualquer pattern que
#     costuma conter segredo (.env*, *.pem, *.key, ~/.aws/credentials,
#     ~/.ssh/, ~/.netrc, ~/.zshrc, zscaler/, ...).
#   Camada 2 (SECUNDARIO): regex de literal nos args. Bloqueia quando o
#     input contem JWT/AWS access key/GitHub PAT inline (sem path).
#
# Origem: ses_f2a16545bffeeCcxIFwZ04VVDa (2026-09-25) - user aprovou Pre+Post
# guard apos token Turso vazar em tool-outputs persistidos. Em 2026-09-25,
# principios reposicionados: deny-list de path e defesa primaria.
#
# Padrao JSON-RPC (Claude Code + Codex):
#   - Bloqueio: {"hookSpecificOutput":{"hookEventName":"PreToolUse",
#     "permissionDecision":"deny","permissionDecisionReason":"..."}}
#   - Permite: {}
#   - STDOUT termina com \n obrigatorio (harness falha com
#     'hook returned invalid pre-tool-use JSON output' sem newline).
#
# Regex (sem lookbehind, compativel com grep -E / RE2):
#   JWT: eyJ[A-Za-z0-9_=]+\.eyJ[A-Za-z0-9_=]+\.[A-Za-z0-9_-]+
#   AWS access key: AKIA[0-9A-Z]{16}
#   GitHub PAT: gh[pousr]_[A-Za-z0-9]{36,255}
set -uo pipefail

input="$(cat)"

emit_json() { printf '%s\n' "$1"; }
emit_allow() { printf '{}\n'; }

extract_field() {
  local key="$1" data="$2"
  printf '%s' "$data" | grep -oE "${key}[[:space:]]*:[[:space:]]*\"[^\"]*\"" | head -n1 | sed -E "s/^${key}[[:space:]]*:[[:space:]]*\"(.*)\"$/\1/"
}

# === CAMADA 1: deny-list de path ===
tool_name="$(extract_field '"tool_name"' "$input")"
candidate=""

case "$tool_name" in
  Read|Edit|Write|MultiEdit|NotebookEdit)
    candidate="$(extract_field '"file_path"' "$input")"
    if [ -z "$candidate" ]; then
      candidate="$(extract_field '"notebook_path"' "$input")"
    fi
    if [ -z "$candidate" ]; then
      candidate="$(extract_field '"path"' "$input")"
    fi
    ;;
  Grep|Glob)
    candidate="$(extract_field '"path"' "$input")"
    ;;
  Bash)
    candidate="$(printf '%s' "$input" | grep -oE '(/|~|\.)[^ "}{]+' | head -n1 || true)"
    ;;
  *)
    candidate=""
    ;;
esac

if [ -n "$candidate" ]; then
  # Deny-list (case-insensitive). Diretorios casam com OU sem trailing /
  # via alternacao (/aws($|/)). Padroes:
  #   .env*, *.pem, *.key, *.p12, *.pfx, id_rsa*, id_ed25519*, id_ecdsa*, id_dsa*
  #   ~/.ssh/, ~/.aws/, ~/.gnupg/, ~/.config/gh/, ~/.docker/, ~/.kube/, zscaler/
  #   .netrc, .npmrc, .pypirc, .pgpass
  #   credentials.json, credentials.yaml
  #   secrets/ (diretorio)
  #   .secret.json, .secret.yaml
  #   .env.local, .env.production, .env.staging
  #   ~/.zshrc, ~/.bashrc, ~/.bash_profile, ~/.profile, ~/.zprofile, ~/.zshenv, ~/.bash_env
  deny_pattern='(/\.env($|\.)|/\.envrc$|\.pem$|\.key$|\.p12$|\.pfx$|/id_rsa|/id_ed25519|/id_ecdsa|/id_dsa|/\.ssh($|/)|/\.aws($|/)|/\.gnupg($|/)|/\.config/gh($|/)|/\.docker($|/)|/\.kube($|/)|\.netrc|\.npmrc|\.pypirc|\.pgpass|/\.aws/credentials|/\.aws/config|/hosts\.yml|/config\.json|/kube/config|\.terraformrc|/\.zshrc$|/\.bashrc$|/\.bash_profile$|/\.profile$|/\.zprofile$|/\.zshenv$|/\.bash_env$|credentials\.json|credentials\.yaml|/secrets($|/)|\.secret\.json|\.secret\.yaml|/\.env\.local$|/\.env\.production$|/\.env\.staging$|zscaler($|/))'

  if printf '%s' "$candidate" | grep -qiE "$deny_pattern"; then
    matched="$(printf '%s' "$candidate" | grep -oiE "$deny_pattern" | head -n1 || true)"
    emit_json "{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"path denylist (${tool_name}): ${candidate} casou '${matched}' - arquivo pode conter secret; use env var ou arquivo fora da deny-list\"}}"
    exit 0
  fi
fi

# === CAMADA 2: regex de literal nos args (cinto + suspensorio) ===
match="$(printf '%s' "$input" | grep -oE 'eyJ[A-Za-z0-9_=]+\.eyJ[A-Za-z0-9_=]+\.[A-Za-z0-9_-]+|AKIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9]{36,255}' | head -1 || true)"

if [ -z "$match" ]; then
  emit_allow
  exit 0
fi

case "$match" in
  eyJ*) kind="JWT" ;;
  AKIA*) kind="AWS_ACCESS_KEY" ;;
  gh*) kind="GITHUB_PAT" ;;
  *) kind="SECRET" ;;
esac

emit_json "{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"detectei ${kind} literal nos args; use env var (ex.: AGENT_SYNC_TURSO_TOKEN) ou arquivo chmod 600 - nunca passe secrets inline\"}}"
exit 0
