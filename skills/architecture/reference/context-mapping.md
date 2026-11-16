# Context map — relationships between bounded contexts

Source: Eric Evans, *Domain-Driven Design Reference*, section **IV. Context Mapping for Strategic
Design** (`domainlanguage.com/wp-content/uploads/2016/05/DDD_Reference_2015-03.pdf`). Rewritten; no
sentence copied. The document defines exactly nine relationship patterns, listed below.

Does not duplicate: the tactical side (aggregate, value object, ubiquitous language, context
boundary) lives in `domain-modeling.md`. This file only covers the **strategic** side — how two
contexts relate, and which pattern pays the translation cost.

## When to apply

Two teams/contexts need to share something and the integration already hurts — a change in one
breaks the other, or nobody knows which model is authoritative. Applying this before that point
produces a pretty diagram and no decision.

## The nine relationship patterns

| Pattern | Relationship | Use when | Accepted cost |
|---|---|---|---|
| **Partnership** | symmetric | development failure in either context fails delivery in both | coordinated planning and joint management of the integration |
| **Shared Kernel** | symmetric | a subset of model, code and database can genuinely be shared | explicit boundary, kept small, nothing changes without consultation |
| **Customer/Supplier Development** | upstream → downstream | the downstream has real priority over the upstream | downstream priorities enter upstream planning, negotiated and budgeted |
| **Conformist** | upstream → downstream | the upstream will not negotiate and translating costs more than adhering | downstream adopts the upstream model, no translation |
| **Anticorruption Layer** | downstream protects itself | upstream model is incompatible or unstable | translation layer exposing the upstream **in your own** model |
| **Open-host Service** | upstream publishes | many consumers, or unknown consumers | open protocol exposing the subsystem as a set of services |
| **Published Language** | language | domain information must circulate between contexts | documented shared language, with translation in and out |
| **Separate Ways** | none | integrating costs more than not integrating | simple, specialized solutions with no connection |
| **Big Ball of Mud** | (pathology) | — | blurred boundaries, mixed models, nobody owns what |

## How to choose

Three questions, in order:

1. **Must it integrate?** If there is no concrete gain → **Separate Ways**. Cheapest answer, least
   chosen.
2. **Who decides?** If the upstream accepts downstream priority → **Customer/Supplier**. If the
   upstream won't negotiate → **Conformist** (adhere) or **Anticorruption Layer** (translate). The
   choice is economic: adhering is cheap now and expensive later; translating is expensive now and
   protects you later.
3. **How many consumers?** Several or external → **Open-host Service** + **Published Language**. If
   both teams truly share the model → **Shared Kernel** (keep it tiny). If delivery only works
   together → **Partnership** (accept the coordination).

Rules of thumb:

- **Anticorruption Layer is the only pattern that protects the downstream without asking anything of
  the upstream.** It is the answer for a vendor, a legacy system, or a team that doesn't answer.
- **Explicit Conformist beats accidental conformism.** Most legacy teams already are conformist; the
  difference is saying it out loud and stopping the pretence of translation.
- **Small Shared Kernel or none.** Every shared line is a line that requires consensus to change.
- **Big Ball of Mud is not a choice** — it is the diagnosis that the map was never drawn.

## Drawing the map

1. List the bounded contexts with **business** names (`Billing`, `Catalog`, `Support`) — not team or
   repository names.
2. For each pair that exchanges information, write both directions: who is upstream (U) and who is
   downstream (D).
3. Label each edge with the pattern and **write the cost next to it** (translation layer, published
   contract, coordination meeting).
4. Mark what does **not** relate (Separate Ways) — the map is also made of absences.
5. Revisit when a context changes owner, model or contract.

## Checklist

- [ ] Every context has a business name and a single owner.
- [ ] Every edge has a direction (U/D) and a named pattern.
- [ ] No edge is left without a declared cost.
- [ ] What doesn't integrate is recorded as Separate Ways.
- [ ] Every Open-host Service has at least one identified consumer.
- [ ] The inherited Big Ball of Mud is on the map, not omitted.
