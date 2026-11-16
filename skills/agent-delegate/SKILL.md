---
name: agent-delegate
description: "Criteria and workflow for when to delegate tasks to subagents/CLIs with isolated context, according to complexity, tokens and output volume."
---

# Delegation between agents/CLIs

Answer in PT-BR, objective (global AGENTS.md). This skill does not create automatic orchestration — it is a checklist to decide whether/where/how to delegate. Supporting infra: shared memory (`memory-mcp`) + host's native subagents + print/session modes that each CLI already exposes.

## Native subagents (isolate context)

Inside Pi, the default path is the `subagent` tool — `delegate-run` remains for crossing CLI. Rule of thumb: **if the output that would come back to the chat is larger than the brief, delegate**.

| Point | Agent | Parallel | Return to the parent |
| --- | --- | --- | --- |
| Web/docs research, compare proposals | `researcher` | 2-4 | `research.md` + summary ≤1.5k tokens |
| Map modules/files | `scout` | 1 per seam | `context.md` with `file:line` |
| Implement 1 point | `worker` | no (1 writer/cwd) | diff + how it validated |
| Review diff/plan | `reviewer` | yes (fresh context) | findings by severity |
| Voluminous output (tests/logs) | any | yes | only verdict + artifact |

Pipeline: **PLAN** (parent) → **RESEARCH∥** → **RECON∥** → **IMPLEMENT** (serial) → **REVIEW** (fresh) → **ACCEPT** (parent).

Minimum brief: objective · cwd/ref · authority (can/cannot edit) · evidence · success criterion · validation · output artifact · when to stop/ask.

Return contract: status, summary ≤1.5k tokens, artifact path, evidence (`file:line` / command). Never paste the whole log — the parent reads the artifact.

A subagent **does not inherit skills nor the global AGENTS.md** by default: embed the critical restrictions in the brief or turn on `inheritGlobalContext`/`inheritSkills` on the agent.

## Execution modes

| Mode | When | How |
| --- | --- | --- |
| **`print`** | mechanical task, easy to audit, permissive target, no expectation of a permission prompt | `claude -p`, `opencode run`, `agent -p`, and only exceptionally `agy -p` |
| **`session`** | target will ask for permission, long task, or you may need to intervene | **interactive** CLI (e.g.: `agy -i` / `--prompt-interactive`, or detachable tmux session). The orchestrator **does not** block on the TTY — it tells the user to attach if they need to approve |

Do not treat all CLIs as equivalent in the “run via Bash” step. The decisive criterion is the harness's **permission friction profile**, not only the model.

## Permission profile per target

| Target | Profile | Print command | Use in delegation |
| --- | --- | --- | --- |
| **OpenCode** | permissive (deny-list; only dangerous patterns ask `ask`) | `opencode run "<message>"` | **default** for headless / mechanical work |
| **Cursor Agent** | medium (`--force` / `--yolo` only if the user authorizes) | `agent -p "<prompt>"` (`--workspace <path>` if needed) | ok in a trusted workspace |
| **Claude Code** | medium/high + safety classifier | `claude -p "<prompt>"` | ok in print; **do not** orchestrate `agy` from it |
| **Antigravity (agy)** | narrow allow-list + sandbox; approving once **does not** generalize (almost literal matching) | `agy -p` / `--print` | **do not** use print by default → `session` mode. Print only if the allowlist already covers exactly the task's tools/commands |
| **Codex** | medium | Codex's non-interactive mode | ok if the task fits in its sandbox |

### Why agy “asks again”

In the Antigravity CLI, `permissions.allow` is usually a list of narrow entries (`command(ls)`, long literal command, specific MCP). Variation in flag/path/tool → new prompt. In `--print` there is no human → it denies and breaks the delegation. OpenCode, by contrast, lets through what is not in the deny-list — that is why it is the default target for automatic delegation.

### Limitation: Claude Code → agy headless

Tested on 2026-09-12: orchestrating `agy --print` (or with `--dangerously-skip-permissions` / `--mode accept-edits`) from Claude Code runs into Claude's own classifier (“Create Unsafe Agents”), not only agy's. Do not insist with flag variations in the same session — the classifier escalates the block. If the user wants agy in this situation: open interactive agy (`session` mode) or delegate via OpenCode.

`--dangerously-skip-permissions` / `--mode accept-edits` in agy: only with the user in front of it and aware — **never** as the default of this skill.

## Before delegating

1. Consult `memory-mcp` (`search_memory` / `list_memories`) and embed the context in the target's prompt.
2. **Calibrate historical cost** via `agent-sync event stats --actor <cli> --last N` (ADR-003). Without history, the decision "which CLI is cheaper" is blind — the default textual heuristic can send it to `claude` when `opencode` has already shown it costs 3x less for the same task class. Prefer `agent-sync event stats --actor claude --last 50` before choosing a target based on model.
3. Choose the target by **permission profile** (table above), then by model/cost.
4. Choose `print` vs `session` mode according to the table.
5. Default when the task is mechanical and the target does not matter: **`opencode run`**.
6. **If the task is in a language/stack covered by a shared skill** of this skills repo (e.g.: `backend`, `object-calisthenics`, `architecture`) — embed the relevant content of that skill in the delegation prompt, not just the bare task. The target (different CLI) does not necessarily carry these skills automatically; embedding guarantees it follows the same quality standards this session would follow. **Status: to be tested** — not yet validated in real delegation (pending since 2026-09-17); in the next Go task delegation, read `backend/reference/go.md` in full and paste the relevant sections (not only the name) in the `delegate-run` prompt, and record the result here.

When recording memories: `agent` = `claude-code` | `codex` | `antigravity` | `opencode`.

## Criteria to decide whether to delegate

1. **Risk/reversibility.** Mechanical and easy to verify → it can go to a cheaper model (preferably OpenCode in print). Architecture, security, irreversible → keep it in the session CLI.
2. **Verification cost.** Only delegate if checking the result is cheaper than doing it yourself.
3. **Required capability.** Tooling exclusive to one CLI restricts the target (regardless of cost).
4. **Context in memory.** Without enough context in `memory-mcp`, the cheap target rebuilds from scratch — close the gap before delegating.
5. **Explicit choice.** You or the user choose target/mode on the spot; do not automate “which model for which task”.

## Flow of a delegation

1. Consult `memory-mcp` for the relevant context.
2. Choose **target** (permission profile) + **mode** (`print` / `session`).
3. Build the prompt already with the embedded context.
4. Run via **`delegate-run`** (preferable to raw Bash):

```bash
# mechanical default
delegate-run start --target opencode --mode print --prompt "…context + task…"

# agy / needs human approval
delegate-run start --target agy --mode session --prompt "…" --workspace /path/do/proj --name slug
delegate-run attach <id>          # if it asks for permission
delegate-run tail <id> -f         # observability
delegate-run status <id>          # stalled_hint if the log is stalled (default 600s)
delegate-run watch <id> --interval 5 --timeout 1800   # poll until done/failed
delegate-run watch <id> --fail-on-stall               # exits 2 if stalled
delegate-run result <id>          # reads DELEGATE_RESULT from the log
delegate-run result <id> --raw    # full JSON
```

   - **`print`**: blocks until it finishes; review the log/`run.log` before accepting.
   - **`session`**: brings up detached tmux (`delegate-<id>`); the orchestrator **does not** stay on the TTY — tell the user to `attach` if the target asks for permission.
5. `delegate-run` **injects** the output contract into the prompt. The child must print:

```text
=== DELEGATE_RESULT ===
{"ok":true,"summary":"…","artifacts":[],"notes":""}
=== END_DELEGATE_RESULT ===
```

   Optional: `store_memory(name="delegate-<id>", …)` to persist beyond the log.
6. Parent: `delegate-run result <id>` (do not parse the model's prose). If `has_result=false`, treat it as incomplete.
7. If a useful decision/learning arises beyond the result → permanent `store_memory` with correct provenance. Draft/test → `scratch: true`.

Data in `~/.cache/agent-sync/delegates/<id>/` (`manifest.json`, `prompt.txt`, `run.log`, `result.json`, `exit_code`). Script: `scripts/delegate-run.sh` (`make install` → `~/.local/bin/delegate-run`).

## Deleting memory

`delete_memory` only removes what was recorded with `scratch: true`. Permanent ones require deliberate manual removal.

## When NOT to delegate

- Overhead (building prompt + running + reviewing) larger than doing it directly.
- Irreversible/sensitive decision — do not outsource the decision itself.
- Target = agy and the task will clearly trigger tools outside the allowlist **and** no one can stay in `session` mode → change the target (OpenCode) or do it yourself.
- Without enough memory/context and producing it is already expensive.
