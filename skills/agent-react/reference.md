# Agent ReAct — reference

## Good cycle (example)

**Goal:** find out why `syncSkills` does not create a symlink in Cursor.

```text
Thought: Hypothesis (unproven) — destination ~/.cursor/skills-cursor is protected and the sync skips it. Validate: Grep isProtectedSkillsDir.
Action: Grep isProtectedSkillsDir in cmd/agent-sync/
Observation: path_test.go marks skills-cursor as protected; Cursor's SkillsDir is ~/.cursor/skills.
Update: Verified that skills-cursor is protected. Hypothesis “apply skips Cursor” still pending → validate with agent-sync -status / apply log.
```

## Hypothesis with validation (good vs bad)

**Good (agent validates):**
> Hypothesis: the binary in `~/.local/bin` is old and has no Cursor target. I will check with `agent-sync -status` and the binary date.

**Good (user validates):**
> Hypothesis: the production deploy still points to the old image. I have no access to the cluster from here. To validate, run: `kubectl get deploy … -o jsonpath='{.spec.template.spec.containers[0].image}'` and paste the output.

**Bad:**
> The problem is the old binary in PATH — that is why Cursor does not sync. (stated as fact, without an Action nor a request to the user)

## Bad cycle (anti-patterns)

| Anti-pattern | Why |
| --- | --- |
| 20-line Thought + 5 tools at once | Mixes hypotheses; Observation becomes ambiguous |
| Repeating the `Read` of the same file “to confirm” | Loop; if you already read it, cite `path:line` |
| “It is probably X” without an Action nor a request to the user | Hypothesis sold as fact; missing Observation |
| Chain A→B→C without validating A | Stacking of assumptions |
| Implementing based only on a hypothesis | Fixes the wrong problem |
| Reopening a discarded hypothesis without a new fact | Waste of budget |
| Continuing after 3 cycles without progress | Blind escalation; it must stop and ask |

## When to parallelize Actions

OK in parallel only if they are **independent** (e.g.: `git status` + `git log` + `git diff` at the start of a commit).

Do not parallelize if B depends on the result of A (e.g.: find file → read file).

## Minimum checkpoint (align with STATE.md)

```text
Subtask: …
Proven: …
Refuted: …
Blocked: …
Next action: …
```

Write it in `STATE.md` / `memory-mcp` when the task is already multi-step or the budget is high — not on every short cycle.
