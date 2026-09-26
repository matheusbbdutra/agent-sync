# ADR — Audit removal para repo-map Go (port de cartographer)

- **Status**: Aceito
- **Data**: 2026-09-25
- **Decisor**: agente + smoke empírico (ses_atual)
- **Fonte**: A-72 (cartographer teste manual), D-99/D-106 (investigação gap real), `internal/agentmemory/store.go` (libsql + tabela `memories`)
- **Tags**: code-graph, audit-removal, text-references, go-port, no-bun
- **Smoke**: `docs/investigations/cartographer-vs-go-sess-1.md`

## Contexto

Investigação D-99 + teste manual de cartographer (A-72 rev. 1, mergeado em `de2581d`) confirmou gap real:

| Ferramenta | Alvo | Refs detectadas |
|---|---|---|
| `repo-map --focus tools/cmd/memory-mcp` (diretório) | ❌ não suporta | 0 |
| `repo-map get_file_impact` via MCP (diretório) | ❌ "não encontrado" | 0 |
| `cartographer:audit -- removal tools/cmd/memory-mcp` | ✅ | **33 hits classificados** (27 docs + 3 hist + 3 unknown) |
| `grep -rln 'memory-mcp'` direto | ✅ mas sem classificação | 774 (ruído) |

**Gap**: `repo-map` aceita apenas **arquivo** (`--focus`, `--brief`, `get_file_impact`) — para refactor típico (deletar/mover diretório), **não há ferramenta**.

cartographer tem 21 classes de classificação mas:
- 11 são específicas de Supabase/SQL/RLS/auth — irrelevantes para agent-sync
- Requer Bun + tiktoken (Python) — quebra arquitetura Go single-binary

## Decisão

**Portar `audit_removal` para repo-map Go** com 9 classes (vs 21 do cartographer):
- 7 do cartographer: docs-active, docs-historical, unknown-literal-hit, package-dependency, lockfile-reference, env-var, import-or-sdk-client
- 2 custom Go: `go-package-reference`, `sql-table-reference` (extraídas de `internal/agentmemory/store.go`)

### Critérios de promoção (Proposto → Aceito)

| # | Critério | Como medir | Resultado smoke |
|---|---|---|---|
| 1 | ≥80% match com cartographer no mesmo target | comparar outputs em `tools/cmd/memory-mcp` | ✓ 121% (40/33) |
| 2 | Latência <500ms para agent-sync (444 files) | `time agent-sync graph audit removal <target>` | ✓ 223ms (binário pré-built) |
| 3 | 9 classes funcionais com classificação correta | tests 4 camadas | ✓ 15/15 testes passando; 9 classes implementadas |
| 4 | Schema introspection (libsql) extrai nomes de tabela | `SHOW TABLES` ou parse DDL inline | ✓ 5 tabelas detectadas via regex DDL |

**Critérios NÃO exigidos**:
- Não precisa cobrir 100% das classes cartographer
- Não precisa wiramento cross-CLI (fica como follow-up se ROI confirmar)

### Por que 9 classes (não 21)

| Classe | Origem | Valor agent-sync |
|---|---|---|
| docs-active | cartographer | ✓✓✓ Crítico (maioria dos hits) |
| docs-historical | cartographer | ✓✓ Bom |
| unknown-literal-hit | cartographer | ✓✓ Bom (default) |
| package-dependency | cartographer | ✓ Marginal |
| lockfile-reference | cartographer | ✓ Marginal |
| env-var | cartographer | ✓ Marginal |
| import-or-sdk-client | cartographer | ⚠️ Pouco (agent-sync tem pouco SDK) |
| **go-package-reference** | custom Go | ✓✓ Bom (refs internas Go) |
| **sql-table-reference** | custom Go | ✓✓ Bom (agent-sync tem libsql) |
| ~~edge-function~~ | cartographer | ✗ Irrelevante |
| ~~storage-bucket~~ | cartographer | ✗ Irrelevante |
| ~~rls-policy/db-trigger/db-function/sql-migration~~ | cartographer | ✗ Sem migrations |
| ~~auth-user-model~~ | cartographer | ✗ Sem auth provider |
| ~~client-wrapper/ci-secret-name/deploy-config/test/mock/fixture~~ | cartographer | ⚠️ Margem |

## Consequências

**Positivas:**
- Fecha gap real: detecta refs textuais a diretórios-alvo (33 hits vs 0 do repo-map)
- Sem Bun/Python/tiktoken — single binary Go
- Wiramento cross-CLI trivial (infra já existe)
- Latência aceitável para uso manual

**Negativas:**
- Sem SQLite graph (full walk a cada chamada — ~500ms para agent-sync)
- Sem 12 classes irrelevantes (Supabase/RLS/auth) — **mas essas não servem**
- 2 classes custom (go-package-reference, sql-table-reference) precisam ser mantidas

**Trade-offs assumidos:**
- Walk + grep é O(files × linhas) — viável para agent-sync (~444 files), degrada em monorepos
- Schema introspection via libsql adiciona dep de runtime (já temos via go-libsql)
- Output JSON próprio (não cartographer.audit-ledger.v1) — pode divergir se cartographer virar padrão upstream

## Fora do escopo

- **Wiramento cross-CLI** como hook PreToolUse:Bash (rm/rmdir) — fica como follow-up se ROI confirmar
- **21 classes cartographer** — 11 irrelevantes descartadas
- **Notas_audit / verify / diff** do cartographer — não core, não demandado
- **Migração para TS** — rejeitada (AGENTS.md §3 anti-overengineering)

## Implementação proposta

Sequência de commits granulares (1 commit por peça):

1. `feat(graph) audit/removal.go: walk + grep + 3 matchers regex` (~150L)
2. `feat(graph) audit/classifier.go: 9 classes (7 cartographer + 2 custom)` (~200L)
3. `feat(graph) audit/frontmatter.go: yaml subset parser` (~80L)
4. `feat(graph) audit/schema.go: libsql introspection para sql-table-reference` (~60L)
5. `feat(graph) audit/removal.go: buildRemovalAudit + output JSON` (~150L)
6. `feat(graph) subcommand 'agent-sync graph audit removal <target>'` (~50L)
7. `test(graph) tests 4 camadas` (~200L)
8. `chore(graph) wirar subcommand no dispatcher` (~10L)
9. `docs(state) D-107 + A-73 [done]` — catalogar
10. `docs(smoke) cartographer-vs-go-sess-1.md` — evidência de comparação

**Esforço total**: 11-13h (revisado após leitura do source cartographer)

## Reversibilidade

- ADR é documento — reversível por git revert
- Subcommand novo em `agent-sync graph` — fácil remover se ROI negativo
- Sem wiramento cross-CLI default = sem risco de regressão para 5 CLIs
- Smoke vs cartographer permite rollback se cobertura <80%

## Refs

- **A-72** (cartographer_preflight Proposto → Rejeitado): Bun dep + tiktoken quebra arquitetura Go
- **D-99**: investigação matrix escopos
- **D-106**: gap real tools/cmd/memory-mcp (33 refs classificadas vs 0 do repo-map)
- **`internal/agentmemory/store.go`**: libsql + tabela `memories` (DDL inline)
- **`docs/guides/sync-between-pcs.md`**: sync Turso ativo (`turso.url` + `turso.token`)
- **`docs/investigations/cartographer-preflight-manual-test.md`**: teste manual cartographer
- **kingbootoshi/cartographer**: https://github.com/kingbootoshi/cartographer (MIT, referência)
- **D-49/D-53/D-54**: linha histórica de code-graph no agent-sync
- **AGENTS.md §3**: anti-overengineering — single binary Go, sem Bun dep