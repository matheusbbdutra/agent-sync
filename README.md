# Agent-Sync 🚀

> 🇺🇸 [English](README.md) · 🇧🇷 [Português](README.pt-BR.md)

Unified repository to version, maintain, and synchronize **Global Rules**, **Skills**, **Specialist Agents**, and **Low-Token Tools** across multiple AI agent ecosystems:
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
│   └── cmd/docs-fetch/     # Downloads/caches docs and extracts text or heading outline (low token)
│   └── cmd/docs-mcp/       # Local (offline) MCP server over the docs cache
├── mirror/sources.json     # Curated official sources to sync into the local cache
├── templates/STATE.md      # Session handoff template (checkpoint/resume)
├── cmd/agent-sync/         # CLI sync orchestrator (+ skill vendor + agent generator)
├── scripts/setup-go.sh     # Go (>= 1.24) bootstrap via mise or official tarball
├── scripts/setup-mcp.sh    # Configures Context7, local docs MCP, and Sentry (optional)
├── LICENSE / NOTICE        # MIT license and third-party skill attribution
├── Makefile                # Automation commands
└── README.md / README.pt-BR.md  # docs (EN default, PT-BR)
```

### Specialist agents

Defined once in `agents/*.md` (frontmatter `name`, `description`, optional `readonly`) and **generated in each CLI's native format** during `-apply`:

| Agent | Role | Read-only |
| --- | --- | --- |
| `spec-planner` | Understands the request and plans before coding | ✅ |
| `mr-reviewer` | Reviews local Git refs with evidence of regressions, security, and impact | ✅ |
| `code-reviewer` | Clean Code, SOLID, Calisthenics, and security | ✅ |
| `security-auditor` | OWASP, injection, XSS, secrets | ✅ |
| `debugger` | Root cause with hypotheses and evidence | ❌ |
| `architecture-reviewer` | Boundaries, DDD, coupling, ADRs | ✅ |
| `test-engineer` | TDD red-green-refactor | ❌ |
| `refactor-specialist` | Incremental, test-guided refactoring | ❌ |
| `db-guardian` | Read-only SQL, LIMIT, PII | ✅ |
| `token-optimizer` | Low-token inspection (ast-outline/trace-strip) | ✅ |
| `sentry-debugger` | Sentry issue triage and root cause | ✅ |

> "Read-only" becomes `permission.edit=deny` in OpenCode, `sandbox_mode=read-only` in Codex, and `readonly: true` in Cursor; elsewhere it is enforced by the prompt.

### Skills vendored from the catalog

Curated by domain in `skills/manifest.json` and imported from [rmyndharis/antigravity-skills](https://github.com/rmyndharis/antigravity-skills) (MIT):

| Domain | Skills |
| --- | --- |
| Backend/quality | `error-handling-patterns`, `code-refactoring-refactor-clean`, `dependency-management-deps-audit`, `codebase-cleanup-tech-debt`, `legacy-modernizer`, `nodejs-backend-patterns`, `code-reviewer`, `debugging-strategies` |
| Architecture | `architecture-patterns`, `architect-review`, `architecture-decision-records` |
| Distributed arch. | `microservices-patterns`, `cqrs-implementation`, `event-sourcing-architect` |
| Security | `auth-implementation-patterns`, `security-auditor`, `sast-configuration`, `backend-security-coder`, `frontend-security-coder` |
| Database | `sql-optimization-patterns`, `database-optimizer`, `database-migrations-sql-migrations`, `postgresql` |
| API/Docs | `openapi-spec-generation`, `api-documenter`, `api-design-principles` |
| Languages | `php-pro`, `python-pro`, `golang-pro`, `go-concurrency-patterns`, `javascript-pro`, `typescript-pro` |
| Testing | `python-testing-patterns`, `javascript-testing-patterns`, `e2e-testing-patterns`, `tdd-orchestrator` |
| Ops/Infra | `incident-response-smart-fix`, `postmortem-writing` |
| Web research | `search-specialist` |
| Context | `context-manager`, `context-management-context-save` |

In addition to the vendored ones, there are **12 authored skills in Portuguese (PT-BR)** (not present in the catalog):
`ddd`, `design-patterns`, `object-calisthenics`, `symfony`, `doctrine`, `phpunit-symfony`, `docs-research`, `context-guard`, `sentry`, `agent-delegate`, `arch-context-check`, and `agent-react`.

### Shared memory across CLIs

- **`memory-mcp`** (`tools/cmd/memory-mcp`): local MCP server over libSQL (`~/.cache/agent-sync/memory.db`, with optional persistent memory sync via Turso Cloud; local FTS5 search) giving Claude Code, Codex, agy, OpenCode, and Cursor access to the same history of decisions/feedback/project context. Requires CGO (`go-libsql`) — accepted as fine for personal use (gcc/clang is already a `make` prerequisite).
- Search today is **FTS5/BM25** (text relevance), no real embeddings yet — the schema already reserves a vector column (`embedding_json`) for a future semantic-search phase.
- Exposed MCP tools: `store_memory` (accepts `scratch: true|false`), `search_memory`, `get_memory`, `list_memories`, `delete_memory` (only removes memories stored with `scratch: true` — permanent ones are refused by design). Provenance enum includes `cursor`.
- **`memory-nudge` hook** (`hooks/memory-nudge.sh` / `.antigravity.sh` / `.opencode.ts` / `.cursor.sh`): capturing memories today depends entirely on the model's self-discipline (no automatic trigger), so `-apply` also installs a harness-level nudge that fires every N tool calls/invocations (default 25, `AGENT_SYNC_MEMORY_NUDGE_THRESHOLD`) asking whether anything from the session should be saved via `store_memory`. Same install mechanism as `context-guard-nudge` (separate counter/threshold), covering all 5 targets:
  - **Claude Code**: `PostToolUse` hook merged into `~/.claude/settings.json`.
  - **Codex**: `PostToolUse` hook merged into `~/.codex/hooks.json`; `protect-mcp` commands are adapted to Codex's native `PreToolUse`/`PostToolUse` schema during synchronization.
  - Hook failures are persisted locally in `~/.cache/agent-sync/hooks/errors.jsonl` (with redaction and rotation); `agent-sync -observability` displays a summary.
  - **Antigravity CLI**: `PreInvocation` hook merged into `~/.gemini/config/hooks.json`.
  - **OpenCode**: `tool.execute.after` plugin copied to `~/.config/opencode/plugins/memory-nudge.ts`. Same best-effort caveat as `context-guard-nudge` ([anomalyco/opencode#13574](https://github.com/anomalyco/opencode/issues/13574)).
  - **Cursor**: `postToolUse` command hook in `~/.cursor/hooks.json` (script under `~/.cursor/hooks/`, output uses native `additional_context`).
- **`agent-delegate`** (skill): criteria for deciding whether/to which CLI-model to delegate a task, using permission-friction profiles (`print` vs `session`; default headless target: OpenCode). Prefer `delegate-run` (`scripts/delegate-run.sh`, installed by `make install`) for log/manifest/tmux instead of raw Bash. Always check `memory-mcp` before building the delegated prompt. Documented limitation: Claude Code cannot orchestrate `agy` in headless mode.
- **`agent-react`** (skill): disciplines the ReAct loop (Thought → Action → Observation) on multi-step tasks — anti-loop, tool budget, and cited evidence; complements `context-guard` / `debugging-strategies` without replacing them.
- **`agent-react-nudge` hook** (`hooks/agent-react-nudge.sh` / `.antigravity.sh` / `.opencode.ts` / `.cursor.sh`): reminder every N tool calls/invocations (default 15, `AGENT_SYNC_REACT_NUDGE_THRESHOLD`) to validate active hypotheses before concluding/implementing. Same install mechanism as the other nudges; OpenCode is best-effort ([anomalyco/opencode#13574](https://github.com/anomalyco/opencode/issues/13574)). Does not semantically guarantee correctness — only reinforces hypothesis ≠ fact.
- **`arch-context-check`** (skill): mandatory checklist before suggesting architecture/Clean Code/DDD/design patterns — cross-references the specialized skills (`ddd`, `design-patterns`, `object-calisthenics`, `architecture-patterns`) with prior decisions in `memory-mcp` and the actual code before giving a suggestion.

### Context and long sessions

- **`context-guard`** (skill): health zones, drift signals, post-compaction re-anchoring, and checkpointing. **Mandatory by default** on multi-step tasks/long sessions (referenced in `global-rules`, at the top and in the final reaffirmation).
- **`STATE.md`**: source of truth to resume sessions. Base it on `templates/STATE.md`; keep it in the project.
- Low-token tools: `ast-outline`, `trace-strip`, `docs-fetch`, `docs-mcp`, `memory-mcp`, `db-guardian` (see `token-saving-toolkit`).
- **`context-guard` hook** (`hooks/context-guard-nudge.sh`): since the skill above only relies on the model's self-discipline, `-apply` also installs a real harness-level nudge that fires every N tool calls (default 40, `AGENT_SYNC_NUDGE_THRESHOLD`) reminding to load `context-guard` and update `STATE.md`. Coverage per CLI:
  - **Claude Code**: `PostToolUse` hook merged into `~/.claude/settings.json` (existing hooks/keys are preserved, idempotent re-apply).
  - **Codex**: `PostToolUse` hook merged into `~/.codex/hooks.json`.
  - **Antigravity CLI**: `PreInvocation` hook merged into `~/.gemini/config/hooks.json` — a different top-level shape (no `hooks` wrapper key: `{"<hook-name>": {"<Event>": [...]}}`) and a different payload than the old Gemini CLI. Uses the event's native `invocationNum` counter instead of keeping its own state file.
  - **OpenCode**: `tool.execute.after` plugin copied to `~/.config/opencode/plugins/context-guard-nudge.ts`. **Best-effort**: OpenCode has an open upstream issue ([anomalyco/opencode#13574](https://github.com/anomalyco/opencode/issues/13574)) where output mutations from this hook aren't always reflected back to the model — the reminder may not reliably reach it.
  - **Cursor**: `postToolUse` in `~/.cursor/hooks.json` returning `additional_context` (native Cursor schema; scripts live under `~/.cursor/hooks/` and are referenced as `./hooks/...`).
- **`bash-guardian`** (`hooks/bash-guardian-patterns.txt`): asks for confirmation before running commands matching known-risky patterns (`rm -rf`, `dd` to a device, `chmod -R 777`, `curl | sh`, `git push --force`, `git reset --hard`, `shutdown`, etc.). Default behavior is always **ask**, never a silent deny. Coverage per CLI:
  - **Claude Code**: patterns added to the native `permissions.ask` list in `settings.json` (e.g. `Bash(rm -rf *)`).
  - **OpenCode**: merged into `permission.bash` in `~/.config/opencode/opencode.json`, each set to `"ask"`. Since OpenCode resolves `permission.bash` by the **last matching rule** (order-sensitive), the merge preserves the original key order of anything already in the file and only appends/reorders the guard's own entries at the end — never re-sorts unrelated keys. **Note**: if a pattern already existed with a different value (e.g. a user-set `"allow"`), the guard overwrites it to `"ask"` — that's the point of the guardrail, but worth knowing before running `-apply` on an existing config.
  - **Antigravity CLI**: `PreToolUse` hook (`hooks/bash-guardian.antigravity.sh`) returning `{"decision":"ask"}` on a match.
  - **Cursor**: `beforeShellExecution` hook (`hooks/bash-guardian.cursor.sh`) returning `{"permission":"ask"}` on a match — Cursor supports interactive confirmation natively here.
  - **Codex**: **not implemented**. Its `PreToolUse` hook only supports binary `allow`/`deny` — `"ask"` is explicitly documented as "parsed but not supported yet." Rather than silently downgrade to `deny` (blocking real work) or `allow` (no protection), Codex is left out of the guard until upstream ships interactive confirmation from hooks.
- **`docs-cache`** (`tools/cmd/docs-cache-write` + `hooks/docs-cache*`): passively caches documentation the agent already looked up via `WebFetch`/`read_url_content` or `context7` (`query-docs` only — `resolve-library-id` is metadata, not doc content), without re-fetching over the network. Grows the offline `docs-fetch` cache organically as libraries get consulted, on top of the manually curated `mirror/sources.json`. `context7` results are cached under a synthetic key (`context7:/<libraryId>/<query>`). Before caching, content passes through `tools/internal/secretscan` (regex-only, no LLM call) that redacts obvious secrets (AWS/GitHub/Slack keys, JWTs, private-key blocks) — see `STATE.md` guidance in `context-guard` for the equivalent discipline where no automatic guardrail exists. Coverage per CLI:
  - **Claude Code / Codex**: shared `PostToolUse` hook (`hooks/docs-cache.sh`), matcher `WebFetch|mcp__context7__.*`. Codex has no full-page fetch tool, so only the `context7` half applies there.
  - **Antigravity CLI**: `PostToolUse` hook (`hooks/docs-cache.antigravity.sh`) matching `read_url_content|call_mcp_tool`. Antigravity's hook payload doesn't include the tool's result — the script reads it from the documented `transcriptPath` (JSONL), locating the entry at `step_index + 1`, confirmed by live-testing against a real `agy` session. The `read_url_content` argument name (`Url`) is a reasonable inference from that tool's PascalCase convention, **not confirmed live** (permission testing for it was blocked by this session's own safety classifier before confirmation).
  - **OpenCode**: `tool.execute.after` plugin (`hooks/docs-cache.opencode.ts`), same best-effort caveat as `context-guard-nudge.ts`.
  - **Cursor**: `postToolUse` matcher `WebFetch` + `afterMCPExecution` matcher `query-docs` (`hooks/docs-cache.cursor.sh` / `docs-cache-mcp.cursor.sh`).

### Research and documentation

- **`docs-research`** (skill): standard for researching official sources (Symfony, Doctrine, PHP, Go, JS/TS…) with citations and low token usage.
- **`docs-fetch`** (tool): downloads and caches docs; query online or offline.
  ```bash
  make mirror                    # sync mirror/sources.json (~65 official sources) into the cache
  docs-fetch -outline https://www.doctrine-project.org/projects/doctrine-orm/en/current/reference/basic-mapping.html
  docs-fetch -search "lazy loading"   # offline search in the cache
  docs-fetch -list                    # cached docs
  ```
  Sources curated in `mirror/sources.json`: Symfony, Doctrine (ORM/DBAL/Collections/Migrations), PHP, PSR, PHPUnit, Composer, Go, TypeScript, JavaScript/MDN, Node, React, Vue, Next.js, Tailwind, Vite, Laravel, Rails, Python, Django, FastAPI, Spring Boot, .NET/C#, Rust, Elixir, Vitest, Jest, Playwright, PostgreSQL, SQLite, MariaDB, MongoDB, Redis, Kafka, Elasticsearch, RabbitMQ, Docker, Kubernetes, Terraform, Nginx, Git, ESLint, OWASP.
- **MCPs** in the 5 targets:
  - `context7` — up-to-date library docs. **Remote** by default; **local/stdio** with `CONTEXT7_LOCAL=1`.
    ```bash
    make mcp                                   # remote
    CONTEXT7_LOCAL=1 make mcp                  # local (stdio, via npx — still requires internet)
    CONTEXT7_API_KEY=xxx make mcp              # higher limits (not stored in the repo)
    ```
  - `docs` — **local offline MCP** (`docs-mcp`) that searches the `docs-fetch` cache. Works without internet after `make mirror`.
  - `memory` — shared local memory (`memory-mcp`) across all CLIs including Cursor (`~/.cursor/mcp.json`).
  - `sentry` — Sentry errors/performance (**optional**, OAuth). Set the URL with org/project:
    ```bash
    SENTRY_MCP_URL=https://mcp.sentry.dev/mcp/<org>/<proj> make mcp
    ```

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

## ⚡ How to Use on Any Machine

### 1. Clone the repository
```bash
git clone <your-repo-url> ~/Documents/agent-sync
cd ~/Documents/agent-sync
```

### 2. Ensure Go (>= 1.24)
```bash
make setup
```
Checks the available `go` and, if missing/outdated, installs the version required by the `go.mod` files via `mise` (if available) or the official tarball under `~/.local` — no `sudo`. The `toolchain` directive in `go.mod` pins the suggested minimum version (`go1.24.13`).

### 3. Build and Install everything
```bash
make install
```
This builds the Go binaries (`agent-sync`, `ast-outline`, `trace-strip`, `db-guardian`, `docs-fetch`, `docs-mcp`, `memory-mcp`, `mr-review-local`, `memory-sync`) and places them in `~/.local/bin/`, along with `agent-sync-session`.

### 4. Sync with all CLIs
```bash
make sync
# or directly:
agent-sync -apply
```

Use `mr-reviewer` in any CLI after `make sync`. You can also run the local collector directly:

```bash
mr-review-local -repo /path/to/checkout -base upstream/main -head origin/test-branch
# add -fetch to update the remotes named by those refs
```

The command returns JSON with commit SHAs and a patch for the agent to review; `-fetch` does not check out or merge. Oversized patches are omitted and marked as partial.

### Sync between two PCs

On each PC, run `memory-sync -init`. It creates `~/.config/agent-sync/config.json` if missing and prints its path. Set `turso.url` to the `libsql://...` URL of the same Turso Cloud database and `turso.token` to this PC's token. The file is created with `0600` permissions, is never overwritten, and stays outside the repository. For projects without a Git remote, configure a stable ID in the `projects` map, using each PC's local path as the key:

```json
{
  "turso": { "url": "libsql://your-database.turso.io", "token": "your-token" },
  "projects": {
    "/home/you/projects/app": "main-app"
  }
}
```

Projects with an `origin` or `upstream` remote automatically use an ID derived from that remote. The MCP records the PC, local path, and project ID; searches can use `project_dir` to resolve the same project across different PC paths.

```json
{
  "turso": {
    "url": "libsql://your-database.turso.io",
    "token": "your-token"
  }
}
```

```bash
memory-sync -init
agent-sync-session ~/Documents/agent-sync codex
# or: agent-sync-session ~/Documents/agent-sync claude
```

The wrapper requires a clean checkout, runs `git pull --ff-only`, refreshes all five CLI installations when the repository changes, and downloads memories before starting the CLI. When the CLI exits, it uploads persistent memories and pushes commits made during the session. It does not commit uncommitted changes. The local SQLite database stays at `~/.cache/agent-sync/memory.db`; scratch memories are excluded. For manual use, run `memory-sync -phase start` before and `memory-sync -phase end` after a session. On conflict, choose a version using `memory-sync -phase resolve-local -conflict <id>` or `memory-sync -phase resolve-remote -conflict <id>`, then retry sync. The Turso Cloud connection still needs validation against your account.

### 5. Documentation MCPs (optional)
```bash
make mirror                    # download official docs into the offline cache
make mcp                       # Context7 (remote) + local offline 'docs' MCP
CONTEXT7_LOCAL=1 make mcp      # Context7 in local mode (stdio)
```

### 6. Check CLI status
```bash
agent-sync -status
```

### 7. Re-import the curated skills from the catalog
```bash
make vendor
# or directly (source configurable with -source):
agent-sync -vendor
```

> Repository root resolution can be forced with the `AGENT_SYNC_HOME` variable.

---

## 🛡️ Included Tools
- **`ast-outline <file>`**: Generates the class/function structure with matching line numbers, saving up to 90% of context tokens.
- **`trace-strip <file_or_pipe>`**: Hides irrelevant stack trace frames from external libraries.
- **`db-guardian -list`** / **`-profile <db> -query "<sql>"`**: Validates queries in read-only mode (blocks mutations, warns about `SELECT *`, and injects `LIMIT`). **It does not open a connection or run the query**; the profile is only for context. Credentials support environment indirection (`${DB_PASSWORD}`).
- **`docs-fetch <url>`**: Downloads and caches a docs page and extracts text (`-outline`, `-grep`, `-raw`, `-refresh`) for low-token lookups.
