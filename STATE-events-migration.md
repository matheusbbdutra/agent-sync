# STATE — events-migration + auto-summarize

## Status final

**MIGRAÇÃO COMPLETA + AUTOMAÇÃO A+C APLICADAS** ✅ (todos os testes passam com `-race`, smoke real validado).

## Mudanças aplicadas neste turno (continuação)

### Auto-summarize (Proposta A) — FEITA

- `hooks/ctx-window-summarize-at-stop.sh`: hook novo. No Stop/agent-stop, decide disparar `ctx-window summarize` em background se (a) tool calls >= `MIN_TOOL_CALLS` (default 100) OU (b) `.agent-sync/summary.md` velho (> `MAX_AGE_HOURS`, default 4h). Best-effort: no-op silencioso se `ctx-window` ausente. Mirror para memory-mcp após sucesso (kind=task_completed).
- `cmd/agent-sync/hooks.go`: `syncCtxWindowSummarizeAtStopHook` wirar nos 4 alvos padrão (Claude Code, Codex, Antigravity, Cursor). Antigravity usa `syncAntigravityFlatHook` (formato flat). Claude/Codex usam `syncHookCommandAtEvent(...,"Stop",...)`.
- `cmd/agent-sync/cursor.go`: novo entry em `cursorManagedHooks()` para o evento `stop` (`ctx-window-summarize-at-stop.sh`).
- `cmd/agent-sync/main.go`: novo hookSpec `ctx-window-summarize-at-stop`.
- `cmd/agent-sync/cursor_test.go`: atualizado para refletir 3 stops (era 2).

### Nudge combinado (Proposta C) — FEITA

- `hooks/ctx-window-nudge.sh`: hook novo. A cada tool call, dispara se (a) `count >= MIN_TOOL_CALLS` (default 80) OU (b) summary.md velho/ausente. Injeta `additionalContext` no PostToolUse pedindo `ctx-window summarize`. Mirror para memory-mcp (kind=guard_nudge).
- `cmd/agent-sync/hooks.go`: `syncCtxWindowNudgeHook` wirar nos 4 alvos padrão (Claude Code, Codex, Antigravity, Cursor).
- `cmd/agent-sync/cursor.go`: novo entry em `cursorManagedHooks()` para `postToolUse` (`ctx-window-nudge.sh`).
- `cmd/agent-sync/main.go`: novo hookSpec `ctx-window-nudge`.
- `cmd/agent-sync/cursor_test.go`: atualizado para 5 postToolUse gerenciados (era 4).

### B (PreCompact Claude Code) — PENDENTE (próximo turno com ADR)

Decisão consciente: parar aqui. Proposta B (hook em PreCompact do Claude Code) merece um turno dedicado por mudar comportamento nativo do harness.

## Validação

- `go test ./...` ambos módulos: **verde**.
- `go test -race ./cmd/agent-sync/`: **verde**.
- Smoke nudge: rodei `ctx-window-nudge.sh` com threshold=1, output JSON correto, reason combinado visível.
- Smoke auto-summarize: rodei com binário mockado, exit 0, summary rodou em background.
- Smoke disable: ambas as variáveis `*_DISABLE=1` corretamente no-op.

## Pendente pós-turno (documentado)

- **Proposta B** (PreCompact Claude Code): hook novo wirado no evento `PreCompact`. Requer ADR.
- OpenCode (5 plugins `.ts` paralelos): `ctx-window-nudge.opencode.ts` + `ctx-window-summarize-at-stop.opencode.ts` + variantes `.v2.ts`. Não-trivial porque exige TypeScript runtime + opencode SDK.
- `scripts/delegate-run.sh`: gravar `task_delegated`/`task_completed` em cmd_start/extract_result.
- README.pt-BR.md: documentar novos hooks no quadro de cobertura.
- ADR formal para a migração session_event → memory-mcp (recomendado).
- Limpeza do JSONL legado após release validar consistência.
