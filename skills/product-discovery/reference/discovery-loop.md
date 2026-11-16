# Discovery loop — 6 phases with a gate

Primary source: `deanpeters/Product-Manager-Skills`, `skills/discovery-process/SKILL.md` (CC BY-NC-SA 4.0).
Rewritten; no sentence copied. Citation by section, not by page — the material is markdown.

## When to use

The cycle runs in 2-4 weeks and serves to explore a new area, investigate a retention/churn drop,
validate an initiative before committing the roadmap, and keep a weekly touch with the customer
(`discovery-process#Key Concepts`).

Do not use when:

- the problem is already well understood → go to execution;
- the stakeholder has already committed to the solution → resolve the alignment first;
- it is a tactical bug or technical debt → there is nothing to discover.

Discovery **is not** cascading (waterfall) research done once before dev, **is not** usability
testing — a test validates the solution, discovery validates the problem — and **does not replace**
delivery (`discovery-process#Anti-Patterns`).

## Phase 1 — Frame the problem

Mandatory output:

```text
Hypothesis: We believe that [persona] suffers from [problem] because [root cause], leading to [consequence].
Research questions: 3-5, each one falsifiable by customer evidence.
Criterion: what validates / what invalidates the problem.
```

Rules:

- Without persona, cause and consequence the sentence becomes a topic. An explicit root cause is
  what allows choosing the experiment later (`discovery-process#Outputs from Phase 1`).
- Research questions must be answerable by past behavior, not by intention.
- If the segment is fuzzy, personas are a hypothesis to test, not a picture of the market.
- If the customer motivation is fuzzy, write the job (the progress they try to get), not the
  feature they asked for (`discovery-process#Phase 1: Frame the Problem`).

Gate: can the hypothesis be falsified with one interview? If not, rewrite it before scheduling.

## Script and biases

The script exists in order to **not lead**. Pairs verified in the material — on the left the
mistake, on the right the usable question (`discovery-interview-prep#Biases to Avoid`):

| Bias | Do not ask | Ask |
|---|---|---|
| Confirmation | "Don't you think X is a problem?" | "Tell me how you live X today." |
| Leading question | "Would you use this?" | "What have you already tried? Why did it work or fail?" |
| Hypothetical | "If we built Y, would you pay?" | "What do you pay for today? Why?" |
| Pitch disguised as research | "We are building Z to solve X" | "I am studying X. Tell me your experience." |
| Yes/no | "Is billing hard?" | "Show me your billing process." |

Session structure: short opening (5 min), core (30-40 min), closing (5 min)
(`discovery-interview-prep#Interview Framework`).

Sample: define segment **and role** (user, buyer, who operates, who approves) — each role sees a
different risk. Record who was missing: a conclusion supported by one voice is a hypothesis, not a
finding (`assumption-mapping#Generating Them`).

## Phase 3 — Conduct

Saturation is the stopping criterion, not the round number of interviews: **the same pain appeared
in 3+ interviews and nothing new came up**. In practice it closes at **5-7 interviews**; if you
still learn something new, schedule **3-5** and count the cost of +1 week
(`discovery-process#Decision Point 2`).

Record per interview, minimum:

- literal quote of the pain (the person's words, do not paraphrase);
- what they **did** about it (workaround, spend, abandonment);
- role and context (company, size, current tool);
- what the interview contradicted of what you thought.

## Phase 4 — Synthesize

Group by pain, not by person. For each pain: count of interviews that support it, representative
quote, observed workaround and what is not known yet.

Evidence is what happened: action, spend, abandonment, wasted time. Opinion — including praise — is
not evidence (`assumption-mapping#Evidence Is What Happened, Not What Was Said`).

A synthesis that changes no decision is a report, not a synthesis.

## Phase 5 — Solutions and experiment

Mandatory structure (opportunity solution tree, `opportunity-solution-tree#What is an Opportunity Solution Tree (OST)?`):

```text
Desired outcome (1 business metric)
├── Opportunity 1 ── solution A / B / C
├── Opportunity 2 ── solution A / B / C
└── Opportunity 3 ── solution A / B / C
```

Rules:

- An opportunity is a **customer problem**, not a disguised feature — "needs a dashboard" is a
  solution (`opportunity-solution-tree#Pitfall 1: Opportunities Disguised as Solutions`).
- Diverge before converging: 3 solutions per opportunity before choosing the POC
  (`opportunity-solution-tree#Pitfall 2: Skipping Divergence`).
- A vague outcome ("improve experience") does not allow measuring; anchor it in a metric
  (`opportunity-solution-tree#Pitfall 3: Outcome is Too Vague`).
- Every chosen solution carries an experiment: what it measures, threshold, cost, timeframe.

### Assumption map

Order each assumption by **importance** (how much it hurts if it is wrong) × **evidence** (how much
is already known). The high-importance and low-evidence quadrant is the experiment queue
(`assumption-mapping#Importance by Evidence`).

Five risk categories — classify each assumption into one
(`assumption-mapping#Five Kinds of Risk`):

| Type | Question |
|---|---|
| Desirable | Does the customer want it, and does it solve a pain they feel? |
| Usable | Can they use it without help? |
| Viable | Does it work for the business: revenue, cost, channel, legal? |
| Feasible | Can it be built and operated? |
| Ethical | Can it cause harm or break trust? |

Generate assumptions by asking "what would have to be true?" along the whole story — first
contact, use, payment, renewal —, by role. Include the boring ones: "the team will have time",
"legal will approve" (`assumption-mapping#Generating Them`).

## Kill condition

Write it before the result, not after:

```text
We kill if [metric] does not reach [threshold] by [date].
We continue if [metric] reaches [threshold] in [minimum sample].
```

An assumption without a kill condition becomes a project by inertia — it is an explicit pitfall of
the material (`assumption-mapping#Pitfall 5: No Kill Condition`).

## Phase 6 — Decide and document

The decision is always one of the three (`discovery-process#Phase 6: Decide & Document`):

- **GO** — goes to the roadmap, with the epic written as a hypothesis and success criterion.
- **PIVOT** — problem validated, solution not; go back to the tree.
- **KILL** — real problem, but not now; record why so it is not rediscovered.

Four simultaneous criteria for GO: problem validated (phases 3-4), solution validated (phase 5),
strategic fit, feasible (`discovery-process#Phase 6: Decide & Document`).

30 min readout: what was validated about the problem, what was validated about the solution,
recommendation and what changes if it is accepted.

## Pitfalls that invalidate the cycle

Extracted from the material (`discovery-process#Common Pitfalls`):

1. **Skipping interviews** and "validating" with analytics data — analytics says where, not why.
2. **Leading question** — the answer confirms what you wanted to hear.
3. **Not saturating** — deciding with 2 conversations and a spreadsheet.
4. **Analysis paralysis** — endless discovery so as not to have to decide.
5. **Discovery as a one-off event** — after the first delivery, nobody talks to the customer again.
