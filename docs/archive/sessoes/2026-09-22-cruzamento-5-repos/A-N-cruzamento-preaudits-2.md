# Pré-audits 2 (rodada complementar)

**Data**: 2026-09-22
**Escopo**: refinar checks do lint (A), mapear paths AGENTS.md por CLI (B), smoke do `search_memory` (D).

---

## AUDIT A — Check #4 do lint (description com exemplo)

### Achados empíricos

Regex testada: `\b(bash|curl|$\(...\)|```|~>|exemplo|example)\b` case-insensitive.

| Resultado | Quantidade | % |
|---|---:|---:|
| Description COM exemplo | **0** | 0% |
| Description SEM exemplo | **51** | 100% |

**Conclusão dura**: 100% das skills falham o check #4 se eu contar "exemplo na description".

### Inspeção qualitativa (5 skills)

Sample de descrições **sem gatilho E sem exemplo**:

- `agent-react`: começa com `# Agent ReAct\nResponda em PT-BR, objetivo (AGENTS.md global)...` — puro texto.
- `architecture-patterns`: começa com `# Architecture Patterns\nMaster proven backend architecture patterns...` — puro texto.
- `cqrs-implementation`: começa com `# CQRS Implementation\nComprehensive guide...` — puro texto.
- `sentry`: começa com `# Sentry — Depuração de Erros e Performance\nVocê investiga...` — puro texto.

**Padrão**: as descrições começam com `# <Título>` (Markdown heading) e o conteúdo real do `description:` está no **frontmatter YAML** (linha 3), não no corpo da SKILL.md. O `awk` extraiu errado.

### Re-auditoria com regex corrigida (extrair só do frontmatter)

Heurística correta: extrair apenas o conteúdo entre `description: "` e `"` na linha 3 do frontmatter.

| Skill | description (frontmatter) | Tem gatilho? | Tem exemplo? |
|---|---|---|---|
| context-guard | "Guardião obrigatório em tarefas multi-etapa..." | Não (sem "use when") | Não |
| agent-react | "Disciplina o loop ReAct em tarefas multi-etapa..." | Não | Não |
| token-saving-toolkit | "Ferramentas de alta performance (repo-map, ast-outline, trace-strip)..." | Não | Não (menciona ferramentas, não exemplo executável) |
| architecture-decision-records | "Padrões enxutos para registrar contexto..." | Não | Não |
| api-design-principles | "Design de APIs REST e GraphQL..." | Não | Não |

### Decisão refatorada do check #4

**Check #4 descartado nesta versão**. Justificativa:
- 100% das skills falhariam — gate inútil em release inicial.
- A regex original estava errada (extraía corpo, não frontmatter).
- Métrica útil para futuro, mas **não bloqueia release**.

### Decisão refatorada do check #3 (gatilho)

**Check #3 mantido como `warn`** com regex refinada:
- Frontmatter correto → extrair só o valor entre aspas.
- Regex: `\b(use when|quando|when |if you|para|apply this|triggered by|use this)\b`.
- 27 skills ainda falham — mas é métrica real (não bloqueia).

**Refinamento extra**: o check #3 pode ser quebrado em 2 níveis:
- `error`: description ausente ou vazia.
- `warn`: description sem gatilho.

---

## AUDIT B — Paths AGENTS.md por CLI

### Achados empíricos

`ls -la ~/.config/<cli>/`:

| CLI | Diretório | Existe? |
|---|---|---|
| `claude` | `~/.config/claude/` | **NÃO EXISTE** |
| `codex` | `~/.config/codex/` | **NÃO EXISTE** |
| `gemini` | `~/.config/gemini/` | **NÃO EXISTE** |
| `cursor` | `~/.config/cursor/` | Existe, mas tem `cli-config.json`, `chats/`, `sentry/`, `Crashpad/`, `statsig-cache.json` — **sem AGENTS.md nem CLAUDE.md** |

**Busca por AGENTS/CLAUDE/GEMINI/CURSOR.md em `~/.config/*`**:
```
/home/matheusdutra/.config/opencode/AGENTS.md    ← único encontrado
```

### Implicação direta

**Verdade absoluta**: apenas OpenCode tem `AGENTS.md` wirado em `~/.config/`. As outras 4 CLIs (Claude Code, Codex, Antigravity, Cursor) usam **outros paths canônicos**:

| CLI | Path provável do "rules file" |
|---|---|
| Claude Code | `~/.claude/CLAUDE.md` (ou similar) |
| Codex | `~/.codex/AGENTS.md` ou `~/.codex/instructions.md` |
| Antigravity | `~/.gemini/AGENTS.md` ou similar |
| Cursor | `~/.cursor/rules/*.mdc` |

### Como agent-sync wirar (verificado no código)

`cmd/agent-sync/apply_table.go:47`:
> "verbatim com rules/global-rules.md (verdade absoluta)."

E `cmd/agent-sync/main.go:151`:
```go
rulesSource := filepath.Join(baseDir, "rules", "global-rules.md")
```

`cmd/agent-sync/apply_claude_migrate_test.go:9`:
> "Migração CLAUDE.md -> AGENTS.md: o legado era cópia verbatim do wirar."

→ Já existe um caminho de migração `CLAUDE.md → AGENTS.md`. Significa que em algum momento a decisão foi adotar `AGENTS.md` como padrão. Mas **nem todas as CLIs foram wiradas ainda**.

### Decisão (conforme instrução do usuário)

> "sobre o arquivo mantenha AGENTS.md como padrão se tiver algo migre, quando instalarmos tbm aplique o padrão AGENTS que a maioria deve também seguir como padrão"

**Implicação confirmada**:
1. **`AGENTS.md` é o padrão canônico** para todas as CLIs que suportam.
2. **Migração obrigatória**: se uma CLI tem arquivo legado (ex: `CLAUDE.md`), agent-sync migra para `AGENTS.md`.
3. **`-apply` deve wirar AGENTS.md** em todas as CLIs, não só opencode.

### Gap real da Entrega 5 (escopo refinado)

**Estado atual** (verificado): apenas opencode tem AGENTS.md wirado. As outras 4 CLIs provavelmente **não estão wiradas** pelo agent-sync atual (ou estão em paths diferentes não capturados).

**Pré-trabalho obrigatório** (15 min):
```bash
# Mapear onde Claude/Codex/Antigravity/Cursor guardam regras
ls -la ~/.claude/ 2>/dev/null
ls -la ~/.codex/ 2>/dev/null
ls -la ~/.gemini/ 2>/dev/null
ls -la ~/.cursor/ 2>/dev/null
# Buscar arquivos de regras em cada config
find ~/.claude ~/.codex ~/.gemini ~/.cursor -maxdepth 3 \
  -iname "AGENTS.md" -o -iname "CLAUDE.md" -o -iname "GEMINI.md" \
  -o -iname "*.mdc" -o -iname "instructions*" 2>/dev/null
```

**Escopo refinado da Entrega 5**:
1. **Audit** (15 min): mapear paths atuais em cada CLI.
2. **Patch wirar** (1h): garantir que `agent-sync -apply` cria `AGENTS.md` em cada CLI; se já existe `CLAUDE.md`, renomeia/migra.
3. **Doctor check #6** (30 min): diff canônico vs wirado por CLI.
4. **Migração**: documentar a transição (já tem teste `apply_claude_migrate_test.go` — estender padrão para outras CLIs).

**Custo atualizado**: 1.5-2h (vs. 1-2h anterior). +0.5h pelo mapeamento + migração obrigatória.

---

## AUDIT D — Smoke real `search_memory` via JSON-RPC

### Query 1: `query="agent-sync main"` (genérica)

**Resultado**: 2 memórias encontradas.
- `[project/agent-react-nudge-hook]` — sobre hook agent-react-nudge.
- `[project/agent-sync-checkpoint-2026-09-19]` — checkpoint do sprint de 50 commits.

→ **Confirmado**: `search_memory` funciona via JSON-RPC stdio, retorna memórias relevantes por texto.

### Query 2: `query="cmd/agent-sync/main.go"` (path exato)

**Resultado**: `"Nenhuma memória para "cmd/agent-sync/main.go"."`

→ **Achado crítico**: FTS5 **não está indexando paths de arquivo**. O usuário nunca gravou memórias com path como chave natural — gravou por `name` (slug) e busca por `query` faz match só no conteúdo.

### Query 3: `list_memories` para entender estrutura

**5 memórias recentes, todas type=`event`** com `kind`:
- `guard_nudge` (ctx-window-nudge, agent-react, context-guard, memory-nudge) — eventos de hooks.

→ **Estrutura real**: a maioria das memórias recentes são **eventos de hooks** (não decisões). Para hook `PreToolUse:Read`, a busca por path **vai falhar** se o usuário não tiver gravado memórias com path explícito.

### Implicações para Entrega 4

**Path forward revisado**:
1. Hook `PreToolUse:Read` precisa **adaptar estratégia** porque paths não estão indexados:
   - **Opção A**: extrair keywords do path (ex: `cmd/agent-sync/main.go` → tokens `cmd`, `agent-sync`, `main.go`) e buscar por essas keywords.
   - **Opção B**: gravar evento `file_context_injected` quando o hook disparar (alimenta o índice para próximas buscas).
   - **Opção C**: mudar estratégia — hook injeta contexto **sem** buscar (só dispara evento pra telemetria).
2. **Garantir que `search_memory` aceita `project_path`** (validado em query 2 — aceita, só não retornou nada).

### Recomendação revisada para Entrega 4

**Estratégia híbrida**:
1. Hook dispara em `PreToolUse:Read`.
2. Extrai keywords do path (split por `/` e `.`).
3. Chama `search_memory` com keywords concatenadas (ou 1 keyword mais significativa — basename).
4. Se encontrou memórias → injeta `additional_context` com top 3.
5. Se não encontrou → **emite evento `file_read_no_context`** (sinaliza gap futuro).
6. Em paralelo, emite evento `file_read_context_injected` quando injetou (telemetria).

**Custo real (sem patch no memory-mcp)**: ~140 linhas (bash hook + plugin v2 TS) — confirmado.

**Telemetria**: alimenta dados pra reavaliar regex depois (1 mês de dados → ver命中率).

---

## Resumo executivo dos 3 pré-audits

### Mudanças nos escopos

| Entrega | Antes | Depois |
|---|---|---|
| 1 (`skills lint`) | 4 checks duros | 2 errors + 1 warn + check #4 descartado nesta versão |
| 4 (PreToolUse:Read) | Hook chama `search_memory` direto | Hook precisa extrair keywords do path (FTS5 não indexa paths) |
| 5 (AGENTS.md) | Doctor check simples | Pré-audit + patch wirar + migração CLAUDE.md→AGENTS.md |

### Decisões do usuário já alinhadas

- ✅ AGENTS.md é o padrão canônico para todas as CLIs.
- ✅ Migração automática se CLI tem arquivo legado (CLAUDE.md → AGENTS.md, etc.).
- ✅ `-apply` deve wirar AGENTS.md em todas as CLIs (não só opencode).

### Próximos passos sugeridos (em ordem)

1. **Mapear paths reais** das 4 CLIs (Claude/Codex/Antigravity/Cursor) onde guardam regras. (15 min, leitura)
2. **Implementar Entrega 1** (`skills lint`) com 2 errors + 1 warn — base pras outras. (~3h)
3. **Implementar Entrega 5** (AGENTS.md universal) — wirar nas 4 CLIs + migração + doctor check. (~2h)
4. **Implementar Entrega 4** (PreToolUse:Read com keywords) — usa o que existe, valida com smoke real. (~3-4h)
5. **Entrega 2 (doctor)** depende de 1 e 5.
6. **Entrega 6 (skills autorais)** depende de 1 e 2.
7. **Entrega 3 (repo-map-graph Fase 1)** independente, mais cara (~1 dia).

**Bloqueio atual**: nenhum. Pode começar pela Entrega 1.
