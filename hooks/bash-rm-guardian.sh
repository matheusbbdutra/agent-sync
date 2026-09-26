#!/usr/bin/env bash
# bash-rm-guardian — PreToolUse:Bash hook que detecta comandos destrutivos
# (rm -rf, rm -r, rm -fr, rmdir, mv sobre diretório) e roda um audit textual
# (repo-map --audit-removal <target>) antes da execução.
#
# Comportamento (A-74):
#   - injectSteps warn: NUNCA bloqueia (não usa deny).
#   - Se blockers == 0: stdout vazio ({}).
#   - Se blockers > 0: stdout com injectSteps listando refs ativas para o
#     modelo revisar antes de prosseguir.
#
# Contrato por CLI:
#   - Antigravity (PreToolUse): {"decision":"allow"} + injectSteps opcional.
#   - Cursor (beforeShellExecution): {"permission":"allow"} + agent_message opcional.
#   - OpenCode (TS): adapter emite stdout específico.
#   - Codex (PreToolUse): hookSpecificOutput.permissionDecision=allow.
#   - Claude: regra permissions em settings.json (não usa este script).
#
# Este script é o CORE compartilhado — cada wrapper fino por CLI invoca
# este core e adapta o output para o contrato.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Modos:
#   - "analyze": recebe command_line em $1, retorna blockers via stdout JSON
#                no contrato neutro {"blockers":[...],"target":"..."}
#   - "wrap-<cli>": recebe o stdin do hook (payload completo), detecta o
#                   command_line, chama analyze, e emite no contrato da CLI
#
# O wrapper por CLI deve exportar AGENT_SYNC_ROOT (root do repo) e usar
# `analyze` para extrair blockers. Este script centraliza o parse.

analyze() {
  local cmd="$1"
  local root="${AGENT_SYNC_ROOT:-.}"

  # Detecta comando destrutivo em diretório: rm -rf, rm -r, rm -fr, rmdir,
  # mv sobre dir. Ignora rm arquivo único (rm foo.txt) para reduzir ruído.
  local target=""
  case "$cmd" in
    *'rm -rf'*|*'rm -fr'*|*'rm -r '*|*'rm -Rf'*|*'rm -fR'*)
      target="$(printf '%s' "$cmd" | awk '{for(i=1;i<=NF;i++) if($i=="rm"||$i~/^rm$/) {for(j=i+1;j<=NF;j++) if($j!~/^-/) {print $j; exit}}}' 2>/dev/null || true)"
      ;;
    *rmdir*)
      target="$(printf '%s' "$cmd" | sed -E 's/.*rmdir[[:space:]]+([^[:space:]]+).*/\1/')"
      ;;
    *'mv '*)
      # mv é suspeito apenas se move diretório — heurística simples: se
      # destino é diretório existente ou origem tem barra, é refactor de dir
      local src dst
      src="$(printf '%s' "$cmd" | awk '{for(i=1;i<=NF;i++) if($i=="mv") {print $(i+1); exit}}')"
      dst="$(printf '%s' "$cmd" | awk '{for(i=1;i<=NF;i++) if($i=="mv") {print $(i+2); exit}}')"
      # só emite target se src contém "/"
      case "$src" in
        */*) target="$src" ;;
      esac
      ;;
  esac

  if [ -z "$target" ]; then
    printf '{"blockers":[],"target":"","reason":"no-destructive-target"}'
    return 0
  fi

  # Resolve target relativo ao root se necessário
  case "$target" in
    /*) ;;
    *)  target="${root%/}/${target}" ;;
  esac

  # Verifica se target é diretório (se for arquivo único, não emite warn)
  if [ ! -d "$target" ]; then
    printf '{"blockers":[],"target":"%s","reason":"target-is-file"}' "$target"
    return 0
  fi

  # Roda repo-map --audit-removal (com timeout 5s para não travar hook).
  # Permite override via AGENT_SYNC_REPO_MAP (útil para testes).
  local repo_map_bin="${AGENT_SYNC_REPO_MAP:-repo-map}"
  local rel_target="${target#${root%/}/}"
  local ledger
  ledger="$(timeout 5 "$repo_map_bin" --root "$root" --audit-removal "$rel_target" 2>/dev/null || true)"
  if [ -z "$ledger" ]; then
    printf '{"blockers":[],"target":"%s","reason":"audit-failed-or-timeout"}' "$rel_target"
    return 0
  fi

  # Extrai blockers (linhas com "X active ... hit(s) remain")
  local blockers
  blockers="$(printf '%s' "$ledger" | grep -oE '[0-9]+ active [^"]+ hit\(s\) remain' || true)"
  if [ -z "$blockers" ]; then
    printf '{"blockers":[],"target":"%s","reason":"no-blockers"}' "$rel_target"
    return 0
  fi

  # Serializa blockers como array JSON
  local blockers_json="["
  local first=1
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    if [ "$first" -eq 1 ]; then first=0; else blockers_json="${blockers_json},"; fi
    blockers_json="${blockers_json}\"$(printf '%s' "$line" | sed 's/"/\\"/g')\""
  done <<< "$blockers"
  blockers_json="${blockers_json}]"

  printf '{"blockers":%s,"target":"%s","reason":"audit-found-refs"}' "$blockers_json" "$rel_target"
}

# Se invocado diretamente com argumento, roda analyze
if [ "${1:-}" = "analyze" ]; then
  shift
  analyze "${1:-}"
  exit 0
fi

# Modo padrão: stdin = payload do hook, parse command_line via detector CLI
# (este modo é raramente usado diretamente; cada wrapper fino deve passar
# command_line explicitamente via "analyze <cmd>")
input="$(cat)"
command_line="$(printf '%s' "$input" | grep -oE '"(command|CommandLine)"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed -E 's/.*:[[:space:]]*"(.*)"/\1/' || true)"

if [ -z "$command_line" ]; then
  printf '{}'
  exit 0
fi

# Desescapar minimo
command_line="$(printf '%s' "$command_line" | sed -e 's/\\"/"/g' -e 's/\\\\/\\/g')"

analyze "$command_line"
exit 0
