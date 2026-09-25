# SMOKE-TEST-O — ADR-002 Estágio B (cross-CLI handoff + migração)

> 2026-09-19 — passo #4 Estágio B. Fecha a ADR-002.
> Alcance: binário `agent-sync state {write,validate,next-action}` exercitado
> por **2 subprocessos reais** sobre o mesmo `.agent-sync/` root, e migração
> do STATE.md manual → JSON canônico → render.

## Setup

Cada teste em `state_cli_cross_process_test.go` constrói o binário uma vez
(sync.Once em `buildAgentSyncBin`), cria `t.TempDir()` com `.agent-sync/` e
dispara `exec.Command(agentSyncBin, "state", <sub>, "-root", <root>)`. Os
processos compartilham apenas o filesystem — sem IPC, sem sockets, sem
memória compartilhada. Esta é a portabilidade cross-CLI que a ADR-002 exige
(Estágio B critério 1).

## Resultados

### Cross-process write → read (handoff A → B)

`TestCrossProcessHandoffWriteReadNextAction` — 1/1 ✅.

| Etapa | Processo | Comando | Resultado |
|---|---|---|---|
| 1 | A | `agent-sync state write -root <root> -from <payload>` | `ok` em stdout, exit 0 |
| 2 | B | `agent-sync state next-action -root <root>` | JSON `{"id":"A-1","title":"ação distinta para handoff A→B","status":"pending"}` em stdout, exit 0 |

O JSON escrito por A é integralmente legível por B sem adaptador. Schema v1
+ lib `jsonschema` (ADR-001) é o contrato portável.

### Validate em subprocesso separado

`TestCrossProcessHandoffValidateEmOutroProcesso` — 1/1 ✅.

Write in-process (test helper) → validate em subprocesso (`exec.Command`):
valida `ok`. Confirma que `jsonschema.Validate` não depende de estado
in-memory — mesma lib carregada em outro processo, mesma validação.

### Writes concorrentes lockless

`TestCrossProcessWritesConcorrentesSemLock` — 1/1 ✅.

Dois subprocessos `state write` simultâneos no mesmo root, sem env
`AGENT_SYNC_SESSION_LOCK=1` (default lockless). Ambos retornam exit 0. JSON
final é integralmente um dos dois payloads (`sess-A` ou `sess-B`), sem
corrompimento. Atomic rename + tmpfile único por PID+nanoTimestamp
(commit `7632c07`) garantem serialização — "último vence" é o esperado.

### Root inválido

`TestCrossProcessHandoffReadEmRootInvalidoFalha` — 1/1 ✅.

`agent-sync state next-action -root /nonexistent-<timestamp>` retorna exit ≠
0 com mensagem estruturada. Sem fallback silencioso.

## Migração do STATE.md

Comando único: `agent-sync state migrate-from-md -root <repo>`.

| Input | Output | Resultado |
|---|---|---|
| `STATE.md` (177 linhas, manual) | `.agent-sync/session-state.json` | 15 decisions, 10 actions, 4 blockers, 0 open_questions, 1 warning (sem `## Perguntas em aberto`) |
| `.agent-sync/session-state.json` (gerado) | `STATE.md` (57 linhas, auto-render) | Header `# AUTO-GENERATED`, 5 seções estruturadas, lista numerada correta (1..10) |

Heurística de parsing (commit `770ee6d`):

- `## <seção>` (h2) → chave do map, sufixo `(...)` removido para tolerância
- `## Decisões` (e variantes `## Decisões ativas (resumo)`) → match por prefixo
- `## Próximas ações` (e `(em ordem de prioridade)`) → idem
- `## Bloqueios` (e `/Bloqueios / perguntas abertas`) → idem
- Bullets `- **<bold>** ...` → `Title` do bold, resto como `Rationale`
- Numerados `1. ✅ <title>` → status `done`, `❌` → `blocked`, sem marca → `pending`
- `git -C <root> rev-parse --abbrev-ref HEAD` + `rev-parse HEAD` + `status --porcelain`

### O que foi perdido (irreversível nesta sessão)

Por design da ADR (MD é view pura, JSON é fonte):

- **Histórico (resumo de sessões anteriores)** — 30 linhas, só em git log
- **Regras ativas (valem para todas as sessões)** — 7 linhas
- **Gaps conhecidos (Fechados/Pendências/Revisita)** — 25 linhas
- **Smoke tests realizados** — 7 linhas

Tudo permanece em `git log` (commit `1433b96 docs(state): sincroniza
2026-09-19`). A view regenerada contém apenas o que cabe no schema v1.

### Limitações conhecidas da heurística

- **`blocking_action_ids` sempre vazio** — regex não infere referência
  cruzada de texto como "bloqueia: A-1, A-2". Mitigação: humano revisa o
  JSON e preenche manualmente quando relevante.
- **Rationale começa com ":"** quando a fonte é `- **<title>**: <rest>` —
  regex captura tudo após o bold. Cosmético, não bloqueia.
- **`session.started_at` = now** — não inferido da data do commit. Schema
  exige o campo; melhor now do que ausente.
- **`working_tree_summary` cru** (`?? tools/ctx-window`) — git porcelain
  sem tradução. Suficiente para o objetivo do campo.

## Como reproduzir

```bash
# Cross-process tests
go test ./cmd/agent-sync/... -count=1 -run CrossProcess -v

# Migração do STATE.md (manual, no repo atual)
go run ./cmd/agent-sync state migrate-from-md -root .
go run ./cmd/agent-sync state render -root . > STATE.md

# Validação do JSON
go run ./cmd/agent-sync state validate -root .
```

## Pendências remanescentes

Pendências que **não** estão no escopo do Estágio B:

- **#5** ADR-003 PoC (event log append-only) — pendente
- **#6** MR-reviewer GitLab v12 — pendente
- **#7** Trilha A (Cursor `stop` hook smoke real) — pendente
- **#9** Gap 4 — budget tracking — rebaixado (30 dias de dados)
- Pre-commit hook comparando hash MD-vs-render JSON (citado na ADR
  Decisão 1, não é critério do Estágio B)

## Conclusão

5/5 testes verdes, migração executada, header `# AUTO-GENERATED` aplicado.
**ADR-002 Estágio B fechado** (passo #4 do STATE). Pendente: mover ADR para
"finalizada" no histórico de decisões (próximo turno ou task dedicada).