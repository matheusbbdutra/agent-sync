# ADR: Code Graph Bounded e Contexto Estruturado Pré-Edição (Cartographer / Graphify)

> ⚠️ **MOVIDO PARA ARCHIVE em 2026-09-25** (D-101). Esta ADR foi **Descartada** por D-64 (2026-09-23) e **subsumida** por D-49/D-53/D-54/D-57 que entregaram `repo-map --mcp` (servidor MCP wirado em 5 CLIs). **Não implementar a partir deste arquivo** — usar docs/ADR-memory-scope-matrix.md e o material em `tools/cmd/repo-map/` como fonte atual.

- **Status**: Descartado (subsumido por D-49/D-53/D-54/D-57; D-64 em 2026-09-23)
- **Data**: 2026-09-23
- **Decisor**: Matheus Dutra
- **Tags**: code-graph, context-management, ast, token-optimization, preflight, cross-cli

---

## Contexto

O repositório `agent-sync` implementa inspeções de baixo custo de tokens via `ast-outline` e `docs-fetch`, além de heurísticas de compactação (`ctx-window-strategy`). No entanto, análises empíricas de 344 sessões reais (`~/.analysis/INVENTARIO.md` e `REPORT.md`) demonstraram que:

1. **Leitura não-estruturada consome janela desnecessária:** Agentes realizam chamadas sucessivas de `read` e `grep` para descobrir símbolos, imports e tabelas, estourando o orçamento de tokens antes de editar.
2. **Ausência de Preflight determinístico:** Edições (`edit`/`write`) ocorrem sem validação prévia de impacto estrutural ou checagem de referências cruzadas.
3. **Lacuna preenchida pelo ecossistema:** Projetos como `kingbootoshi/cartographer` (v2 CLI + SQLite tipado com Zod) e `Graphify-Labs/graphify` (mapeamento de grafos de chamadas e dependências) demonstram que passar ao LLM um **brief delimitado por orçamento de tokens** (`brief --path <file> --tokens <limit>`) aumenta drasticamente a precisão da edição com fração do custo de contexto.

---

## Decisão

Adotar uma arquitetura de **Code Graph Bounded** no `agent-sync` com os seguintes pilares:

### 1. Modelo de Grafo em SQLite com Esquema Tipado
- Manter índice local (`.agent-sync/graph.sqlite`) gerado a partir de análise estática.
- Esquema fechado cobrindo nós essenciais: `File`, `Package`, `Symbol` (função, classe, interface), `DbTable`, `Migration`, `EnvVar` e arestas estruturadas (`CONTAINS`, `IMPORTS`, `CALLS`, `QUERIES_TABLE`, `DEPENDS_ON`).
- Cada nó e aresta registra proveniência auditável com número de linha (`line`) e hash do arquivo para invalidação incremental.

### 2. Ferramenta de Contexto Delimitado (`brief`)
- Expor via CLI/MCP uma ferramenta que compõe um resumo delimitado em tokens:
  ```bash
  agent-sync graph brief --path <caminho> --max-tokens 1500 --mode <planning|implementation|review>
  ```
- O brief prioriza: (a) símbolos e assinaturas do arquivo alvo, (b) dependências diretas importadas, (c) referências de banco de dados/migrations relacionadas, e (d) comandos de teste associados.

### 3. Preflight Hook Pré-Edição (`PreToolUse:edit|write`)
- Validar se o arquivo a ser editado possui impacto em outros módulos ou contratos de banco conhecidos.
- Se o agente tentar editar sem antes consultar a estrutura ou preflight, emitir aviso/nudge contextual.

### 4. Matriz de Cobertura Cross-CLI (5xN)

| CLI | Estado | Mecanismo de Integração |
|---|---|---|
| **Claude Code** | ✅ Coberto | Hook `PreToolUse` invocando `agent-sync graph preflight` + MCP server `cartographer`/`agent-sync-graph`. |
| **OpenCode v2** | ✅ Coberto | Plugin `ctx.tool.hook('execute.before')` com injeção de brief estruturado via `ctx.session.hook('context')`. |
| **Codex** | 🟡 Contornável | Wrapper de comando e MCP stdio configurado nas tools locais. |
| **Antigravity** | 🟡 Contornável | MCP server local registrado no catálogo de MCPs do agente. |
| **Cursor** | 🟡 Contornável | Ferramenta exposta via `~/.cursor/mcp.json` e rules de pré-leitura estruturada. |

---

## Status Descartado (2026-09-23, auditoria D-64)

Este ADR é **descartado** porque foi **subsumido** pelas decisões D-49, D-53, D-54 e D-57 (todas no STATE.md), que evoluíram o conceito de Code Graph para a entrega concreta `repo-map --mcp` — divergindo do desenho original deste ADR (`agent-sync graph brief --path --max-tokens`):

- **D-49** (STATE.md:64): decisão original do Code Graph em SQLite + brief delimitado por tokens + preflight hook pré-edição.
- **D-53** (STATE.md:69): expansão do Code Graph — Fase 2 extratores relacionais DB/Env + blast radius + vínculo código-teste no `repo-map`. Tarefas A-43 (Fase 2) + A-44 (MCP server).
- **D-54** (STATE.md:70): **A-44 concluída** — Servidor MCP Code Graph (`repo-map --mcp`) implementado e integrado cross-CLI com 3 ferramentas: `get_file_impact` (blast radius, callers, tabelas DB, env vars, comando de teste), `get_symbol_callers` (rastreio reverso), `repo_summary` (hubs centrais). Testes verdes em `tools/cmd/repo-map/mcp_test.go`. Smoke via JSON-RPC stdio validado. `scripts/setup-mcp.sh` registra o MCP nas 5 CLIs.
- **D-57** (STATE.md:73): README sincronizado com a estrutura modular, documentando `repo-map`, o servidor MCP code-graph e os novos skills.

**Por que descartar (não re-propor):** o `brief --path --max-tokens` deste ADR previa uma CLI/MCP nova (`agent-sync graph brief`); a entrega real virou `repo-map --mcp` com 3 tools já wiradas nas 5 CLIs e testadas. Manter este ADR como Re-Proposto duplicaria a fonte de verdade e confundiria auditorias futuras.

**Anti-regressão:** se um dia o formato de brief delimitado por tokens for reintroduzido, criar ADR nova (não reviver este). O histórico permanece em `git log --follow docs/ADR-code-graph-bounded-context.md`.

## Consequências

**Positivas:**
- Redução de até 80% das chamadas exaustivas de `read` e `grep` durante investigação de dependências.
- Prevenção de quebras silenciosas em bancos de dados e interfaces externas antes da escrita.
- Integração harmoniosa com os binários existentes em Go (`ast-outline`).

**Negativas / Trade-offs:**
- Necessidade de manter cache incremental atualizado (invalidação por mtime/sha256).
- Custo de execução inicial de indexação na abertura de repositórios grandes.
