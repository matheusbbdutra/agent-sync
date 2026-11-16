# Review Rubric

> Absorbed by architecture (2026-11-16). Original content preserved: review method, impact attribution, gates, good practices and pitfalls.

Reference loaded by `../SKILL.md` §4. The origin's persona, technology catalog and prompt examples are not instruction and were discarded.

## 1. Method (8 steps, in this order)

1. **Context** — raise the current state: existing boundaries, known constraints, decision already recorded. Cite `path:line`.
2. **Impact** — assign High, Medium or Low (table below) before giving an opinion. Undeclared impact becomes taste.
3. **Adherence** — check whether the proposal respects the dependency rule and the principles of `../SKILL.md` §2.
4. **Violations** — identify a concrete anti-pattern, not a general impression.
5. **Recommendation** — propose a named and delimited refactoring, with the trade-off of each option.
6. **Scale** — say what happens with 10× the volume, the team and the deploy frequency.
7. **Record** — require an ADR when the change touches contract, boundary or technology ([adr.md](adr.md)).
8. **Next step** — close with a verifiable action (command, test, gate) and what is still a hypothesis.

## 2. Impact attribution

| Impact | Criterion | Effect on the verdict |
|---|---|---|
| High | changes public contract, context boundary, shared data model or persistence technology | requires a deterministic validation plan **before** approving |
| Medium | new module, new external dependency, layer change | approves with condition and ADR |
| Low | internal detail, renaming, local extraction | approves with a test that covers the behavior |

## 3. Quality rubric (axes)

- **Boundaries** — does each context have its own owner, deploy and model? A rule without a single owner is a blocker.
- **Coupling** — does the core import infrastructure? Does a vendor type appear in the business rule? Is an ACL missing?
- **Scalability** — is the declared bottleneck the real one (N+1, long transaction, lock, serialized call) or is it a guess?
- **Consistency** — does the transaction respect the aggregate's boundary? Is there an idempotent consumer and compensation?
- **Testability** — does the rule run without database, HTTP and network? If it needs infrastructure, the dependency points to the wrong side.
- **Origin of the claim** — each finding says whether it comes from the skill, from a decision recorded in memory/ADR or from new analysis of the code (`path:line`). Opinion without a source does not enter the report.

## 4. Approval gates

Fails without discussion:

- Approving a High-impact change without a validation plan (`go test`, smoke, acceptance criterion).
- Finding without `path:line` evidence or without an alternative with a trade-off.
- Assumption of behavior not verified in the code presented as fact.
- Pending hypothesis delivered as conclusion; discarded premise reintroduced without justification.

## 5. What the review must confirm

1. **Dependency rule** — dependency always points inward.
2. **Interface segregation** — small and focused interfaces, not an `interface`-God.
3. **Rule in the domain** — framework out of the core.
4. **Independent test** — core testable without infrastructure.
5. **Bounded context** — explicit domain boundary.
6. **Ubiquitous language** — consistent terminology between code and business.
7. **Thin controller** — delegates to the use case.
8. **Rich model** — behavior next to the data.

## 6. Common pitfalls

- **Anemic model** — entity with only data; rule in a giant `service`.
- **Framework coupling** — business rule depending on the framework.
- **Fat controller** — business rule in the HTTP handler.
- **Repository leak** — ORM object exposed out of the adapter.
- **Missing abstraction** — concrete dependency inside the core.
- **Over-engineering** — Clean Architecture applied to simple CRUD.
