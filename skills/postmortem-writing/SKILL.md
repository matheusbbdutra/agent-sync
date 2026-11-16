---
name: postmortem-writing
description: "Use when writing incident postmortems with root cause, timeline, and corrective actions. Creation of structured and blameless postmortems with a proven root cause and corrective actions."
---

# Postmortem Writing

A lean guide for post-incident analysis, focusing on a proven root cause, mitigation and prevention of recurrence.

## Use this skill when

- Documenting production failures, contract breaks or severe incidents.
- Investigating complex bugs with multiple contributing factors to avoid recurrence.

## Do not use this skill when

- Incident in progress (stabilize and mitigate first).
- Simple bug or typo fixed with no systemic impact.

## Mechanics and Principles

1. **Blameless Culture**: Focus on system failures, test gaps and missing guards — never on individuals.
2. **Proven Root Cause**: Differentiate symptom from root cause. Use the "5 Whys" technique based on evidence and logs, never on assumptions.
3. **Corrective Actions with an Owner**: Every action must have a priority (P0/P1), an owner and a deadline, preferably tied to automated tests to prevent regression.

## Canonical Postmortem Template

```markdown
# Postmortem: [Incident Title]

**Date**: YYYY-MM-DD | **Severity**: [SEV1 | SEV2 | SEV3] | **Duration**: [X min]
**Authors**: [@author] | **Status**: [Draft | Reviewed | Done]

## Executive Summary
- Brief summary of what broke, the impact on the system/users and how it was stabilized.

## Timeline (UTC)
- `HH:MM` - Triggering event (deploy, migration, external failure).
- `HH:MM` - First alert / detection.
- `HH:MM` - Mitigation action started.
- `HH:MM` - System stabilized.

## Root Cause Analysis
- **Immediate Cause**: What failed directly in the code/infra.
- **5 Whys**:
  1. Why did it fail? -> ...
  2. Why did that happen? -> ...
  3. Why was it not detected earlier? -> ...
- **Contributing Factors**: Test gaps, missing monitoring or incorrect assumptions.

## What Worked vs What Failed
- **Worked**: Fast alerts, agile rollback, useful logs.
- **Failed**: Lack of prior validation, inadequate timeout, false positive.

## Preventive Actions (Action Items)

| Priority | Action | Owner | Deadline | Prevention / Detection |
|------------|------|-------------|-------|----------------------|
| P0 | Add automated regression test | @user | YYYY-MM-DD | Prevention |
| P1 | Adjust monitoring/guard threshold | @user | YYYY-MM-DD | Detection |
```

## Anti-patterns

- Blaming "human error" instead of the absence of tests and safeguards.
- Stopping at the symptom (e.g.: "memory ran out") without understanding the trigger ("why was there a leak").
- A postmortem without practical prevention actions with defined deadlines.
