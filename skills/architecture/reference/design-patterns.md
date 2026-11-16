# Design (GoF) and Domain Patterns

> Absorbed by architecture (2026-11-16). Original content preserved.

A pattern is **shared vocabulary**, not a goal. Apply it only when there is a concrete pain (variation, coupling or duplication). The golden rule: **composition over inheritance** and **depend on abstractions, not on implementations**.

## Use this skill when

- Designing components and deciding how to decouple responsibilities.
- Removing long `if/else`/`switch` chains that grow with each new case.
- Isolating third-party code (databases, SDKs, HTTP) behind your own interface.
- Introducing extension points without changing existing code (Open/Closed).
- Modeling persistence and domain rules with Repository/Specification.

## Do not use this skill when

- The problem is a one-off, localized fix.
- The abstraction would exist for only one use case (no real variation).
- The team has no way to test the added indirection.

## Instructions

1. Identify the **axis of variation**: what changes often and what stays stable?
2. Prefer the simplest solution that isolates that axis; only then choose the pattern.
3. Name classes by domain intent, not by the pattern (`CalculadoraDeFrete`, not `FreteStrategy`).
4. Validate with tests: the pattern must reduce the cost of adding the next case.
5. If a pattern does not eliminate an `if`/coupling nor improve the test, **do not apply it**.

## Creational

### Factory Method / Abstract Factory

- **Problem:** the client needs to create objects without knowing the concrete classes.
- **When to use:** there are families of related objects or the creation has rules/lookup.
- **Avoid:** when a direct `new` already solves it and the class is stable.
- **Signal in the code:** a creation `switch` scattered across several points.

### Builder

- **Problem:** constructors with many optional parameters or construction in steps.
- **When to use:** complex, immutable objects assembled step by step.
- **Avoid:** objects with 2–3 simple fields.

## Structural

### Adapter

- **Problem:** integrating a third-party API without contaminating the domain with its format.
- **When to use:** SDKs, gateways, HTTP clients, legacy libraries.
- **Signal:** vendor types leaking into the business rules.

### Decorator

- **Problem:** adding orthogonal behaviors (log, retry, cache, metrics) without changing the class.
- **When to use:** a stack of responsibilities that varies by context.
- **Avoid:** when inheritance or single injection is enough.

### Facade

- **Problem:** a complex subsystem exposed to many clients.
- **When to use:** simplifying an entry point or isolating a module.
- **Avoid:** a "God-facade" that only forwards calls.

## Behavioral

### Strategy

- **Problem:** variants of an algorithm chosen at runtime (shipping, discount, tax).
- **When to use:** the same contract with interchangeable implementations.
- **Avoid:** when there is only one variant or the choice is fixed at build time.
- **Signal:** `if tipo == A: ... elif tipo == B: ...` growing.

### Observer / Publish-Subscribe

- **Problem:** notifying multiple interested parties when something changes.
- **When to use:** domain events, decoupling of side effects.
- **Avoid:** when the synchronous and explicit flow is easier to debug.

### Command

- **Problem:** encapsulating an action as an object (queue, undo/redo, audit).
- **When to use:** operations need to be parameterized, queued or recorded.
- **Avoid:** simple direct calls.

### Template Method

- **Problem:** a fixed skeleton with variable steps.
- **When to use:** when inheritance is acceptable and the skeleton is really stable.
- **Avoid:** prefer Strategy + composition when the variants multiply.

## Completing the GoF catalog

The 13 patterns that were missing, in the same format — so as not to choose a pattern by catalog, but by symptom.

### Remaining creational

#### Prototype

- **Problem:** creating variations of an expensive object without repeating the construction.
- **When to use:** a base configuration with small differences; deep copy with defined semantics.
- **Avoid:** when a constructor with explicit parameters is more readable.
- **Cost:** a shallow `clone` is a trap — a shared reference becomes a silent bug.

#### Singleton

- **Problem:** a single resource in the process (pool, cache, registry).
- **When to use:** almost never as a pattern; prefer injecting a single instance.
- **Avoid:** disguised global state, hidden dependency, a test that does not isolate.
- **Error signal:** test code needs to reset the singleton between cases.

### Remaining structural

#### Bridge

- **Problem:** two dimensions vary independently (abstraction × implementation).
- **When to use:** to avoid a `m×n` subclass explosion.
- **Avoid:** when one of the dimensions never varies.
- **Signal:** classes of the kind `RelatorioPDFMensal`, `RelatorioHTMLMensal`, `RelatorioPDFAnual`…

#### Composite

- **Problem:** treating leaf and group in the same way (tree of parts).
- **When to use:** recursive hierarchy (menu, file, expression, org chart).
- **Avoid:** when the structure is flat and stable.
- **Cost:** operations that only make sense on a leaf end up badly placed.

#### Flyweight

- **Problem:** thousands of objects with repeated intrinsic state consuming memory.
- **When to use:** after **measuring** consumption; cache of shared immutable parts.
- **Avoid:** preventive optimization — it is the pattern that shows up most without need.
- **Signal:** a memory profile pointing at the same repeated value.

#### Proxy

- **Problem:** interposing control on access to an object (lazy, permission, cache, remote).
- **When to use:** the control is orthogonal to the object and must not live in it.
- **Avoid:** when Decorator already expresses the same (Proxy controls, Decorator adds).
- **Cost:** indirection hides latency; propagate error and timeout explicitly.

### Remaining behavioral

#### Chain of Responsibility

- **Problem:** several possible handlers, and the chosen one depends on the content.
- **When to use:** validation pipeline, middleware, approval by authority level.
- **Avoid:** when the order is irrelevant or there is exactly one handler.
- **Cost:** a badly configured chain swallows the request without an error — end it with a handler that fails loud.

#### Mediator

- **Problem:** `n` objects talking to each other form `n²` couplings.
- **When to use:** central coordination of complex interactions (UI, orchestration).
- **Avoid:** when the mediator becomes a God-object that concentrates all the rule.
- **Signal:** each new participant forces changing all the others.

#### Memento

- **Problem:** undoing/restoring state without exposing the object's internals.
- **When to use:** undo, snapshot before a risky operation, checkpoint.
- **Avoid:** large or frequent state — memory cost dominates.
- **Cost:** a deep snapshot of the whole aggregate is expensive; capture only what the restoration requires.

#### State

- **Problem:** behavior changes according to the state, and today that is a giant `switch`.
- **When to use:** explicit and finite transitions (order, subscription, job).
- **Avoid:** when there are 2 trivial states — an enum and an `if` are enough.
- **Signal:** `switch (status)` repeated in several methods of the same class.

#### Visitor

- **Problem:** adding a new operation to a closed hierarchy without changing the classes.
- **When to use:** stable structure, operations that grow (report, validation, export).
- **Avoid:** when the hierarchy changes often — each new type breaks all the visitors.
- **Cost:** `double dispatch` is hard to read; in Go, prefer a localized type switch.

#### Interpreter

- **Problem:** evaluating a small recurring language (rule, filter, expression).
- **When to use:** a small and stable grammar; Specification no longer handles composition.
- **Avoid:** when a ready-made parser (or an existing DSL) solves it without writing a grammar.
- **Error signal:** reimplementing an expression parser by hand.

#### Iterator

- **Problem:** traversing a collection without exposing its internal structure.
- **When to use:** almost never explicitly — the language already offers it (`for range`, `range`-protocol).
- **Avoid:** creating your own type when a native slice/stream solves it.
- **Value signal:** only when the collection is lazy, remote or paginated.

## Selection by symptom

Use the "try first" column before accepting the pattern's indirection.

| Symptom in the code | Try first | Pattern if it persists |
|---|---|---|
| `if/switch` growing per variant | parameter + function | Strategy / State |
| scattered creation `switch` | explicit constructor | Factory Method |
| third-party type leaking into the domain | conversion function | Adapter (+ ACL) |
| stack of log/retry/cache around a call | single injection | Decorator / Proxy |
| request traveling through N forwarding layers | direct call | Facade (or remove the layer) |
| `m×n` subclasses for two dimensions | one dimension only | Bridge |
| tree of parts with uniform operations | simple recursion | Composite |
| thousands of objects with the same internal value | measure memory | Flyweight |
| permission/lazy/remote on object access | check in the caller | Proxy |
| `n²` couplings between participants | simple events | Mediator |
| new operation on a closed hierarchy | free function | Visitor |
| undo/state restoration | explicit copy of the fields | Memento |
| sequence of handlers by content | ordered `if` | Chain of Responsibility |
| traversing a paginated/remote collection | slice already loaded | Iterator |

## Domain and infrastructure patterns

### Repository

- **Problem:** decoupling the domain from the persistence mechanism, exposing collections.
- **When to use:** aggregates with read/write by identity.
- **Avoid:** a generic repository per table; model per aggregate.
- **Signal:** SQL/ORM invading the use cases.

### Unit of Work

- **Problem:** coordinating changes from several repositories in one transaction.
- **When to use:** multiple aggregates changed in the same operation.
- **Avoid:** operations on a single aggregate (a local transaction already solves it).

### Specification

- **Problem:** combinable and reusable filter/validation rules.
- **When to use:** composite criteria (AND/OR/NOT) that vary by use case.
- **Avoid:** when a simple predicate is enough.

## Anti-over-engineering checklist

- [ ] Is there real variation (more than one concrete case)?
- [ ] Does the pattern remove coupling or an `if` that would grow?
- [ ] Can the unit be tested in isolation after the change?
- [ ] Does the name reveal domain intent (not the pattern's name)?
- [ ] Did adding the next case become cheaper?

If two or more answers are "no", **do not apply** the pattern.
