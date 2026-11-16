# ADR Guidelines (selection, writing and governance)

> Load it when the question is **whether** the decision needs an ADR, **how** to write it to survive an audit and **how** to chain supersessions.
> Template and local conventions (`docs/ADR-*`, `D-N`/`A-N` cycle) live in `adr.md`; here is the content criterion, not the form.
> Citations use the `p.N` marker of the origin digest.

## 1. Which decisions deserve an ADR

Open an ADR when the decision touches one of these triggers. No trigger, no ADR.

- Another person will question the decision later; without a record, the same discussion comes back (p.287).
- The decision is structuring: contract, boundary, persistent technology, communication pattern (p.286).
- The justification does not fit in the commit or in the PR without becoming an essay (p.286).
- The decision exists today only in an email or chat thread — the book is direct: email communicates well and documents badly (p.287).
- Do not open an ADR for an internal implementation detail, renaming or bug fix; in those cases the decision is already the diff (p.287).

## 2. Structure: five mandatory sections, plus two

The book fixes five main sections: Title, Status, Context, Decision and Consequences (p.289).

| Section | Required content | Typical error |
| --- | --- | --- |
| Title | names the decision taken, not the theme | generic title ("decision about messaging") |
| Status | current state, with a link to the successor when there is one | outdated status, superseded ADR still marked as in force |
| Context | situation and forces that require the decision (p.289) | context that only repeats the decision |
| Decision | "we will ..." in the imperative (p.292) | "I think ... would be best" |
| Consequences | trade-off assumed, including the negative risks (p.289) | list of benefits only |

Two additional sections that the book treats as part of the artifact:

- **Compliance**: how the decision will be measured and governed; and where the test that verifies it lives (p.294, p.295).
- **Notes**: original author, dates and approver (p.295).

- Without the Compliance section, the decision cannot be verified later — it stays in the field of intention (p.294).
- Without Notes, nobody knows whom to ask when the decision is questioned (p.295).
- The seven sections together, not five out of seven: a context without a consequence is a proposal, not a decision (p.289).

## 3. Writing: imperative, double justification, negative consequence

- Write in the imperative: "we will use asynchronous messaging between services" (p.292).
- Never write in the first person singular. "I think ... would be best" does not record a decision, it records an opinion (p.286).
- Every ADR needs a technical **and** a business justification (p.286).
- The four most common business justifications according to the book: cost, time to market, user satisfaction and strategic positioning (p.286).
- The negative consequence is mandatory. The book's case is exemplary: swapping gRPC for messaging increased latency, and the ADR did not record why — the decision was reverted without anyone knowing the original reason (p.293).
- Consequence written with a number when there is one: expected latency, monthly cost, assumed throughput (p.293).
- If you cannot write a negative consequence, the decision was not analyzed as a trade-off (p.289).

## 4. Compliance: the decision needs a place to die

- The Compliance section forces the architect to think about how the decision will be measured and governed (p.294).
- Declare the mechanism: manual verification with an owner, or automated verification in the build (p.294).
- Point to the exact place of the verification (file, test, gate), not the intention "vamos monitorar" (p.295) [we will monitor].
- Manual compliance is accepted only when there is no automated way; in that case declare the cadence (p.294).
- A decision without Compliance is the one that erodes first: nobody knows when it stopped being valid (p.294).

## 5. Status, supersession and RFC

- When replacing a decision, the link is **bidirectional**: the old one receives "Superseded by N" and the new one records that it supersedes N (p.295).
- Record the supersession date in Notes; without a date, the timeline of decisions is lost (p.295).
- A decision that needs open review uses RFC status with an explicit deadline — the book's example is literal: "Request For Comments, Deadline 09 JAN 2010" (p.291).
- An RFC without a deadline becomes a postponed decision: the deadline is what turns a comment into a decision (p.291).
- A status with an expired deadline is a non-existent decision: either it becomes accepted, or it becomes a new ADR with a new deadline (p.291).
- A change of course generates a new ADR that references the previous one; supersession is the mechanism of change, not editing the old document (p.295).

## 6. The three decision anti-patterns, in sequence

Verify in this order when auditing a set of ADRs:

1. **Covering Your Assets** — the decision was postponed for fear of being wrong (p.286). Correction: decide at the last responsible moment, not at the first.
2. **Groundhog Day** — the decision comes back because the justification was not recorded (p.287). Correction: Context + Consequences filled in, not just Decision.
3. **Email-Driven Architecture** — the decision exists only in email/chat and is therefore lost (p.287). Correction: move what matters to the canonical repository.

- The three produce the same final symptom: "as pessoas perdem, esquecem ou nem sabem que uma decisao foi tomada" (p.287) [people lose, forget or do not even know that a decision was made].
- Detection by symptom: an ADR whose deadline is always postponed is Covering Your Assets (p.286); an ADR redone with the same theme is Groundhog Day (p.287); a decision that only shows up in an email search is Email-Driven Architecture (p.287).
- The three apply to a set of ADRs, not to an isolated ADR: a single document can be correct and the process still sick (p.287).
- Auditing a repo with ADRs: if the three sections Context/Consequences/Compliance are empty, the three anti-patterns are active by definition (p.286, p.294).

## 7. ADR guideline per type of recurring decision

The API book attaches an "ADR Guideline" to each decision that repeats, instead of leaving the criterion in the running text:

| Recurring decision | Guideline in the book | Page |
| --- | --- | --- |
| Choosing the API standard (REST, gRPC, GraphQL) | ADR Guideline: Choosing an API Standard | p.46 |
| Adopting contract testing and which methodology | ADR Guideline: Contract Testing | p.73 |
| Selecting API gateway | ADR Guideline: Selecting an API Gateway | p.116 |
| Choosing OAuth2 grants | ADR Guideline: Which OAuth2 Grants | p.213 |

- Derived rule: decisions that reappear in every project get their own guideline, with the same fields as an ADR (p.33, p.46).
- The base format the book uses is the minimum: status, context, decision, consequences (p.33).
- When writing a new ADR of a type already covered by a guideline, follow the guideline and record only what is specific to the project (p.46).

## 8. Classify before deciding: Type 1 versus Type 2

- Classify the decision as Type 1 (hard or expensive to revert) or Type 2 (reversible) before choosing the process (p.261).
- An irreversible decision is not a problem in itself: the problem is irreversibility without analysis (p.228).
- Choosing gateway, service mesh or edge technology is, in the book, explicitly a Type 1 decision — it requires an ADR before executing (p.261).
- Type 2 accepts a quick decision with a short record; using a heavy process on a reversible decision costs speed with no gain (p.261).
- Type 1 with a light process produces lock-in and unaccounted exit cost (p.228, p.261).
- Classification test: name the concrete cost of reverting (migration, broken contract, data to move). If the cost is low or unknown, treat it as Type 2 and proceed (p.261).
- For Type 1, the ADR is written **before** executing; for Type 2, the record can come after the execution (p.261).

## 9. Checklist before opening or closing the ADR

- [ ] The decision passes one of the triggers of §1 (p.286, p.287).
- [ ] Classified as Type 1 or Type 2, and the process corresponds (p.261).
- [ ] Title names the decision, not the theme (p.289).
- [ ] Context describes the forces, does not repeat the decision (p.289).
- [ ] Decision is in the imperative "we will ..." (p.292).
- [ ] There is a technical and a business justification (p.286).
- [ ] Consequences includes the negative, with a number when there is one (p.289, p.293).
- [ ] Compliance points to the mechanism and to the place of the verification (p.294, p.295).
- [ ] Notes brings author, dates and approver (p.295).
- [ ] If it replaces another ADR, both links exist and the date is recorded (p.295).
- [ ] None of the three anti-patterns of §6 is active (p.286, p.287).
- [ ] The five sections of the book are present (Title, Status, Context, Decision, Consequences) (p.289).
- [ ] If the status is RFC, there is an explicit deadline (p.291).

## Sources

- **Fundamentals of Software Architecture — An Engineering Approach** (Mark Richards, Neal Ford), ch. 19 "Architecture Decisions" (p.286, p.287, p.289, p.291, p.292, p.293, p.294, p.295).
- **Mastering API Architecture** (James Gough, Daniel Bryant, Matthew Auburn), Introduction (p.33), ch. 1 "ADR Guideline: Choosing an API Standard" (p.46), ch. 2 "ADR Guideline: Contract Testing" (p.73), ch. 3 "ADR Guideline: Selecting an API Gateway" (p.116), ch. 7 "ADR Guideline: Which OAuth2 Grants" (p.213), ch. 8 "Using Fitness Functions" (p.228), ch. 10 "Understanding Decision Types" (p.261).
