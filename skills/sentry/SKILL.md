---
name: sentry
description: "Use when investigating errors or performance issues reported in Sentry. Error debugging and tracing in Sentry: triage of issues, breadcrumbs and transactions."
---

# Sentry — Error and Performance Debugging

You investigate problems from **Sentry** and reach the root cause in the code, with evidence. Never treat only the symptom.

## Use this skill when

- Investigate a **issue/event** in Sentry (exception, crash, regression).
- Understand grouping (why it became 1 or N issues) and alert noise.
- Correlate a production error with the **release/commit** and the code.
- Analyze **performance** (transactions/spans, p95, slow queries).
- Configure the SDK (DSN, release, environment, sample rates, PII).

## Do not use this skill when

- The error is reproducible and clear locally without telemetry (debug directly).
- The task is only monitoring/alerts without a concrete incident.
- It is a vulnerability → use `security-auditor`.

## Instructions

1. Collect **event context**: project, environment, release, first/last occurrence, number of events and affected users.
2. Read the **stack trace** (filter noise with `trace-strip`); identify the application frame.
3. Review **breadcrumbs** (the actions that preceded it) and **tags/context** (user, url, request, device).
4. **Locate it in the release code** (`path:line`) — confirm the commit.
5. Formulate **hypotheses** (the input/state that triggered it) and validate each one.
6. Identify the **root cause**, propose the minimal fix and a test that prevents recurrence.
7. Confirm that the error stops in the next release; consider a regression alert.

## Key concepts

- **Organization / Project / Environment:** scope; always confirm the environment (prod ≠ staging).
- **Issue × Event:** an *issue* is the grouping; an *event* is an occurrence with its own data.
- **Release / Dist:** ties events to a version/distribution; use it to find **suspect commits** and regressions.
- **Breadcrumbs:** trail of actions before the error (HTTP, logs, navigation).
- **Tags × Context:** tags are indexed/filterable; context is per-event structure.
- **Fingerprint:** grouping rule; changing it can merge/split issues.
- **Transaction / Span:** performance tracing; identify the slow span on the path.

## Triaging an issue

- **Impact:** frequency, unique users, affected environments, whether it is a regression (new in release X).
- **Error × cause:** the message is not always the cause; follow the innermost frame **of your code**.
- **Request data:** method, route, status, body (with PII masked).
- **Hypothesis → evidence:** every hypothesis needs evidence (log, code, reproduction).

## Source maps, symbols and grouping

- **JS/TS:** without **source maps** the trace shows up minified — ensure the upload (`sentry-cli sourcemaps upload` / build plugin) per release.
- **Go/native:** send **debug symbols** when applicable.
- **Grouping:** messages with dynamic values (IDs, dates) can fragment into several issues → use fingerprinting/`scope.setFingerprint`. The opposite (distinct errors in the same issue) indicates a generic fingerprint.
- **Merge/Split/Ignore:** group correctly; ignore only real noise (with a rule and a deadline).

## Performance / Tracing

- Compare the **p95/p99** of the suspect span; look for N+1, external calls, serialization.
- Use the **trace** of a slow event to find the culprit span; correlate it with the code.
- Measure, do not speculate: collect a baseline before/after the fix.

## SDK configuration (common pitfalls)

- **DSN** via environment variable — never hardcoded/committed.
- **release** and **environment** always set (otherwise you lose correlation).
- **Sample rates:** `traces_sample_rate`/`profiles_sample_rate` according to volume; mind the cost.
- **PII:** keep `send_default_pii` off by default; configure scrubbing.
- **Framework integrations:** Symfony (`SentryBundle`), Laravel, Express/Node, browser JS — check the framework config.
- **Swallowed errors:** do not capture and ignore without rethrowing; avoid duplicating events (capture at the same level).

## Tools

- **Sentry MCP** (optional): `https://mcp.sentry.dev/mcp/{org}/{project}` (OAuth) — lets you fetch issues/events and traces directly. See `mcp-advisor`.
- **Sentry API / UI:** for one-off queries and canonical links.
- **`trace-strip`**: filter library frames when analyzing the trace.
- **`ast-outline`**: locate the function in the release code before reading the snippet.

## Guardrails

- **Never** display or paste PII/secrets (e-mails, tokens, user data): reference the position, mask it.
- Investigation is **read-only** by default; behavior change only with an intentional, tested fix.
- Cite the evidence (`event`, `path:line`, `release`); distinguish verified fact from hypothesis.
