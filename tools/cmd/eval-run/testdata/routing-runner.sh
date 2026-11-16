#!/usr/bin/env bash
# Runner de roteamento: le {"case":{...}} de stdin e imprime em stdout so o id
# da skill escolhida. EVAL_CLI (ex.: 'opencode run') e obrigatorio; ausente = exit 2.
set -euo pipefail

if [ -z "${EVAL_CLI:-}" ]; then
  echo "routing-runner: defina EVAL_CLI (ex.: EVAL_CLI='opencode run')" >&2
  exit 2
fi

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
skills_dir=$here/../../../../skills
[ -d "$skills_dir" ] || { echo "routing-runner: sem $skills_dir" >&2; exit 2; }

# input.text: o caso chega em uma linha e o texto nao tem aspas nem escape.
payload=$(cat)
text=$(printf '%s' "$payload" | tr -d '\n' | sed -e 's/.*"text"[[:space:]]*:[[:space:]]*"//' -e 's/".*//')

# Catalogo: name + description do frontmatter de cada skills/*/SKILL.md.
catalog=$(awk '
  FNR == 1 { fm = 0; name = ""; desc = ""; block = 0 }
  /^---[[:space:]]*$/ {
    fm++
    if (fm == 2 && name != "") { gsub(/[[:space:]]+/, " ", desc); print "- " name ": " desc }
    next
  }
  fm == 1 {
    if ($0 ~ /^name:/) { sub(/^name:[[:space:]]*/, ""); name = $0; next }
    if ($0 ~ /^description:/) {
      sub(/^description:[[:space:]]*/, ""); gsub(/^["\047]|["\047]$/, ""); desc = $0
      block = (desc ~ /^[>|][-+]?$/); if (block) desc = ""
      next
    }
    if (block) {
      if ($0 ~ /^[[:space:]]+/) { sub(/^[[:space:]]+/, ""); desc = desc " " $0 }
      else block = 0
    }
  }
' "$skills_dir"/*/SKILL.md)

prompt=$(printf '%s\n' "Voce roteia o pedido abaixo para uma skill. Responda apenas o name dela:" \
  "$catalog" "" "Pedido:" "$text")

# EVAL_CLI sem aspas de proposito: divide "opencode run" em duas palavras.
out=$(printf '%s\n' "$prompt" | $EVAL_CLI)
last=$(printf '%s\n' "$out" | awk 'NF { line = $0 } END { print line }')
printf '%s\n' "$last" | sed -e 's/`//g' -e "s/^[[:space:]\"']*//" -e "s/[[:space:]\"']*\$//"
