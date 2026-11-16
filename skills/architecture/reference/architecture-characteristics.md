# Architecture Characteristics (-ilities)

> Load it when the task is to decide **which** non-functional requirements count, how to measure them and how to keep them from eroding.
> It does not repeat the review rubric, the dependency rule nor the ADR template; for boundary and service granularity the route is `microservices-patterns`.
> Citations use the `p.N` page marker of the origin digest (not the printed footer).
> Reading order: §2 is the pipeline; §12 is what fails; §13 is the delivery gate.

## 1. Triage: is this an architectural characteristic?

Raise a requirement to an architectural characteristic only if the three criteria are true (p.71):

| # | Criterion | Test question | Fails if |
| --- | --- | --- | --- |
| a | Design consideration **outside the domain** | does the requirement speak of how the system behaves, not of what the business does? | the item is a business rule disguised as an -ility |
| b | **Influences** some structural aspect | does changing it change boundary, deploy, data model or topology? | the default design already solves it |
| c | **Critical** to the application's success | does failing at it make the system fail for the user? | it is a preference, not a requirement |

- The three hold together or the item is not architectural (p.71).
- A requirement that fails (b) is code hygiene. Hygiene is accepted without a trade-off discussion; architecture is not (p.71).
- The book's example separates integration from domain: "third-party payment processor" is a consideration outside the domain; "in-application payment processing" is domain (p.72).
- Consequence: an adjective with no structural effect does not enter the list. An inflated list produces generic design (§4, §10).
- Security, performance, privacy and auditability go through the same triage — none enters by default (p.71).
- If you cannot answer (b) with a boundary, a deploy or a data model, the answer is "no" (p.71, p.72).

## 2. Full pipeline (in this order)

Do not skip a step. Each one produces the artifact that the next one consumes.

| # | Step | Output artifact | Exit criterion |
| --- | --- | --- | --- |
| 1 | Identify explicit and implicit | raw list with origin per item | every item traces back to a requirement, business decision or domain knowledge (p.80, p.88) |
| 2 | Reduce | short list (target: 3) | stakeholder chose the top 3 (p.81) |
| 3 | Define objectively | metric + target + condition | two teams agree on what passes and what fails (p.92) |
| 4 | Measure | current value or declared baseline | there is a measured number, not only a target (p.92) |
| 5 | Govern | fitness function in the build | the build fails when the characteristic erodes (p.97, p.99) |
| 6 | Scope | characteristic per quantum | quantum named with its components and data (p.105, p.111) |

- Jumping from step 1 to 5 produces verification of a characteristic that nobody asked for (p.80, p.97).
- Skipping 3 produces a metric invented by whoever implements, not agreed with whoever pays (p.92).
- Skipping 6 produces a global SLA that no component sustains on its own (p.111).

- The order matters: identifying before reducing avoids cutting the characteristic that nobody remembered to cite (p.80).
- The output of the pipeline is a short list with numbers, not a requirements document (p.81, p.92).
- Each step has an owner: 1 and 2 belong to the architect with the stakeholder; 3 and 5 belong to the architect with the team (p.81, p.100).

Pipeline output, minimum form (one block per accepted characteristic):

```
caracteristica: <nome>
metrica:        <o que se mede>            # etapa 3 (p.92)
alvo:           <numero>                   # etapa 3 (p.92)
condicao:       <janela, carga, ambiente>  # etapa 3 (p.92)
baseline:       <valor medido hoje>        # etapa 4 (p.92)
fitness:        <verificacao que roda no build>  # etapa 5 (p.97, p.101)
quantum:        <componentes + dados>      # etapa 6 (p.105, p.111)
```

A block with an empty `fitness` is a characteristic accepted but not governed (p.97). A block with an empty `quantum` becomes a global SLA by omission (p.111).

## 3. Step 1 — extract explicit AND implicit

- **Explicit** ones are written in the requirement: number of users, SLA, volume, cited latency. Scan the entire document, not only the non-functional requirements section (p.80).
- **Implicit** ones are not written and come from domain knowledge. The book is explicit: many architectural characteristics are not specified in requirements documents (p.88).
- Decode business language into its engineering equivalent instead of accepting the adjective (p.85).
- Do not translate a business requirement into a single -ility: "agility" is not time to market, it is agility + testability + deployability (p.82).
- Extraction questions, one per bullet:
  - how many simultaneous users and in which peak window? (p.80)
  - which external SLA has already been promised to a client or contract? (p.80)
  - what data volume and growth rate? (p.80)
  - which business event concentrates load (campaign start, closing, month rollover)? (p.80)
  - which operational loss has the business already declared unacceptable? (p.85)
- A characteristic without a business owner does not survive the priority discussion in step 2 (p.81).
- Mark each item as "measured" or "assumed". An assumption without measurement enters step 4 as a pending item, not as a fact (p.92).

## 4. Step 2 — reduce, asking only for the top 3

- Stated goal: keep the final list as short as possible (p.80).
- Ask the stakeholders for the three most important ones, in any order (p.81).
- "Em qualquer ordem" [in any order] is the part that gets ignored by mistake: demanding a full ranking turns priority into a dispute over position (p.81).
- Do not try to rank 15 items; the result is a technical tie and a decision by hierarchy (p.81).
- Basis for the limit: there is no better architecture, only the least bad (p.78); what is optimized is the collection of trade-offs (p.90).
- If the stakeholder insists that "tudo e critico" [everything is critical], the answer is not a longer list: it is choosing which failure is acceptable when two characteristics conflict (§10) (p.90).
- If the 3 chosen ones do not cover the real risk already known, record the divergence; do not add a fourth in silence (p.80, p.81).
- Repeat the extraction when the requirement changes phase (pilot to production); the list of 3 is of the moment, not eternal (p.80).

## 5. Step 3 — objective definition and composition

- Every accepted characteristic needs an objective definition: metric, target and measurement condition. An objective definition solves the book's three discussion problems (p.92).
- Minimum form, in this order: metric (what is measured), target (the number), condition (window, load and environment in which the number holds) (p.92).
- Missing the condition, the target cannot be audited: the same "200 ms" means different things at p50 and p99, under load and at rest (p.92).
- A composite characteristic is decomposed before designing. The book decomposes agility into modularity, deployability and testability (p.82, p.92).
- Desk test: two people read the definition and reach the same verdict on a concrete case. If they do not, the definition does not exist (p.92).
- Also define what does **not** count; without that the measurement becomes a discussion of interpretation after the incident (p.92).
- Refuse a characteristic that exists only as an adjective in the document ("escalavel", "seguro") [scalable, secure] (p.92).

Definition errors that show up in practice:

- metric without a target: there is no way to fail anything, only to describe (p.92);
- target without a condition: the same number holds for rest and peak, which is never true (p.92);
- composite metric that nobody knows how to decompose: a sign that the characteristic has not been understood yet (p.82, p.92);
- definition copied from the vendor: it describes the product, not your application (p.92).

## 6. Step 5 — govern with a fitness function, not with review

- Book definition: "qualquer mecanismo que fornece uma avaliacao objetiva de integridade de alguma caracteristica arquitetural ou combinacao delas" [any mechanism that provides an objective assessment of the integrity of some architectural characteristic or combination of them] (p.97).
- Code review helps, but it happens too late in the cycle to be the governance mechanism (p.99).
- The concrete example in the book is a layering rule written as an architecture test in ArchUnit (p.101).
- Explain the purpose of the verification to the team **before** imposing it; a rule imposed without a stated reason is worked around (p.100).
- Operating rules:
  - one fitness function per critical characteristic; a characteristic without verification is not governed (p.97);
  - the verification runs in the build and fails the build; a dashboard alert is not a gate (p.99, p.101);
  - manual verification only for what is not automatable, with declared owner and cadence (p.99);
  - the rule is born from the objective definition of step 3, not from the architect's preference (p.92, p.97).
- Expected effect: silent layer erosion and dependency cycles stop arriving late, when they are already in production (p.99).

The definition at p.97 is broad ("qualquer mecanismo"), so the form is the architect's choice. The only form that the book's text exemplifies is an architecture test, with a layering rule in ArchUnit (p.101).

- Choose the form by the characteristic, not by the tool: architecture test for dependency/layer (p.101); any other form is worth it only if it produces an objective assessment of integrity (p.97).
- The choice of form does not change the gate: whatever it is, it must be able to fail the build (p.99).
- The same pattern applies to the API contract: a spec diff that breaks compatibility fails the build and requires a conscious override (see `api-operations.md`).

## 7. Step 6 — scope by quantum, not by system

- Book definition: a quantum is the "artefato deployavel de forma independente com alta coesao funcional e connascence sincrona" [independently deployable artifact with high functional cohesion and synchronous connascence] (p.105).
- The data is part of the quantum: the database enters the scope (p.106).
- A legacy system with a single shared database is, by definition, a quantum of one (p.106).
- Characteristics are scoped at the quantum level (p.111).
- Derived rule: components linked by a synchronous call cannot have very different operational characteristics. A caller that requires 99,9% over a callee of 99% inherits 99% (p.105, p.106).
- There is no "the SLA of the system": there is the SLA of each quantum, and the synchronous path is worth its weakest link (p.106).
- Three scope invalidation checks:
  - does the component enter more than one independent deploy? then it belongs to more than one quantum (p.105);
  - does the characteristic depend on shared data? the owner of the data defines the quantum (p.106);
  - do two synchronous parts require incompatible numbers? either they become distinct quanta, or one of the goals is fiction (p.105, p.106).
- Before promising a global number, list the quanta and the SLA of each one; the real promise is the product of the path (p.111).

Record per quantum (the artifact of step 6):

```
quantum:        <nome do artefato deployavel>       # p.105
componentes:    <lista sincrona entre si>           # p.105
dados:          <banco/schema que o quantum possui> # p.106
caracteristicas: <as que valem para ESTE quantum>   # p.111
```

- A quantum without a data list is incomplete: the data is part of the scope (p.106).
- Two quanta with incompatible numbers on the synchronous path are a false promise to the user (p.105, p.106).

## 8. Counting quanta to decide monolith versus distributed

- "A decisao fundamental repousa em quantos quanta a arquitetura descobre durante o design" [the fundamental decision rests on how many quanta the architecture discovers during design] (p.129).
- Monolith versus distributed is the architect's first determination, before choosing the style (p.276).
- Rule: if a single set of characteristics serves the whole system, prefer a monolith; distributing only pays off when the characteristics diverge (p.129).
- Decide in the components phase, not after the code is written: reverting topology with the system in production costs the entire migration (p.129, p.276).
- Before approving distribution, measure the environment's real latency (average and p95/p99) and multiply by the chained calls of the flow: 10 calls x 100 ms = 1 s added (p.137).
- The long tail, not the average, defines the viability of the response requirement (p.137).
- Distribution without characteristic divergence pays the distributed cost with no return; the criterion of p.129 is the quantum count, not the technological fashion.
- If the divergence is only of team (ownership) and not of characteristic, the problem is organizational; solve it with a module boundary, not with a separate deploy (p.129, p.276).

Preconditions for approving distribution (all verifiable):

- [ ] the characteristic divergence is named and measured on both sides (p.129);
- [ ] the environment's p95/p99 latency is measured, not estimated (p.137);
- [ ] the number of chained calls of the critical flow is counted (p.137);
- [ ] the cost of reverting the topology was declared (p.129, p.276).

## 9. Splitting a component: divergence is the cause

- Legitimate reason to split: divergence of characteristic or of audience, not functional taste (p.127).
- The book's case: Bid Capture is split into Bid Capture and Auctioneer Capture because the characteristics required differ (p.127).
- Both extremes are wrong: granularity too fine generates excessive communication; too coarse generates high internal coupling and a difficult deploy (p.123).
- Do not derive components from the data model (entity trap): a component represents a domain workflow, not a table (p.124).
- If the design is an object-relational mapping, the real requirement is CRUD and does not need architecture (p.124).
- A single component for functional convenience forces the same SLA for the critical and the massive audience — a symptom of a wrong boundary (p.123, p.127).
- Service boundary (transaction, data isolation, iteration) is a subject of `microservices-patterns`; here stays the characteristic criterion (p.123, p.127).

## 10. Trade-off, "least worst" and conflict between characteristics

- Every decision has a counterpart; if you cannot name the loss, the analysis is not finished (p.36).
- The why is more important than the how: record the reason, not only the result (p.36).
- Optimize the collection of trade-offs, not each isolated characteristic (p.90).
- Declare what was sacrificed. An omitted characteristic does not disappear: it comes back as a bug with no owner (p.90).
- When two characteristics conflict, the output artifact is the explicit choice of which failure is acceptable, with the number on each side (p.90).
- An unwritten trade-off is a trade-off that will be reopened by whoever did not take part — record it in the ADR (p.36).

## 11. From the characteristic to the recorded decision

- The choice of the 3 characteristics and the topology are a structuring decision: they go to an ADR with context, discarded options and negative consequences (p.36).
- The ADR cites the metric and the condition of step 3; without them the decision cannot be re-evaluated later (p.92).
- The fitness function is the executable part of the decision; the ADR points to where it lives in the build (p.97, p.101).
- Template and writing rules: `adr-guidelines.md`.

## 12. Anti-patterns

- **Generic architecture**: long list of characteristics, all important, design with no focus (p.78, p.90).
- **Hygiene elevated to architecture**: every adjective in the requirement became a structural need (p.71, p.72).
- **Unprioritized list**: 12 characteristics without the top 3 declared (p.80, p.81).
- **Composite without decomposition**: "usabilidade" or "agilidade" [usability, agility] in the design with no derived metric (p.82, p.92).
- **Definition without condition**: target without window, load and environment; unauditable (p.92).
- **Governance by review**: no verification in the build; erosion discovered months later (p.99).
- **Fitness function without explanation**: the team works around the rule it does not understand (p.100).
- **Global SLA**: one number for the entire system, ignoring quantum and data dependency (p.106, p.111).
- **Distributed without divergence**: microservices with a single set of characteristics (p.129).
- **Entity trap**: one component per table, reproducing the database in the application layer (p.124).
- **Latency on paper**: distribution approved without the environment's measured p95/p99 (p.137).
- **Characteristic without an owner**: an accepted item that no stakeholder claims in step 2 (p.81).
- **Metric without baseline**: written target with no measured current value; one does not know whether it improved or worsened (p.92).

## 13. Checklist before closing the list

- [ ] Each characteristic passes the three criteria (p.71).
- [ ] Explicit and implicit raised; origin of each item recorded (p.80, p.88).
- [ ] Each item marked as measured or assumed (p.92).
- [ ] The top 3 were chosen by the stakeholder, in any order (p.81).
- [ ] Each one has metric + target + condition; the composites were decomposed (p.82, p.92).
- [ ] There is a measured baseline, not only a target (p.92).
- [ ] Each critical characteristic has a fitness function in the build, and the team knows why it exists (p.97, p.100).
- [ ] Each characteristic is assigned to a named quantum (p.111).
- [ ] Topology (monolith x distributed) decided by the quantum count (p.129).
- [ ] The environment's p50/p95/p99 latency is in the distribution decision (p.137).
- [ ] The trade-off is written, with what was sacrificed (p.36, p.90).
- [ ] The structuring decision became an ADR with the negative consequences (p.36; see `adr-guidelines.md`).

## Sources

- **Fundamentals of Software Architecture — An Engineering Approach** (Mark Richards, Neal Ford), ch. 2 "Architectural Thinking" (p.36), ch. 4 "Architecture Characteristics Defined" (p.71, p.72, p.78), ch. 5 "Identifying Architectural Characteristics" (p.80, p.81, p.82, p.85, p.88, p.90), ch. 6 "Measuring and Governing Architecture Characteristics" (p.92, p.97, p.99, p.100, p.101), ch. 7 "Scope of Architecture Characteristics" (p.105, p.106, p.111), ch. 8 "Component-Based Thinking" (p.123, p.124, p.127), ch. 9 "Foundations" (p.137), ch. 17 "Microservices" (p.256), ch. 18 "Choosing the Appropriate Architecture Style" (p.276).
- **Mastering API Architecture** (James Gough, Daniel Bryant, Matthew Auburn), ch. 5 (p.160) — fitness function applied to the API contract, referenced in §6.
