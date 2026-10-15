# ADR — Aposentadoria do Cursor como target wirado (5 CLIs canônicos)

- **Status**: Aceito (decisão já executada em `3a973d5` + `9725590`; este ADR é o
  registro formal retroativo, escrito na sessão de documentação de 2026-10-15)
- **Data**: 2026-10-15
- **Decisor**: agente + usuário (sessão "menos linguagens, menos CLI")
- **Fonte**: `git show 3a973d5` (corte), `git show 9725590` (resíduos),
  `internal/target/target.go:38-100` (estado atual), `rules/global-rules.md §4`
- **Tags**: cli, escopo, cross-cli, remoção, policy

## Contexto (verificável)

1. **Gatilho — uso real, não preferência técnica.** O usuário declarou: *"O problema é ficar
   varias e varias linguagens entende? Por exemplo nem cursor eu to usando mais, talvez ainda
   codex, e os demais."* Manter hooks wirados numa CLI sem uso custa **um contrato de host a
   mais** por feature (matriz 5×N de `ADR-trilha-c-cobertura-cross-cli.md`), mais uma
   linguagem e mais um caminho de merge a manter.
2. **Inventário do que o Cursor tocava** (antes do corte): 3 wirers dedicados —
   `internal/hooks/hooks_cursor_apply.go` (106L), `hooks_cursor_merge.go` (140L),
   `hooks_cursor_command.go` (28L); branches `HooksFormat == "cursor"` em 13 arquivos
   `internal/hooks/hooks_*.go`; 11 scripts `hooks/*.cursor.*`; smoke dedicado
   `internal/hooks/smoke_real_a76_test.go` (233L); pacote de migração
   `internal/apply/cursor_migrate_test.go`.
3. **Cursor era o host com contrato mais divergente**: snake_case (`additional_context`,
   `followup_message`), merge próprio de `hooks.json` (`cursorManagedHooks`, com o campo
   `loop_limit` que nenhum outro host tem) e wire em `hooks.json` em vez de `settings.json`.
   Era o único host cuja remoção exigia um merge custom, não só apagar uma entrada de tabela.
4. **Custo do corte**: `3a973d5` — 47 arquivos, **+42 / −1418**; `9725590` — resíduos fora do
   wiramento (enum `clis`/`mcpNames` do `tools/actorvocab`, enum de `actor` em 4 schemas JSON,
   `parseCursorPayload`/`handoff cursor`/`knownCLIs` do `ctx-window`, `upsert_cursor` do
   `setup-mcp.sh`, `delegate-run`, `agents/*`).
5. **Estado atual (fonte da verdade)**: `internal/target/target.go:38-100` devolve 5 targets —
   `claude` (:47), `codex` (:56), `antigravity` (:65), `opencode` (:76), `cline` (:84).

## Decisão

### §1. Os 5 CLIs canônicos são `claude`, `codex`, `antigravity`, `opencode` (v2-only) e `cline`

Qualquer feature nova nasce com **matriz de 5 colunas** (regra 5×N da
`ADR-trilha-c-cobertura-cross-cli.md`), nunca 6. Cline entra como coluna de primeira classe
(era a 6ª CLI e hoje é uma das 5).

### §2. Não existe "modo legacy Cursor" nem flag de reativação

O corte é físico: bloco `cursor` removido de `GetTargetsForHome`, wirers e scripts deletados.
Reintroduzir o target exige **ADR nova** + reimplementação dos 3 wirers; o contrato snake_case
e o merge de `hooks.json` seguem recuperáveis do histórico (`git show 3a973d5^`).

### §3. Resíduos aceitos (não são suporte ao Cursor)

- `internal/pathutil/pathutil.go:74-77` — filtro do diretório built-in `~/.cursor/skills-cursor`
  (defesa para quem ainda tem `~/.cursor` no disco: nunca copiar skills built-in de terceiro).
- Comentários históricos que citam o comportamento antigo: `internal/state/model.go:396`,
  `internal/budget/status.go:110`, `internal/hooks/apply_table.go:33,46`,
  `tools/cmd/ctx-window/*`, `tools/cmd/false-success-guard/*`.
- ADRs e smokes antigos (`docs/SMOKE-TEST-*`, `docs/PLAYBOOK-K-reopen.md`) mantêm o texto
  original: eram verdadeiros na data e reescrevê-los apagaria trilha de auditoria.

### §4. Reconciliação documental (escopo desta entrega)

Corrigidos os documentos que são **instrução** (não histórico):

| Documento | Correção |
|---|---|
| `README.md` + `README.pt-BR.md` | lista de CLIs 6→5; remoção do bullet e da section "Cursor specifics"; remoção da menção a `readonly: true (Cursor)` |
| `docs/ADR-linguagem-hooks-por-host.md` | "6 hosts"→5 e remoção da linha Cursor da tabela |
| `docs/ADR-trilha-c-cobertura-cross-cli.md` | lista das 5 CLIs passa a citar `cline` no lugar do Cursor |
| `docs/PADRAO-HOOKS-CLIS.md`, `docs/guides/ideal-flow.md` | nota de aposentadoria no topo; matriz de colunas preservada como registro histórico |
| `internal/apply/command.go:29,132` | help de `-target` listava 4 CLIs (faltava `cline`); passa a listar as 5 |

## Consequências

### Positivas

- **−1418 LOC** e um contrato de host a menos: menos branches `HooksFormat`, menos merge custom,
  menos superfície de regressão por feature nova.
- A matriz cross-CLI fica verificável contra `GetTargetsForHome` (5 = 5), em vez de depender de
  memória sobre qual host cada hook nasceu.
- **Menos peso de linguagem por host**: 11 scripts shell e 3 wirers dedicados do Cursor deixam
  de existir — é a mesma direção da `rules/global-rules.md §4`.

### Negativas / limitações

- Quem voltar a usar Cursor fica sem wiramento até ADR nova (rollback é `git revert`, não flag).
- Documentos históricos (smokes, ADRs antigas, `STATE.md`, `session-state.json`) continuam
  citando Cursor — trilha de auditoria, não instrução (declarado no §3).

## Reversibilidade

- `git revert 9725590 3a973d5` (nesta ordem) restaura wirers, scripts, testes e o enum do
  `actorvocab`; `internal/target/target_test.go` volta a exigir 6 targets.
- Estado local do usuário (`~/.cursor/hooks.json`, `~/.cursor/rules/`) **não** é tocado pelo
  agent-sync: remoção é manual, deliberada e fora do escopo do binário.

## Não escopo

- Migrar/limpar `~/.cursor/*` na máquina do usuário.
- Reintroduzir Cursor como target (exigiria ADR nova + 3 wirers).
- Reescrever smokes e ADRs históricas para "apagar" o Cursor (violaria a trilha de auditoria).

## Refs

- `internal/target/target.go:38-100` — os 5 targets atuais (fonte da verdade)
- `git show 3a973d5` — corte (47 arquivos, +42/−1418); `git show 9725590` — resíduos
- `docs/ADR-trilha-c-cobertura-cross-cli.md` — regra 5×N (5 colunas)
- `docs/ADR-linguagem-hooks-por-host.md` — linguagem por host (revisada em 2026-10-15)
- `docs/PADRAO-HOOKS-CLIS.md:19-27` — matriz de equivalência de eventos (coluna Cursor = histórico)
- `rules/global-rules.md §4` — política de linguagens (menos superfície por host)
