#!/usr/bin/env bash
# Adapter fino do Impeccable design hook no Antigravity (regra §4: sem logica
# de negocio aqui). O contrato dos eventos (PostToolUse -> pending,
# PreInvocation -> injectSteps) e a chamada ao detector vivem no binario Go
# `agent-sync hook impeccable`. Fail-open: qualquer falha vira `{}`.
set -uo pipefail

BIN="${AGENT_SYNC_BIN:-}"
# Prefere o binario instalado por `make install`; em algumas maquinas o PATH
# aponta para um agent-sync antigo (ex.: go install) sem o subcomando novo.
if [ -z "$BIN" ] && [ -x "$HOME/.local/bin/agent-sync" ]; then
  BIN="$HOME/.local/bin/agent-sync"
fi
if [ -z "$BIN" ]; then
  BIN="$(command -v agent-sync 2>/dev/null || true)"
fi
if [ -z "$BIN" ]; then
  printf '{}'
  exit 0
fi

out="$("$BIN" hook impeccable --host=antigravity)" || {
  printf '{}'
  exit 0
}
if [ -z "$out" ]; then
  out='{}'
fi
printf '%s' "$out"
