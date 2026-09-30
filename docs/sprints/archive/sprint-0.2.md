# Sprint S-0.2 — Reavaliação de gaps 🟡/⛔ da Trilha C + briefing A-28 + 3 itens inspirados em ai-memory (akitaonrails)

**Status**: in_progress (desde 2026-09-24)
**Branch**: (nenhuma — mudança puramente documental)
**Mapa**: `docs/sprints/sprint-0.2.json`
**Predecessor**: S-0.1 arquivado em `docs/sprints/archive/sprint-0.1.json` (status `done`)
**Prior art analisado**: [github.com/akitaonrails/ai-memory](https://github.com/akitaonrails/ai-memory) (Rust, 8.3k stars) — análise em 2026-09-24, ses_f2c3cbf5fffeG2uu6hbJJFamFK.

## Contexto

S-0.1 fechou A-36 (refator `cmd/agent-sync/` em `internal/*`) e A-37 (migração física
`next_actions`→git log via 5 Etapas) — itens centrais da Sprint. Os 4 itens restantes
do catálogo S-0.1 (A-23, A-24, A-25, A-28) foram explicitamente marcados como
**"vão pra S-0.2"** durante o planning da S-0.1 (vide `sprint-0.1.md:96`).

Em 2026-09-24, nesta mesma sessão, o usuário pediu análise do projeto
[ai-memory](https://github.com/akitaonrails/ai-memory) (akitaonrails) para verificar
se há coisa a ser reaproveitada das propostas. Cruzamento feito com a realidade
do `agent-sync` identificou:

- **Capacidades que JÁ TEMOS** (verificado em código+docs): hooks cross-CLI wirados
  em 5 CLIs, captura append-only `.agent-sync/session-event.jsonl`, validação
  jsonschema embedded, `state next-action` como handoff cross-CLI, source-of-truth
  em git (STATE.md + session-state.json via D-7), SQLite local + FTS5, sync
  opcional Turso Cloud, 1 MCP tool genérico (`store_memory`), zero-LLM default
  (D-6), sanitização typed boundary (D-12).
- **Gaps reais identificados**: (1) `memory_briefing`/`memory_explore` tipados;
  (2) claim-once handoff protocol; (3) tier aging Working/Episodic/Semantic/
  Procedural; (4) cross-project messaging; (5) multi-user auth; (6) auto-improvement
  LLM consolidation.
- **Decisão consciente**: NÃO replicar 23 MCP tools do ai-memory (filosofia
  diferente — subcommands Go via `internal/*`, D-47/A-36). NÃO introduzir LLM
  (rejeitado em D-6). NÃO migrar para Rust (decisão D-47/A-36). A reaproveitar:
  só 3 conceitos pequenos (briefing, claim-once, tier aging) — viram A-51/A-52/A-53
  nesta Sprint.

S-0.2 nasce da regra **D-37 licao aplicada**: gaps 🟡/⛔ viram A-N estruturados. Estes 4 já
são A-N estruturados (criados em D-37), mas ficaram órfãos no fim de S-0.1. S-0.2 dá
a eles uma sprint dona para reavaliação de status com base em evidência documental
externa (changelog Cursor, feature request status, docs Antigravity) — **sem código novo**.

Escopo da Sprint é **decisão/documentação**, não implementação:
- A-23, A-24, A-25 → reavaliar se mantêm 🟡/⛔ ou migram para ✅
- A-28 → manter briefing vivo, sem atacar sub-itens nesta Sprint

## Items em ordem cronológica

### A-23 — C-3 Antigravity tokens via PreInvocation (Final: Mantida)

> **Reavaliação 2026-09-24** (ses_f2c3cbf5fffeG2uu6hbJJFamFK-resumed, Entrega 2 Trilha A): docs Antigravity CLI redirecionam para `/docs/getting-started?tab=cli`; sub-URLs (`/docs/cli/hooks`, `/docs/cli/hooks/preInvocation`) retornam 404 — payload fields não documentado em URL pública (verificado via `curl -I`, 6 URLs testadas). Heurística de transcript (parse de transcript_path JSONL com tokens_in/tokens_out + fallback 200k) validada em D-32 contra payload codex, simetria Claude/Codex sustenta confiança. **Veredito: Mantida 🟡**. Reabrir somente se usuário reportar anomalia ou Antigravity publicar docs de PreInvocation payload. **Status da tarefa no JSON**: continua `pending` (reavaliação ≠ conclusão — heurística continua wirada como rede-de-segurança).

**Estado atual** (D-32, 2026-09-20): wirado como hook bash `hooks/token-nudge.check.sh`
em PreInvocation do Antigravity. Status 🟡 porque **só wirado**, sem smoke dedicado
cross-CLI cross-Wire (a hipótese era "funciona por simetria com Claude/Codex", mas isso
é racionalização).

**Ação esperada em S-0.2**:
1. Ler doc pública do Antigravity sobre PreInvocation payload (input fields + output fields)
2. Verificar se payload tem campo nativo de tokens OU se hook recebe apenas metadata leve
3. Decidir entre:
   - ✅ **promover para Aceita**: se a heurística de transcript parse + fallback default 200k
     for comprovadamente correta para Antigravity (com smoke real)
   - 🟡 **manter**: se ainda houver dúvida, mas com nota "aguardando release N+1 do
     Antigravity com field nativo"
   - ⛔ **aceitar como gap**: se o PreInvocation do Antigravity não fornecer hooks
     suficientes para implementar tokens via heurística

**Sem prazo**: Antigravity não tem changelog público sistemático (verificar manualmente).
**Não-escopo**: não escrever smoke runtime nesta Sprint (separado).

### A-24 — C-3 Cursor tokens via heurística de transcript (Final: Mantida)

> **Reavaliação 2026-09-24** (ses_f2c3cbf5fffeG2uu6hbJJFamFK-resumed, Entrega 2 Trilha A): feature request Cursor forum [#147216](https://forum.cursor.com/t/cursor-hooks-token-usage-support/147216) tem 9 posts; última reply do staff (@deanrie) em **2026-09-10** — literal: *"nothing has changed since June... Token usage metadata input and output counts still isn't included in hook payloads... no promised timeline"*. Status: tracking, sem ETA. **Veredito: Mantida 🟡**. Heurística transcript continua sendo única opção. Trigger de reavaliação: (a) staff Cursor responder com commit público, ou (b) changelog mencionar `token metadata in hooks` / `usage payloads`. Próxima sondagem recomendada: 2026-10-24 (30 dias) ou após changelog relevante. **Status da tarefa no JSON**: continua `pending`.

**Estado atual** (D-32, 2026-09-20): mantido 🟡 como **rede de segurança** — hook bash
lê transcript do Cursor e extrai tokens via substring match (mesma técnica que Claude
Code, sem field nativo). PLAYBOOK-K cita feature request
[forum.cursor.com/t/cursor-hooks-token-usage-support/147216](https://forum.cursor.com/t/cursor-hooks-token-usage-support/147216).

**Ação esperada em S-0.2**:
1. Checar status atual do feature request (tem reply da equipe Cursor? tem merge previsto?)
2. Ler Cursor changelog mais recente (procurar "hooks", "tokens", "context")
3. Decidir entre:
   - ✅ **promover**: se Cursor lançou suporte nativo (atualizar A-16)
   - 🟡 **manter**: se ainda sem suporte (manter heurística como rede-de-segurança,
     atualizar nota com nova data de checagem)
   - ⛔ **aceitar**: se Cursor declarou que nunca vai expor (descartar heurística,
     manter fallback genérico)

**Trigger externo**: próxima release do Cursor (cadência ~2-4 semanas).

### A-25 — C-1 Cursor PreCompact ⛔ (Final: Mantida)

> **Reavaliação 2026-09-24** (ses_f2c3cbf5fffeG2uu6hbJJFamFK-resumed, Entrega 2 Trilha A): 4 artigos do changelog Cursor ago-set 2026 verificados via grep (`projects` 2026-09-10, `rollouts-and-security-reviewer` 2026-09-23, `self-hosted-machines` 2026-08-27, `start-from-scratch` 2026-08-19) — **nenhum** menciona hook block nativo, `beforeCompact`, `PreCompact`, `stop hook block`, ou `context metadata`. **Veredito: Mantida ⛔**. Gap continua aceito. Trigger de reavaliação: changelog Cursor mencionar `hook block`, `beforeCompact`, `PreCompact stop`, ou equivalente. Próxima sondagem: 2026-10-24 (30 dias) ou após release Cursor com menção a hook block. **ADR-precompact-snapshot-cross-cli.md Decisão 4** (C-1 Cursor ⛔) permanece válida. **Status da tarefa no JSON**: continua `pending`.

**Estado atual** (D-28, 2026-09-20): gap aceito como ⛔. Cursor não tem hook PreCompact
nem mecanismo equivalente de block (observacional apenas — não consegue bloquear
compactação programaticamente). ADR Decisao 4 da `ADR-precompact-snapshot-cross-cli.md`
formaliza o gap.

**Ação esperada em S-0.2**:
1. Checar Cursor changelog: hook `beforeSubmitPrompt` ou `beforeReadFile` virou
   `beforeCompact`? Algum equivalente?
2. Checar feature requests abertos relacionados
3. Decidir entre:
   - Se sim → virar A-51+ próprio para implementação (criar item estruturado)
   - Se não → manter ⛔, atualizar nota com nova data de reavaliação

**Trigger externo**: próxima release do Cursor (cadência ~2-4 semanas). Reavaliação
trimestral segundo ADR Decisao 4.

### A-28 — Briefing de nova sessão (Final: Mantida)

> **Progresso parcial em 2026-09-24** (ses_f2c3cbf5fffeG2uu6hbJJFamFK-resumed, wrap-up parcial): **2 de 4 sub-itens atacados**:
> - **Sub-item (ii) DONE**: sincronização README.pt-BR.md entregue em commit `4365eab` (Skills section 39+12=51, env vars expandidas 4 colunas, Cursor specifics com caveat Cloud Agents + nota Context7 não-offline).
> - **Sub-item (iii) DONE**: agente `frontend-developer` criado em commit `733f5da` (70 linhas, escopo distinto de `frontend-ui-designer` — implementador vs auditor).
> - **Pendentes**: (i) SMOKE-TEST-U — smoke real via endpoint HTTP `POST /api/session/{id}/compact` (A-22/D-38 fechou probe, falta runtime real); (iv) livre escolha do usuário.
>
> **Veredito Mantida**: briefing continua ativo (sub-itens i e iv pendentes); progresso parcial registrado na nota. **Decisão taxonômica**: não introduzir verdict `Parcial` (nunca usado em S-0.1/S-0.2 — escolhido `Mantida` para preservar taxonomia estável). Sub-itens atacados (ii, iii) já viraram entregas próprias — não recriar como A-N+ retroativos.

**Estado atual**: briefing vivo (não é uma ação, é uma lista de opções para o usuário).

**4 sub-itens a critério do usuário**:
1. **SMOKE-TEST-U** — smoke real via endpoint HTTP `POST /api/session/{id}/compact`
   (A-22/D-38 fechou probe, falta runtime). Validaria que o probe funciona end-to-end,
   não só via SDK.
2. **Sincronizar README.pt-BR.md via A-27** — trazer PT-BR para a mesma estrutura
   compacta do EN (162 linhas).
3. **Sub-agente custom `frontend-developer`** — gap real backend=12/frontend=0.
4. **Livre escolha** — outras tarefas a critério do usuário.

**Ação esperada em S-0.2**: **nenhuma execução**. O briefing permanece ativo como
referência. Sub-itens viram A-N+ próprios quando o usuário decidir atacar (conforme
D-37 licao: gaps precisam virar A-N estruturados, não ficar como bullet em briefing).

### A-51 — Memory Briefing estruturado (Final: Aceita)

> **Entrega 2026-09-24** (ses_f2c3cbf5fffeG2uu6hbJJFamFK-resumed, Entrega 5 Trilha A, commit `5432b05`): subcommand `agent-sync state briefing` entregue. Adicionado em `internal/state/render.go` (~150 linhas). 4 seções no JSON output (sem flag `-json`/`-limit`): `events_recent` (últimas 10 via `event.ReadEvents`), `next_action` (via shim `NextAction` em `model.go:394`), `tasks_pending` (10 primeiros com status='pending'), `errors_recent` (últimas 10 de `~/.cache/agent-sync/hooks/errors.jsonl`). **Decisão arquitetural**: `briefing` é **case dentro de `internal/state/render.go`** (`RunCommand`), NÃO pacote novo `internal/briefing/` — `briefing` é visão agregada do state, não domínio novo. **Helpers duplicados de `internal/apply/observability.go`** (~30 linhas: `hookErrorEvent`, `resolveHookLogPath`, `readHookEvents`) — zero acoplamento entre pacotes (D-47/A-36). **Limites fixos 10/10/10/10** (sem flag `-limit` — preferência do usuário registrada em `preferencia-limite-fixo-sem-flag-limit`). Validação: `go test ./...` 13/13 verde; `make install` ok; smoke real retorna JSON com 4 seções (`events_recent=1`, `next_action=A-20`, `tasks_pending=10`, `errors_recent=10`). Wrap-up estrutural pendente (D-79 + status `pending`→`done`).

**Origem**: análise do projeto [ai-memory (akitaonrails)](https://github.com/akitaonrails/ai-memory)
em 2026-09-24 — eles têm `memory_briefing` + `memory_explore` como ferramentas MCP tipadas.

**Estado atual**: nudges ad-hoc por CLI (`agent-react-nudge.stop.cursor.sh`,
`memory-nudge.sh` cross-CLI) mas **não há subcommand estruturado** que agregue
estado recente do projeto.

**Proposta**: criar `agent-sync state briefing` (~150 linhas + testes) que retorna
JSON estruturado com:
- Contagem de eventos por CLI nas últimas N horas (de `.agent-sync/session-event.jsonl`)
- Próximas ações pendentes (já existe via `state next-action`)
- Regras violadas recentes (hooks `false-success-guard`)
- Handoff pendente (via `A-N` em `state.tasks[]` com `status=pending`)

**Reutiliza**: session-event.jsonl (D-22) + session-state.json (D-7) + nudges wirados.

**Decisão consciente**: NÃO replicar 23 MCP tools do ai-memory (filosofia diferente:
subcommands Go via `internal/*`, D-47/A-36). Subcommand `state briefing` é 1 ponto
de entrada, não 23.

**Anti-overengineering**: NÃO adicionar LLM-driven synthesis (rejeitado em D-6);
output é JSON estruturado determinístico.

### A-52 — Claim-once handoff (Proposta)

**Origem**: ai-memory tem `memory_handoff_begin/list/accept/cancel` (claim-once,
identity-keyed) — protocolo typed para evitar race entre 2 CLIs pegando a mesma
ação.

**Estado atual**: `agent-sync state next-action` retorna A-N **sem claim**, permitindo
race entre 2 CLIs wiradas no mesmo projeto.

**Proposta**: adicionar campos `claimed_by` (string CLI kind) + `claimed_at` (RFC3339)
no `SessionTask` struct (~50 linhas + 2 testes):
- Se `claimed_by != ""` e `now - claimed_at < timeout` → erro `"já reivindicado por {cli_kind}"`
- Timeout configurável via env var `AGENT_SYNC_HANDOFF_TIMEOUT` (default 1h)
- Comandos novos: `agent-sync state claim <A-N>` e `agent-sync state release <A-N>`

**Sem cross-process lockfile** (D-2 confirmou lockless default com tmpfile + retry).

**Migration**: tasks existentes têm `claimed_by=""` (backward compatible — read
ignora tasks já reivindicadas há mais de 24h mesmo sem claim explícito).

### A-53 — Memory tier aging (Proposta — DEFERIDA para S-0.3+)

**Origem**: ai-memory define 4 tiers (Working/Episodic/Semantic/Procedural) com
decay multi-curva (`lambda` per tier, `salience · exp(-lambda·Δt) + access_reinforcement`,
half-life per tier, contradiction flagging).

**Estado atual**: `.agent-sync/session-event.jsonl` retém eventos **sem aging**
(só rotação 10MB single-rotation em D-22). Tudo vira "episódico perpétuo".

**Proposta**: adicionar campo `tier` (enum `working|episodic|semantic|procedural`)
em `SessionEvent`; implementar sweep de decay com half-life per tier.

**Decisão DEFERIDA para S-0.3+**: schema bump + migration de eventos legados é
**alto risco** (envolve `internal/event/`, `internal/state/`, schema embedded,
e re-normalização do JSONL atual). Fora do escopo de S-0.2.

**Anti-overengineering**: NÃO replicar decay formula verbatim (eles têm
half_life_days, salience, access_count, breadth_weight, contradiction flagging
— tudo opt-in lá). Nossa versão deve começar mínima (só tier enum + sweep básico)
e evoluir organicamente. NÃO implementar `_slots/`, `page_access`, `page_feedback`
— cada um é decisão separada quando atacado.

### A-54 — Memory Query estruturado (Proposta)

**Origem**: ai-memory `memory_query` (FTS5 + entity-match + graph-neighbour RRF + vector RRF + authority adjustment + raw fallback). É o tool mais usado deles.

**Estado atual**: temos `memory-mcp` `store_memory` (escrita genérica) + `agent-sync skills index` + `agent-sync event stats` + `agent-sync state read`. **Nenhum destes cruza fontes** ou faz ranking.

**Proposta**: subcommand `agent-sync memory query <query> [-cli X] [-kind Y] [-since Z] [-limit N]` (~150 linhas + testes). Cruza:
- `.agent-sync/session-event.jsonl` via FTS5 (já temos `event read`)
- `.agent-sync/session-state.json` direto (já temos `state read`)
- `STATE.md` markdown via grep ou índice FTS5 (decidir quando atacado)

Sem entity index ou vector embedding (escopo separado, evita dependência LLM).

**Formato de exposição**: subcommand Go (decidir quando atacado).

### A-55 — Memory Recent (Final: Aceita)

> **Entrega 2026-09-24** (ses_f2c3cbf5fffeG2uu6hbJJFamFK-resumed, Entrega 3 Trilha A, commit `1ed47ba`): subcommand `agent-sync memory recent` entregue. Pacote novo `internal/memory/command.go` (~90 linhas + 2 testes). Wrapper minimalista sobre `event.ReadEvents` — **sem schema bump last_accessed_at, sem score de autoridade** (decisão do usuário: wrapper puro). Default `-last=10`. Saída JSONL cru (igual `event read`, permite piping jq). Roteamento em `cmd/agent-sync/main.go`. Validação: `go test ./...` 13/13 pacotes verde; smoke real contra `.agent-sync/session-event.jsonl` deste projeto retornou JSONL parseável. Wrap-up estrutural (D-77 + status done) em entrega subsequente.

**Origem**: ai-memory `memory_recent` (páginas atualizadas recentemente, com score de autoridade).

**Estado atual**: `agent-sync event read -last N` lê JSONL cru sem ranking por relevância/autoridade.

**Proposta**: subcommand `agent-sync memory recent [-limit N] [-since T]` (~30 linhas + testes). Wrapper sobre `event read -last N` com ranking simples (timestamp + contagem de acessos se houver). Trivial.

### A-56 — Memory Read Page (Proposta)

**Origem**: ai-memory `memory_read_page` (lê arquivo markdown completo + frontmatter).

**Estado atual**: `agent-sync state read` lê `session-state.json`; `cat docs/adr/X.md` lê markdown manualmente.

**Proposta**: subcommand `agent-sync memory read-page <path>` (~80 linhas + testes). Lê arquivo markdown sob `docs/adr/`, `docs/sprints/`, `docs/guides/` retornando body inteiro + frontmatter parseado (YAML). Reutiliza leitor markdown se já houver; senão `gopkg.in/yaml.v3` ou similar.

### A-57 — Memory Read Session (Proposta)

**Origem**: ai-memory `memory_read_session_observations` (paging + body-cap sobre observações de uma sessão).

**Estado atual**: `agent-sync event read -session <id>` retorna tudo de uma vez sem paging/body-cap.

**Proposta**: subcommand `agent-sync memory read-session -session <id> [-limit N] [-offset N] [-body-max-chars N]` (~60 linhas + testes). Resolve lacuna real: hoje `event read` retorna tudo de uma vez, pode ser >1000 linhas para sessão longa.

### A-58 — Memory Feedback (Proposta)

**Origem**: ai-memory `memory_feedback` (helpful/not_helpful/stale/wrong → bump/floor salience + lint finding).

**Estado atual**: zero mecanismo de feedback. Decisões D-N são editadas à mão no `session-state.json` quando há erro.

**Proposta**: subcommand `agent-sync memory feedback <path> <signal> [--reason <text>]` (~80 linhas + testes). Sinais:
- `helpful`/`not_helpful`: bump/floor `salience` (campo novo em SessionPage — schema bump deferido para A-53)
- `stale`/`wrong`: gera lint finding em `memory_feedback.jsonl` separado (audit trail append-only)

Sem dependência LLM. Quando implementado, amarra com A-61 (memory lint).

### A-59 — Memory Write Page (Proposta)

**Origem**: ai-memory `memory_write_page` (escrita tipada de página wiki com `scope` + `expires_at`).

**Estado atual**: `memory-mcp` `store_memory` é genérico (não tipado por scope, sem TTL).

**Proposta**: subcommand `agent-sync memory write-page <path> --body <md> [--scope project|global] [--expires-at <RFC3339>]` (~120 linhas + testes). Schema:
- `scope` default = `project`; `global` = cross-project (escopo `_global` reservado, análogo ao ai-memory)
- `expires_at` RFC3339 ou date-only — ativa via A-53 quando implementado
- Reescreve `store_memory` do memory-mcp com contrato tipado; mantém alias para back-compat

### A-60 — Memory Delete Page (Proposta)

**Origem**: ai-memory `memory_delete_page` (delete idempotente com admission chain para auditoria).

**Estado atual**: `agent-sync state write` faz overwrite. Sem `delete` explícito.

**Proposta**: subcommand `agent-sync memory delete-page <path>` (~40 linhas + testes). Idempotente: erro se path não existe. Admission chain (`op=delete`) para auditoria. Sem cross-project (escopo = projeto atual).

### A-61 — Memory Lint (Proposta)

**Origem**: ai-memory `memory_lint` (rule-based + LLM contradiction findings).

**Estado atual**: `agent-sync doctor` (D-58) cobre health checks (binários, skills, schemas, plugins). Mas **lint de conteúdo** (dangling refs, frontmatter inválido, páginas órfãs) não existe.

**Proposta**: subcommand `agent-sync memory lint` (~200 linhas + testes). Detecta:
- **Dangling refs**: `[[X]]` apontando para path inexistente
- **Frontmatter inválido**: YAML malformado em markdown com frontmatter
- **Páginas órfãs**: markdown em `docs/` não linkado por nenhuma outra página

Zero-LLM. Decidir default-on vs opt-in quando atacado.

## Decisões adiadas

| Item | Motivo | Vai pra |
|---|---|---|
| Smoke runtime A-23/A-24 | Requer CLI wirada e sessão planejada (~1h cada); S-0.2 é só decisão documental (reavaliação feita 2026-09-24, veredito Mantida — ver seções dos itens) | S-0.3 ou sessão avulsa |
| Implementação A-25 se Cursor lançar block nativo | Reavaliação 2026-09-24 confirmou gap ⛔ mantido (4 changelogs ago-set 2026 sem hook block nativo); depende de feature request ser aceita pela equipe Cursor (imprevisível) | A-62+ próprio se acontecer (próximo ID livre após A-61) |
| Atacar sub-itens A-28 (i/ii/iii) | Decisão do usuário; S-0.2 só mantém briefing | S-0.3 ou sessões avulsas |
| Remover aviso D-61 do STATE.md | Workaround temporário de D-61 — remoção é tarefa separada fora do escopo | S-0.3 ou decisão ad-hoc |
| Implementação A-51/A-52/A-53 | S-0.2 é só decisão/documentação; implementação exige código novo + testes + smoke | S-0.3 ou sprint dedicada |

## Riscos / gaps conhecidos

- **A-23/A-24/A-25 dependem de changelog externo** — não posso forçar avaliação
  periódica; cadência da decisão é a cadência dos releases dos CLIs. Reavaliação
  parcial em 2026-09-24 (Entrega 2 Trilha A) mudou 3 itens de Proposta → Final:Mantida
  com evidência externa (changelog Cursor ago-set 2026 sem hook block nativo; feature
  request #147216 sem ETA; docs Antigravity CLI inacessíveis). Reavaliações futuras
  continuam necessárias (trigger: changelog/feature request com mudança real).
- **A-28 sem dono** — é briefing, não ação; fica em `state.tasks[]` como `pending`
  indefinidamente até o usuário escolher um sub-item.
- **Convenção D-65**: mudança de fase exige perguntar ao usuário. S-0.2 não move nada
  automaticamente — agente só aplica `Investigation` ou `Complexibility` se o usuário
  pedir explicitamente.

## Métricas de saída

Quando esta Sprint for `done`, deve ser verdade **tudo abaixo**:

- [ ] Os 15 itens (A-23, A-24, A-25, A-28 + A-51, A-52, A-53 + A-54..A-61) saíram de
      `phase=Proposta` para `phase=Final` (qualquer combinação de Aceita/Re-Proposta/
      Descartada conforme reavaliação). **Progresso 2026-09-24 (Entrega 2 Trilha A)**:
      3/15 itens já em Final (A-23/24/25 = Mantida com evidência externa); 12/15
      ainda em Proposta (A-28, A-51/52/53, A-54..A-61).
- [ ] Cada item tem `note` justificado com evidência externa (link changelog, doc,
      feature request status, prior-art comparison) — não apenas "decidi X"
- [x] A-23/A-24/A-25: decisão fundamentada em changelog/feature requests externos
      (release-triggered). **Satisfeito 2026-09-24**: A-23 (docs Antigravity inacessíveis,
      heurística validada em D-32), A-24 (Cursor forum #147216 com staff reply
      2026-09-10 sem ETA), A-25 (4 changelogs Cursor ago-set 2026 sem hook block nativo).
- [ ] A-51/A-52/A-53: decisão de implementação ou rejeição fundamentada na comparação
      com ai-memory (akitaonrails) — não apenas "parece útil"
- [ ] A-54..A-61: cada tool do ai-memory catalogado tem nota explicando por que vale
      replicar (referência específica ao tool original, não só "úteis em geral")
- [ ] Formato de exposição (subcommand Go vs MCP tool) decidido para pelo menos 1 dos
      A-54..A-61 antes do fim da Sprint (deferido para ataque individual, conforme
      resposta do usuário 2026-09-24)
- [ ] `state.tasks[]` continua com A-20 e A-47 como `pending` (não migrados para S-0.2;
      têm prazo próprio — vide `sprint-0.1.md:73-92`)
- [ ] Catálogo desta Sprint movido pra `docs/sprints/archive/` com status `done`
