---
name: product-discovery
description: "Use when discovering and validating a product problem before building: product hypothesis, retention/churn drop, customer interview script, prioritization with RICE/ICE and roadmap cut, and who decides what (DACI). Discovery process in 6 phases with a verifiable exit gate and a condition to kill the initiative. Does not generate open-ended ideas (product-brainstorming) nor audit a product that is already built (gtp)."
---

# Product Discovery

Discovery answers **"does the problem exist and is it worth solving now?"** — not "how to implement". The expensive mistake is not choosing the wrong solution: it is building with confidence a solution to a problem nobody confirmed. Each phase below ends in an artifact and a gate; without the artifact, the phase did not close.

Discovery is not ideation (`product-brainstorming`), is not usability testing of a finished solution and does not replace delivery — it runs in parallel, typically 1-2 cycles per quarter ([discovery-process](reference/discovery-loop.md#when-to-use)).

## Use this skill when

- There is a product hypothesis ("users abandon because of X") and there is no evidence yet.
- Retention, activation or churn dropped and the cause is unknown.
- A customer interview script and sample must be assembled, or a script that leads the answer must be reviewed.
- What goes into the roadmap is about to be decided and a defensible score + decision owner are needed.
- An initiative needs an explicit kill condition before it gets budget.
- Someone asked for a feature and the request must be separated from the problem.

## Do not use this skill when

- The goal is to generate or expand solution alternatives → `product-brainstorming`.
- The product already exists and the request is to interrogate it from the user's point of view → `gtp`.
- The decision is technical (architecture trade-off, library choice) → `architecture`, `backend`.
- The decision is about measuring LLM output quality → `llm-eval`.
- The problem is execution management (sprint, task, deadline) — it is not discovery.

## Instructions

Short-circuit a phase only when its output already exists and is written.

1. **Frame the problem in one falsifiable sentence** before any interview: "We believe that [persona] suffers from [problem] because [root cause], leading to [consequence]." Without persona, cause and consequence, it is not a hypothesis — it is a topic ([Phase 1](reference/discovery-loop.md#phase-1--frame-the-problem)). Output: 1 hypothesis, 3-5 research questions, validation/invalidation criterion.
2. **Define the sample and the script before scheduling**: who (segment and role), how many, and what each question tries to falsify. A script with a question in the future, hypothetical, or of opinion about the idea is wrong ([biases](reference/discovery-loop.md#script-and-biases)).
3. **Conduct until saturation, not until the calendar**: saturation is the same pain appearing in 3+ interviews with no new insight; in practice it closes at 5-7 interviews. Not saturated → 3-5 more, and record that cost ([Decision Point 2](reference/discovery-loop.md#phase-3--conduct)).
4. **Separate evidence from opinion in the synthesis**: evidence is what the person did or paid; "I would use it" is opinion. Record the pain with the count of interviews that support it and what was left out ([Phase 4](reference/discovery-loop.md#phase-4--synthesize)).
5. **Map the 5 risk categories before choosing a solution**: desirable, usable, viable, feasible, ethical — and say which evidence is missing in each one. An assumption without evidence and of high importance is where the experiment goes ([assumption map](reference/discovery-loop.md#phase-5--solutions-and-experiment)).
6. **Generate solutions as a tree, not as a list**: 1 outcome → 3 opportunities → 3 solutions per opportunity → 1 experiment per chosen solution ([OST](reference/discovery-loop.md#phase-5--solutions-and-experiment)).
7. **Choose the prioritization framework by stage, not by taste**: RICE requires reach/impact data; pre-product-market-fit with RICE is a fit error ([prioritization](reference/prioritization.md#choosing-the-framework-by-stage)). Record the framework used and when it will be reassessed.
8. **Name the kill condition together with the success one**: "we kill it if [metric] does not reach [threshold] within [timeframe]". Without a kill condition, the experiment only confirms ([Pitfall 5](reference/discovery-loop.md#kill-condition)).
9. **Close with an explicit GO / PIVOT / KILL** against 4 criteria: problem validated, solution validated, strategic fit, feasible. A decision that is none of the three was not made ([Phase 6](reference/discovery-loop.md#phase-6--decide-and-document)).
10. **Assign a single owner per decision** and write it in the decision log: what was decided, who decided, who was consulted, date and what would reopen it ([DACI](reference/prioritization.md#decision-rights-daci)).

## Honesty gate

Fail the phase that misses any item:

- [ ] The hypothesis names persona, cause and consequence — and can be falsified.
- [ ] Every pain cited has an interview count and at least one literal quote recorded.
- [ ] No conclusion depends on a single voice (say who you did **not** hear).
- [ ] Each critical assumption has evidence or a designated experiment, with a numeric threshold.
- [ ] A kill condition is written before the result.
- [ ] The final decision is GO, PIVOT or KILL, with owner and date in the log.

## References

- [discovery-loop](reference/discovery-loop.md) — 6 phases, hypothesis, sample, script and biases, saturation, synthesis, opportunity solution tree, assumption map, kill condition, decision.
- [prioritization](reference/prioritization.md) — framework choice by stage, RICE/ICE, score template, DACI/RACI, decision log.

## Sources

- `deanpeters/Product-Manager-Skills` — `skills/discovery-process` (6 phases, saturation, GO/PIVOT/KILL), `skills/opportunity-solution-tree` (4 levels), `skills/assumption-mapping` (5 risks, evidence vs opinion), `skills/discovery-interview-prep` (biases and question/answer pairs), `skills/prioritization-advisor` (framework landscape, RICE formula), `skills/decision-rights-matrix` (RACI vs DACI, one decision maker).
- `dend/awesome-product-management` (CC0-1.0) — reading index; canonical basis: Torres, *Continuous Discovery Habits*; Cagan, *Inspired*.
- Rewritten content; no literal excerpt from the sources. The primary source is CC BY-NC-SA 4.0 — do not copy text from it into this repo.
