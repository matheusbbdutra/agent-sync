---
name: context-window-strategy
description: "Use when sessions approach the context limit and you need incremental summary. Estratégia de janela deslizante (últimos K turnos) e handoff via resumos incrementais."
---

# Context Window Strategy

> Strategy to keep the agent coherent across long sessions without exploding token cost.

## When to use

- Session already past ~40 tool calls or about to overflow the token budget.
- Reanchoring after provider-side compaction/summary.
- Defining the initial configuration of a new project that will run multiple sessions.

## Do not use when

- Short task (1-2 turns) — overhead is not worth it.
- You only need to inspect history — use `-show` directly.

## Protocol

### 1. Active window = last K tool calls verbatim

- Default K: **5** (aligned with the paper).
- Per-session configurable via `ctx-window set-k <session> <N>`.
- Position in the prompt: **right after the system prompt, before the current question** — model attention is highest at the edges.

### 2. On-demand compaction

When the window overflows (tokens > budget or tool calls > threshold):

```
[context grows]
       │
       ▼ overflow (budget/threshold)
  ┌────────────┐
  │ summarizer │ ← own model (default) | Ollama (privacy) | heuristic (fallback)
  └─────┬──────┘
        ▼
  [system + summary + last K verbatim + question]
        │
        ▼
  session continues with a clean window
```

The summary is versioned as `summary_v{N}.md` for auditing.

### 3. Summarizer — who generates the summary

| Option | When | Cost | Privacy |
| --- | --- | --- | --- |
| **A. Own model** (default) | whenever the session has an LLM | API tokens | data goes to API |
| **B. Ollama local** | when data must not leave the machine | electricity | total |
| **C. Heuristic pure** | when A and B are unavailable | zero | total |

**Why the own model is the default:** compaction happens exactly when the context is full and about to be reset. The model has the full context available, will discard the history right after, so the token cost of summarization is not wasted.

### 4. Summary format

YAML/Markdown structured with 6 sections (see `prompts/summarize.md`):

1. **Decisions** — markers like "decidimos", "vamos usar", "foi definido" (or English equivalents when locale is set).
2. **Active hypotheses** — "hipótese:", "vou testar", "if X then Y".
3. **Artifacts** — `path:line`, commands, short code blocks.
4. **Resolved errors** — root cause + fix.
5. **Next steps** — open tasks.
6. **Constraints** — non-negotiable rules.

### 5. Configuration

```bash
# During agent-sync -apply: asked interactively
# Override via env:
export AGENT_SYNC_SUMMARIZER=agent           # default
export AGENT_SYNC_SUMMARIZER=ollama:qwen2.5:3b
export AGENT_SYNC_SUMMARIZER=heuristic

# Persistent config in ~/.config/agent-sync/config.json:
{
  "summarizer": "ollama:qwen2.5:3b",
  "k": 5,
  "budget_tokens": 8000
}
```

### 6. Tool commands

```bash
ctx-window summarize               # generates project summary and saves to <projectRoot>/.agent-sync/summary.md
ctx-window show [session]          # shows summary + working memory + versions
ctx-window set-k <session> <N>     # adjusts K (working memory, default 5)
ctx-window doctor                  # detects models, local configs, and summarizers
ctx-window benchmark <dataset>     # runs empirical battery (Phase 0)
```

### 7. Trigger hooks and Handoff

- **PostToolUse / tool.execute.after**: captures arguments and outputs into the local working memory (zero LLM overhead).
- **SessionStart / PreInvocation**: restores the project's `.agent-sync/summary.md` and working memory turns automatically when starting a new session.
- **Nudge**: warns when token consumption crosses the threshold so the agent/user can run `ctx-window summarize` on demand.

## Limits and care

- **Do not invent paper numbers without citing.** The 91.6% comes from arXiv 2606.10209v1 — any other percentage needs a verified source.
- **K=5 is a default, not absolute truth.** Phase 0 (empirical calibration) must validate before being locked in.
- **The summary budget** (e.g., 500, 1000 tokens) **must be calibrated empirically**, not guessed. Without Phase 0, any budget is a guess.

## Quick verification

- `ctx-window doctor` returns 0 and lists the active summarizer.
- `ctx-window show <session>` prints the current summary.
- The hook does not fire below the threshold (no noise).

## Related skills

- `context-guard` — complementary (drift, compaction, STATE.md).
- `context-manager` — conceptual; this skill is the concrete implementation.
- `context-management-context-save` — generic catalog skill; not a replacement.