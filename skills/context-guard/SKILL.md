---
name: context-guard
description: "Mandatory use when the task is multi-step, to keep STATE.md and protect the context window."
---

# Context Guard

> **Mandatory by default:** if the task has 3+ steps or the session is long, this skill must be loaded and `STATE.md` kept. It is not optional.

Long sessions **degrade silently**: the agent starts to ignore rules it followed, contradicts old decisions and "forgets" what is in the **middle** of the context window (*lost in the middle*). Compaction then discards instructions without warning. This skill fights that through **behavioral self-discipline**.

## Use this skill when

- Multi-step task or a session that has already passed ~40 steps/tools.
- The agent contradicts a previous decision or the style/naming starts to vary.
- Right after a **compaction** event/context summary.
- Scope growing without limit or answer quality dropping.

## Do not use this skill when

- Short and direct task (few steps), with no risk of drift.
- The problem is only reducing tokens of a one-off read → use `token-saving-toolkit`.

## Instructions

1. Estimate the current "zone" from the volume and from the drift signals.
2. In the yellow zone, checkpoint and **recite** the active rules.
3. In the red zone, stop, verify the rules and hand off to a new session.
4. After compaction, **re-read** the rules and revalidate the next action before executing.
5. Reduce context with the low-token tools (below).

## Health zones

| Zone | Signal | Action |
| --- | --- | --- |
| 🟢 Green | start, few steps | normal operation |
| 🟡 Yellow | ~40 steps OR 1st drift signal | checkpoint + recite + reduce exploratory reads |
| 🔴 Red | ~60 steps OR clear contradiction/drift | stop, verify rules, handoff, suggest a new session |

> Step counts are heuristics: prioritize the **drift signals** over the number.

## Drift signals (any one → reanchor)

- Contradicts a decision made earlier in this session.
- Names/style/conventions diverge from the start.
- Reads a file and "remembers" content different from the current one.
- Repeats an attempt already refuted (loop).
- Ignores a rule it followed a moment ago.
- Scope grows without control.

## Anti-compaction anchoring

When there is a compaction (sudden loss of earlier context):

1. **RE-READ** the rules/`global-rules` — never trust memory.
2. **RECITE** in 1–2 lines the 3–5 most critical active rules.
3. **VERIFY** whether the next action respects those rules before executing.
4. For any earlier decision, **RE-READ the source** (`STATE.md`, code, commit) — never assume.
5. Restock the key facts at the **end** of the reasoning (recency), not in the middle.

## Checkpoint and handoff (STATE.md + memory-mcp)

- On every relevant state change, keep `STATE.md` in the project (use `templates/STATE.md` as a base).
- **Integration with `memory-mcp` (Maximum Token Economy):**
  - In addition to `STATE.md`, write concise checkpoints to `memory-mcp` using `store_memory(type="project", name="checkpoint-<projeto>", description="Resumo de estado e decisões de <projeto>", content="...", scratch=true)`.
  - When starting a new session or reanchoring after compaction, first query `get_memory(name="checkpoint-<projeto>")` or `search_memory(query="checkpoint <projeto>")` to restore the context in ~50-100 tokens, reading the full `STATE.md` only if additional details are strictly necessary.
- Checkpoint format: **current state**, **decisions made**, **files touched**, **next steps**, **blockers**.
- When reaching the red zone: write the `STATE.md`, update the checkpoint in `memory-mcp` and propose a **new session** resuming from it.
- `STATE.md` is the source of truth on disk; the checkpoint in `memory-mcp` is the fast anchor across CLIs and compactions.
- **Never copy secrets/credentials into `STATE.md` or `memory-mcp`** (API keys, tokens, passwords) even if they appear in the conversation — describe the problem without pasting the value. Unlike `docs-cache` (which has an automatic redaction filter), the writing of `STATE.md` and of `memory-mcp` is done by you directly; the discipline here is yours, there is no harness guardrail intercepting.

## Context hygiene (use these tools)

- `ast-outline <file>` — structure instead of the whole file.
- `trace-strip` — prunes framework/vendor stack traces.
- `docs-fetch -outline|-grep|-max` and `docs-mcp` — doc snippets, not whole pages.
- `db-guardian` — avoids a giant result (`LIMIT`, explicit columns).
- When investigating, **cite the snippet** (`path:line`) instead of pasting blocks.

## Automatic reminder (hook)

- If a message appears starting with `[agent-sync]` demanding `context-guard`/`STATE.md`, it comes from the `hooks/context-guard-nudge.sh` hook (or the equivalent OpenCode plugin), triggered every N tool calls — it is not the user and it is not a hallucination. Act as asked (load this skill, update `STATE.md`); there is no need to repeat the whole checklist out loud, just follow it.

## Checklist

- [ ] Am I aware of which zone I am in (green/yellow/red)?
- [ ] Are the active rules recited (start and end)?
- [ ] Are decisions and state in `STATE.md`?
- [ ] Am I using outline/grep/retrieval instead of dumping files?
- [ ] After compaction, did I re-read the rules and revalidate the action?
- [ ] Is there a contradiction/drift? If so, did I stop to reanchor?

## References

- [context-window-strategy](reference/context-window-strategy.md) — Sliding window K, compaction and structured summary.
