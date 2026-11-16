---
name: agent-react
description: "Use when the task has 3+ steps, requires hypothesis validation, or risks looping. Disciplines the ReAct loop in multi-step tasks, forcing hypothesis validation before concluding."
---

# Agent ReAct

Answer in PT-BR, objective (global AGENTS.md). The harness already does ReAct “raw”; this skill **disciplines** the loop — it does not replace `context-guard`, `debugging-strategies` nor `token-saving-toolkit`.

## Use this skill when

- Task with 3+ steps or several tool calls until it closes.
- Hypothesis needs to be tested with evidence (code, log, doc, command).
- Risk of loop: same Action repeated without progress.
- User asks for ReAct, structured scratchpad or “think → act → observe”.

## Do not use this skill when

- Short and direct task (1–2 steps, obvious answer).
- Only missing loading a domain skill (`backend`, `architecture`, etc.) — load that one, not this protocol.
- Problem is window health/drift → `context-guard`.
- Bug with a debug playbook → also load `debugging-strategies`.

## Cycle (mandatory)

In each iteration, in this order:

1. **Thought** — 1–3 lines: goal of the cycle, current hypothesis (marked as unproven), what was already refuted.
2. **Action** — **one** tool call that **validates or refutes** the hypothesis (or a parallel batch only if independent).
3. **Observation** — record the fact returned (not what you expected).
4. **Update** — promote to fact (with evidence), keep pending, or **discard**. A refuted hypothesis **does not** come back without new evidence.

Do not write a long monologue in Thought. Do not call a tool “just to see” without a hypothesis.

## Hypothesis ≠ fact (mandatory contract)

A hypothesis **may** exist (“I believe that X”), but **never** circulates as a conclusion until a positive Observation.

Every active hypothesis requires a **validation plan** in the same turn:

| Who validates | When | How to communicate |
| --- | --- | --- |
| **Agent** | Has tool/access (code, command, doc, log) | Formulate the hypothesis → execute the Action → only then conclude |
| **User** | Missing environment, credential, UI, production, data that only they have | “Hypothesis: … To validate, run/confirm: …” — **stop** and wait |

Rules:

- Prohibited to stack “probably A → therefore B → therefore C” without validating A.
- Prohibited to proceed with implementation/refactor based only on an unvalidated hypothesis.
- In the response to the user, label: **verified** vs **hypothesis (pending validation)**.
- If you cannot validate it now, say what is missing — do not fill it in with fake certainty.

## Anti-loop

- Same Action (same relevant args) + same result → **prohibited** to repeat. Change the hypothesis or the Action.
- Refuted hypothesis → list it under “discarded” and do not retest without new data.
- After **3 cycles without progress** (without a useful new fact) → stop, summarize what you know/do not know and ask or change strategy (another skill, delegation, handoff).

## Budget

| Signal | Action |
| --- | --- |
| ~8–12 tool calls in the same subtask without closing it | Short checkpoint + re-evaluate the goal |
| ~15+ tools or yellow zone of `context-guard` | Load `context-guard`, update `STATE.md` |
| No clear progress | Stop and communicate the block — not “one more attempt” |

## Evidence

- Cite a verified source: `path:line`, command output, doc excerpt.
- Prohibited to state code/API behavior without having read/run it in this session (global anti-hallucination rule).
- Prefer `ast-outline` / partial reads / `docs-fetch` over dumping the whole file (`token-saving-toolkit`).

## Scratchpad (optional, short)

Use it only if it helps not to lose the thread — do not paste it in the final response to the user:

```text
Goal: …
Hypotheses: [active + how to validate] / [discarded + reason]
Verified: …
Last: Thought → Action → Observation
Next validate: (me: tool…) | (user: command/action…)
```

## Integration with other skills

| Situation | Skill |
| --- | --- |
| Long session / drift / compaction | `context-guard` + `STATE.md` |
| Bug / root cause | `debugging-strategies` |
| Reduce tokens during inspection | `token-saving-toolkit` |
| Delegate to another CLI/model | `agent-delegate` |
| Architecture / DDD / patterns | `architecture` |

## Closure

Before declaring it done: the evidence matches the conclusion; no pending hypothesis was sold as fact; discarded hypotheses were not reintroduced; no orphan Action without a used Observation.

## Automatic reminder (hook)

- If a message appears starting with `[agent-sync]` demanding hypothesis validation / `agent-react`, it comes from the `hooks/agent-react-nudge*` hook (or OpenCode plugin), every N tool calls (default 15, `AGENT_SYNC_REACT_NUDGE_THRESHOLD`) — it is not the user. Act: validate the active hypothesis or ask the user for the step; there is no need to recite the whole protocol.

## Additional resources

- Good vs bad cycle examples: [reference.md](reference.md)
