# Architecture Decision Records (ADR)

> Absorbed by architecture (2026-11-16). Original content preserved (local conventions `docs/ADR-*`, `D-N`/`A-N` kept).

Lean patterns for recording context, options and consequences of structuring technical decisions.

## Use this skill when

- Documenting architectural decisions, choice of libs, communication patterns or schemas.
- Recording significant technical trade-offs and contract evolution rules.

## Do not use this skill when

- The change is an internal implementation detail, maintenance routine or bug fix.
- The decision is already covered by a global rule or an existing repository convention.

## Conventions in this repository (`agent-sync`)

1. **Location and Names:**
   - Documents in `docs/ADR-<topico>.md` (e.g.: `docs/ADR-001-schema-output.md` or `docs/ADR-session-event-jsonl-append-only.md`).
   - Direct synchronization with `STATE.md` / `.agent-sync/session-state.json` through decision IDs `D-N` and actions `A-N`.
2. **Lifecycle:**
   - `Proposto` → `Implementado` → `Aceito` (after real smoke/validation) → `Superado` (linking the replacement) [Proposed → Implemented → Accepted → Superseded].
3. **Decision Principles:**
   - Contracts closed by default (`additionalProperties: false` in JSON Schema). Exceptions must be justified in the ADR.
   - Architecture changes require a deterministic verification plan (e.g.: documented smoke scenarios).

## Canonical Template (Lean MADR)

```markdown
# ADR-NNN: [Concise Title]

## Status
[Proposto | Implementado | Aceito | Superado por ADR-MMM]

## Context & Drivers
- What motivated the decision? What are the technical requirements and constraints?
- Real problem and limitations of the current approaches.

## Options Considered
- **Option 1 (Chosen)**: Pros and cons.
- **Option 2**: Why it was discarded.

## Decision
- What was decided, scope and technologies involved.

## Consequences & Trade-offs
- **Positive**: Immediate gains and guarantees.
- **Negative / Risks**: Debts taken on or additional complexity.
- **Mitigation**: How the risks will be worked around.

## Acceptance Criteria / Smoke
- Practical conditions to move the status from `Implementado` to `Aceito`.
```

## Anti-patterns

- Generic ADR without practical impact or without a link to code/tasks (`A-N`).
- Omitting trade-offs or negative consequences of the choice.
- Editing the history of an already accepted ADR without recording a new decision or marking it as superseded.
