# ADR: Reformulação da taxonomia de estado (D/A/C/B → 3 tipos)

**Status**: Aceito (promovido por D-62 em 2026-09-23)
**Data**: 2026-09-21
**Decisor**: Matheus Dutra
**Tags**: meta, state, taxonomy, governance

## Contexto

O `session-state.json` cresceu organicamente com 4 prefixos (D-N, A-N, C-N, B-N)
que viraram ambíguos com o tempo. Mapeamento retroativo (2026-09-21):

| Tipo | Count | Conteúdo real |
|---|---|---|
| `decisions` (D) | 46 | ~30% decisão arquitetural, ~50% observação factual / relatório de coisa feita, ~20% decisão técnica pontual |
| `next_actions` (A) | 32 | 23 done (72%) virou log retroativo, 8 pending, 1 cancelled |
| `blockers` (B) | 2 | ambos cancelados |
| `trilhas` (C, dentro de D/A) | 5 | categoria organizacional misturada com backlog |

**Sintomas concretos que a ambiguidade causou:**
- D-37 (2026-09-21): "gaps 🟡/⛔ não viram A-N estruturadas" — workflow falha
  porque ninguém sabe se gap vira D (decisão), A (ação), ou mudança na trilha.
- A-33 virou log retroativo de 4 commits — não é mais "ação".
- C-1..C-5 são categorias cross-CLI; A-14..A-18 são entregas que **referenciam**
  a trilha — mas a relação não é estrutural no JSON, é só texto.

## Decisão

Reduzir para **3 tipos claros**, cada um com critério de entrada e saída explícito:

### 1. `decisions` (era D-N)

**O que entra:** uma escolha arquitetural que **limita opções futuras**. Se outra pessoa (ou eu daqui 6 meses) vai perguntar "por que foi feito assim?", a resposta está aqui.

**Quando escrever:**
- ADR novo (referência cruzada `adr: ADR-NNN`)
- Decisão técnica que **bloqueia** alternativas (ex.: "lib = v6", "lockless")
- Mudança de regra que invalida decisão anterior

**Quando NÃO escrever:**
- Observação factual sem implicação de design (mover para descrição do ADR-mae)
- Relatório de implementação (mover para `commit_log` ou `deliveries`)
- Estado atual do repo (mover para `state` ou inferir do git)

### 2. `tasks` (era A-N + C-N de backlog)

**O que entra:** trabalho pendente **com critério de done verificável**. Status: `pending` | `in_progress` | `done` | `cancelled`.

**Quando escrever:**
- TODO que vai virar commit
- Bug que precisa ser corrigido
- Feature com escopo definido (mesmo que escopo = 1 linha)

**Quando NÃO escrever:**
- Relatório de coisa feita (mover para `deliveries`)
- Categoria organizacional sem trabalho concreto (vira `tag` ou `group` em outro lugar)

### 3. `issues` (era B-N + gaps 🟡/⛔)

**O que entra:** coisa **quebrada ou bloqueante** que **impede trabalho** ou **representa risco conhecido**.

**Quando escrever:**
- Bug conhecido sem fix imediato
- Gap de cobertura cross-CLI (🟡 ou ⛔ da matriz 5xN)
- Blocker externo (depende de upstream, decisão de produto, etc.)

**Quando fechar:** virou `task` (vai ser atacado) ou foi resolvido por mudança externa.

### O que morre

| Antigo | Novo | Razão |
|---|---|---|
| C-N (Trilha) | vira `tag` em `tasks` ou `issues` | categoria organizacional, não dado |
| A-N done com relatório | vira `deliveries[]` (novo) com referência ao commit | log é commit_log do git, não estado |
| B-N cancelado | some (não precisa persistir) | cancelado = não aconteceu |
| D-N que é observação | vira descrição do ADR-mae | sem implicação de design |

### Migração

**Não destruir dados.** Estratégia:

1. Adicionar campo `legacy_kind` em cada item do JSON:
   - `decision.kind: "architectural" | "factual" | "retrospective"` (default: "architectural")
   - `task.kind: "todo" | "delivery"` (default: "todo")
   - `issue.kind: "bug" | "gap" | "blocker"` (default: "bug")
2. Itens `factual` e `retrospective` em decisions: marcar mas **não exigir migração** — eles continuam válidos, só com tag que diz "isto é relatório, não decisão ativa".
3. Trilhas C-N: virar `tags` aplicadas via campo `task.tags: ["C-N"]`.
4. Script de migração único (`scripts/migrate-taxonomy-state.py`) que adiciona os campos em uma passada, commit único, auditável.

### Critério de promoção Proposto → Aceito

- [x] ADR aceito (review do usuário — D-63, 2026-09-23)
- [x] Migration script commitado e executado em session-state.json (legacy_kind 100% aplicado em D-48.x.x; próximo passo é remoção física dos 42 next_actions / 2 blockers, gated por decisão do usuário)
- [x] `agent-sync state validate` passa (verificado em 2026-09-23 na sessão ses_f2f469b62ffeg0dnKYpMl5ey5l)
- [x] Render do STATE.md mantém informação (D-61 em 2026-09-23 adicionou aviso "legacy — prefira Tarefas/Issues" preservando substrings exigidas pelos testes cross-CLI)
- [ ] Próxima ação criada segue a taxonomia nova (smoke: `agent-sync task add "..."` ou subcommand equivalente — não implementado ainda)
- [x] Smoke real: adicionar 1 task, 1 issue, 1 decision novos; verificar render (TestRunStateWriteSmokeCriterio6Taxonomia em internal/state/render_test.go, PASS em 2026-09-23, D-62)

**Status atual (2026-09-23)**: 4/6 critérios satisfeitos. Pendente apenas (1) review do usuário para promoção formal e (5) subcommand `task add`/`issue add`.

### Reversão

`git revert <commit-da-migration>` reverte os campos adicionados. JSON volta ao estado anterior sem perda (só campos novos viraram opcionais). Decisão pode ser desfeita sem impacto em outras ADRs.

### Anti-over-engineering (AGENTS.md §3)

- **Não criar `deliveries[]` novo agora** — o git log já tem SHA + mensagem. Se quiser ver entregas, `git log --oneline | grep -E '^[a-f0-9]+ feat|fix'` resolve.
- **Não criar subcommand novo** (`task add`, `issue add`) na mesma entrega. Smoke pode ser via `state write` + edit manual.
- **Não mudar schema_version** — campos novos são opcionais.

## Pendências separadas

- Decidir se `deliveries[]` vira schema formal ou fica só no git log (proposta: **fica no git log**, anti-over-engineering).
- Decidir se a numeração A-N preserva histórico (proposta: **sim**, não renumerar; tasks ganham `legacy_kind` mas mantêm ID).
