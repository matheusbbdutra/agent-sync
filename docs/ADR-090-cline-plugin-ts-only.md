# ADR-090 — Plugin Cline 100% TS autocontido (revoga ADR-80 + ADR-87)

- **Status**: Aceito
- **Data**: 2026-09-27
- **Decisor**: usuário (ses_atual, decisão verbal "Revisão completa: Go sai, tudo vira TS", refinada após reconhecimento factual)
- **Revoga**: `docs/ADR-cline-hooks-mcp-wiramento.md` (Aceito rev. 3) e `docs/ADR-cline-plugin-ts-first.md` (Aceito 2026-09-27)
- **Substitui por**: este documento
- **Fonte**: `/home/matheus_dutra/.claude/plans/delegated-honking-pebble.md` (plano de execução, 12 passos)

## Contexto

O usuário identificou que o adapter Cline (`cline-plugin/index.ts:80-95`) é proxy fino via `execFileSync("agent-sync", ["hook","cline",...])`, e que o padrão oficial do Cline é `AgentPlugin` TS nativo (`docs.cline.bot/sdk/plugins`). Decisão inicial: "Go sai, tudo vira TS". Reconhecimento factual (`Explore` agent, ses_atual) revelou que 7 subsistemas core (libsql, audit, ctx-window, budget, memory-mcp, docs-cache-write, repo-map) **não são específicos do Cline** e sustentam Claude Code / Cursor / Antigravity / Codex também. Reescrevê-los em TS seria reescrever ~70% do codebase. Usuário confirmou (segunda `AskUserQuestion`) que só os arquivos **específicos do Cline** são eliminados; core Go é preservado.

## Decisão

### Eliminado (específico do Cline)

- `internal/hooks/cline_bridge.go` (725 LOC) — bridge proxy que normalizava payload Cline→Claude e executava scripts shell
- `internal/hooks/cline_bridge_test.go` (595 LOC)
- `internal/hooks/apply_cline.go` (258 LOC) — wirer que instalava o plugin em `~/.cline/plugins/_installed/local/`
- `internal/hooks/apply_cline_test.go` (189 LOC)
- `internal/hooks/apply_cline_prune_test.go` (139 LOC)
- 13× `hooks/*.sh` wirados via Cline (ver lista em `cline_bridge.go:98-123`)
- `hooks/cline-wiramento-smoke.sh`

### Criado (TS autocontido em `cline-plugin/`)

- `cline-plugin/src/plugin.ts` — `AgentPlugin` nativo (callbacks fazem lógica in-process)
- `cline-plugin/src/payload.ts` — normalização Cline→Claude (~80 LOC TS)
- `cline-plugin/src/bridge.ts` — wrapper `execFile` para chamar binários core Go quando preciso (~30 LOC)
- `cline-plugin/src/hooks/<name>.ts` (15 arquivos) — pattern `*.v2.ts` (`id + setup assíncrono`)
- `cline-plugin/scripts/install.ts` — wirer TS (~120 LOC)
- `cline-plugin/tests/*.test.ts` — `node --test` (zero deps novas)

### Preservado (core cross-CLI)

- `internal/agentmemory/*` (libsql)
- `internal/audit/*` (classifier 9 classes)
- `tools/cmd/ctx-window/*` (summarize/handoff)
- `tools/cmd/budget/*` (ledger agent_tasks)
- `mcp.go` (memory-mcp RPC server)
- `tools/cmd/docs-cache-write/*`
- `tools/cmd/repo-map/*` (com `--audit-removal`)

Esses são consumidos via `execFile` dos hooks TS (10 dos 15 hooks). Os outros 5 (`context-guard-nudge`, `agent-react-nudge`, `principles-inject`, `secret-guard`, `wiramento-smoke`) são puro TS.

### Matriz hooks TS × binário core

| Hook TS | Binário Go chamado | Motivo |
|---|---|---|
| `bash-rm-guardian.ts` | `repo-map --audit-removal` | walk+regex 9 classes |
| `memory-nudge.ts` | `memory-mcp record_event` | RPC + FTS5 libsql |
| `memory-observe.ts` | `memory-mcp record_event` | idem |
| `memory-prune-session-start.ts` | `memory-mcp prune` | idem |
| `memory-consolidate-stop.ts` | `memory-mcp consolidate` | idem |
| `ctx-window-nudge.ts` | `ctx-window status` | heurística Go |
| `ctx-window-summarize-at-stop.ts` | `ctx-window summarize` | CLI summarizer |
| `token-nudge.ts` | `agent-sync budget nudge` | heurística Go |
| `agent-task-record-stop.ts` | `agent-sync budget write` | ledger Go |
| `docs-cache.ts` | `docs-cache-write` | single-purpose Go |
| `context-guard-nudge.ts` | — | puro TS |
| `agent-react-nudge.ts` | — | puro TS |
| `principles-inject.ts` | — | puro TS |
| `secret-guard.ts` | — | puro TS |
| `wiramento-smoke.ts` | — | puro TS |

## Consequências

### Positivas

- **Cline nativo TS**: plugin segue o padrão `@cline/sdk`, sem proxy
- **Zero shell no runtime Cline**: `hooks/*.sh` deletados
- **Zero Go específico de Cline**: `cline_bridge.go` + `apply_cline.go` deletados
- **Manutenibilidade**: cada hook tem 1 fonte de verdade (TS), tipos locais em `types.ts`
- **Saldo**: −1700 LOC, ganho de coesão

### Negativas

- **`execFile` overhead**: 10 dos 15 hooks ainda subprocessam binários Go (latência ~5-15ms por chamada)
- **`AGENT_SYNC_BIN` env var obrigatória**: hooks TS precisam resolver path do binário
- **Build step extra**: `npx tsc -p cline-plugin/` antes de `make cline-install`
- **Wirer TS separado do `agent-sync apply`**: usuário roda `make cline-install` (não mais `agent-sync -target cline`)

## Critérios de aceite (verificação empírica)

| # | Critério |
|---|---|
| 1 | `go build ./...` continua verde |
| 2 | `npx tsc -p cline-plugin/` exit 0 |
| 3 | Plugin wirado em `~/.cline/plugins/_installed/local/agent-sync-hooks-<hash12>/package/` |
| 4 | Smoke `cline -t "ping"` gera log em `/tmp/agent-sync-cline-wiramento/` |
| 5 | `beforeTool` injeta `appendContext` (principles-inject) |
| 6 | `afterTool` chama `memory-mcp record_event` (memory-observe) |
| 7 | Wiramento idempotente (2× `make cline-install` não duplica) |
| 8 | Sem regressão nos 4 CLIs shell-only |

## Plano de execução (12 passos)

Ver `/home/matheus_dutra/.claude/plans/delegated-honking-pebble.md` §"Ordem de execução":

1. ✅ ADR-90 (este doc)
2. ⏳ `cline-plugin/src/payload.ts` + tests
3. ⏳ `cline-plugin/src/bridge.ts`
4. ⏳ 3 hooks PoC (principles-inject puro, memory-nudge bridge, ctx-window-summarize bridge)
5. ⏳ `cline-plugin/src/plugin.ts` carregando PoC
6. ⏳ Validar `tsc` + wirar manual + smoke `cline -t`
7. ⏳ Migrar 12 hooks restantes
8. ⏳ `cline-plugin/scripts/install.ts` (wirer TS)
9. ⏳ Deletar Go engine (bridge + wirer + tests)
10. ⏳ Deletar `hooks/*.sh` (15 arquivos)
11. ⏳ Makefile targets (`make cline-plugin-build`, `make cline-install`)
12. ⏳ STATE.md + memory-mcp `task_completed`

## Reversibilidade

- **Reverter este ADR**: `git revert <commit>` → bridge/wirer Go voltam; shell scripts voltam; `cline-plugin/src/*` adicionado mas inerte (não wirado).
- **Build quebrado por upgrade de Cline SDK**: ajustar `cline-plugin/src/types.ts` (shape `AgentPlugin`) + lógica no mesmo PR.

## Não escopo

- Reescrever libsql/audit/ctx-window/budget/memory-mcp/docs-cache-write/repo-map em TS (rejeitado por anti-overengineering)
- Migrar shell adapters de Claude Code/Cursor/Antigravity/Codex (esses CLIs não têm SDK TS no escopo)
- Adicionar `tsc` ao `make setup` (tarefa filha A-91)

## Refs

- `/home/matheus_dutra/.claude/plans/delegated-honking-pebble.md` (plano completo)
- `internal/hooks/cline_bridge.go` (725 LOC, deletar)
- `internal/hooks/apply_cline.go` (258 LOC, deletar)
- `cline-plugin/index.ts` (207 LOC, refatorar)
- `hooks/*.v2.ts` (12 arquivos, pattern de referência)
- `docs.cline.bot/sdk/plugins` (shape do `AgentPlugin`)