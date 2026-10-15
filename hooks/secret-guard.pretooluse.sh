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
# Antigravity (doc: agy-customizations/docs/hooks.md, secao "PreToolUse
# Contract"): `decision` (allow/deny/ask) e OBRIGATORIO e o payload usa
# `toolCall.name` / `toolCall.args.*` — nao `tool_name` / `file_path`.
# Sem `decision`, `{}` e interpretado como deny e o PreToolUse (matcher "*")
# bloqueia TODA tool call. Detectado em runtime 2026-10-14: o agente nao
# executava nem editava nada. O script decide o contrato pelo shape do
# payload (`toolCall` presente => Antigravity), mesmo padrao de
# bash-guardian.antigravity.sh.
#
# Regex (sem lookbehind, compativel com grep -E / RE2):
#   JWT: eyJ[A-Za-z0-9_=]+\.eyJ[A-Za-z0-9_=]+\.[A-Za-z0-9_-]+
#   AWS access key: AKIA[0-9A-Z]{16}
#   GitHub PAT: gh[pousr]_[A-Za-z0-9]{36,255}
set -uo pipefail

input="$(cat)"

emit_json() { printf '%s\n' "$1"; }

# Contrato de saida por CLI. `toolCall` no payload = Antigravity.
is_antigravity=0
case "$input" in
  *'"toolCall"'*) is_antigravity=1 ;;
esac

emit_allow() {
  if [ "$is_antigravity" -eq 1 ]; then
    printf '{"decision":"allow"}\n'
  else
    printf '{}\n'
  fi
}

emit_deny() {
  if [ "$is_antigravity" -eq 1 ]; then
    emit_json "{\"decision\":\"deny\",\"reason\":\"$1\"}"
  else
    emit_json "{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"$1\"}}"
  fi
}

extract_field() {
  local key="$1" data="$2"
  printf '%s' "$data" | grep -oE "${key}[[:space:]]*:[[:space:]]*\"[^\"]*\"" | head -n1 | sed -E "s/^${key}[[:space:]]*:[[:space:]]*\"(.*)\"$/\1/"
}

# === CAMADA 1: deny-list de path ===
# `candidates` = lista (newline-separated) de strings a testar. Antigravity
# nao documenta nomes de tool por tool; em vez de enumerar, testamos TODAS as
# strings de `toolCall.args` contra a deny-list (shape-agnostic). jq recursivo
# evita include-ir metadado do payload (workspacePaths, transcriptPath, ...) que
# fica FORA de args e causaria deny falso (ex.: sessao aberta em ~/.ssh).
candidates=""
tool_name=""

if [ "$is_antigravity" -eq 1 ]; then
  tool_name="$(printf '%s' "$input" | jq -r '.toolCall.name // ""' 2>/dev/null)"
  candidates="$(printf '%s' "$input" | jq -r '.toolCall.args | .. | strings' 2>/dev/null)"
else
  tool_name="$(extract_field '"tool_name"' "$input")"

  case "$tool_name" in
    Read|Edit|Write|MultiEdit|NotebookEdit)
      candidates="$(extract_field '"file_path"' "$input")"
      if [ -z "$candidates" ]; then
        candidates="$(extract_field '"notebook_path"' "$input")"
      fi
      if [ -z "$candidates" ]; then
        candidates="$(extract_field '"path"' "$input")"
      fi
      ;;
    Grep|Glob)
      candidates="$(extract_field '"path"' "$input")"
      ;;
    Bash)
      candidates="$(printf '%s' "$input" | { grep -oE '(/|~|\.)[^ "}{]+' || true; } | head -n1)"
      ;;
    *)
      candidates=""
      ;;
  esac
fi

if [ -n "$candidates" ]; then
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
  # Nota: `/config\.json` foi removido em 2026-09-29 — sem constraint de
  # diretorio, bloqueava legitimo como `~/.gemini/config/config.json`,
  # `~/.config/<app>/config.json` etc. Secrets reais ja estao cobertos
  # pelos patterns de diretorio acima (`/\.docker($|/)`, `/\.aws($|/)`,
  # `/\.kube($|/)`).
  deny_pattern='(/\.env($|\.)|/\.envrc$|\.pem$|\.key$|\.p12$|\.pfx$|/id_rsa|/id_ed25519|/id_ecdsa|/id_dsa|/\.ssh($|/)|/\.aws($|/)|/\.gnupg($|/)|/\.config/gh($|/)|/\.docker($|/)|/\.kube($|/)|\.netrc|\.npmrc|\.pypirc|\.pgpass|/\.aws/credentials|/\.aws/config|/hosts\.yml|/kube/config|\.terraformrc|/\.zshrc$|/\.bashrc$|/\.bash_profile$|/\.profile$|/\.zprofile$|/\.zshenv$|/\.bash_env$|credentials\.json|credentials\.yaml|/secrets($|/)|\.secret\.json|\.secret\.yaml|/\.env\.local$|/\.env\.production$|/\.env\.staging$|zscaler($|/))'

  while IFS= read -r candidate; do
    [ -z "$candidate" ] && continue
    if printf '%s' "$candidate" | grep -qiE "$deny_pattern"; then
      matched="$(printf '%s' "$candidate" | { grep -oiE "$deny_pattern" || true; } | head -n1)"
      emit_deny "path denylist (${tool_name}): ${candidate} casou '${matched}' - arquivo pode conter secret; use env var ou arquivo fora da deny-list"
      exit 0
    fi
  done <<< "$candidates"
fi

# === CAMADA 2: regex de literal nos args (cinto + suspensorio) ===
match="$(printf '%s' "$input" | { grep -oE 'eyJ[A-Za-z0-9_=]+\.eyJ[A-Za-z0-9_=]+\.[A-Za-z0-9_-]+|AKIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9]{36,255}' || true; } | head -1)"

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

emit_deny "detectei ${kind} literal nos args; use env var (ex.: AGENT_SYNC_TURSO_TOKEN) ou arquivo chmod 600 - nunca passe secrets inline"
exit 0
