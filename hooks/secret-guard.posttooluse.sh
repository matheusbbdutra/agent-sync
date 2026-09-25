#!/usr/bin/env bash
# Hook PostToolUse: redaciona output se contiver secret literal OU se
# o path do arquivo lido estiver na deny-list (redacao total).
#
# Defesa em 2 camadas (espelhando o PreToolUse):
#   Camada 1: se tool_name e Read/Edit/etc. e o file_path casa a deny-list
#     (mesma regex do pretooluse.sh), substitui TODO o tool_output por
#     <REDACTED:FILE_IN_DENYLIST>. Justificativa: arquivo na deny-list
#     NAO deveria ter sido lido (PreToolUse deveria ter barrado);
#     redacao total garante defesa em profundidade.
#   Camada 2: regex de literal (JWT/AWS/GitHub PAT) preservada como
#     antes para outputs de arquivos fora da deny-list.
#
# Padrao JSON-RPC: recebe JSON com tool_output via stdin, retorna JSON
# com tool_output redacted (substitui matches por <REDACTED:TIPO>) OU
# retorna input intacto se nada mudou.
#
# STDOUT termina com \n obrigatorio (mesma razao do pretooluse.sh).
#
# IMPORTANTE: este hook roda DEPOIS da tool. Output persistido em disco
# fica redacted, mas agente JA viu o secret em memoria de trabalho. Por
# isso a defesa em profundidade com PreToolUse (que impede o tool call
# antes).
set -uo pipefail

input="$(cat)"

emit_json() { printf '%s\n' "$1"; }

extract_field() {
  local key="$1" data="$2"
  printf '%s' "$data" | grep -oE "${key}[[:space:]]*:[[:space:]]*\"[^\"]*\"" | head -n1 | sed -E "s/^${key}[[:space:]]*:[[:space:]]*\"(.*)\"$/\1/"
}

# Extrai o tool_output (JSON value escapado pode ter \n literais; tratamos simples).
tool_output="$(printf '%s' "$input" | grep -oE '"tool_output"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed -E 's/^"tool_output"[[:space:]]*:[[:space:]]*"(.*)"$/\1/')"

if [ -z "$tool_output" ]; then
  # Sem tool_output (caso edge: hook chamado sem output de tool); repassa intacto.
  printf '%s\n' "$input"
  exit 0
fi

# === CAMADA 1: path denylist -> redacao total ===
tool_name="$(extract_field '"tool_name"' "$input")"
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
  *)
    candidate=""
    ;;
esac

if [ -n "$candidate" ]; then
  deny_pattern='(/\.env($|\.)|/\.envrc$|\.pem$|\.key$|\.p12$|\.pfx$|/id_rsa|/id_ed25519|/id_ecdsa|/id_dsa|/\.ssh($|/)|/\.aws($|/)|/\.gnupg($|/)|/\.config/gh($|/)|/\.docker($|/)|/\.kube($|/)|\.netrc|\.npmrc|\.pypirc|\.pgpass|/\.aws/credentials|/\.aws/config|/hosts\.yml|/config\.json|/kube/config|\.terraformrc|/\.zshrc$|/\.bashrc$|/\.bash_profile$|/\.profile$|/\.zprofile$|/\.zshenv$|/\.bash_env$|credentials\.json|credentials\.yaml|/secrets($|/)|\.secret\.json|\.secret\.yaml|/\.env\.local$|/\.env\.production$|/\.env\.staging$|zscaler($|/))'

  if printf '%s' "$candidate" | grep -qiE "$deny_pattern"; then
    # Redacao total: substitui tool_output por <REDACTED:FILE_IN_DENYLIST>.
    emit_json "<REDACTED:FILE_IN_DENYLIST:${candidate}>"
    exit 0
  fi
fi

# === CAMADA 2: regex de literal (defesa em profundidade) ===
redacted="$tool_output"
redacted="$(printf '%s' "$redacted" | sed -E 's/eyJ[A-Za-z0-9_=]+\.eyJ[A-Za-z0-9_=]+\.[A-Za-z0-9_-]+/<REDACTED:JWT>/g')"
redacted="$(printf '%s' "$redacted" | sed -E 's/AKIA[0-9A-Z]{16}/<REDACTED:AWS_ACCESS_KEY>/g')"
redacted="$(printf '%s' "$redacted" | sed -E 's/gh[pousr]_[A-Za-z0-9]{36,255}/<REDACTED:GITHUB_PAT>/g')"

# Se nada mudou, repassa input intacto (evita hook mudo em tool normal).
if [ "$redacted" = "$tool_output" ]; then
  printf '%s\n' "$input"
  exit 0
fi

# Substitui o tool_output no JSON. Para simplicidade, mantemos apenas
# tool_output (descarta outros campos do input; hook PostToolUse de Claude/
# Codex aceita tool_output como unico campo).
printf '%s\n' "$redacted"
exit 0
