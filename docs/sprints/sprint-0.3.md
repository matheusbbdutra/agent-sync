# Sprint S-0.3 — Camada 4 memory-mcp: feedback + write-page + lint

**Status**: in_progress (desde 2026-09-24)
**Branch**: (nenhuma — mudanças são em código + docs)
**Mapa**: `docs/sprints/sprint-0.3.json`
**Predecessor**: `docs/sprints/sprint-0.2.json` (status `done`, ends 2026-09-24)
**Prior art**: [github.com/akitaonrails/ai-memory](https://github.com/akitaonrails/ai-memory) — Camada 4 (`memory_feedback`, `memory_write_page`, `memory_lint`)

## Contexto

S-0.2 fechou as Camadas 1-3 do memory-mcp:

- **Camada 1** (A-56/A-57/A-60): shins finos de read_page / read_session / delete_page sobre `store_memory` / `Get` / `Delete`. 3 entregas.
- **Camada 2** (A-52): state claim/release — mas é interno ao `agent-sync`, não MCP.
- **Camada 3** (A-54): `memory_query` como alias de `search_memory` (BM25 + decay exponencial já existentes em `Store.Search`).

S-0.2 fechou com **9 itens entregues** + ADR A-53 (tier aging deferido) + status `done` no commit `6b53d06`. Padrão de validação elevado para 4 camadas (go test + smoke stdio + wirado cross-CLI + execução real via delegate Claude headless — confirmado em A-54 com 10 memórias retornadas em probe real).

3 itens da **Camada 4 do ai-memory** ficaram DEFERIDOS em S-0.2 (não por risco arquitetural, mas por falta de sprint): **A-58 (feedback), A-59 (write-page), A-61 (lint)**. S-0.3 é a sprint dona deles.

Escopo da Sprint é **implementação** (não decisão/documentação como S-0.2):
- A-58 → subcommand Go + append-only JSONL
- A-59 → subcommand Go + schema tipado (alias de store_memory)
- A-61 → subcommand Go + 3 regras rule-based

Decisões a fechar **durante** a sprint (não antes, para não antecipar sem evidência):

1. **A-58 feedback útil?** Sinais `helpful`/`not_helpful` viram bump/floor em `salience` (requer schema bump imediato, escopo cresce) OU ficam só como registro de auditoria sem efeito no ranking (trivial, utilidade reduzida)?
2. **A-59 backward-compat**: `store_memory` vira alias fino que delega para `write-page` (zero quebra, +20L) OU convive lado-a-lado (manutenção duplicada, zero risco)?
3. **A-61 default-on vs opt-in**: default-off é mais seguro (não surpreende em CI); default-on força correção imediata?

Padrão de validação a aplicar (replicar A-54):
1. `go test` unitário (helper de teste + 3+ cenários).
2. Smoke stdio real contra binário compilado.
3. Wirado cross-CLI: `claude mcp list` + `agy mcp list` + `opencode mcp list` (todos `Connected`).
4. **Execução real via delegate**: `echo PROMPT | claude -p --model claude-haiku-4-5 --allowedTools mcp__memory__*` retornando evidência real.

## Items em ordem cronológica

### A-58 — Memory Feedback (Proposta)

> **Origem**: ai-memory `memory_feedback` (helpful/not_helpful/stale/wrong → bump/floor `salience` + lint finding).

**Estado atual** (2026-09-24): zero mecanismo de feedback. Decisões D-N são editadas à mão no `session-state.json` quando há erro. Sem audit trail.

**Proposta**: subcommand `agent-sync memory feedback <path> <signal> [--reason <text>]` (~80 linhas + testes). Sinais:
- `helpful`/`not_helpful`: bump/floor `salience` (campo NOVO em SessionPage — schema bump deferido até A-53)
- `stale`/`wrong`: gera lint finding em `memory_feedback.jsonl` separado (audit trail append-only)

Sem dependência LLM. Quando implementado, amarra com A-61 (memory lint) — sinais `stale`/`wrong` viram findings que `lint` lê.

**Decisão a fechar ao atacar**: começar só-como-auditoria (helper de escrita JSONL) e deixar bump para A-53? **Default proposto: sim** — trivial, útil, sem bloquear nada.

**Sem dependência externa**.

### A-59 — Memory Write Page (Proposta)

> **Origem**: ai-memory `memory_write_page` (escrita tipada de página wiki com `scope` + `expires_at`).

**Estado atual** (2026-09-24): `memory-mcp` `store_memory` é genérico (não tipado por scope, sem TTL).

**Proposta**: subcommand `agent-sync memory write-page <path> --body <md> [--scope project|global] [--expires-at <RFC3339>]` (~120 linhas + testes). Schema:
- `scope` default = `project`; `global` = cross-project (escopo `_global` reservado)
- `expires_at` RFC3339 ou date-only — ativa via A-53 quando implementado

**Decisão crítica a fechar ANTES de codar**: `store_memory` vira **alias fino** que delega para `write-page` (clients existentes continuam funcionando) OU convive **lado-a-lado** (mais código, zero risco)?

**Default proposto**: alias fino com delegação. Estimativa: +20L para manter backward-compat. Anti-overengineering (AGENTS.md §3) — preferir alias.

**Quando alias estiver wirado**: re-rodar probe das 4 camadas para confirmar que `store_memory` continua funcionando exatamente como antes.

### A-61 — Memory Lint (Proposta)

> **Origem**: ai-memory `memory_lint` (rule-based + LLM contradiction findings).

**Estado atual** (2026-09-24): `agent-sync doctor` (D-58) cobre health checks (binários, skills, schemas, plugins). Mas **lint de conteúdo** (dangling refs, frontmatter inválido, páginas órfãs) não existe.

**Proposta**: subcommand `agent-sync memory lint` (~200 linhas + testes). Detecta:
1. **Dangling refs**: `[[X]]` apontando para path inexistente
2. **Frontmatter inválido**: YAML malformado em markdown com frontmatter
3. **Páginas órfãs**: markdown em `docs/` não linkado por nenhuma outra página

Zero-LLM (D-6). Implementação isolada — 3 regras independentes, ~60-70L cada.

**Decisão a fechar ao atacar**: default-on (roda em todo `doctor`) vs opt-in (flag `--lint`)? **Default proposto: opt-in com flag `--lint`** — não surpreende em CI, deixa usuário chamar quando quiser.

**Trade-off de parser YAML**: adicionar `gopkg.in/yaml.v3` (1 import, +~30L transitivo) OU parser manual minimal (~40L, mais bugs)? **Default proposto**: yaml.v3 se já estiver em deps; senão parser manual.

## Decisões adiadas

| Item | Motivo | Vai pra |
|---|---|---|
| A-53 (tier aging) | Schema bump em SessionEvent + migration + 3 decisões B/C/D em aberto | S-0.4 ou posterior (4 gatilhos de reabertura documentados em ADR-A53) |
| A-47 (revisão ADRs) | Prazo 2026-10-23 (~29 dias) | sessão avulsa antes do prazo |
| A-20 (smoke T5 30d) | Prazo 2026-10-21 (~27 dias) | sessão avulsa antes do prazo |
| Sub-itens A-28 pendentes | Decisão do usuário; sem sprint dona | S-0.4+ se houver |

## Riscos / gaps conhecidos

- **A-59 quebra store_memory?** Não se alias for o caminho escolhido. Probe das 4 camadas (especialmente #4 — execução real via delegate Claude invocando `store_memory` e validando retorno idêntico) cobre o risco.
- **A-61 escopo explode?** Risco real se virar 10+ regras. Mitigação: cortar em 3 regras (acima), deixar expansão para S-0.4+. Cada regra com `// TODO` marcando extensão futura.
- **A-58 trivial vira trivial-mas-inútil?** Se só-como-auditoria for escolhido, `memory_feedback.jsonl` cresce sem nunca ser lido por nada. Mitigação: A-61 (lint) lê o JSONL e gera findings — fecha o ciclo.

## Métricas de saída

Quando esta Sprint for `done`, deve ser verdade **tudo abaixo**:

- [ ] A-58/A-59/A-61 saíram de `phase=Proposta` para `phase=Final` (todos `Aceita` se 4 camadas de evidência forem atingidas).
- [ ] Cada item tem `note` justificado com link para commit + 4 camadas de validação (go test + smoke stdio + wirado cross-CLI + delegate real).
- [ ] `go test ./...` verde em todos os pacotes tocados.
- [ ] Nenhuma regressão nas Camadas 1-3 (A-54, A-56, A-57, A-60 continuam funcionando com mesmas respostas).
- [ ] `memory_feedback.jsonl` wirado e testado (A-58).
- [ ] `store_memory` continua funcionando como antes (A-59, se alias for escolhido).
- [ ] `agent-sync memory lint` retorna lista de findings em JSON (A-61).
- [ ] Sprint movida para `docs/sprints/archive/` com status `done` (ritos D-65+D-37).
- [ ] `non_goals` honrados: A-53/A-47/A-20 não tocados.
