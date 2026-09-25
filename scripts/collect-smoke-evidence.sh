#!/usr/bin/env bash
# Coleta evidência do smoke ADR-003 em YAML pronto para colar em
# docs/smoke-evidence/adr-003-sess-N.md.
#
# Uso: collect-smoke-evidence.sh -session N [-root DIR] [-duration MIN] [-compact yes|no] [-out FILE]
#
# Saída: YAML com metadados da sessão, contagem de eventos por kind/actor,
# e placeholders para os 6 critérios (C1-C6) definidos em SMOKE-TEST-Q.
# Edite os placeholders manualmente após revisar o comportamento real
# (não há como o script saber se você atingiu ≥30min ou simulou compactação).
set -euo pipefail

session=1
root="."
duration_min=0
compact="no"
out=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    -session)   session="$2"; shift 2 ;;
    -root)      root="$2"; shift 2 ;;
    -duration)  duration_min="$2"; shift 2 ;;
    -compact)   compact="$2"; shift 2 ;;
    -out)       out="$2"; shift 2 ;;
    -h | --help)
      sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) echo "arg inválido: $1" >&2; exit 2 ;;
  esac
done

if [ "$session" != "1" ] && [ "$session" != "2" ]; then
  echo "session deve ser 1 ou 2" >&2; exit 2
fi

if ! command -v agent-sync >/dev/null 2>&1; then
  echo "agent-sync não está no PATH" >&2; exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq necessário para parsear stats" >&2; exit 1
fi

date_utc=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
branch=$(git -C "$root" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "?")
head=$(git -C "$root" rev-parse HEAD 2>/dev/null || echo "?")

# Tenta o binário instalado; se 'event' não existir (binário antigo sem
# ADR-003), cai para 'go run ./cmd/agent-sync event' direto do repo.
# Detecta via 'event -help' — wrapper antigo retorna uso genérico em stderr.
event_cmd=""
if agent-sync event -help 2>&1 | grep -q "read\|stats\|tail"; then
  event_cmd="agent-sync event"
else
  event_cmd="go run ./cmd/agent-sync event"
fi
stats_json=$($event_cmd stats -root "$root" 2>/dev/null \
  || echo '{"total":0,"by_kind":{},"by_actor":{}}')
events_total=$(printf '%s' "$stats_json" | jq -r '.total // 0')
by_kind=$(printf '%s' "$stats_json" \
  | jq -r '.by_kind // {} | to_entries | map("    \(.key): \(.value)") | .[]' 2>/dev/null \
  || true)
by_actor=$(printf '%s' "$stats_json" \
  | jq -r '.by_actor // {} | to_entries | map("    \(.key): \(.value)") | .[]' 2>/dev/null \
  || true)

yaml=$(cat <<EOF
---
session: $session
date_utc: $date_utc
duration_min: $duration_min
branch: $branch
head: $head
events_total: $events_total
by_kind:
$by_kind
by_actor:
$by_actor
criteria:
  C1_duracao: pass | fail
  C2_eventos: pass | fail
  C3_mix_kinds: pass | fail
  C4_sobrevive_compact: pass | fail
  C5_state_render: pass | fail
  C6_actor_correto: pass | fail
compact_test:
  triggered: $compact
  trigger_time_min: 0
  recovered_via: '<agent-sync event read --last N -root .>'
notes: |
  <observações livres>
EOF
)

if [ -n "$out" ]; then
  printf '%s\n' "$yaml" > "$out"
  echo "evidência gravada em $out" >&2
else
  printf '%s\n' "$yaml"
fi
