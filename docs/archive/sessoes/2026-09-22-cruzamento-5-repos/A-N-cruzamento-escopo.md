# Escopo detalhado — 6 entregas do cruzamento 5 repos

> **Premissas verificadas empiricamente**:
> - AGENTS.md NÃO está no repo agent-sync (só wirado em `~/.config/opencode/AGENTS.md` e outras 4 CLIs).
> - Não existe `agent-sync doctor` nem `agent-sync lint` (grep em `cmd/agent-sync/` = vazio).
> - `tools/jsonschema/` já tem 5 schemas — base pronta.
> - 4/50 skills têm exemplo `bash` (gap real de "linteria").
> - Subcommands atuais: state/event/budget/agents/hooks/skills/apply/status/vendor/observability/precompact-snapshot.
> - Padrão de entrada de subcommand: `runSkillsIndexWithBase` (injetável) + `runSkillsIndex` (production). Já estabelecido.

Convenções do projeto (para seguir à risca):
- Nomes Go: `runXxxCommand` + `runXxxWithBase` (testabilidade).
- Schemas em `tools/jsonschema/schemas/*.json`, validados no binário via lib `tools/internal/jsonschema/`.
- Smoke tests em `docs/SMOKE-TEST-X.md` + 1 fixture real.
- ADR antes de feature nova se houver trade-off (ADR-001 fechado por padrão).
- Sub-deliverable = 1 commit granular + STATE A-N/D-N.

---

## Entrega 1 — `agent-sync skills lint` (OmniRoute #5B)

### Por que primeiro
Bloqueia débitos técnicos nas próximas entregas (qualquer skill nova passa pelo gate). É fundamento barato.

### Escopo concreto

**Arquivos novos** (~3):
- `cmd/agent-sync/skills_lint.go` — `runSkillsLint` + `runSkillsLintWithBase(dir) (Report, error)` + dispatcher `runSkillsCommand` ganha case `lint`. Padrão idêntico a `skills_index.go`.
- `cmd/agent-sync/skills_lint_test.go` — ~6 testes (sucesso com 0 erros, skill sem frontmatter, `name` divergente da pasta, `description` ausente, `description` sem gatilho, exemplo bash ausente).

**Regras duras** (4 checks; anti-overengineering — começar só com o que pega os débitos reais):

| # | Regra | Severidade | Mensagem |
|---|---|---|---|
| 1 | `SKILL.md` tem frontmatter YAML | error | `skill/<id>: missing frontmatter` |
| 2 | `name:` no frontmatter bate com nome da pasta | error | `skill/<id>: name "<x>" != folder "<y>"` |
| 3 | `description:` tem ≥ 1 gatilho (regex `\b(use|when|when|triggered by|if you|para|quando)\b`, case-insensitive) | warn | `skill/<id>: description without trigger verb` |
| 4 | `description:` menciona ≥ 1 exemplo (`bash`, `curl`, ou bloco ` ``` `) | info | `skill/<id>: no executable example in description` |

**Saída**: tabela com colunas `skill | status | issue` (text) ou JSON `{total, errors, warnings, infos, items: [...]}`.

**Saída de erro**: exit 1 só se houver `errors > 0` (warnings/info não bloqueiam). Conforme AGENTS.md §3.

**Não fazer**:
- Não validar conteúdo do `SKILL.md` (Markdown correctness).
- Não validar schemas externos.
- Não tocar em skills/agents/scripts fora de `skills/<id>/SKILL.md`.
- Não criar flag `--strict` (warnings já são reportadas, basta exit code).

**Esforço**: ~200 linhas Go + ~150 linhas test. Análogo a A-29 (skills index).

**Critério de done**:
- `go test ./cmd/agent-sync` verde (6 testes novos).
- Smoke em repo real: `agent-sync skills lint` retorna `errors=0` (50 skills atuais cumprem tudo? — provavelmente não, ver #4).
- ADR-curto em `docs/ADR-skills-lint.md` (opcional se não houver trade-off).

**Achado provável** (a verificar): o gate #4 vai pegar **46 skills sem exemplo** — pode ser bloqueador demais. Decisão: começar como `info` (não bloqueia) e virar `warn` se a métrica for usada.

---

## Entrega 2 — `agent-sync doctor` (OmniRoute #5C)

### Por que segundo
Observa a saúde do repo inteiro. Reusa tudo o que Entrega 1 + o que já existe.

### Escopo concreto

**Arquivos** (~2):
- `cmd/agent-sync/doctor.go` — `runDoctor(baseDir) (Report, error)` que agrega N checks via sub-funções.
- `cmd/agent-sync/doctor_test.go` — ~5 testes (cada check isoladamente + composição).

**Checks compostos** (6 — todos reusando subsistemas existentes):

| # | Check | Subsistema reusado | Saída esperada |
|---|---|---|---|
| 1 | `skills index` retorna `installed = total, missing = 0, orphan = 0` | `runSkillsIndex` (existente) | status + diff |
| 2 | `skills lint` retorna `errors = 0` | Entrega 1 | status + diff |
| 3 | Todos os 5 schemas JSON validam contra seu próprio JSON Schema | `tools/internal/jsonschema` | `OK 5/5` ou lista falhas |
| 4 | Todos os 10 plugins OpenCode wirados existem em `~/.config/opencode/plugins/` e carregam sem erro TS | hook runtime (D-33/D-44) | `OK 10/10` ou lista ausentes |
| 5 | Binários Go em `~/.local/bin/` estão atualizados (mtime ≥ source) | `make install` | `OK n/n` ou lista stale |
| 6 | `AGENTS.md` wirado em cada CLI bate com o canônico (se houver) | regra nova (ver Entrega 5) | `OK` ou `drift: claude, codex` |

**Saída**: tabela com `check | status | detail`. `--json` para machine-readable. Exit 1 se qualquer check `error`; warnings são reportados mas não bloqueiam.

**Não fazer**:
- Não criar sistema de plugins pra extensibilidade (YAGNI).
- Não rodar rede (doctor é local-only).
- Não tentar auto-fix — só reportar. AGENTS.md §3: "Cirúrgico".

**Esforço**: ~180 linhas Go + ~120 linhas test.

**Critério de done**:
- Todos os checks isoladamente testáveis (mock-friendly).
- `agent-sync doctor` em repo limpo retorna `OK 6/6`.
- Documentado em `docs/guides/doctor.md` (~40 linhas, opcional).

---

## Entrega 3 — `repo-map-graph` Fase 1 (cartographer #1)

### Por que terceiro
Já temos `tools/cmd/repo-map/` (CLI básico) — Fase 1 estende com **grafo tipado** + **brief bounded**.

### Escopo concreto (só Fase 1)

**Arquivos novos** (~4):
- `tools/jsonschema/schemas/repo-graph.json` — schema do grafo (subset enxuto do cartographer).
- `tools/cmd/repo-map-graph/main.go` — entrypoint CLI.
- `tools/cmd/repo-map-graph/build.go` — extração do grafo (reusa `tools/internal/astoutline` se possível).
- `tools/cmd/repo-map-graph/brief.go` — renderiza brief bounded por tokens.

**Schema `repo-graph.json`** (subset enxuto, ~5 nós × 4 edges):
- Nós: `File`, `Symbol`, `Package`, `Import`, `DbTable` (5 tipos).
- Edges: `CONTAINS` (Package→File), `DEFINES` (File→Symbol), `IMPORTS` (File→Import→File), `QUERIES` (File→DbTable).
- Cada edge tem `provenance: {source, line, evidence}`.

**CLI `repo-map-graph`** (3 subcommands):
- `repo-map-graph build --root <dir> --out <file.json>` — gera `repo-graph.json`.
- `repo-map-graph brief --in <file.json> --path <file> --budget-tokens 2000` — bounded brief.
- `repo-map-graph query --in <file.json> --kind imports --for <file>` — query simples.

**Brief bounded** (igual cartographer): aceita `--budget-tokens`, retorna `{nodes, edges, summary, truncated: bool, token_count}`. Algoritmo: BFS a partir do path, soma tokens estimados, trunca quando estoura.

**Não fazer (Fase 1)**:
- Não fazer MCP server (Fase 2).
- Não implementar adoption score (Fase 3).
- Não cobrir SQL/IaC/CI/env-var extractors — só imports + symbols (Fase 1 enxuta).
- Não integrar com hook ainda (Fase 2).

**Esforço**: ~350 linhas Go (schema + 3 subcommands + brief bounded) + ~200 linhas test.

**Critério de done**:
- Schema valida contra si mesmo (lib jsonschema já wirada).
- Em repo de teste (ex: `agent-sync` mesmo), `repo-map-graph build` gera `repo-graph.json` válido.
- `repo-map-graph brief --path cmd/agent-sync/main.go --budget-tokens 500` retorna grafo resumido, `truncated: true`.
- ADR-curto `docs/ADR-repo-map-graph.md` (~50 linhas) explicando escopo + Fase 2/3 planejadas.

---

## Entrega 4 — Hook `PreToolUse:Read` consultando memory-mcp (claude-mem #3)

### Por que quarto
Reusa tudo: `memory-mcp`, `session-event.jsonl`, `session-state.json`. Aditivo, baixo risco.

### Escopo concreto

**Arquivos novos** (~2):
- `hooks/memory-pretooluse-read.sh` (~40 linhas, padrão idêntico a `token-nudge.check.sh`).
- `hooks/memory-pretooluse-read.opencode.v2.ts` (~80 linhas, padrão D-26 dos plugins v2).

**Comportamento**:
1. Hook recebe payload com `tool_input.file_path`.
2. Chama `~/.local/bin/memory-mcp query --path <file> --format json` (já existe — verificar tools).
3. Se encontrou memórias relevantes (`decisions`, `notes`, `warnings` para esse path), injeta 1 linha em stderr no formato que o opencode injeta no system prompt (`"additional_context": "..."`).
4. Emite evento `memory_pretooluse_inject` no `session-event.jsonl` (schema já existe, novo `kind`).

**Wirar em runtime** (1-2 commits):
- Patch em `cmd/agent-sync/hooks.go` adiciona `memory-pretooluse-read` ao `standardHooks` registry.
- Patch em `cmd/agent-sync/hooks_opencode_plugin.go` (ou equivalente) wirar plugin v2.

**Schema novo**:
- Adicionar `memory_pretooluse_inject` ao enum `kind` em `tools/jsonschema/schemas/session-event.json` (bump 1.1 → 1.2, padrão A-26).

**Critério de done**:
- `go test ./cmd/agent-sync` verde (teste novo do wirar).
- Smoke D-31-style: payload opencode real com `file_path` conhecido → hook emite evento no JSONL → grep encontra.
- `agent-sync doctor` (Entrega 2) ganha check #7 opcional: "memory-mcp acessível via `~/.local/bin/memory-mcp query`".

**Achado a verificar (anti-alucinação D-24)**: `memory-mcp query --path` provavelmente **não existe** hoje — vai precisar subcommand novo no `tools/cmd/memory-mcp/`. **Escopo atualizado**:
- Adicionar `query --path X --format json` ao `memory-mcp` (~50 linhas Go).
- Backward-compatible (defaults seguros).

**Esforço real**: ~200 linhas Go + bash + TS = ~330 linhas totais.

---

## Entrega 5 — Padronizar AGENTS.md entre CLIs (OmniRoute #5A)

### Por que quinto
Problema verificado empiricamente: AGENTS.md wirado em opencode tem 30 linhas, mas o canônico do agent-sync **não está no repo** — então o wirar copia de algum lugar não-versionado. Drift certo.

### Escopo concreto

**Decisão arquitetural antes (mini-ADR)**:
- **Opção A** (recomendada): AGENTS.md canônico mora em `rules/global-rules.md` do agent-sync (já existe, 90 linhas). O wirar copia esse arquivo para `~/.config/{cli}/AGENTS.md`. Já é o que parcialmente acontece.
- **Opção B** (descartada por enquanto): symlink. Não funciona cross-CLI (cada CLI tem formato próprio — Claude aceita symlink, Cursor não).
- **Opção C** (YAGNI): cópia + check de hash no doctor. Já é o que doctor faria.

**Entrega mínima**:
1. Mover `~/.config/opencode/AGENTS.md` para `rules/global-rules.md` se não for igual — **verificar primeiro** (`diff`).
2. Garantir que `cmd/agent-sync/apply_metadata.go` (ou equivalente) **sincroniza `rules/global-rules.md` → `~/.config/<cli>/AGENTS.md`** em todo `-apply`. Se já faz, é zero código; se não, patch de ~15 linhas.
3. Adicionar check **#6 do doctor**: hash de cada `AGENTS.md` wirado bate com hash do canônico. Reporta `drift: <clis>`.

**Esforço real**: depende da auditoria. Cenários:
- Cenário simples (já wirar funciona, só falta check): ~80 linhas em `doctor.go` + 1 teste.
- Cenário médio (wirar não sincroniza): ~150 linhas + 1 teste.
- Cenário complexo (formato diverge entre CLIs): ADR primeiro, decisão do usuário, então implementação.

**Pré-trabalho** (sem patch):
```bash
# diff para mapear divergência
diff -u rules/global-rules.md ~/.config/opencode/AGENTS.md | head -40
md5sum rules/global-rules.md ~/.config/opencode/AGENTS.md ~/.claude/AGENTS.md ~/.codex/AGENTS.md ~/.gemini/AGENTS.md ~/.cursor/AGENTS.md 2>/dev/null
```

**Critério de done**:
- `agent-sync doctor` reporta `AGENTS.md OK 5/5` (ou `drift: <lista>` se houver).
- `agent-sync -apply` é idempotente (segundo run não muda nada).

---

## Entrega 6 — Skills `repo-auditor` + `agent-learn` (claude-code-setup #2 + headroom #4B)

### Por que sexto (último)
Fecha o loop: as 5 anteriores instrumentam o repo, essas 6 transformam observações em ação.

### Escopo concreto

**6a — Skill `repo-auditor`** (1 arquivo):

`skills/repo-auditor/SKILL.md` (~80 linhas, mesmo padrão das outras skills):

```yaml
---
name: repo-auditor
description: "Audita o repo agent-sync em busca de débitos técnicos, oportunidades de refator e gaps cross-CLI. Use when the user asks 'audit this repo', 'find tech debt', 'what should we improve', or when the user wants a periodic health check."
---

# Repo Auditor

Read-only. Roda `agent-sync doctor`, `agent-sync skills lint`, `agent-sync skills index -orphans`, e opcionalmente `git log --oneline -30`. Emite 1-3 recomendações curtas priorizadas.

## Saída

Tabela markdown com colunas: `Severidade | Área | Recomendação | Esforço | Evidência`.

## Não faz

- Não edita arquivos.
- Não commita.
- Não invoca subagents que modificam.
```

**Wirar**: já wirado pelo `agent-sync -apply` automaticamente (skill em `skills/` entra no scan — ver D-43).

**6b — Skill `agent-learn`** (1 arquivo):

`skills/agent-learn/SKILL.md` (~70 linhas):

```yaml
---
name: agent-learn
description: "Analisa session-event.jsonl recente e STATE.md para identificar padrões recorrentes de erro e propor nova skill ou bullet em global-rules.md. Use when the user asks 'learn from failures', 'extract lessons', 'update rules based on history'."
---

# Agent Learn

Lê `.agent-sync/session-event.jsonl` (últimas N=100 entradas) + `STATE.md` (decisões D-N recentes).
Procura por: loops (mesma action 3+ vezes), hipóteses refutadas não documentadas, hooks bloqueando comandos destrutivos, gaps cross-CLI (matriz 5xN incompleta).
Emite proposta de bullet ou skill nova. **Não aplica sozinho** — pede confirmação antes.

## Saída

```text
Padrões detectados: N
Proposta 1: <bullet para rules/global-rules.md>
  Evidência: D-37, D-44 (cite)
  Diff proposto:
    + <linha nova>
Proposta 2: <nova skill X>
  Justificativa: ...
```
```

**Wirar**: idem, automático.

**Critério de done**:
- 2 skills passam no lint da Entrega 1.
- Wirar via `agent-sync -apply` confirma em 1 CLI.
- `repo-auditor` consegue carregar e listar as saídas do doctor.

**Esforço**: ~150 linhas Markdown (2 SKILL.md).

---

## Entrega extra (defasada) — ADR `cache-aligner` (headroom #4A)

### Não é código — só documentação

`docs/ADR-cache-aligner-risco.md` (~50 linhas):

```markdown
# ADR-NNN: Risco de invalidação de prefix-cache em compressão de prompts

## Status
Aceito (documentação de risco, sem implementação)

## Contexto
O `tools/cmd/ctx-window/` faz summary incremental do system prompt. Headroom (labs externos) alerta que reescrita de prompt pode invalidar prefix-cache do provider (Anthropic, OpenAI), o que é pior que não comprimir.

## Decisão
NÃO implementar compressão semântica do system prompt no agent-sync. Justificativa: o ganho marginal de compressão vs. o risco de invalidação de cache não foi medido, e o `ctx-window` já opera por sliding window (mantém últimos K verbatim + summary incremental, sem reescrever prefixos).

## Trigger para reabrir
- Se `ctx-window` virar gargalo medido (latência > 5% por sessão).
- Se headroom ou similar publicar benchmark com CacheAligner hit-rate > 95% em workloads similares.

## Referências
- `tools/cmd/ctx-window/` (atual implementação).
- ADR Trilha C (`docs/ADR-trilha-c-cobertura-cross-cli.md`) — já alinha com princípio conservador.
```

**Esforço**: 30 min, 1 arquivo.

---

## Cronograma e dependências

```
Entrega 1 (skills lint)            ─┐
                                    ├── pré-requisito da 2 e 6
Entrega 2 (doctor)                  ─┤
                                    │
Entrega 3 (repo-map-graph Fase 1)   ─┤── independente
                                    │
Entrega 4 (PreToolUse:Read)         ─┤── depende de memory-mcp query (audit primeiro)
                                    │
Entrega 5 (AGENTS.md padronizar)    ─┤── pré-audit com diff obrigatório
                                    │
Entrega 6 (skills autorais)        ─┴── depende de 1 e 2 funcionando
```

**Sequência sugerida** (se for executar):
1. Entrega 1 (3-4h, isolada)
2. Entrega 5 pré-audit (30 min) → Entrega 5 completa (1-2h)
3. Entrega 2 (3-4h, depende de 1)
4. Entrega 4 (4-5h, depende de audit de memory-mcp)
5. Entrega 6 (2h, depende de 1 e 2)
6. Entrega 3 (1 dia, maior escopo)
7. ADR cache-aligner (30 min, anytime)

**Total estimado**: ~3-4 dias de trabalho focado, distribuídos em 1-2 semanas respeitando seu ritmo de A-N granulares.

---

## Decisões a tomar antes de começar

1. **Entrega 1 check #4** (exemplo `bash` em description): bloquear ou só `info`? — afeta 46 skills atuais.
2. **Entrega 4 pré-requisito**: memory-mcp tem `query --path` hoje? (audit rápido) — se não, escopo cresce ~50 linhas.
3. **Entrega 5 pré-audit**: rodar `diff rules/global-rules.md ~/.config/opencode/AGENTS.md` antes de planejar patch.
4. **Entrega 3 escopo Fase 1**: confirmar 5 nós × 4 edges é o subset certo ou já cobrir SQL/IaC?

Recomendo começar por **Entrega 1** + **pré-audits de 2, 4 e 5** em paralelo (1 turno só), e abrir A-38/A-39 no STATE formal só depois da Entrega 1 fechada.
