# Agent-Sync

> [English](README.md) · [Português](README.pt-BR.md)

Versiona, mantém, e sincroniza **Global Rules**, **Skills**, **Agents** e **Tools** entre 5 CLIs:
- **Claude Code** (`~/.claude`)
- **Codex / OpenAI** (`~/.codex`)
- **Google Antigravity CLI** (`~/.gemini`) — successor to the now-deprecated standalone Gemini CLI (retired 2026-06-18 for non-enterprise accounts)
- **OpenCode** (`~/.config/opencode`)
- **Cursor** (`~/.cursor`) — IDE + Agent CLI (`agent` / `cursor-agent`)

---

## 📦 Repository Structure

```text
agent-sync/
├── rules/
│   └── global-rules.md     # Global rules (Clean Code, OWASP, Anti-Hallucination, Data Guardians)
├── skills/
│   ├── manifest.json       # Curation: IDs + source (rmyndharis/antigravity-skills, MIT) used by `-vendor`
│   ├── token-saving-toolkit/ # Guidance for concise reading via AST and log trimming
│   ├── mcp-advisor/          # MCP adoption assessment
│   └── <41 skills>/          # Vendored from the catalog (backend, security, DB, API, languages, tests, ops, context)
├── agents/                 # Authored specialist agents (canonical), generated per CLI on `-apply`
│   ├── spec-planner.md, code-reviewer.md, security-auditor.md, debugger.md
│   └── architecture-reviewer.md, test-engineer.md, refactor-specialist.md, db-guardian.md, token-optimizer.md, sentry-debugger.md, mr-reviewer.md
├── tools/                  # High-speed Go utility binaries
│   ├── cmd/ast-outline/    # Extracts classes/methods instead of reading whole files (Go, Python, TS, PHP)
│   ├── cmd/trace-strip/    # Removes framework noise from error logs
│   ├── cmd/db-guardian/    # Validates read-only SQL (blocks mutations, injects LIMIT); does NOT run queries
│   ├── cmd/docs-fetch/     # Downloads/caches docs and extracts text or heading outline (low token)
│   ├── cmd/docs-mcp/       # Local (offline) MCP server over the docs cache
│   ├── cmd/memory-mcp/     # Shared memory MCP server and pipeline (SQLite FTS5 + ephemeral buffer)
│   └── cmd/repo-map/       # Bounded code graph (blast radius, callers, code-graph MCP server)
├── mirror/sources.json     # Curated official sources to sync into the local cache
├── templates/STATE.md      # Session handoff template (checkpoint/resume)
├── templates/skill.md.tpl  # Template for project-local skills (`agent-sync skills new`)
├── cmd/agent-sync/         # CLI sync orchestrator (+ skill vendor + agent generator)
├── scripts/setup-go.sh     # Go (>= 1.24) bootstrap via mise or official tarball
├── scripts/setup-mcp.sh    # Configures Context7, local docs MCP, and Sentry (optional)
├── Makefile                # Automation commands
├── docs/                    # Architecture Decision Records (ADRs) & Multi-CLI Hook Patterns
└── README.md / README.pt-BR.md  # docs (EN default, PT-BR)
```

### Specialist agents (em `agents/*.md`)

| Agente | Read-only |
| --- | --- |
| `spec-planner`, `code-reviewer`, `mr-reviewer`, `security-auditor`, `architecture-reviewer`, `db-guardian`, `token-optimizer`, `sentry-debugger` | ✅ |
| `debugger`, `test-engineer`, `refactor-specialist` | ❌ |

`readonly: true` vira `permission.edit=deny` (OpenCode), `sandbox_mode=read-only` (Codex), `readonly: true` (Cursor). Aplicado por `-apply`.

### Skills

**39 skills vendored** (curados em `skills/manifest.json`, fonte
[rmyndharis/antigravity-skills](https://github.com/rmyndharis/antigravity-skills), MIT), cobrindo backend, arquitetura, segurança, DB, API, linguagens, testes, ops.

**12 skills authored (PT-BR)**: `ddd`, `design-patterns`, `object-calisthenics`, `symfony`, `doctrine`, `phpunit-symfony`, `docs-research`, `context-guard`, `sentry`, `agent-delegate`, `arch-context-check`, `agent-react`.

Total: **51 skills** wiradas via `agent-sync -vendor` + `-apply`.

### Shared memory across CLIs

`memory-mcp` (SQLite + Turso Cloud opcional) + nudge hooks + skills
relacionados. → Detalhes em [`docs/guides/memory-cross-cli.md`](docs/guides/memory-cross-cli.md).

### Context and long sessions

- **`context-guard`** (skill): health zones, drift signals, post-compaction re-anchoring. **Mandatory by default** on multi-step tasks/long sessions (referenced in `rules/global-rules.md`).
- **`STATE.md`**: source of truth to resume sessions. Base it on `templates/STATE.md`.
- **context-window-strategy**: sliding window + incremental summary (paper de referência + comandos em [`docs/guides/context-window-details.md`](docs/guides/context-window-details.md)).
- Hooks de nudge/guard/docs-cache instalados em runtime por `-apply` (não listados aqui — cada um em sua ADRs ou `hooks/<name>.sh`).
### Research and documentation

- **`docs-research`** (skill): standard para pesquisar docs oficiais com citações.
- **`docs-fetch`** (tool): baixa + cachea docs; offline após `make mirror`. ~65 fontes curadas em `mirror/sources.json` (Symfony, Doctrine, PHP, Go, JS/TS, Java, Rust, Python, Docker, K8s, Terraform, PostgreSQL, etc).
- **MCPs** wirados em todas as 5 CLIs:
  - **`context7`** — docs atualizados de libs. Remote por default; local/stdio com `CONTEXT7_LOCAL=1` (mas ainda chama API context7.com, não offline).
  - **`docs`** — local/offline, busca no cache do `docs-fetch`.
  - **`memory`** — memória compartilhada (`memory-mcp`). Detalhes em `docs/guides/memory-cross-cli.md`.
  - **`code-graph`** — structural impact analysis & callers via `repo-map --mcp`.
  - **`sentry`** (opcional) — errors/performance via OAuth. URL via `SENTRY_MCP_URL=https://mcp.sentry.dev/mcp/<org>/<proj>`.

> Context7 é hosted: mesmo em modo local (stdio) chama a API context7.com. Para offline real, use o MCP `docs` + `make mirror`.

### Cursor specifics

On `-apply` / `-target cursor`, agent-sync writes:

| Artifact | Path |
| --- | --- |
| Global rules | `~/.cursor/rules/agent-sync-global.mdc` (`alwaysApply: true`) — local file rules; do **not** confuse with account-synced User Rules in the UI |
| Skills | `~/.cursor/skills/<name>/` (symlinks) — **never** `~/.cursor/skills-cursor/` (Cursor built-ins) |
| Subagents | `~/.cursor/agents/<name>.md` (`model: inherit`, optional `readonly: true`) |
| Hooks | `~/.cursor/hooks.json` + scripts in `~/.cursor/hooks/` |
| MCP | via `make mcp` → `~/.cursor/mcp.json` (`mcpServers`) |

Caveats: user-level hooks do **not** run on Cloud Agents; Cloud Agents only pick up project `.cursor/hooks.json`. UI User Rules are account-synced and have no filesystem path — the `.mdc` under `~/.cursor/rules/` is the versionable stand-in.

> Context7 is hosted: even in local mode (stdio) the server calls the context7.com API — it is not offline. For true offline, use the `docs` MCP + `make mirror`.

---

## How to use

```bash
git clone <repo> ~/Documents/agent-sync && cd ~/Documents/agent-sync
make setup    # instala Go >= 1.24 (mise ou tarball oficial; sem sudo)
make install  # compila e instala em ~/.local/bin
make sync     # wire de regras+skills+agents nas 5 CLIs (equivalente: agent-sync -apply)
```

`mr-review-local` em qualquer CLI após `make sync`:

```bash
mr-review-local -repo /path -base upstream/main -head origin/<branch>
```

Retorna JSON com SHAs + patch para o agente revisar. `-fetch` atualiza remotes nomeados (sem checkout/merge). Patches gigantes são omitidos e marcados parciais.

### Sync between two PCs

Sincronização de memória entre múltiplos PCs via Turso Cloud (`memory-sync -init` + `agent-sync-session`). → Detalhes em [`docs/guides/sync-between-pcs.md`](docs/guides/sync-between-pcs.md).

### Comandos opcionais

```bash
make mirror       # baixa ~65 docs oficiais no cache local offline
make mcp          # Context7 (remote) + docs MCP local
agent-sync -status    # estado do wirar nas CLIs
make vendor       # re-importa 39 skills curated do catalog
```

### Variáveis de ambiente

| Variável | Default | Efeito |
| --- | --- | --- |
| `AGENT_SYNC_HOME` | (heuristic) | Força o repo root (CI, symlinks, tests). |
| `AGENT_SYNC_DRY_RUN=1` | `0` | `-apply` em dry-run. |
| `AGENT_SYNC_PRETOOLUSE_VALIDATE=1` | (não persistido) | Ativa hook `shell-validate`.

### Environment variables

| Variable | Default | Applies to | What it does |
|---|---|---|---|
| `AGENT_SYNC_HOME` | (heuristic) | `agent-sync`, `agent-sync-session` | Forces the repository root path. Useful when the binary is called from outside the repo (CI, symlinks, tests). Takes priority over `cwd` and `os.Executable()`. |
| `AGENT_SYNC_DRY_RUN=1` | `0` | `agent-sync -apply` | Enables dry-run mode (equivalent to the `-dry-run`/`-n` flag): shows what would be done without writing to disk. Useful for `make apply DRY_RUN=1`. |
| `AGENT_SYNC_PRETOOLUSE_VALIDATE=1` | (not persisted) | `shell-validate` hook | Activates the `shell-validate` hook (flags likely-invalid shell commands before execution). `make apply` persists it automatically in `~/.zshrc`/`~/.bashrc`; without this env, the hook is a silent no-op. |

---

## Included tools (low-token inspection)

Binários Go wirados via `make install` em `~/.local/bin`:

- `ast-outline <file>` — extrai classes/métodos (Go/Py/TS/PHP), ~90% economia de tokens.
- `trace-strip <pipe>` — remove frames de stack trace de vendor/framework.
- `db-guardian -query "<sql>"` — valida SQL em modo read-only (bloqueia mutações, `SELECT *`, injeta `LIMIT`). **Não** roda queries.
- `docs-fetch <url>` — baixa + cachea docs (offline após `make mirror`).
- `memory-mcp` — servidor MCP de memória compartilhada (veja `docs/guides/memory-cross-cli.md`).
- `repo-map` — bounded code graph, blast radius, reverse callers and `code-graph` MCP server.
- `ctx-window summarize` — janelamento de contexto (veja `docs/guides/context-window-details.md`).

Cada binário tem `--help` próprio.
