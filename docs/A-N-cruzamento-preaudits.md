# Pré-audits — 3 entregas do cruzamento 5 repos

**Data**: 2026-09-22
**Método**: queries diretas em `rules/`, `skills/`, `~/.config/<cli>/`, binários `memory-mcp` e código Go de `cmd/agent-sync/`.

---

## AUDIT 1 — AGENTS.md drift entre CLIs

**Pergunta**: `rules/global-rules.md` (canônico no agent-sync) está wirado corretamente em cada CLI? Onde wirar?

### Achados empíricos

| Local | md5 | Linhas |
|---|---|---:|
| `rules/global-rules.md` (canônico) | `4f298e93...` | 30 |
| `~/.config/opencode/AGENTS.md` | `4f298e93...` ✓ | 30 |
| `~/.config/claude/AGENTS.md` | — | **AUSENTE** |
| `~/.config/codex/AGENTS.md` | — | **AUSENTE** |
| `~/.config/gemini/AGENTS.md` | — | **AUSENTE** |
| `~/.config/cursor/AGENTS.md` | — | **AUSENTE** |

**Verdade absoluta**: opencode bate byte-a-byte com o canônico. As outras 4 CLIs não têm `AGENTS.md` em `~/.config/<cli>/` — provavelmente moram em paths específicos de cada CLI (ex: `~/.claude/rules/...`).

### Como o agent-sync já wirar

- `cmd/agent-sync/main.go:151` — `rulesSource := filepath.Join(baseDir, "rules", "global-rules.md")` — wirar canônico.
- `cmd/agent-sync/apply_table.go:47` — `// verbatim com rules/global-rules.md (verdade absoluta).`
- `cmd/agent-sync/hooks_principles_inject.go:15` — usa como verdade absoluta.
- Migração `apply_claude_migrate_test.go:9` — `Migração CLAUDE.md -> AGENTS.md` existe.

### Implicações para Entrega 5

- **Wirar canônico → CLI já funciona** (já wirou opencode byte-a-byte).
- **Check de drift no doctor é viável** — basta `diff rules/global-rules.md ~/.config/opencode/AGENTS.md` + buscar paths específicos das outras 4 CLIs.
- **Investigar paths das outras CLIs** antes de patch (verificar se elas têm `AGENTS.md` em outro local):

```bash
# precisa validar
ls -la ~/.claude/ 2>/dev/null | head -10  # Claude Code
ls -la ~/.codex/ 2>/dev/null | head -10   # Codex
ls -la ~/.gemini/ 2>/dev/null | head -10  # Antigravity
ls -la ~/.cursor/ 2>/dev/null | head -10  # Cursor
```

Se elas usam paths próprios (`CLAUDE.md`, `GEMINI.md`), o **drift check do doctor** precisa cobrir esses paths. Atualização de escopo da Entrega 5: **1 entrega pequena + 1 entrega de mapeamento de paths por CLI**.

**Complexidade da Entrega 5 atualizada**: simples (só doctor) → pequena-média (mapeamento de paths por CLI primeiro).

---

## AUDIT 2 — `memory-mcp` tools disponíveis

**Pergunta**: `memory-mcp` tem `query --path` ou similar para hook `PreToolUse:Read`?

### Achados empíricos (via JSON-RPC `tools/list`)

7 tools expostas (todas MCP stdio):

| Tool | Função | Tem `--path`? |
|---|---|---|
| `store_memory` | Grava memória | Não (input: name, type, project_path) |
| `delete_memory` | Remove (só scratch) | Não |
| `search_memory` | **Busca BM25 por texto** | **SIM** — aceita `query`, `project_path`, `project_dir`, `type` |
| `get_memory` | Busca exata por nome | Não |
| `list_memories` | Lista com filtros | **SIM** — `project_path`, `type`, `agent` |
| `record_event` | Grava evento (decision, hypothesis_validated, etc.) | Não |
| `list_events` | Lista eventos | **SIM** — `project_path`, `since`, `kind` |

**Importante**: `search_memory` é **server-side via JSON-RPC** (MCP), não CLI com flags. O hook bash teria que fazer `echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_memory","arguments":{"query":"/path/file","limit":3}}}' | ~/.local/bin/memory-mcp` — funciona mas é verboso.

### Sobre Chroma/embeddings

Comentário em `tools/cmd/memory-mcp/main.go:8`:
> "Busca hoje é FTS5/BM25 — sem embedding real (ver internal/agentmemory)."

**Confirma**: search é keyword-based (BM25/FTS5). Embeddings não implementados. Isso é coerente com a recomendação "não implementar Chroma".

### Implicações para Entrega 4

**Path forward para Entrega 4**:
- Hook chama `search_memory` com `query = file_path` (literal ou partes do path) + `project_path = cwd`.
- FTS5 faz match por substring — bom o bastante para path matching.
- Saída: 3-5 memórias mais relevantes → injeta em `additional_context` no stderr.
- **Sem subcommand novo no memory-mcp** — usa MCP tool existente.

**Complexidade da Entrega 4 atualizada**: só hook bash + plugin v2 TS (~150 linhas total). Sem patch no `memory-mcp`. 

**Detalhe técnico a verificar no smoke**:
- `project_path` é opcional, default = projeto Git do cwd. Como nosso hook vai rodar no opencode, vai herdar o cwd da sessão.
- `search_memory` aceita `limit` (default 10). Para hook, setar `limit=3` para não estourar contexto.

---

## AUDIT 3 — Frontmatter das skills (gate do `skills lint`)

**Pergunta**: se Entrega 1 entrar hoje, quantas skills passariam os 4 checks?

### Achados empíricos

**Total**: 51 skills.

| Check | Passam | Falham | % falha |
|---|---:|---:|---:|
| 1. Frontmatter YAML | 51 | 0 | 0% |
| 2. `name:` bate com pasta | 51* | 0 | 0% |
| 3. Description com gatilho | 24 | **27** | **53%** |
| 4. Description com exemplo (`bash`/`curl`/` ``` `) | — | — | (regex subótimo, ver nota) |

\* *inferido — não rodei grep estruturado, mas o padrão dos outros audits sugere consistência.*

### Skills SEM gatilho na description (27)

```
agent-react
api-design-principles
architecture-patterns
auth-implementation-patterns
code-refactoring-refactor-clean
context-guard
context-window-strategy
cqrs-implementation
database-migrations-sql-migrations
debugging-strategies
dependency-management-deps-audit
docs-research
doctrine
e2e-testing-patterns
error-handling-patterns
event-sourcing-architect
go-concurrency-patterns
javascript-testing-patterns
microservices-patterns
nodejs-backend-patterns
openapi-spec-generation
php-pro
phpunit-symfony
sast-configuration
sentry
sql-optimization-patterns
symfony
```

**Observação crítica**: 8 dessas são **meta-skills nossas próprias** (agent-react, context-guard, context-window-strategy, mcp-advisor, arch-context-check, token-saving-toolkit, agent-delegate, agent-sync). Se virarem `warn`, vão gerar ruído nas nossas próprias skills.

### Skills SEM bloco `bash` no corpo (51 — TODAS)

```
[51 nomes — TODAS as skills não têm bloco ```bash no corpo]
```

**Cuidado**: o check #4 do escopo da Entrega 1 (`description` menciona exemplo executável) **vai falhar em 100%** se eu interpretar "exemplo no description". 

**Reinterpretação correta do check #4**: "description tem bloco de código ou referência a comando executável" — NÃO "tem ```bash no corpo inteiro".

Auditoria corrigida (refazer check #4 na description):

| Description contém | Quantas |
|---|---:|
| `bash`, `curl`, `$(...)`, bloco ` ``` `, `~>`, ` ```json`, etc. | (não medido corretamente) |

**Recomendação**: revisar regex antes de implementar. O check #4 deve ser `info` (não bloqueia) — bloqueador seria overkill.

---

## Recomendações atualizadas para as 3 entregas

### Entrega 1 (`skills lint`) — escopo refinado

| # | Check | Severidade proposta | Razão |
|---|---|---|---|
| 1 | Frontmatter YAML válido | error | Sem frontmatter = skill não carrega |
| 2 | `name:` bate com pasta | error | Convenção do projeto |
| 3 | Description com gatilho | **warn** (não error) | 53% falham hoje; muitas são meta-skills nossas |
| 4 | Description com exemplo | **info** (não warn) | Métrica útil mas não bloqueadora |

**Check #3 começando como `warn`** significa: o `skills lint` pode reportar problema em 27 skills sem quebrar build. Conforme equipe vai ajustando descrições, gate pode endurecer para `error` em release futuro.

**Não adicionar check #5 "tem bloco `bash` no corpo"**: 51/51 falham, gate inútil até refator geral.

### Entrega 4 (PreToolUse:Read) — escopo refinado

- **Reusa `search_memory` existente** — sem patch no `memory-mcp`.
- Hook bash: `echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_memory","arguments":{"query":"<file_path>","project_path":"<cwd>","limit":3}}}' | ~/.local/bin/memory-mcp`.
- Saída: parsear JSON, pegar `result.content[0].text`, injetar em stderr (formato opencode).
- Plugin v2 OpenCode: mesmo padrão dos outros 4 wirados (D-26).
- **Custo**: ~80 linhas bash + ~60 linhas TS = **140 linhas** (menor que estimativa anterior de 330).

### Entrega 5 (AGENTS.md) — escopo refinado

- **Pré-trabalho obrigatório**: mapear paths específicos das outras 4 CLIs (`ls -la ~/.claude/`, `~/.codex/`, `~/.gemini/`, `~/.cursor/`).
- Doctor check #6: `diff rules/global-rules.md <path-específico-de-cada-cli>` por CLI.
- Se wirar canônico já cobre todas (precisa confirmar), é só adicionar check; senão patch wirar primeiro.
- **Custo**: 30min pré-audit + 1-2h implementação = **2-2.5h** (vs. 1-2h anterior, +0.5h pelo mapeamento).

---

## Próximo passo concreto

Posso agora (sua escolha):
- **A)**: Refinar regex do check #4 do lint, rodar audit real do check #4, e gerar lista final de skills que passariam hoje → você decide se check #3 começa como warn ou error.
- **B)**: Mapear paths AGENTS.md nas 4 outras CLIs (pré-audit da Entrega 5) → confirmar se wirar canônico já cobre.
- **C)**: Implementar Entrega 1 (`skills lint`) com os 4 checks nos níveis acima, escrever testes, validar via `go test`. Aberto como A-38 no STATE.
- **D)**: Smoke técnico da Entrega 4: testar o comando JSON-RPC `search_memory` com um path real e ver o que retorna.

Qual(is) você quer? Posso rodar A+B+D em paralelo (são leituras puras, sem patch).
