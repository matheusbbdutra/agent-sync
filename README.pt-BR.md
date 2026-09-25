# Agent-Sync

> [English](README.md) · [Português](README.pt-BR.md)

Versiona, mantém e sincroniza **Regras Globais**, **Skills**, **Agentes Especialistas** e **Ferramentas de Baixo Consumo de Tokens** entre 5 CLIs:
- **Claude Code** (`~/.claude`)
- **Codex / OpenAI** (`~/.codex`)
- **Google Antigravity CLI** (`~/.gemini`) — sucessora da Gemini CLI standalone (encerrada em 18/06/2026 para contas não-enterprise)
- **OpenCode** (`~/.config/opencode`)
- **Cursor** (`~/.cursor`) — IDE + Agent CLI (`agent` / `cursor-agent`)

---

## 📦 Estrutura do Repositório

```text
agent-sync/
├── rules/
│   └── global-rules.md     # Regras globais (Clean Code, OWASP, Anti-Alucinação, Data Guardians)
├── skills/
│   ├── manifest.json       # Curadoria: IDs + origem (rmyndharis/antigravity-skills, MIT) usada por `-vendor`
│   └── <52 skills>/        # Vendorizadas e autorais (backend, segurança, DB, API, linguagens, testes, ops, contexto)
├── agents/                 # Agentes especialistas autorais (canônicos), gerados por CLI no `-apply`
│   ├── spec-planner.md, code-reviewer.md, security-auditor.md, debugger.md, frontend-ui-designer.md
│   └── architecture-reviewer.md, test-engineer.md, refactor-specialist.md, db-guardian.md, token-optimizer.md, sentry-debugger.md, mr-reviewer.md
├── tools/                  # Binários utilitários de alta velocidade em Go
│   ├── cmd/ast-outline/    # Extrai classes/métodos em vez de ler arquivos inteiros (Go, Python, TS, PHP)
│   ├── cmd/trace-strip/    # Remove ruídos de frameworks em logs de erro
│   ├── cmd/db-guardian/    # Valida SQL read-only (bloqueia mutações, injeta LIMIT); NÃO roda queries
│   ├── cmd/docs-fetch/     # Baixa/cacheia docs e extrai texto ou outline de títulos (baixo token)
│   ├── cmd/docs-mcp/       # Servidor MCP local (offline) sobre o cache de docs
│   ├── cmd/memory-mcp/     # Servidor MCP e pipeline de memória contínua (SQLite FTS5 local + buffer efêmero)
│   └── cmd/repo-map/       # Grafo de código limitado (blast radius, chamadas reversas, MCP code-graph)
├── mirror/sources.json     # Fontes oficiais curadas para sincronizar no cache local
├── templates/STATE.md      # Template de handoff de sessão (checkpoint/retomada)
├── templates/skill.md.tpl  # Template para criação de skills de projeto (`agent-sync skills new`)
├── cmd/agent-sync/         # Orquestrador de sincronização CLI (+ vendor de skills + gerador de agentes)
├── scripts/setup-go.sh     # Bootstrap do Go (>= 1.24) via mise ou tarball oficial
├── scripts/setup-mcp.sh    # Configura Context7, MCPs locais (docs, memory, code-graph) e Sentry (opcional)
├── Makefile                # Comandos de automação
├── docs/                   # Architecture Decision Records (ADRs) e Padrões de Hooks Multi-CLI
└── README.md / README.pt-BR.md  # Documentação (EN padrão, PT-BR)
```

### Agentes especialistas (em `agents/*.md`)

| Agente | Read-only |
| --- | --- |
| `spec-planner`, `code-reviewer`, `mr-reviewer`, `security-auditor`, `architecture-reviewer`, `db-guardian`, `token-optimizer`, `sentry-debugger` | ✅ |
| `debugger`, `test-engineer`, `refactor-specialist`, `frontend-ui-designer` | ❌ |

`readonly: true` vira `permission.edit=deny` (OpenCode), `sandbox_mode=read-only` (Codex), `readonly: true` (Cursor). Aplicado por `-apply`.

### Skills

**39 skills vendored** (curadas em `skills/manifest.json`, fonte
[rmyndharis/antigravity-skills](https://github.com/rmyndharis/antigravity-skills), MIT), cobrindo backend, arquitetura, segurança, DB, API, linguagens, testes, ops.

**12 skills authored (PT-BR)**: `ddd`, `design-patterns`, `object-calisthenics`, `symfony`, `doctrine`, `phpunit-symfony`, `docs-research`, `context-guard`, `sentry`, `agent-delegate`, `arch-context-check`, `agent-react`.

Total: **51 skills** wiradas via `agent-sync -vendor` + `-apply`.

### Memória compartilhada entre CLIs

`memory-mcp` (SQLite FTS5 + Turso Cloud opcional) + buffer de observação não-bloqueante (`postToolUse`) + consolidação em fim de turno (`Stop`). → Detalhes em [`docs/guides/memory-cross-cli.md`](docs/guides/memory-cross-cli.md).

### Contexto e sessões longas

- **`context-guard`** (skill): zonas de saúde, sinais de drift, reancoragem pós-compactação. **Obrigatória por padrão** em tarefas multi-etapa/sessões longas (referenciada em `rules/global-rules.md`).
- **`STATE.md`**: fonte da verdade para retomar sessões. Baseie-se em `templates/STATE.md`.
- **`context-window-strategy`**: sliding window + resumo incremental (paper de referência + comandos em [`docs/guides/context-window-details.md`](docs/guides/context-window-details.md)).
- **Preservação de Cache KV**: política `CacheAligner` que garante que nudges e resumos sejam injetados apenas na cauda do contexto, preservando até 90% de desconto de prompt caching nos provedores.

### Pesquisa, Grafo de Código e Documentação

- **`docs-research`** (skill): padrão para pesquisar documentações oficiais com citações.
- **`docs-fetch`** (tool): baixa + cacheia docs; offline após `make mirror`. ~65 fontes curadas em `mirror/sources.json`.
- **`repo-map`** (tool): extrai blast radius, chamadas reversas de símbolos, tabelas de banco e comandos de testes cirúrgicos.
- **MCPs** wirados nas 5 CLIs:
  - **`context7`** — documentação atualizada de bibliotecas (remoto por padrão).
  - **`docs`** — local/offline, busca no cache do `docs-fetch`.
  - **`memory`** — memória contínua compartilhada (`memory-mcp`). Detalhes em `docs/guides/memory-cross-cli.md`.
  - **`code-graph`** — análise estrutural de impacto e símbolos via `repo-map --mcp`.
  - **`sentry`** (opcional) — erros e performance via OAuth (`SENTRY_MCP_URL`).

### Especificidades do Cursor

No `-apply` / `-target cursor`, o agent-sync grava:

| Artefato | Caminho |
| --- | --- |
| Regras globais | `~/.cursor/rules/agent-sync-global.mdc` (`alwaysApply: true`) — regras locais em arquivo; **não** confundir com User Rules da UI sincronizadas pela conta |
| Skills | `~/.cursor/skills/<name>/` (symlinks) — **nunca** `~/.cursor/skills-cursor/` (built-ins do Cursor) |
| Subagentes | `~/.cursor/agents/<name>.md` (`model: inherit`, opcional `readonly: true`) |
| Hooks | `~/.cursor/hooks.json` + scripts em `~/.cursor/hooks/` |
| MCP | via `make mcp` → `~/.cursor/mcp.json` (`mcpServers`) |

**Atenção**: hooks de nível de usuário **não** rodam em Cloud Agents; Cloud Agents só carregam `.cursor/hooks.json` do projeto. UI User Rules são sincronizadas pela conta e não têm caminho no sistema de arquivos — o `.mdc` em `~/.cursor/rules/` é o substituto versionável.

> Context7 é hosted: mesmo em modo local (stdio) o servidor chama a API context7.com — não é offline. Para offline real, use o MCP `docs` + `make mirror`.

---

## Como usar

```bash
git clone <repo> ~/Projects/agent-sync && cd ~/Projects/agent-sync
make setup    # instala Go >= 1.24 e pre-requisitos de runtime
make install  # compila e instala binários em ~/.local/bin
make sync     # aplica regras, skills, agentes e hooks nas 5 CLIs (alias de agent-sync -apply)
```

`mr-review-local` em qualquer CLI após `make sync`:

```bash
mr-review-local -repo /path -base upstream/main -head origin/<branch>
```

Retorna JSON com SHAs e patches para revisão cirúrgica do agente.

### Sincronização entre dois PCs

Sincronização de memória entre múltiplos computadores via Turso Cloud (`memory-sync -init` + `agent-sync-session`). → Detalhes em [`docs/guides/sync-between-pcs.md`](docs/guides/sync-between-pcs.md).

### Comandos opcionais

```bash
make mirror           # baixa ~65 documentações oficiais para cache offline
make mcp              # configura MCPs nas CLIs instaladas
agent-sync -status    # verifica o estado de sincronização das CLIs
make vendor           # re-importa skills catalogadas
```

### Variáveis de ambiente

| Variável | Padrão | Aplica-se a | O que faz |
|---|---|---|---|
| `AGENT_SYNC_HOME` | (heurística) | `agent-sync`, `agent-sync-session` | Força o diretório raiz do repositório. Útil quando o binário é chamado de fora do repo (CI, symlinks, testes). Tem prioridade sobre `cwd` e `os.Executable()`. |
| `AGENT_SYNC_DRY_RUN=1` | `0` | `agent-sync -apply` | Habilita modo dry-run (equivalente à flag `-dry-run`/`-n`): mostra o que seria feito sem escrever em disco. Útil para `make apply DRY_RUN=1`. |
| `AGENT_SYNC_PRETOOLUSE_VALIDATE=1` | (não persistido) | hook `shell-validate` | Ativa o hook `shell-validate` (sinaliza comandos shell provavelmente inválidos antes da execução). `make apply` persiste automaticamente em `~/.zshrc`/`~/.bashrc`; sem essa env, o hook é um no-op silencioso. |

---

## Ferramentas inclusas (Inspeção de Baixo Token)

Binários Go instalados via `make install` em `~/.local/bin`:

- `ast-outline <file>` — extrai classes e métodos (Go/Py/TS/PHP), ~90% de economia de tokens.
- `trace-strip <pipe>` — remove ruído de frameworks em stack traces de logs.
- `db-guardian -query "<sql>"` — valida SQL em modo estritamente read-only (bloqueia mutações, `SELECT *`, injeta `LIMIT`). **Não** executa queries.
- `docs-fetch <url>` — baixa e cacheia documentações para busca offline.
- `memory-mcp` — servidor MCP de memória contínua e pipeline de observação.
- `repo-map` — análise de grafo de código, blast radius, chamadas reversas e servidor MCP `code-graph`.
- `ctx-window summarize` — janelamento de contexto e resumo incremental.

Cada binário possui `--help` detalhado.
