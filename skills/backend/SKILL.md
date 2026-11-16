---
name: backend
description: "Use when writing, reviewing or debugging server-side code in Go/Golang, Rust, TypeScript, Node or PHP/Symfony — goroutine, channel, concurrency, CLI, API, transaction, migration, unit test. Applies backend principles (boundaries, contracts, concurrency, persistence, observability) and routes to reference/go.md, reference/ts.md, reference/php.md, reference/rust.md."
---

# Backend

Layer of **principles** for server code, valid in any stack of the catalog. It is not an API catalog nor a framework manual: syntax, library and recipe per language stay in `reference/`.

## When it triggers

- Writing/reviewing server code in **Go/Golang, Rust, TypeScript, Node, PHP/Symfony**.
- Deciding **concurrency** (goroutine, channel, worker pool, mutex, async), **CLI**, **HTTP API**, **transaction**, **migration**, **unit test**.
- Boundary doubt: where the business rule lives, what is payload, what is infrastructure.

## When it does NOT trigger

- System design, macro layers, ADR → `architecture`.
- Browser/UI E2E test → `e2e-testing-patterns`.
- Pure shape refactor (long method, cascading `else`) → `object-calisthenics`.
- Isolated SQL/index tuning → `sql-optimization-patterns`.
- Public API contract (OpenAPI) → `openapi-spec-generation`; domain modeling → `architecture` (reference/domain-modeling.md).

## 1. Boundaries (dependency rule applied to code)

The dependency points inward: `handler → use case → domain`, never the inverse.

- The domain does not import `net/http`, `express`, ORM, `Symfony\Component\...` nor `tauri`.
- Infrastructure **implements** interfaces declared inside (port); it does not dictate them.
- Objective test of the rule: the domain package compiles without the I/O layer (`go build ./domain/...`).

## 2. Contracts

- **Error is value, not flow exception:** in Go `return fmt.Errorf("carregar pedido %s: %w", id, err)`; in TS explicit `Result<T, E>` or typed error at the edge. `panic`/`throw` only for programming bug.
- **Validation at the edge:** a single point validates the payload (DTO/schema) and converts to the domain type. After the edge the type guarantees the invariant — no defensive `if` spread around.
- **Never crosses the edge:** ORM entity, framework `Request`/`Response`, `context.Context` of transport inside the domain, raw SQL in the application, `time.Now()` without injected clock, secret in log.
- Error exposed to the client is stable (code + short message); internal detail stays in the correlated log.

## 3. Concurrency: criterion and cost

Choose by the **shape of the work**, not by fashion:

| work | choice | cost |
|---|---|---|
| Concurrent I/O, short fan-out, ≥4 independent tasks | goroutine + `errgroup` / limited `Promise.all` | requires explicit cancellation and parallelism limit |
| mutation of shared state | `sync.Mutex` or channel serializing single access | contention; short operation → mutex beats channel |
| heavy CPU-bound | fixed worker pool (`GOMAXPROCS`) / `worker_threads` | saturates the process: never per request |
| simple sequential, < ~10 ms total | **nothing** | concurrency here is only a new bug |

Every concurrent path has **cancellation** (context/`AbortSignal`), **limit** and **wait** (WaitGroup/`close`). Leaked goroutine and promise without `await` are the same defect with different names.

## 4. Persistence

- **Transaction** has its boundary in the use case — not in the repository, not in the controller. One handler, at most one transaction.
- **Migration** is versioned in VCS, idempotent and **never** edited after applied. Destructive goes in compatible steps: expand (nullable column) → backfill → contract (drop).
- **Idempotency:** write triggered by event/retry carries an idempotent key (message id, `unique constraint`). Retry without idempotency is corruption with delay.
- N+1 and lazy loading in loop are boundary defects: the data leaves loaded from the repository, it is not pulled during serialization.

## 5. Tests: what each level proves

| level | proves | waste in it |
|---|---|---|
| unit | rule and edge case of the pure domain | booting container/DB; mocking what you do not own |
| integration | contract with real DB/queue/HTTP | reasserting the rule already proved in the unit |
| end to end | that the composition wires up (one happy path) | rule matrix per E2E: expensive and fragile |

- Test that cannot fail for a business reason is ceremony: delete it.
- Determinism: clock, randomness and network injected. `sleep` in the test is a scheduled flake.
- Coverage is a consequence, not a goal: 100% of getter does not pay for an edge case.

## 6. Observability

- **Structured log** (key/value, `slog` / `pino`), never prose assembled with `fmt.Sprintf`: searchable field beats pretty sentence.
- **Correlation:** request/trace id in the context since the edge and repeated in every log of the path — including in the error returned to the client.
- Logs **once per failure**, where there is context to decide (not in each `if err`); no secret/PII.
- Measure the boundary: latency per route/dependency and error count per code. Without a number, "it is slow" is an opinion.

## 7. Routing to reference/

| open | when |
|---|---|
| [reference/go.md](reference/go.md) | Go/Golang: goroutine, channel, worker pool, `context`, profiling — playbook in [reference/go/implementation-playbook.md](reference/go/implementation-playbook.md) |
| [reference/ts.md](reference/ts.md) | TypeScript/JavaScript/Node: strict typing, async, Express/Fastify, middleware — playbook in [reference/ts/implementation-playbook.md](reference/ts/implementation-playbook.md) |
| [reference/php.md](reference/php.md) | PHP/Symfony/Doctrine/PHPUnit — **not maintained** (0 active PHP project); punctual consultation only |
| [reference/rust.md](reference/rust.md) | Rust: ownership/borrowing and when it is worth it on this machine (draft about the local crates) |

Do not open reference by reflex: open it when the question is **about language** (syntax, library API, idiom). Design question stays in this file.

## 8. Anti-patterns (concrete symptom)

- **Fat handler:** route file with >50 lines mixing validation, SQL and rule → the rule leaked into the transport.
- **God-service:** one `Service` with 15 injected dependencies; changing one requirement touches four unrelated methods.
- **`panic`/generic `catch`** swallowing error: log without cause, 500 with `Request failed`, retry that never converges.
- **Transaction per repository:** each `save` opens a transaction; two related writes become non-atomic under concurrency.
- **Orphan goroutine/promise:** process does not terminate, test hangs, memory grows in steps.
- **Mock in place of integration:** green suite and production broken at the database contract — symptom: no test fails when renaming the column.
- **Prose log:** `grep` does not find the problematic request; correlation lost between services.

## 9. Final checklist

- [ ] Does the dependency point inward? does the domain compile without the I/O layer?
- [ ] Does the error carry context (`%w`/`cause`) and is it handled a single time?
- [ ] Does every concurrent path have cancellation, limit and wait?
- [ ] Is the write idempotent and the transaction in the use case?
- [ ] Is the migration additive and reversible in the plan?
- [ ] Does each test level prove what only it proves?
- [ ] Structured log with correlation, no secret/PII?
- [ ] Did external behavior change? then there is a new test — otherwise, it is a refactor.
