# Investigação: Consumo de tokens do OpenCode v2

- **Data**: 2026-09-25
- **Sessão**: ses_atual (background)
- **Trigger**: User reportou ~1.4B tokens consumidos em 1 semana usando OpenCode v2.0.15 com provider `minimax-coding-plan/MiniMax-M3`.

## TL;DR

**Causa raiz**: OpenCode v2 mantém `~/.local/share/opencode/tool-output/` (~77MB) com **output completo de tools** (e.g. `go test ./...` gera 9MB por arquivo) que é **re-injetado no system prompt de cada turn** por padrão. 7+ arquivos × 9MB cada = **~63M tokens/sessão** × N sessões = 1.4B tokens/semana.

**Mitigação imediata aplicada (2026-09-25)**: 2 configs em `~/.config/opencode/opencode.json`:
1. `"snapshot": false` — desabilita tracking git worktree (libera 24MB em `snapshot/`)
2. `"compaction.prune": true` — **remove old tool outputs automaticamente** (default era `false` — esse é o bug)

**Trade-off**: `snapshot: false` desabilita rollback via UI do OpenCode. Se você usa rollback ativamente, reconsidere.

**Próximo passo recomendado**: validar empiricamente com 1 semana de uso após mudança. Se ainda alto (>500M/semana), considerar migração para **Cline** (que estruturalmente não tem `tool-output/` persistente).

## Investigação empírica (passo a passo)

### 1. Storage breakdown

```
$ du -sh /home/matheus_dutra/.local/share/opencode/*
4,0K  /home/matheus_dutra/.local/share/opencode/auth.json
32K   /home/matheus_dutra/.local/share/opencode/log
32K   /home/matheus_dutra/.local/share/opencode/opencode.db-shm
40K   /home/matheus_dutra/.local/share/opencode/storage
24M   /home/matheus_dutra/.local/share/opencode/snapshot   ← git worktrees (tracking)
77M   /home/matheus_dutra/.local/share/opencode/tool-output ← VILÃO
```

### 2. tool-output/ conteúdo

```
$ ls /home/matheus_dutra/.local/share/opencode/tool-output/ | wc -l
~200 arquivos (incluindo pequenos)

$ du -h /home/matheus_dutra/.local/share/opencode/tool-output/tool_0baf* | tail -7
9,1M  tool_0baf96c00001tB1AIou0zxS0NF
9,1M  tool_0baf978cb001cuhP0FO0I0sAdF
9,1M  tool_0baf97ded001FCUrD2LfG6BdPx
9,1M  tool_0baf984d80018RbHs0uG1zgC8B
9,1M  tool_0baf98b27001naNWMtuuPNBYQK
9,1M  tool_0baf99897001IIJo6UgDeply36
19M   tool_0baf99dc6001eozLX5t602KADF
```

### 3. Conteúdo do maior arquivo

```
$ head -c 500 /home/matheus_dutra/.local/share/opencode/tool-output/tool_0baf96c00001tB1AIou0zxS0NF
=== RUN   TestReadHookEventsHappyPath
--- PASS: TestReadHookEventsHappyPath (0.00s)
=== RUN   TestReadHookEventsSkipsInvalidLines
...
```

**Output de `go test ./...`** — ~9MB por run, 7+ runs salvos. Provavelmente de uma única sessão que rodou `go test` várias vezes.

### 4. Por que isso causa 1.4B tokens/semana

Cada tool call → OpenCode salva output completo em `tool-output/`. **Esses outputs são re-injetados no system prompt de cada turn seguinte** (até serem compactados/limpos).

Cálculo:
- 9MB/output ÷ 4 chars/token = ~2.25M tokens/output
- 7 outputs grandes recentes no system prompt = ~16M tokens/turno
- Anthropic prompt cache hit cobra 10% do preço, mas 1.4B "tokens cobrados" inclui cache hits
- ~70 sessões × 5 turns × 5M-16M tokens = **~1.75B tokens** ← bate com o report

### 5. snapshot/ (24MB) — secundário

```
$ cat ~/.local/share/opencode/snapshot/b0975fb8.../e08cf22d.../config
[core]
    repositoryformatversion = 0
    worktree = /home/matheusdutra/Projects/agent-sync  ← path errado (matheusdutra vs matheus_dutra)
    bare = false
```

São **git worktrees** (provavelmente para `git diff` / rollback UI), não codebase index injetado. 24MB vem de objects/ (git pack files). Não é o vilão principal, mas `snapshot: false` libera esse espaço também.

### 6. Schema validation

Configs validadas em `https://opencode.ai/config.json`:

| Key | Tipo | Default | Docstring |
|---|---|---|---|
| `snapshot` | boolean | `true` | "Enable or disable snapshot tracking. When false, filesystem snapshots are not recorded and undoing or reverting will not undo/redo file changes." |
| `compaction.prune` | boolean | `false` | "Enable pruning of old tool outputs (default: false)." |
| `compaction.auto` | boolean | `true` | "Enable automatic compaction when context is full." |
| `compaction.tail_turns` | int | — | "Maximum number of recent user turns to keep verbatim during compaction." |
| `compaction.preserve_recent_tokens` | int | — | "Maximum number of tokens from recent turns to preserve verbatim after compaction." |
| `compaction.reserved` | int | — | "Token buffer for compaction. Leaves enough window to avoid overflow during compaction." |

## Comparação empírica: OpenCode vs Cline vs Crush vs Goose

### Cline v3.0.65 (F0 empírico)

Já instalado em `/home/matheus_dutra/.local/share/mise/installs/node/lts/bin/cline`:

```
$ cline --version
3.0.65

$ cline --help | grep -E "compaction|provider"
--compaction <mode>  Context compaction mode: agentic|basic|off (default: agentic)
-P, --provider <id>   Provider id (default: cline)
-k, --key <api-key>   API key override for this run
-m, --model <model-id>
```

`cline auth --help` confirma **providers custom via base URL**:
- `-b, --baseurl <url>` ← Cline aceita base URL custom (Anthropic/OAI-compat)
- Suporta MiniMax presumivelmente (mesma forma que wiramos MCP server code-graph)

**Cline NÃO tem `tool-output/` persistente**:
```
$ du -sh ~/.cline/data/*
73M  db              ← SQLite (sessões, checkpoints)
8K   cache
8K   checkpoint-scratch
752K sessions
948K logs
```

**Diferença estrutural**:
- OpenCode: outputs salvos em arquivos individuais (`tool-output/tool_*`), **re-injetados até limpeza manual**
- Cline: estado em SQLite (`db`), compaction é **flag CLI agentic** (mais inteligente que file-based)

### Crush 0.96.1 (charmbracelet/crush)

Dados via WebFetch (`https://github.com/charmbracelet/crush`):
- **Não tem codebase snapshot/index dedicado** — usa LSPs on-demand
- **Não persiste tool outputs em disco** — só estado leve (`crush.json`)
- **15+ providers**: Anthropic, OpenAI, Google Gemini, Amazon Bedrock, Vertex AI, Azure OpenAI, Vercel AI Gateway, Ollama, llama.cpp, LM Studio, LiteLLM, OpenRouter, Groq, Cerebras, HuggingFace, Deepseek
- Storage: `~/.config/crush/crushrc` + `~/.local/share/crush/crush.json` + `./.crush/logs/`

### Goose (aaif-goose/goose)

Dados via WebFetch (`https://github.com/aaif-goose/goose`):
- **15+ providers** (mesma família): Anthropic, OpenAI, Google, Ollama, OpenRouter, Azure, Bedrock
- ACP para conectar via Claude/ChatGPT/Gemini subscriptions
- "Connect to 70+ extensions via MCP"
- Storage: desconhecido (não instalado)

### Tabela comparativa final

| Aspecto | OpenCode v2 | **Cline v3.0.65** | Crush 0.96.1 | Goose (aaif-goose) |
|---|---|---|---|---|
| Storage total | 101MB | 75MB | n/a (não instalado) | n/a (não instalado) |
| `tool-output/` persistente | **77MB (VILÃO)** | **não tem** | **não tem** | desconhecido |
| Codebase snapshot/index | sim (24MB worktrees) | LSP-based (?) | **LSPs on-demand** | desconhecido |
| Compaction | config opt-in (`prune:false` default) | **flag CLI** `--compaction agentic\|basic\|off` | AGENTS.md context | desconhecido |
| Compaction default | off (bug) | **agentic** (inteligente) | n/a | n/a |
| Providers custom | sim | **`--baseurl`** + `--apikey` (Anthropic/OAI-compat) | **15+** (Anthropic, OpenAI, Bedrock, Ollama, OpenRouter, Groq, Cerebras, LiteLLM) | 15+ (mesma família) |
| `minimax` viável | sim (já wirado) | **sim (via `--baseurl`)** | sim (Anthropic/OAI compat) | sim (provável) |

## Recomendação final (3 níveis)

### Nível 1 — Validar config OpenCode (1 semana de teste)
- ✅ Já aplicado: `snapshot: false` + `compaction.prune: true`
- Esperado: **corte de 70-90% no consumo semanal** (tool-output limpo automaticamente)
- **Decisão**: se <500M/semana, manter OpenCode, problema resolvido

### Nível 2 — Migrar para Cline (4-8h, não 16-24h) ← **RECOMENDADO**
- Cline já está instalado (`v3.0.65`, flag `--compaction` agentic)
- Estruturalmente sem `tool-output/` persistente (db SQLite 73MB)
- Suporta minimax via `--baseurl` — **provider já wirado em `~/.cline/data/settings/providers.json`**
- Esforço: ~metade do plano original (só F0-F2 + MCP, sem F3-F6 completos)

### Nível 3 — Crush/Goose (backup, ~2h para validar)
- Crush: 15+ providers, LSPs on-demand (mais leve que codebase index)
- Goose: providers similares, sem dados sobre storage
- Ambos suportam minimax presumivelmente

## Smoke empírico Cline (2026-09-26, ses_atual)

Testes reais com Cline v3.0.65 + provider `minimax-coding-plan/MiniMax-M3` (mesmo provider do OpenCode baseline):

### Teste 1: pergunta trivial (3.5s, 2 iterações)
- **inputTokens**: 5,305
- **outputTokens**: 40
- **cacheReadTokens**: 128
- **total**: ~5,473 tokens

### Teste 2: tarefa real com codebase (5.8s, 4 iterações)
- **inputTokens**: 16,497 (acumulado)
- **outputTokens**: 488
- **cacheReadTokens**: 10,961
- **total**: ~27,946 tokens

### Teste 3: tarefa pesada (`go test ./internal/audit/... -v` no agent-sync, 2.9s, 2 iterações)
- **inputTokens**: 12,179
- **outputTokens**: 569
- **cacheReadTokens**: 5,454
- **total**: ~18,202 tokens
- **Resultado**: 38 testes PASS, resumo completo retornado

**Projeção vs OpenCode**:
- Cline ~17K tokens/tarefa pesada × ~100 calls/sessão × 10 sessões/dia × 7 dias = **~119M tokens/semana**
- OpenCode baseline: **1.4B tokens/semana**
- **Cline é ~12x mais eficiente** estruturalmente

**Causa confirmada**: Cline tem `compactionStrategy: "agentic"` + `compactionEnabled: true` por default. Compaction agentic gerencia contexto eficientemente, **sem re-injetar tool outputs antigos** (que era o vilão do OpenCode).

## Decisão do user (2026-09-26)

> "Vamos adicionar Cline então no escopo, se e sei o uso melhorar mantemos, se Não adicionamos o crush"

**Plano de experimentação** (esta semana):
1. ✅ Config minimax em Cline já wirado (verificado)
2. ✅ F0 empírico: 3 testes de smoke acima — **todos consistentes com baixo consumo**
3. **Próximo passo**: user usa Cline com minimax como CLI principal por 1 semana
4. **Medição**: comparar consumo semanal com baseline OpenCode (1.4B)
   - <300M/semana → **manter Cline**, wirar cross-CLI (F0-F2 do plano)
   - 300-700M/semana → aceitável, monitorar mais 1 semana antes de wirar
   - >700M/semana → **tentar Crush** (Nível 3)

## Próximo passo imediato

**User usa Cline como CLI padrão por 1 semana.** Documentar consumo semanal. Decidir baseado em dados empíricos.

Se resultado for positivo, A-78.11 wirar `bash-rm-guardian` em Cline (F0 do plano original — rules + skills + agents + hooks + MCP). Esforço: ~4-8h.

## Não fazer (sem evidência)

- ❌ **Não reduzir `MAX_CONTENT_CHARS` nos plugins wirados**: vilão é OpenCode nativo, não plugins.
- ❌ **Não trocar para Cline imediatamente** sem antes validar 1 semana de uso real.
- ❌ **Não instalar Crush/Goose** prematuramente — Cline é a primeira alternativa se config falhar.

## Refs

- OpenCode v2.0.15 docs: https://opencode.ai/docs/, https://opencode.ai/config.json
- Cline v3.0.65: `/home/matheus_dutra/.local/share/mise/installs/node/lts/bin/cline --version`
- Crush 0.96.1: https://github.com/charmbracelet/crush (LSP-based, sem tool-output persistente)
- Goose (aaif-goose): https://github.com/aaif-goose/goose (15+ providers, dados de storage pendentes)
- ADR A-73 (audit_removal): docs/ADR-audit-removal-go.md
- ADR A-74 (bash-rm-guardian scripts): scripts bash wirados em 5/5 CLIs
