# Cruzamento: 5 repos × agent-sync

> **Escopo**: extrair dos 5 repos **padrões reusáveis** que possamos adaptar ao **projeto agent-sync** (este repo), não ao setup geral do opencode. Aplicar a regra "nada se cria, tudo se copia" com adaptação — não cópia literal.

**Inventário real do agent-sync** (referência):

| Componente | Estado |
|---|---|
| Skills curadas | 51 (em `skills/` + `skills/manifest.json`) |
| Agents authored | 12 (em `agents/`) |
| Tools/binários Go | 15 (`tools/cmd/*`): ast-outline, ctx-window, db-guardian, docs-cache-write, docs-fetch, docs-mcp, false-success-guard, git-diff-summary, memory-mcp, memory-sync, mr-collect-cli, mr-review-local, repo-map, shell-validate, trace-strip |
| Schemas JSON | 5 (`tools/jsonschema/schemas/*.json`): agent_tasks, precompact-snapshot, session-event, session-state, token-budget-status |
| Hooks | ~30 cross-CLI (Claude/Codex/Antigravity/Cursor/OpenCode v1+v2) |
| CLIs wiradas | 5 (Claude, Codex, Antigravity, OpenCode, Cursor) |
| Padrão de plugins v2 OpenCode | TS com `Plugin.define()` + `ctx.tool.hook('execute.before/after')` + `ctx.session.hook('compaction')` |

---

## 1. kingbootoshi/cartographer — **Adaptação viável: `tools/cmd/repo-map`**

### O que ele faz de reusável
- **Schema tipado (Zod v4) de grafo de codebase** com 21 tipos de nó × 18 tipos de aresta, evidência de linha por edge.
- **Extractors multilíngues**: imports/símbolos/SQL/IaC/CI/env vars.
- **Brief builder bounded** com `requestedTokens`/`hardLimitTokens`.
- **Adoption score** (métrica de uso do grafo).
- **Audit ledger** para refactor.
- **MCP server** com 9 tools.

### O que JÁ temos em agent-sync
- `tools/cmd/repo-map/` (174 linhas prod, 175 test) — binário **homônimo** ao conceito dele. Provavelmente faz só indexação rasa.
- Hook `hooks/repo-map-warmup.sh` (54 linhas) e variante `repo-map-warmup.cursor.sh` (14 linhas) — wirado em runtime.
- Schema `tools/jsonschema/schemas/` tem 5 schemas, **nenhum é de mapa de repo**.

### Gap real (verificável)
1. **Não temos schema de grafo** — não dá pra descrever `File→IMPORTS→File` ou `DbTable←SERVICE_QUERIES_TABLE←Service` tipado.
2. **Não temos extractors multilíngues** — só ast-outline para outlines estruturais (não relacional).
3. **Não temos brief builder bounded** — quando o agente precisa de "contexto curado sobre X", só temos `ast-outline` + `grep`.
4. **Não temos adoption score** — não medimos se o hook `repo-map-warmup` é útil de fato.
5. **Não temos MCP exposto** — `repo-map` é só CLI; pra usar dentro de sessão do agente precisa hook+MCP.

### Adaptação proposta (cirúrgica, faseada)

**Fase 1 — Mínimo viável** (~1 entrega):
- Adicionar schema `repo-graph.json` com 3-4 tipos de nó (File, Symbol, Import, Package) e 2-3 edges (IMPORTS, CONTAINS, REFERENCES). Validado no jsonschema lib que já temos (já wirada).
- Espelhar a CLI: `tools/cmd/repo-map-graph/` (sibling de `repo-map/`) com `--root`, `--format json|brief`, `--budget-tokens N`. Sem extração pesada: reusar `ast-outline` pra symbols + `grep` pra imports.
- Brief bounded: trunca saída por `--budget-tokens`, devolve JSON `{nodes, edges, summary, truncated}`.

**Fase 2 — MCP** (se Fase 1 útil):
- Expor `repo-map-graph` como tools MCP: `repo_graph_brief(path)`, `repo_graph_impact(file)`, `repo_graph_diff(old, new)`.
- Hook `repo-map-warmup` ganha variante que injeta brief no `UserPromptSubmit` quando o usuário mencionar um path.

**Fase 3 — Métricas**:
- Adoption score no `session-event.jsonl` (já temos o schema). Cada carregamento do MCP emite um evento `repo_graph_brief_used` com tokens economizados.

**Esforço**: Fase 1 ~1-2 entregas A-N (analogamente a A-29/A-30 do histórico). Fase 2/3 ficam pra depois.

**Risco**: schema tem que ser fechado por padrão (ADR-001 princípio). Edge provenance `source: filesystem|git|package-manager`.

---

## 2. anthropics/claude-plugins-official → `claude-code-setup` — **Adaptação viável: skill meta-auditor**

### O que ele faz de reusável
- Skill `claude-automation-recommender` (read-only) que varre projeto e recomenda top 1-2 automações por categoria (MCP/skills/hooks/subagents/commands), com 5 references curadas.

### O que JÁ temos em agent-sync
- `skills/manifest.json` já é curadoria (35→45→50 skills). Mas **não há skill que audite o repo agent-sync mesmo** em busca de melhorias.
- `agent-sync skills index` (A-29) já detecta orphans/missing — parcialmente o que ele faz.

### Gap
- Não temos um **meta-skill** que rode periodicamente e sugira: "faltam MCP servers", "esses 3 hooks estão deprecated", "esse agent authored pode virar skill". Isso casa com `skills new` (A-32).

### Adaptação proposta (1 skill nova)
- Skill `repo-auditor` (authored) que: (1) lista skills vs agents vs hooks vs plugins do agent-sync; (2) cruza com manifest.json; (3) emite 1-2 recomendações curtas (DRY: reusar `skills index` + subcommand hipotético `agent-sync audit`).
- Invocável via `/repo-auditor` (user-only) ou auto-load em `UserPromptSubmit` com threshold baixo (1x por dia).
- Custo: 1 skill nova em `skills/repo-auditor/SKILL.md` (~80 linhas), zero Go novo.

---

## 3. thedotmack/claude-mem — **Adaptação viável, mas atenção ao escopo**

### O que ele faz de reusável
- Hook pipeline observacional: `PostToolUse:*` → grava observação comprimida; `PreToolUse:Read` → busca memórias do path; `Stop` → sumariza sessão.
- Worker service como daemon (inicializado uma vez, hooks só disparam runner).
- Storage SQLite + Chroma (embeddings) para retrieval semântico por path.

### O que JÁ temos em agent-sync
- `tools/cmd/memory-mcp/` (544 linhas prod) — servidor MCP de memória.
- `tools/cmd/memory-sync/` (89 linhas) — sincronização.
- Já temos **5 schemas** (`session-state`, `session-event`, `agent_tasks`, `precompact-snapshot`, `token-budget-status`) — **mais maduros** que o do claude-mem.
- `hooks/memory-*` wirados em runtime.

### Gap real
1. **Hook `PreToolUse:Read` → buscar memórias do path**: provavelmente NÃO temos. O `memory-nudge.opencode.ts` deve ser só reativo.
2. **Compressão de observações via Claude Agent SDK**: NÃO temos (não usamos SDK pra comprimir).
3. **Worker daemon com hooks disparam só `bun-runner`**: arquitetura similar ao nosso `agent-task-record.stop.sh` (já BG=1).

### Adaptação proposta (cirúrgica)
- **Reaproveitar** o que já temos: 5 schemas JSON + memory-mcp + session-event.jsonl.
- **Adicionar** UM hook `PreToolUse:Read` que consulta memory-mcp por path antes de ler o arquivo. Schema: já temos `session-state.json` com `decisions` por `project` — index por path.
- **Não implementar**: worker daemon separado (overkill — bash hook wirado já é suficiente, padrão D-31).
- **Não implementar**: Chroma. Retrieval por path exato + prefixo cobre 90% do uso. Embeddings só se virar gargalo medido.

**Custo**: 1 hook novo `hooks/memory-pretooluse-read.sh` (~30 linhas, mesmo padrão do `token-nudge.check.sh`). Zero Go.

---

## 4. headroomlabs-ai/headroom — **Adaptação seletiva, NÃO copiar proxy**

### O que ele faz de reusável
- ContentRouter especializado por tipo (JSON/code/prose) com compressores diferentes.
- **CacheAligner** — não invalida prefix-cache do provider.
- **CCR** reversível (guarda originais por hash, retrieve on-demand).
- **`headroom learn`** — minera sessões falhas e escreve em AGENTS.md.
- Plugin OpenCode oficial.

### O que JÁ temos em agent-sync
- `tools/cmd/ctx-window/` (2756 linhas prod — **o maior do projeto**) já faz gerência de janela com K + summary incremental. É nosso **equivalente direto** ao que headroom faz para input tokens.
- `tools/cmd/token-nudge/` wirado como hook cross-CLI.
- `agent-sync skills index` (A-29) é um exemplo simples de "learn on demand".

### Gap real
1. **CacheAligner**: NÃO temos (e nem temos como medir facilmente — fica dentro do provider). Risco se um dia formos comprimir prompts é invalidar o prefix-cache. **Não implementar agora** — documentar como ADR-cuidado.
2. **Output token reduction**: NÃO temos (só `ctx-window` cuida de input). Útil mas arriscado.
3. **`headroom learn`**: NÃO temos. Mas temos `STATE.md` que já é o "diário" + `A-N/D-N` que é o "log estruturado". Adaptar é fácil.

### Adaptação proposta (2 entregas)

**A. ADR-cache-aligner.md** (~50 linhas):
- Documentar o risco: comprimir system prompt pode invalidar prefix-cache do Anthropic/OpenAI.
- Decisão: **não comprimir prompts por enquanto**. Justificar com benchmark do `ctx-window` (já é incremental summary, não reescrita).
- Trigger pra reabrir: se um dia `ctx-window` virar gargalo e quisermos compressão semântica.

**B. Skill `agent-learn`** (1 skill nova, ~60 linhas):
- Roda sob demanda (`/agent-learn`): lê `session-event.jsonl` + `STATE.md` mais recentes, identifica padrões de erro (loop, hipóteses refutadas, comandos destrutivos bloqueados).
- Escreve proposta de bullet em `rules/global-rules.md` ou nova skill.
- Inverso do `repo-auditor` (skill #2): este **propõe mudanças concretas**.

---

## 5. diegosouzapw/OmniRoute — **Padrões reusáveis, não o gateway**

### O que ele faz de reusável
- **AGENTS.md como single source of truth** + `CLAUDE.md`/`GEMINI.md` só com deltas (`@AGENTS.md` import).
- **Skills geradas de fonte canônica** (`<!-- generated by ... -->`).
- **Quality gates automatizados** (lint+test+cycles+docs).
- **MCP server próprio** (110 tools).

### O que JÁ temos em agent-sync
- **AGENTS.md é a fonte** (já adotamos o padrão). Mas `~/.claude/AGENTS.md` e `~/.config/opencode/AGENTS.md` **duplicam** o do agent-sync, sem import.
- **5 schemas JSON** já são "fonte canônica" para os eventos. Mas as **SKILL.md não são geradas** — são editáveis manualmente.
- **Quality gates**: temos `go test ./...` no Makefile, mas **não temos gate para skills** (validar frontmatter, exemplos, etc).

### Gap real
1. **AGENTS.md duplicado** sem import: 5 cópias do mesmo arquivo, qualquer edição manual em um não propaga.
2. **Skills sem lint**: se alguém criar `skills/foo/SKILL.md` sem frontmatter, ninguém detecta.
3. **No `agent-sync doctor`**: não temos comando que valide **tudo** (manifest coerente, schemas válidos, hooks executáveis, plugins v2 carregam, AGENTS.md consistente entre CLIs).

### Adaptação proposta (3 entregas)

**A. Padronizar AGENTS.md entre CLIs** (~1 entrega pequena):
- Decidir: AGENTS.md do agent-sync vira fonte; em cada CLI wirar **symlink** ou usar `@imports/AGENTS.md` se a CLI suportar.
- Se não suportar, wirar via `agent-sync -apply` e detectar drift em `agent-sync doctor`.

**B. Skill lint** (subcommand + skill):
- `agent-sync skills lint` valida: frontmatter YAML válido, `name` bate com pasta, `description` tem gatilhos (regex simples), tem pelo menos 1 exemplo executável.
- Bloqueia commit via pre-commit hook (opcional).
- **Cuidado AGENTS.md §3**: não over-engineer — começar com 3-4 checks duros, evoluir com demanda.

**C. `agent-sync doctor`** (subcommand compound):
- Já temos `ctx-window doctor` interno. Falta um `agent-sync doctor` que rode: `skills index` + `skills lint` + schema validate + plugins v2 check + binários Go presentes.
- Custo: ~150 linhas em `cmd/agent-sync/doctor.go`. Reusa tudo que já existe.

---

## Resumo — priorização re-calculada para agent-sync

| # | Repo | Adaptação | Custo | ROI |
|---|---|---|---|---|
| 1 | cartographer | `repo-map-graph` (3 fases) — **começar pela Fase 1 (schema+CLI brief)** | M | **Alto** |
| 2 | OmniRoute | `agent-sync doctor` + `skills lint` + AGENTS.md padronizado | P-M | **Alto** |
| 3 | claude-mem | Hook `PreToolUse:Read` consultando memory-mcp por path | P | Médio |
| 4 | claude-code-setup | Skill `repo-auditor` | P | Médio |
| 5 | headroom | ADR-cache-aligner + skill `agent-learn` | P | Baixo-Médio |

**Sequência sugerida** (respeitando prioridade + dependência):
1. `agent-sync skills lint` (OmniRoute #5B) — bloqueia débitos técnicos, é fundamento pras próximas.
2. `agent-sync doctor` (OmniRoute #5C) — observa saúde do repo.
3. `repo-map-graph` Fase 1 (cartographer #1) — schema + CLI brief.
4. Hook `PreToolUse:Read` (claude-mem #3) — reusa memory-mcp existente.
5. Skill `repo-auditor` (claude-code-setup #2) + Skill `agent-learn` (headroom #4B) — fechamento do loop de auditoria.
6. ADR-cache-aligner (headroom #4A) — só doc, não código.

**Não fazer**:
- Copiar proxy do headroom (overlap total com `ctx-window`).
- Worker daemon do claude-mem (overlap com hook bash já BG).
- Chroma/embeddings (não temos demanda medida).
- Gateway multi-provider do OmniRoute (nada a ver com o escopo do agent-sync).

---

## Apêndice — mapeamento repo → arquivo existente

| Repo | Arquivo no repo fonte | Inspiração em agent-sync |
|---|---|---|
| cartographer | `src/code-graph/schema.ts` (Zod) | `tools/jsonschema/schemas/repo-graph.json` (proposto, Fase 1) |
| cartographer | `src/code-graph/brief.ts` | `tools/cmd/repo-map-graph/` (proposto) |
| claude-code-setup | `plugins/claude-code-setup/skills/claude-automation-recommender/SKILL.md` | `skills/repo-auditor/SKILL.md` (proposto) |
| claude-mem | `plugin/hooks/hooks.json` (PostToolUse, PreToolUse:Read) | `hooks/memory-pretooluse-read.sh` (proposto) |
| headroom | `plugins/opencode/` (hook-shim + provider) | já coberto por `tools/cmd/ctx-window/` |
| headroom | `crates/headroom-core/src/cache_aligner.rs` | `docs/ADR-cache-aligner.md` (proposto) |
| OmniRoute | `AGENTS.md` (single source + @import) | `~/.config/opencode/AGENTS.md` vira symlink (proposto) |
| OmniRoute | `src/lib/agentSkills/generator.ts` | `tools/cmd/skills-lint/` (proposto) |
| OmniRoute | `Makefile` (check, check:cycles, check:docs-all) | `agent-sync doctor` (proposto) |
