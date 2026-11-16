---
name: repo-auditor
description: "Audits the repository for technical debt, refactoring opportunities and environment integrity. Use when the user asks 'audit the repo', 'check technical debt', 'audit repo', 'check the project health', or before large refactorings."
---

# Repo Auditor

Read-only skill for integrity diagnosis, technical debt identification and parity checking of the `agent-sync` environment.

## Procedure

1. **Overall Integrity**: Run `agent-sync doctor` (or `agent-sync doctor -json`) to inspect the 6 canonical subsystems (skills_index, skills_lint, schemas, opencode_plugins, binaries, agents_md).
2. **Skill Quality**: If there are alerts, run `agent-sync skills lint` to isolate divergences in frontmatter, naming or triggers.
3. **Catalog & Orphans**: Run `agent-sync skills index -orphans` to identify skills on disk that are not registered in the curated catalog.
4. **Recent History**: Evaluate `git log --oneline -20` to check whether recent commits introduced undocumented structural pending items.

## Expected Output

Present a concise summary in a Markdown table with:

| Severity | Area | Recommendation | Effort | Evidence |
|---|---|---|---|---|
| Critical / High / Medium / Low | Affected subsystem | Concrete recommended action | Low / Medium / High | Command, path or diff |

## Limitations (Read-Only)

- Do not make automatic changes without explicit confirmation.
- Do not commit or run destructive commands.
- Keep the focus on actionable and pragmatic diagnoses.
