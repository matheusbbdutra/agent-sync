# ADR — Matriz de escopos de memória (4 eixos)

- **Status**: Proposto
- **Data**: 2026-09-25
- **Decisor**: agente + usuário (sessão de investigação empírica de overhead de hooks A-71)
- **Fonte**: investigação empírica de overhead de hooks (A-71, 2026-09-25, bench N=200), D-86 (fontes de lixo memory-mcp), D-87 (A-66 fix high-signal), D-88 (A-67 prune + stats), D-89 (A-68 auto-prune)
- **Tags**: memory-mcp, observation-pipeline, scope-matrix, scratch-retention, kind-taxonomy

## Contexto

`memory-mcp` recebe gravações de 3 fontes distintas sem diferenciação semântica:

1. **Auto-hooks wirados** (`memory-observe.posttooluse.sh`, `ctx-window-nudge.sh` mirror, `memory-nudge.sh`, `context-guard-nudge.sh`) — escrevem em background a cada tool call
2. **Comandos manuais** — não existe subcommand `agent-sync memory add` hoje
3. **Tool calls do modelo** — raro, sem interface explícita

### Estado empírico verificado (D-88 + bench N=200 A-71)

- **95% das memórias são `scratch=true`** sem critério semântico (136/143 em D-88)
- Todas com `kind=note` genérico
- Em sessão longa (1000 tool calls), `memory-observe` sozinho grava ~300 buffer-records (~300KB SQLite writes)
- Bench N=200 (2026-09-25): `memory-observe` custa 9ms/fire + 1 fork `memory-mcp` por mutação

### Problema concreto

- Retrieval de memórias úteis mistura com operacional descartável
- Prune 7d (D-88) e auto-prune (D-89) limpam o que é útil junto com o lixo
- Não há como distinguir "decisão arquitetural verificável" de "reflexo de tool call"
- `OBSERVE_SCOPE` atual tem 1 eixo (`off|high-signal|all`) e filtra por `tool_name` (proxy errado — `Bash` nem sempre é mutação, `Edit` nem sempre é decisão)

### Causas-raiz

1. **Filtro por tool_name é semântica fraca**: `Bash` que executa `git status` ≠ `Bash` que executa `rm -rf /`. Hoje os 2 são tratados igual.
2. **Auto-hook e manual compartilham mesmo canal**: não há como garantir que auto-hook não polui permanent.
3. **Sem critério de "vale lembrar"**: tudo vira `kind=note` porque é o único kind default.

## Decisão

Adotar matriz de escopos com **4 eixos ortogonais** que se combinam para classificar cada memória.

### §1. Eixos da matriz

**Eixo 1 — `source`** (quem gravou):
- `auto-hook` (sistema automático via bash/TS wirado) — default
- `manual` (humano via CLI command)
- `agent` (modelo decidiu salvar via tool call explícita)

**Eixo 2 — `kind`** (o quê semanticamente):
- `decision` (escolha arquitetural/técnica com critério)
- `hypothesis_validated` (causa raiz comprovada)
- `task_completed` (trabalho verificável entregue)
- `guard_nudge` (alerta operacional, efêmero)
- `action` (reflexo de tool call, descartável)
- `state_render` (snapshot do STATE.md)
- `note` (observação solta, kind default para retro-compat)
- `blocker` (impedimento)
- `open_question` (dúvida pendente)

**Eixo 3 — `scope`** (filtro operacional — substitui `high-signal`):
- `off` — nada grava
- `operational` — `source=auto-hook` + `kind ∈ {action, guard_nudge, state_render}` (default NOVO)
- `meaningful` — `source ∈ {manual, agent}` + `kind ∈ {decision, hypothesis_validated, task_completed}` (sem auto-hook)
- `all` — tudo grava (legado, equivalente a `high-signal`)

**Eixo 4 — `retention`** (ciclo de vida):
- `scratch` (7d, prune por D-88/D-89) — default para `source=auto-hook`
- `permanent` (sem prune) — exclusivo para `source ∈ {manual, agent}`

### §2. Regra de ouro

**Auto-hook grava APENAS `kind ∈ {action, guard_nudge, state_render}` com `retention=scratch`**.

Decisões, hipóteses validadas e trabalhos verificáveis só viram `retention=permanent` via comando manual (`agent-sync memory add --kind=...`) ou decisão explícita do agente via tool call dedicada.

**Garantia de integridade**: se um auto-hook tentar gravar `kind ∈ {decision, hypothesis_validated, task_completed}` em `scope=operational`, é silenciosamente descartado. Permanent não pode ser poluído por sistema automático.

### §3. Default muda

`OBSERVE_SCOPE=operational` substitui `high-signal` como default.

`high-signal` e `all` continuam disponíveis (backward compat — ver §6).

### §4. Comando manual novo

```bash
agent-sync memory add --kind=<kind> --note="..." [--source=manual|agent] [--session-id=<>]
```

- Defaults: `source=manual`, `retention=permanent`
- Kinds válidos para manual: `decision`, `hypothesis_validated`, `task_completed`, `note`, `blocker`, `open_question`
- Validação: `--note` não vazio (mín 5 chars), `--kind` na enum
- Saída: ID da memória criada + confirmação textual

### §5. Hooks wirados declaram source/kind

Wiramento central em `internal/hooks/apply_table.go` (já existe infraestrutura) para que cada hook wirado passe os eixos corretos ao invocar `memory-mcp`:

| Hook wirado | source | kind | retention |
|---|---|---|---|
| `memory-observe.posttooluse.sh` | `auto-hook` | `action` | `scratch` |
| `ctx-window-nudge.sh` mirror | `auto-hook` | `guard_nudge` | `scratch` |
| `memory-nudge.sh` | `auto-hook` | `guard_nudge` | `scratch` |
| `context-guard-nudge.sh` | `auto-hook` | `guard_nudge` | `scratch` |
| `memory-prune-session-start.sh` alerta | `auto-hook` | `state_render` | `scratch` |

### §6. Schema memory-mcp (backward compat)

Adicionar campos opcionais `source`, `kind`, `retention` em:

- `record_event` (`tools/cmd/memory-mcp/main.go`)
- `buffer_record` (`tools/cmd/memory-mcp/main.go`)

Defaults se omitidos pelo caller: `source=auto-hook`, `kind=note`, `retention=scratch` (não muda comportamento atual para callers que ignoram os eixos).

**Backward compat**: campos antigos (`agent`, `note`) preservados. DBs existentes migram sem script — campos novos são nullable.

### §7. Validação empírica (critério de promoção)

Rodar `memory-mcp stats` antes/depois em DB real. Esperado:

- Antes: ~95% scratch sem `kind` explícito
- Depois: ~70% scratch com `kind ∈ {action, guard_nudge, state_render}` classificado + ~30% permanent com `kind ∈ {decision, hypothesis_validated, task_completed}`

Métrica de sucesso: **>50% das memórias scratch pós-deploy têm `kind` declarado** (vs 0% hoje).

## Consequências

**Positivas:**

- DB semanticamente filtrável (query por `kind=decision` retorna só decisões verificáveis)
- `permanent` protegido contra gravação acidental de auto-hook (regra §2)
- Retrieval de memórias úteis melhora (não mistura com operacional)
- Telemetria futura (A-70) consegue correlacionar kind/source por hook
- Comando manual dá controle explícito ao humano sobre o que é permanente

**Negativas:**

- Mudança de default pode surpreender usuário acostumado com `high-signal`
- Comando manual novo precisa documentação (`docs/guides/memory.md` ou similar)
- Schema ganha 3 campos opcionais (impacto mínimo — nullable)
- Wiramento central precisa atualização em 5 hooks bash wirados

**Trade-offs assumidos:**

- Backward compat preservada (`high-signal` e `all` continuam funcionando como hoje)
- Default `operational` é mais conservador (pode perder algo que `high-signal` gravava — mas nada que era permanente de fato)
- Decisão explícita do agente para salvar `permanent` via tool call — exige que modelo conheça a interface (documentar)
- `kind=note` continua existindo como escape (auto-hook pode usar se kind apropriado não casa)

## Implementação

Sequência de commits granulares (1 commit por peça, ordem importa):

1. `feat(memory) schema record_event aceita source/kind/retention opcionais`
2. `feat(memory) subcommand 'agent-sync memory add --kind=...' (manual/agent)`
3. `feat(hooks) wiramento declara source=auto-hook + kind correto por hook`
4. `feat(hooks) OBSERVE_SCOPE=operational default + filtro por source/kind`
5. `test(memory) testes 4 camadas + smoke stats antes/depois`
6. `docs(state) D-N wrap-up + atualização A-71`

**Reversibilidade**: todos os commits podem ser revertidos isoladamente sem quebrar wiramentos existentes (campos opcionais + back-compat).

## Promoção Proposto → Aceito

| # | Critério | Estado |
|---|---|---|
| 1 | Schema `memory-mcp` aceita 4 eixos com back-compat (campos opcionais) | ⏳ |
| 2 | Comando `agent-sync memory add --kind=` funciona para todas as kinds | ⏳ |
| 3 | 5 hooks wirados declaram `source=auto-hook` + `kind` correto | ⏳ |
| 4 | Default `OBSERVE_SCOPE=operational` filtra `kind ∈ {action, guard_nudge, state_render}` | ⏳ |
| 5 | `high-signal` e `all` continuam funcionando (back-compat) | ⏳ |
| 6 | `memory-mcp stats` mostra >50% das scratch com `kind` declarado (vs 0% hoje) | ⏳ |

**Critérios de aceitação**: 6/6 + smoke DB real com stats antes/depois documentado.

## Referências

- D-86 (STATE.md) — fontes de lixo memory-mcp identificadas (14 hooks wirados)
- D-87 (STATE.md) — A-66 entregue: 85% redução via `count%THRESHOLD` + allowlist high-signal
- D-88 (STATE.md) — A-67 entregue: prune 7d + stats; achado 95% scratch sem critério
- D-89 (STATE.md) — A-68 entregue: auto-prune SessionStart + alerta staleness
- `tools/cmd/memory-mcp/main.go` — entrypoint memory-mcp CLI
- `tools/internal/agentmemory/store.go` — Store layer
- `hooks/memory-observe.posttooluse.sh` — D-87 fix high-signal (allowlist)
- `internal/hooks/apply_table.go` — wiramento central cross-CLI