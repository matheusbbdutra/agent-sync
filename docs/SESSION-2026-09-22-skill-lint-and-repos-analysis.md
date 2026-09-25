# Sessão 2026-09-22 — Skill lint + AGENTS.md canônico + cruzamento com 5 repos

> **Modo**: context-guard (zona 🟢 → amarela com checkpoint) + agent-react.
> **Resumo compacto**: gerado via `ctx-window summarize -cli opencode -timeout 2m <session>` (versão 1, 5 turnos). Próxima sessão retoma por aqui.

---

## 1. Commits entregues (8)

```
6772148 feat(skills): subcommand 'agent-sync skills lint' (gate real, 3 errors duros)
b469bf6 fix(skills): adicionar gatilho em architect-review - ultima sem gate
4eb1caf fix(skills): adicionar gatilho 'Use when' em skills de linguagens (12)
ff57005 fix(skills): adicionar gatilho 'Use when' em skills de backend/web/db/security (20)
949e5e5 fix(skills): adicionar gatilho 'Use when' em skills de arquitetura (7)
e49ea0a fix(skills): adicionar gatilho 'Use when' em skills de disciplina (3)
e2ce4db fix(target): Antigravity wirar AGENTS.md (estava wirar GEMINI.md)
96a11a9 refactor(rules): AGENTS.md como padrao canonico em todas as 5 CLIs
```

## 2. Estado do projeto antes vs depois

### Antes
- 240 skills + 214 agents wiradas no opencode, divergentes do catálogo curado (51+12).
- Apenas opencode tinha `AGENTS.md` wirado (1/5 CLIs).
- Gemini tinha `GEMINI.md` legado (3/5 sem padronização).
- Cursor usava `.mdc` em `~/.cursor/rules/` (formato próprio).
- **Nenhum gate** para validar SKILL.md.
- 27 skills sem gatilho no description → gate de qualidade seria inútil.

### Depois
- 51 skills + 12 agents wiradas no opencode, todas do catálogo agent-sync.
- 5/5 CLIs com `AGENTS.md` wirado (md5 `4f298e93...` idêntico em todas).
- 3 legados removidos (`CLAUDE.md`, `GEMINI.md`, `.mdc`).
- **`agent-sync skills lint`** como gate real (3 errors duros, exit 1).
- 51/51 skills cumprem os 3 checks.

## 3. Decisões aplicadas

| # | Decisão | Como |
|---|---|---|
| 1 | AGENTS.md é padrão universal | `target.go` de todas as 5 CLIs aponta para `AGENTS.md` |
| 2 | Migração automática do legado | `removeLegacyClaudeRules` + `removeLegacyGeminiRules` + `removeLegacyCursorRules` |
| 3 | Gate de qualidade é honesto | 3 errors duros (frontmatter, name, gatilho). Sem warn permanente que vira ruído |
| 4 | Lint nasce passando | Patches em 4 commits granulares corrigem as 39 skills sem gatilho ANTES de habilitar o gate |
| 5 | Cursor não é caso especial | Removido `syncCursorRules` + `cursorRulesFrontmatter`. `applyRunner` agora usa `copyFile` direto |

## 4. Arquivos novos / modificados

**Novos**:
- `cmd/agent-sync/skills_lint.go` (207 linhas) — subcommand + 3 checks + 1 regex de gatilho
- `cmd/agent-sync/skills_lint_test.go` (125 linhas) — 7 testes
- `docs/A-N-cruzamento-escopo.md` — escopo detalhado das 6 entregas do cruzamento 5 repos
- `docs/A-N-cruzamento-preaudits.md` — pré-audits 1 (3 audits)
- `docs/A-N-cruzamento-preaudits-2.md` — pré-audits 2 (3 audits: lint check, paths reais, smoke search_memory)
- `docs/repos-cruzamento.md` — cruzamento dos 5 repos × agent-sync (relatório)

**Modificados** (8 commits acima): `target.go`, `apply_fs.go`, `apply_runner.go`, `hooks_cursor_apply.go`, `hooks_cursor_apply_test.go`, `skills_index.go`, 39 SKILL.md.

## 5. Validação empírica

| Métrica | Antes | Depois |
|---|---:|---:|
| Skills wiradas no opencode | 240 | **51** |
| Agents wirados no opencode | 214 | **12** |
| Skills movidas para `disabled/` | 0 | **189** |
| Agents movidos para `disabled/` | 0 | **202** |
| CLIs com AGENTS.md wirado | 1/5 | **5/5** |
| Legados de regras | 3 | **0** |
| Skills com gatilho no description | ~12/51 | **51/51** |
| `agent-sync skills lint` errors | n/a | **0** |
| Testes Go | verdes | verdes (7 novos) |

---

# Análise consolidada dos 5 repos × o que aproveitar

> **Origem**: cruzamento inicial feito em `.analysis/REPORT.md` + refinamento após auditoria. Foco: **o que adaptamos ao agent-sync**, não ao setup geral.

## Repositórios analisados

1. ~~cartographer-project/cartographer~~ (SLAM — descartado, sem intersecção)
2. **kingbootoshi/cartographer** (CLI + plugin de mapa de codebase)
3. **anthropics/claude-plugins-official** → `claude-code-setup` (skill meta-auditor)
4. **thedotmack/claude-mem** (memória comprimida observacional)
5. **headroomlabs-ai/headroom** (compressão de tokens)
6. **diegosouzapw/OmniRoute** (gateway multi-provider, padrões reusáveis)

## Padrões reusáveis — priorização para próxima sessão

| # | Repo | Adaptação ao agent-sync | Status | Custo | ROI |
|---|---|---|---|---|---|
| 1 | **kingbootoshi/cartographer** | `tools/cmd/repo-map-graph/` Fase 1 (schema Zod-like + CLI brief bounded) | Não iniciado | M | **Alto** |
| 2 | **OmniRoute** | `agent-sync doctor` (6 checks compostos: lint + index + schemas + plugins + binários + AGENTS.md) | Não iniciado, deps do #1 acima | M | **Alto** |
| 3 | **claude-mem** | Hook `PreToolUse:Read` consultando memory-mcp por path (FTS5 keyword-based) | Não iniciado, audit feito | P-M | Médio |
| 4 | **claude-code-setup** | Skill authored `repo-auditor` (meta-auditoria do repo agent-sync) | Não iniciado, deps de #1 e #2 | P | Médio |
| 5 | **headroom** | ADR `cache-aligner-risco.md` (documenta risco, não implementa) + skill `agent-learn` | ADR pode ser feito já | P | Baixo-Médio |

## Por que essa ordem

1. **`repo-map-graph` primeiro**: ataca nossa deficiência #1 — contexto curado bounded antes de edits. O cartographer é o único dos 5 com essa capacidade; o que temos (`tools/cmd/repo-map/`, 174 linhas) é só indexação rasa.
2. **`doctor` segundo**: основа-se no lint que acabamos de implementar. Reusa tudo o que já temos — baixo risco, alto ganho de observabilidade.
3. **`PreToolUse:Read` terceiro**: audit feito, sem patch no memory-mcp necessário. JSON-RPC via bash hook + plugin v2 OpenCode.
4. **`repo-auditor` quarto**: depende de 1 e 2 funcionando. Skill meta que fecha o loop de auditoria.
5. **ADR cache-aligner + `agent-learn`**: baixa prioridade, podem ser feitos em paralelo com qualquer outra entrega.

## O que NÃO aproveitar

- ❌ **Proxy do headroom** → `tools/cmd/ctx-window/` (2756 linhas) já cobre gerenciamento de janela.
- ❌ **Worker daemon do claude-mem** → hooks bash já rodam em BG=1 (padrão D-31).
- ❌ **Chroma/embeddings** → `search_memory` já usa FTS5/BM25 (verificado em `memory-mcp/main.go:8`). Sem demanda medida.
- ❌ **Gateway multi-provider do OmniRoute** → fora do escopo do agent-sync (somos setup, não gateway).
- ❌ **AGENTS.md como symlink** → algumas CLIs não suportam; wirar via copyFile e detectar drift com `doctor`.

## Detalhamento do escopo de cada uma (já em `docs/A-N-cruzamento-escopo.md`)

### Entrega 1 — `repo-map-graph` Fase 1 (cartographer)

**Arquivos** (4 novos):
- `tools/jsonschema/schemas/repo-graph.json` — 5 nós × 4 edges (File, Symbol, Package, Import, DbTable + CONTAINS, DEFINES, IMPORTS, QUERIES) com provenance obrigatória.
- `tools/cmd/repo-map-graph/main.go` — entrypoint CLI.
- `tools/cmd/repo-map-graph/build.go` — extração (reusa `tools/internal/astoutline`).
- `tools/cmd/repo-map-graph/brief.go` — renderiza brief bounded por `--budget-tokens`.

**3 subcommands**:
- `repo-map-graph build --root <dir> --out <file.json>`
- `repo-map-graph brief --in <file.json> --path <file> --budget-tokens 2000`
- `repo-map-graph query --in <file.json> --kind imports --for <file>`

**Não fazer (Fase 1)**: MCP server (Fase 2), adoption score (Fase 3), SQL/IaC/CI/env-var extractors (deixar pra depois).

**Custo**: ~350 linhas Go + ~200 linhas test.

### Entrega 2 — `agent-sync doctor` (OmniRoute)

**Arquivos** (2 novos):
- `cmd/agent-sync/doctor.go` — `runDoctor(baseDir)` agregando 6 checks.
- `cmd/agent-sync/doctor_test.go` — 5 testes isolados.

**6 checks compostos** (todos reusando subsistemas existentes):
1. `skills index` retorna `installed = total, missing = 0, orphan = 0` (já temos).
2. `skills lint` retorna `errors = 0` (acabamos de implementar).
3. Todos os 5 schemas JSON validam contra seu próprio JSON Schema (lib `tools/internal/jsonschema`).
4. Todos os 10 plugins OpenCode wirados existem em `~/.config/opencode/plugins/` e carregam.
5. Binários Go em `~/.local/bin/` estão atualizados (mtime ≥ source).
6. `AGENTS.md` wirado em cada CLI bate com o canônico.

**Não fazer**: sistema de plugins pra extensibilidade, rede, auto-fix.

**Custo**: ~180 linhas Go + ~120 linhas test.

### Entrega 3 — Hook `PreToolUse:Read` + memory-mcp (claude-mem)

**Audit feito** (ver `docs/A-N-cruzamento-preaudits-2.md`):
- `search_memory` aceita `query`, `project_path`, `limit`.
- FTS5/BM25 confirmado (sem embeddings).
- **Achado crítico**: FTS5 não indexa paths — hook precisa **extrair keywords do path** (basename + tokens antes do `/`).
- Path literal retorna 0 memórias; keyword matching retorna 2-3.

**Arquivos** (2 novos):
- `hooks/memory-pretooluse-read.sh` (~40 linhas, padrão `token-nudge.check.sh`).
- `hooks/memory-pretooluse-read.opencode.v2.ts` (~80 linhas, padrão D-26 dos plugins v2).

**Wirar**: patch em `cmd/agent-sync/hooks.go` adiciona `memory-pretooluse-read` ao `standardHooks`.

**Schema**: bump `session-event.json` 1.1 → 1.2, adiciona kind `memory_pretooluse_inject` (padrão A-26).

**Custo real**: ~140 linhas (sem patch no memory-mcp — usa tool MCP existente).

### Entrega 4 — Skill `repo-auditor` (claude-code-setup)

**Arquivo** (1 novo): `skills/repo-auditor/SKILL.md` (~80 linhas).

**Função**: read-only, roda `agent-sync doctor` + `skills lint` + `skills index -orphans` + opcionalmente `git log --oneline -30`. Emite 1-3 recomendações curtas priorizadas.

**Não faz**: editar arquivos, commitar, invocar subagents que modificam.

**Wirar**: automático via `-apply` (skill em `skills/` é detectada por `apply_fs.go:59`).

**Custo**: ~80 linhas Markdown.

### Entrega 5 — Skills `repo-auditor` + `agent-learn` (claude-code-setup + headroom)

Skill `agent-learn` (1 novo): `skills/agent-learn/SKILL.md` (~70 linhas). Lê `.agent-sync/session-event.jsonl` (últimas 100) + `STATE.md`. Identifica padrões de erro (loops, hipóteses refutadas, comandos destrutivos bloqueados). Propõe bullet em `global-rules.md` ou nova skill. **Não aplica sozinho** — pede confirmação.

**Custo**: ~70 linhas Markdown.

### Entrega extra — ADR `cache-aligner-risco.md` (headroom)

**Arquivo** (1 novo): `docs/ADR-cache-aligner-risco.md` (~50 linhas).

Documenta o risco de compressão invalidar prefix-cache do provider. Decisão: NÃO comprimir prompts (ctx-window já faz sliding window sem reescrita). Trigger para reabrir: ctx-window virar gargalo medido, ou headroom publicar benchmark com hit-rate > 95%.

**Custo**: 30 min, 1 arquivo.

---

# Próxima sessão — checklist de retomada

1. **Ler este arquivo primeiro** (`docs/SESSION-2026-09-22-skill-lint-and-repos-analysis.md`).
2. **Rodar `agent-sync skills lint`** — deve retornar 0 errors. Se retornar, algo regrediu.
3. **Decidir qual entrega abrir** — sugestão: Entrega 2 (`doctor`) já que dependemos só do lint (pronto) e do index (pronto).
4. **Não duplicar trabalho** — `docs/A-N-cruzamento-escopo.md` tem o escopo detalhado de cada entrega. Use como referência, não re-pense.

## Estado da sessão pelo `ctx-window summarize`

```
summarized: version 1 (previous 0); cli=local; summarizer=auto->heuristic; model=; turns=5
```

5 turnos resumidos, heurística local (sem LLM call). Próxima sessão começa limpa.

## Pendências registradas

- `go test ./cmd/agent-sync/...` retorna verde mas `make lint` falha em arquivos pré-existentes (17 arquivos Go fora do meu escopo, débitos pré-existentes — **não tocar** conforme AGENTS.md §3 anti-overengineering).
- 105 comandos wirados no opencode não auditados quanto a catálogo divergente (análogo ao que fizemos com skills/agents). Fora do escopo desta sessão.
- replicação do cleanup nas 4 CLIs que não opencode (Claude/Codex/Antigravity/Cursor) — as 4 já foram wiradas via `./agent-sync -apply`, mas o "mover extras para `disabled/`" foi só feito no opencode. Se outras CLIs tiverem catálogo divergente análogo, precisa de auditoria similar.
