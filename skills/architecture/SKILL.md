---
name: architecture
description: "Use when modeling complex domains, deciding boundaries, or reviewing architecture. Triggers: DDD, bounded context (delimited context), aggregate/aggregate root, ubiquitous language, Clean Architecture, Hexagonal (ports and adapters), dependency rule between layers, ADR/decision record, modular monolith, decomposition into microservices, architectural review."
---

# Architecture

Boundary before pattern. An architectural decision exists only when there is a concrete pain: an invariant that is hard to maintain, a change that crosses modules, an unstable contract between teams. Without pain, applying a pattern is pure cost.

## 0. Pre-flight (before proposing anything)

1. **Specialized skill** — does the subject already have an owner? See §7 and load the neighboring skill instead of reimplementing it here. Do not answer from generic memory when the repo already has curated material.
2. **Decision in force** — look in `docs/ADR-*.md` and in the project memory. Contradicting a recorded decision requires explicit discussion, never silent omission.
3. **Real code** — read the excerpt and cite `path:line`. Do not describe presumed behavior.
4. **Declared depth** — answer §1 before modeling: a CRUD domain does not receive DDD.

Every answer separates three origins: what comes from this skill, what comes from a recorded decision, and what is new analysis with `path:line`. An accepted decision goes back to memory (`store_memory`, `type: project`, `scratch: false`, correct provenance) — without that, the step 2 lookup does not find it later.

## 1. Does DDD pay the bill?

| Signal in the domain | Use DDD | Do not use |
|---|---|---|
| Rules with exceptions that change per contract | yes | CRUD, screens, reports |
| Same term, different meanings per area | yes | single and stable vocabulary |
| Module/team boundaries in dispute | yes | small app, one team, no boundary |
| Model evolution foreseen for years | yes | disposable prototype |

Strategic and tactical detail: [reference/domain-modeling.md](reference/domain-modeling.md). Load it when the task involves an invariant, an aggregate or a bounded context. When the question is **how two contexts relate** (who is upstream, translation, conformism): [reference/context-mapping.md](reference/context-mapping.md).

## 2. Architectural style and dependency rule

- **Dependency points inward**: business rule knows no framework, no database and no HTTP.
- **Hexagonal**: the core exposes ports (interfaces) and the infrastructure implements adapters; changing technology becomes changing the adapter.
- **Layers by name are not architecture**: `service/` and `utils/` with free cross-import keep the coupling intact.

Package structure in Go (`internal/`) and TypeScript, with the same example solved in both languages: [reference/architectural-styles.md](reference/architectural-styles.md).

## 3. Code patterns (GoF and domain)

A pattern only enters with a **real axis of variation**. Legitimate signals: a `switch` that grows with each new case, a vendor type leaking into the business rule, a third-party SDK coupled to the core.

Catalog, when to use/avoid and anti-over-engineering checklist: [reference/design-patterns.md](reference/design-patterns.md).

## 4. Architectural review

Order the findings by risk, not by taste. Every finding carries severity, `path:line` evidence and an alternative with a trade-off.

- **Blocker** — violates the dependency rule, shares an entity/table between contexts, invariant without a single owner.
- **Risk** — N+1, transaction crossing aggregates, unversioned contract, coupling to vendor without an ACL.
- **Acceptable debt** — declared shortcut, with a review trigger.

Full rubric (quality, scalability, consistency, gates): [reference/review-rubric.md](reference/review-rubric.md).

When the discussion is about scale (performance vs scalability, CAP, failover, shard, write-behind cache, back pressure): [reference/scalability-tradeoffs.md](reference/scalability-tradeoffs.md).

## 5. Recording the decision (ADR)

A structuring decision without a record comes back as a discussion. The ADR covers context, discarded options and consequences — including the negative ones.

In this repo: document in `docs/ADR-<topico>.md`, cycle `Proposto → Implementado → Aceito → Superado` [Proposed → Implemented → Accepted → Superseded], link with `STATE.md`/`.agent-sync/session-state.json` through `D-N`/`A-N`.

Template and anti-patterns: [reference/adr.md](reference/adr.md).

## 6. Consistency and transactions

- Invariant **inside** the aggregate: immediate transaction, a single one.
- Invariant **between** aggregates: eventual consistency by event, with a compensation policy.
- A rule that must be atomic between two "aggregates" is a sign that they are a single aggregate.

## 7. Routes (do not duplicate here)

| Subject | Skill |
|---|---|
| Decomposition, service boundaries, distributed resilience | `microservices-patterns` |
| CQRS, projections, read models | `cqrs-implementation` |
| Event sourcing, event store, replay | `cqrs-implementation` (reference/event-sourcing-architect.md) |
| Code rules: indentation, no `else`, first-class collections | `object-calisthenics` |
| Persistence, migration, database transaction | `backend`, `sql-optimization-patterns` |
| API contract and versioning | `api-design-principles`, `openapi-spec-generation` |
| Language and idioms of Go/TS | `backend` (reference/go.md, reference/ts.md) |
| Delegation to another CLI with this context embedded | `agent-delegate` |

## 8. Verification gates

- `agent-sync skills lint` → 0 errors; no `collision` warning citing `architecture`.
- `agent-sync doctor` → `skills_index` with `missing=0` and `orphan=0` after touching `skills/`.
- Code: `go build ./... && go test ./...`; TS: `tsc --noEmit` plus the tests of the touched package.

## 9. Anti-patterns

- Layer created "for the future"; interface with a single stable implementation.
- DDD in CRUD; god aggregate with dozens of entities and a concurrency lock.
- Entity from one context imported into another; shared table as integration.
- Repository returning an ORM entity; SQL/HTTP inside the domain.
- ADR that omits the trade-off or is edited after being accepted.
- "Microservice" with a shared database: that is a distributed monolith.

## 10. Checklist before delivering

- [ ] Every claim has a `path:line` or a cited external source.
- [ ] Dependencies point to the domain; no infrastructure in the business rule.
- [ ] Boundaries and owners explicit; no shared model without translation (ACL).
- [ ] Small aggregates, single root, transaction respecting the boundary.
- [ ] Decision recorded in an ADR when it changes contract, boundary or technology.
- [ ] Trade-off declared; discarded alternative cited.
