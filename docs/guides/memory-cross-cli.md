# Shared memory across CLIs

Detalhes operacionais do `memory-mcp` e dos hooks relacionados ao
compartilhamento de estado entre CLIs (referência expandida, no README
apenas link).

## `memory-mcp` server

- **Binário**: `tools/cmd/memory-mcp`.
- **Storage**: local SQLite (`~/.cache/agent-sync/memory.db`) com sync
  opcional via Turso Cloud (libSQL).
- **Search**: FTS5/BM25 (text relevance). Schema reserva `embedding_json`
  para embeddings semânticas em fase futura.
- **Cobertura MCP**: Claude Code, Codex, Antigravity CLI, OpenCode,
  Cursor — mesma história de decisões/feedback/project context.
- **Build dep**: CGO (`go-libsql`); gcc/clang já é pré-requisito de
  `make`.
- **Tools MCP expostos**:
  - `store_memory` (`scratch: true|false`) — scratch pode ser deletada;
    permanente recusada por design.
  - `search_memory` — busca textual.
  - `get_memory`, `list_memories`, `delete_memory` (somente scratch).

## `memory-nudge` hook

Captura de memórias depende da disciplina do modelo (sem trigger
automático). `-apply` instala nudge em harness-level a cada N tool
calls/invocations (default 25, `AGENT_SYNC_MEMORY_NUDGE_THRESHOLD`).

**Cobertura cross-CLI** (`hooks/memory-nudge.{sh,antigravity.sh,opencode.ts,cursor.sh}`):

| CLI | Hook event | Local |
| --- | --- | --- |
| Claude Code | `PostToolUse` | merged into `~/.claude/settings.json` |
| Codex | `PostToolUse` | merged into `~/.codex/hooks.json`; `protect-mcp` adaptado |
| Antigravity CLI | `PreInvocation` | merged into `~/.gemini/config/hooks.json` |
| OpenCode | `tool.execute.after` plugin | `~/.config/opencode/plugins/memory-nudge.ts` (best-effort, [#13574](https://github.com/anomalyco/opencode/issues/13574)) |
| Cursor | `postToolUse` | `~/.cursor/hooks.json` + `additional_context` |

**Failure tracking**: `~/.cache/agent-sync/hooks/errors.jsonl` (redaction
+ rotação). `agent-sync -observability` mostra summary.

## Skills relacionados (referência rápida)

- **`agent-delegate`** (`skills/agent-delegate/SKILL.md`): critérios para
  delegar task a outro CLI/model baseado em permission-friction profile
  (`print` vs `session`; headless default OpenCode). Preferir
  `delegate-run` (`scripts/delegate-run.sh`) por log/manifest/tmux.
  Limitações: Claude Code não orquestra `agy` em headless.
- **`agent-react`** (`skills/agent-react/SKILL.md`): disciplina ReAct
  (Thought → Action → Observation) — anti-loop, tool budget, cited
  evidence. Complementa `context-guard` e `debugging-strategies` sem
  substituir.
- **`agent-react-nudge`** hook: reminder a cada N tool calls (default 15,
  `AGENT_SYNC_REACT_NUDGE_THRESHOLD`) para validar hipóteses ativas.
  OpenCode best-effort (#13574). Não garante correção — reforça
  hipótese ≠ fato.
- **`arch-context-check`** (`skills/arch-context-check/SKILL.md`):
  checklist obrigatório antes de propor arquitetura/Clean Code/DDD/
  design patterns — cruza com `memory-mcp` + código real.
