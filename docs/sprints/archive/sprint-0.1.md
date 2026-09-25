# Sprint S-0.1 — Limpeza de duplicação + fechamento do refator

**Status**: done (2026-09-23 → 2026-09-24)
**Branch**: `feat/d48-schema-bump`
**Mapa**: `docs/sprints/archive/sprint-0.1.json`

## Contexto

Esta Sprint nasce do **D-48** (reformulação da taxonomia D/A/C/B → `decisions/tasks/issues`,
Aceito em 2026-09-23 via D-63) e do **D-37** (workflow falha: gaps 🟡/⛔ não viravam itens
estruturados).

Em 2026-09-23, **D-61** adicionou avisos "legacy — prefira Tarefas/Issues" aos headers do
STATE.md para limpar a confusão visual sem quebrar o contrato de testes cross-CLI.
Mas a duplicação física persiste: 42 `next_actions` espelham 43 `tasks`, e 2 `blockers`
espelham 2 `issues` (B-1/B-2 já cancelados e migrados para C-3).

A Trilha C (cobertura cross-CLI) está **Aceita** via D-30..D-36 e D-54..D-55 (5xN validado
em smoke runtime). O que falta é:

1. Fechar **A-36** (refator `cmd/agent-sync/` em `internal/*`, ~95% feito)
2. Resolver **A-37** (migração física `next_actions`→git log)
3. Reabrir **A-47** (revisão dos 2 ADRs Re-Propostos em 2026-10-23)
4. Não perder de vista **A-20** (smoke T5 comparativo pre/post b7bc037, janela 30 dias)

## Items em ordem cronológica

### A-36 — Refator cmd/agent-sync/ em internal/* (Final: Aceita)

**Status**: fechado em 2026-09-24 (ses_f2cd32a0fffeMHoXEfOrNzw4t6). 5 commits granulares + este commit de fechamento.

| Commit | Descrição |
|---|---|
| `d780a07` | extrair `internal/pathutil` e `internal/skills` (Passo 1) |
| `33ab770` | extrair `internal/event`, `internal/state`, `internal/budget` (Passo 2) |
| `de95635` | extrair `internal/target`, `internal/doctor`, `internal/opencode` (Passo 3) |
| `d2216ee` | extrair `internal/agents`, `internal/hooks` (Passo 4) |
| `31a835c` | extrair `internal/apply` + enxugar `cmd/agent-sync/main.go` para 42 linhas (Passo 5) |
| `af959e3+` | smoke real verde (D-68): 12 subcomandos testados contra `.agent-sync/session-state.json` real (schema 1.2), `main.go` permanece 42 linhas |

**Critério de done** ✅:
- [x] `go test ./...` verde em 12/12 pacotes
- [x] `main.go` < 100 linhas (atingido: 42)
- [x] Smoke real do binário reinstalado (todos subcommands: state/event/budget/skills/doctor, incluindo write com falha graciosa + unknown subcommand)
- [x] Nenhum `.go` de produção fora de `main.go` + `internal/` (cmd/agent-sync/ tem apenas main.go, main_test.go, session_test.go)

### A-37 — Migração física next_actions→git log (Refinamento)

**Status**: Etapa 1 entregue em 2026-09-24 (ses_f2cd32a0fffeMHoXEfOrNzw4t6, caminho C').
Pendentes: Etapas 2-5.

**Plano aprovado** (checkpoint `checkpoint-d48xx-cleanup-handoff`, 2026-09-23):

1. **[OK] Commit 1**: parar de renderizar `next_actions`/`blockers` em `RenderStateMD` (esconde)
   — entregue em 2026-09-24 via caminho C' (-12 linhas em model.go; ajusta assert
   `A-1` em TestSessionStateRenderStable com comentario referenciando A-37).
   `go test ./...` verde 12/12. Diff final: 2 arquivos, +6/-14.
2. Commit 2: remover campos do struct + normalize + validate cruzado
3. Commit 3: remover arrays do JSON + ajusta schema
4. Commit 4: ajustar tests
5. Commit 5: atualizar `ADR-taxonomia-estado.md` Proposto→Aceito (já feito em D-63)

**Anti-over-engineering**: não criar `deliveries[]` schema (git log tem SHA + mensagem);
não criar subcommand `task add`/`issue add` (escopo separado); não renumerar IDs.

**Checkpoint de Etapa 1** (`checkpoint-a37-caminho-c-falha-em-renderstable`, 2026-09-24):
caminho C puro falhou em TestSessionStateRenderStable (exigia literal `A-1` no render).
Lição: validar caminho contra TODOS os testes que tocam o trecho, não só os que têm
substring de seção. Caminho C' ajusta o assert removendo `A-1` (mantém `D-1`) e
documenta o porquê no próprio teste.

### A-47 — Revisão 2026-10-23 dos 2 ADRs Re-Propostos (Proposta)

**Status**: marco temporal definido em 2026-09-23 (D-64).

**2 ADRs a revisar**:

1. `ADR-agents-md-session-guard.md` — hook não implementado na prática
2. `ADR-fim-de-turno-hooks.md` — Trilha A wirada+testada (✅), Trilha B hipótese não validada

**Ação esperada na data**: reavaliar viabilidade ou descartar cada um. O ID da `D-N` que vai registrar o resultado **ainda não existe** — vai ser definido quando a revisão acontecer (provavelmente próximo a D-48 sequencial, mas não garantido).

### A-20 — Smoke T5 pre/post b7bc037 (Proposta)

**Status**: janela 30 dias a partir de 2026-09-21 (D-36).

**Hipótese a validar**: revert do fix primitivo `b7bc037` reintroduz bug de persistência
de objeto no `ctx.storage` do OpenCode v2, **ou** a premissa original (objetos não
persistem) era racionalização incorreta — e nesse caso o fix é desnecessário.

**Quando abrir**: 2026-10-21 (~30 dias após D-36). Implementação: smoke controlado
comparativo pre/post com chave de cleanup explícita.

### A-23, A-24, A-25 — Gaps 🟡/⛔ da Trilha C (Proposta)

Catalogados para visibilidade, **não para implementação nesta Sprint** (vão pra S-0.2).

**Encerramento S-0.1**: itens migrados para o catálogo de S-0.2 (sem decisão nesta Sprint; sem mudança de verdict; sem re-execução). Ver `docs/sprints/sprint-0.2.json`.

### A-28 — Briefing de nova sessão (Proposta)

Depende de items anteriores (especialmente A-36/A-37).

**Encerramento S-0.1**: item migrado para S-0.2 como briefing vivo (4 sub-itens: SMOKE-TEST-U, A-27 sync README.pt-BR.md, frontend-developer, livre escolha do usuário). Pendências com prazo/escopo externo permanecem como `pending` em `state.tasks[]` (sem sprint dona).

### B-1, B-2 — Issues legacy canceladas (Final: Descartada)

Ambas canceladas e migradas para C-3 conforme D-32 + D-36. Mantidas como referência
histórica até A-37 decidir sobre remoção física.

## Decisões adiadas

| Item | Motivo | Vai pra |
|---|---|---|
| Schema `deliveries[]` formal | git log já entrega SHA + mensagem; criar schema novo = over-engineering (D-48) | — (rejeitado) |
| Subcommand `task add`/`issue add` | Smoke pode ser via `state write` + edit manual (ADR-taxonomia-estado §Anti-over-engineering) | S-0.2 (escopo separado) |
| Hook `agent-sync sprint update` | Preferência por movimentação manual de fase | — (rejeitado por ora) |
| Hook automático trava ">5 Propostas" | Preferência por convenção revisada no fim de sprint | — (rejeitado por ora) |
| Implementação gaps C-1/C-3 (A-23/A-24/A-25) | Cada gap exige decisão individual sobre block nativo vs heurística | S-0.2 |

## Riscos / gaps conhecidos

- **A-37 depende de decisão do usuário** sobre preservação de histórico (não posso
  automatizar; rito > tool).
- **A-36 pode ter regressão não detectada** — `main.go` foi enxugado de 301 para 42 linhas
  mas smoke real do binário reinstalado ainda não foi feito nesta sessão.
- **D-64 deixou 2 ADRs Re-Propostos com prazo curto** (30 dias) — se A-47 não for atacado
  em 2026-10-23, viram órfãos (D-37 reincidência).

## Métricas de saída

Status final em 2026-09-24 (momento do arquivamento):

- [x] `go test ./...` verde em 12/12 pacotes
- [x] `main.go` < 100 linhas (já em 42, manter)
- [x] `session-state.json` **sem** arrays `next_actions` e `blockers` (A-37 entregue em D-70, 5 Etapas)
- [ ] STATE.md renderiza sem warning "legacy — prefira Tarefas/Issues" (D-61 invertido — fora do escopo desta Sprint; D-61 é workaround temporário, remoção exige decisão separada)
- [ ] Decisão sobre revisão dos 2 ADRs Re-Propostos registrada (D-N a definir quando acontecer em 2026-10-23 — adiada com prazo)
- [x] Catálogo desta Sprint movido pra `docs/sprints/archive/` com status `done`

**Itens não fechados nesta Sprint** (com justificativa):
- Remoção do aviso D-61 → tarefa aberta separada, fora do escopo de S-0.1
- A-47 revisão dos 2 ADRs → adiada com prazo 2026-10-23 (~30 dias após D-65), sem sprint dona
- A-20 smoke T5 → adiada com prazo 2026-10-21 (~30 dias após D-36), sem sprint dona
- A-23/A-24/A-25/A-28 → migrados para S-0.2
