---
name: agent-learn
description: "Analyzes the recent execution history in session-event.jsonl and the decisions in STATE.md to identify recurring failure patterns and propose pragmatic improvements to the rules and skills. Use when the user asks 'learn from the mistakes', 'extract lessons', 'learn from failures', 'which patterns can we improve', or at the end of long sessions."
---

# Agent Learn

Analytical skill to mine session history, refine heuristics and propose the continuous evolution of the global rules (`rules/global-rules.md`) and tools.

## Procedure

1. **Event Reading**: Analyze the latest entries of `.agent-sync/session-event.jsonl` (or run `agent-sync event stats`).
2. **Error Patterns**: Identify:
   - Tool loops or frequent retries.
   - Hypotheses refuted without documentation of the root cause.
   - Commands blocked by guards (`bash-guardian`, `shell-validate`).
   - Cross-CLI support gaps between the 5 tools.
3. **Recent Decisions**: Cross-check with `STATE.md` (Decisions D-N section) to avoid proposing what has already been formalized.
4. **Pragmatic Proposal**: Formulate 1-2 lean proposals (a new bullet in `rules/global-rules.md` or a trigger adjustment in an existing skill).

## Output Format

```markdown
### Detected Patterns
- **Pattern X**: Occurrence in sessions Y and Z. Proven root cause.

### Action Proposal (Pragmatic)
- **Target File**: `rules/global-rules.md` (or specific skill)
- **Suggested Diff**:
  ```diff
  + <concise new principle>
  ```
- **Justification**: Prevents recurrence with minimal operational impact.
```

## Guidelines

- Never apply changes silently; present the proposal and wait for user approval.
- Always prioritize pragmatism and simplicity (minimum functional text).
