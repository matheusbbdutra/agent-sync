# Prioritization and decision rights

Primary source: `deanpeters/Product-Manager-Skills`, `skills/prioritization-advisor/SKILL.md` and
`skills/decision-rights-matrix/SKILL.md` (CC BY-NC-SA 4.0). Rewritten; no sentence copied.

Prioritization is a decision, not a spreadsheet. Two separate questions: **which framework fits the
stage** and **who decides**. Choosing the score without defining the decision maker produces a
number nobody follows.

## Choosing the framework by stage

Choose by stage, team context, decision type and available data — not by preference
(`prioritization-advisor#Application`, questions 1-4).

| Situation | Framework | Why |
|---|---|---|
| There is usage data and metrics per initiative | **RICE** | Requires estimable reach and impact |
| No data, quick decision | **ICE** (impact, confidence, ease) | Structured gut-check |
| Visual portfolio choice | **Value × Effort** (2×2) | Quick wins vs strategic bets |
| Multiple criteria already agreed | **Weighted scoring** | Criteria with declared weight |
| Needs to force a hard choice | **MoSCoW** | Forcing function |
| Deadline and window matter more than value | **Cost of Delay** | Urgency |
| Objective → initiative | **Impact mapping** | Ties feature to outcome |
| User journey | **Story mapping** | Narrative flow |
| Basic vs delighter | **Kano** | Satisfaction class |
| Satisfaction gap | **Opportunity scoring** | Importance × satisfaction |
| Customer allocates budget | **Buy-a-Feature** | Preference with constraint |

Rules:

- **Pre-product-market-fit with RICE is a fit error**: there is no reliable reach before knowing
  who the customer is (`prioritization-advisor#Example 2: Bad Framework Match`).
- Changing framework every cycle is whiplash: nobody learns to read the score
  (`prioritization-advisor#Pitfall 2: Framework Whiplash`).
- Declare **when to reassess**: stage change, team reorganization, stakeholder change,
  or when the framework starts getting in the way (`prioritization-advisor#Reassess When`).

## RICE

```text
score = (Reach × Impact × Confidence) / Effort
```

Scales verified in the material (`prioritization-advisor#Step 1: Define Scoring Criteria`):

- **Reach** — how many users per month/quarter.
- **Impact** — 1 = minimal, 2 = high, 3 = massive.
- **Confidence** — 50% (weak data), 80% (good data), 100% (certainty).
- **Effort** — person-month, including design, engineering and QA.

Usage rules:

- Score with PM, design and engineering together; a PM scoring alone is an explicit pitfall
  (`prioritization-advisor#Pitfall 4: Solo PM Scoring`).
- Do not inflate Confidence to make the initiative climb — it is the field that distorts the most
  (`prioritization-advisor#Step 2: Score Each Feature`).
- Sort by score, review the top 10-20 with stakeholders and adjust by strategy; the score does not
  capture everything (`prioritization-advisor#Step 4: Review and Adjust`).
- A score is an input, not a verdict (`prioritization-advisor#Pitfall 3: Treating Scores as Gospel`).

Minimum template:

| Initiative | Reach | Impact | Confidence | Effort | Score | Reach evidence |
|---|---|---|---|---|---|---|
| A | 1200 | 2 | 80% | 3 | 640 | funnel, previous month |
| B | 400 | 3 | 50% | 1 | 600 | interview (n=6) |

The "reach evidence" column is what keeps the table from becoming an opinion with two decimal places.

## Decision rights (DACI)

RACI maps **work**: responsible (does it), accountable (owns the result, one person),
consulted (input before), informed (knows after). DACI maps **one decision**: driver (runs the
process, usually the PM), approver (one person who decides), contributors (bring input and
evidence), informed (`decision-rights-matrix#RACI and DACI`).

What survives any variant (RAPID, consent model): **one decision maker**.

Rules:

- Two accountable = the matrix decided nothing. If the honest answer is "both", **split the
  decision**: "what to build" belongs to the PM, "how to build" belongs to engineering
  (`decision-rights-matrix#One Accountable Person`).
- Do not map everything. Start with the 5-8 decisions where friction already exists
  (`decision-rights-matrix#Pick the Decisions That Hurt`):

  | Decision | Typical conflict |
  |---|---|
  | What goes into the roadmap | sales request vs strategy vs capacity |
  | Price and discount rule | sales vs product vs finance |
  | Scope and requirements | product vs engineering |
  | Launch go / no-go | product vs sales vs support vs legal |
  | Portfolio entry/exit (includes discontinuation) | product vs finance vs leadership |
  | Specific commitment to a customer | sales vs engineering |

- Separate three things that people confuse: **who decides**, **who must be consulted**
  (input heard ≠ agreement) and **who can veto** (rare, and only for a declared reason, such as
  legal or security) (`decision-rights-matrix#Decide Vs. Input Vs. Veto`).
- Consulted acting as approver is the most common conflict; approver deciding without consulting is
  the second (`decision-rights-matrix#Pitfall 2: Consulted Acting as Approver`).
- Name who escalated and where to, before you need it (`decision-rights-matrix#Step 3: Define Forums and Escalation`).

### Decision log

One line per decision, always in the same place (`decision-rights-matrix#Step 4: Set the Log`):

```text
date | decision | approver | consulted | evidence used | what would reopen it
```

Without a log, the decision is reopened by whoever was not in the room — and the next meeting starts
from zero.

## Pitfalls

- **No framework**: priority becomes whoever shouts loudest (`prioritization-advisor#Pitfall 5: No Framework at All`).
- **Score as truth**: the number decides by itself and the strategy disappears (`Pitfall 3`).
- **PM scoring alone**: a score without engineering is a made-up effort estimate (`Pitfall 4`).
- **HiPPO**: an executive defines the roadmap and the framework becomes theater
  (`roadmap-planning#Pitfall 2: Prioritizing by HiPPO`).
- **Mapping everything**: a 40-line matrix that nobody maintains (`Pitfall 4: Mapping Everything`).
- **Assignment without agreement**: a role assigned in the document and not confirmed with the person
  (`decision-rights-matrix#Pitfall 3: Assignments Nobody Agreed`).
