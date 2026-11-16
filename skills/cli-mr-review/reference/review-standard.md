# Review standard

Source: Google `eng-practices` (CC BY 3.0). Rewritten; literal excerpts quoted in quotes.
Citation by file#section.

Complements `code-review-checklist.md` (what to inspect); here is **the approval criterion,
the finding severity and what to do when author and reviewer disagree**.

## The approval standard

> "reviewers should favor approving a CL once it is in a state where it definitely improves the
> overall code health of the system being worked on, even if the CL isn't perfect"
> — `reviewer/standard.md#The Standard of Code Review`

Practical consequences:

- There is no "perfect" code, only **better** code. Holding a merge for polish is a loss for the team,
  not rigor (`standard.md#The Standard of Code Review`).
- The counterweight is the prohibition on degrading: nothing authorizes merging a change that **worsens**
  code health, except an emergency (`standard.md#The Standard of Code Review`).
- Incremental improvement justifies approval; "it is not the design I would have done" does not justify blocking.

## Comment severity

Label the comment so the author knows what is mandatory (`reviewer/comments.md#Label comment severity`):

| Marker | Meaning | Can the author ignore it? |
|---|---|---|
| (no prefix) | must be resolved to approve | no |
| `Nit:` | polish, correct but not essential | yes |
| `Optional:` / `Consider:` | suggestion, valid alternative | yes |
| `FYI:` | information, no action | yes |

Comment rules:

- Explain **why**, not just what (`comments.md#Explain Why`). A finding without a reason becomes personal
  preference and opens an unproductive discussion.
- A purely educational comment is born as `Nit:` (`standard.md#Mentoring`).
- Asking for an explanation of a passage you did not understand is legitimate, provided it is not merely
  unnecessary complexity (`comments.md`).

## What counts as a problem

Technical facts beat opinion and preference (`standard.md#Principles`). On style, the project's style guide
is the authority; a style point absent from the guide is preference — if there is no precedent in the
code, accept the author's (`standard.md#Principles`).

In design, "almost never a matter of style": if the author demonstrates with data or an engineering
principle that both options are valid, the reviewer accepts theirs; otherwise, the design principle decides
(`standard.md#Principles`).

List of what is looked for (`reviewer/looking-for.md`):

- design suited to the system, not generic;
- functionality does what the author wanted, including for the end user;
- UI change makes sense and does not break the flow;
- concurrency/parallelism are safe;
- **there is no complexity beyond what is needed**, nor "for the future" code that is not yet required;
- unit tests present **and well designed**;
- clear names for everything;
- comments explain **why**, not **what**;
- documentation up to date;
- conformance with the style guide.

## Change size

A CL must change **one thing** (`developer/small-cls.md`). A small change is reviewed faster,
more deeply, with fewer bugs, less rework, simpler merge and rollback
(`small-cls.md`).

Useful limits, all from the same file:

- include the related test code in the same change;
- everything the reviewer needs to understand the change goes in together (except future work);
- the system keeps working for users and for the team after the change;
- not so small that the global effect becomes incomprehensible;
- deleting an entire file counts as one line.

## Speed

> "One business day is the maximum time it should take to respond to a code review request"
> — `reviewer/speed.md`

- What is optimized is the **team's** speed, not the reviewer's convenience
  (`speed.md`).
- A slow review is not neutral: it increases the chance of code health degradation through the accumulation of shortcuts
  (`speed.md`).
- A fast response throughout the process is cheaper for the team than a late "perfect" review
  (`speed.md`).
- Approve when: the reviewer trusts that the points will be handled, and the remaining comments do not
  **need** to be resolved (`speed.md`).

## Conflict and escalation

Resolution order (`standard.md#Resolving Conflicts`):

1. consensus based on the review guide;
2. direct conversation (in person/video) when the comment does not resolve it — and the result **must**
   be recorded as a comment on the change;
3. escalation: team discussion, tech lead, maintainer or eng manager.

> "Don't let a CL sit around because the author and the reviewer can't come to an agreement."
> — `standard.md#Resolving Conflicts`

## Translation to local review (`cli-mr-review`)

- Every finding in the report receives a severity: no prefix = blocks approval; `Nit:` does not block.
- A finding without a "why" and without a reference (line, rule, doc) goes back for rewriting, not to the author.
- Approval is decided by "improves code health", not by a complete checklist.
- A change too large to review for real is a first-order finding: ask for a split
  (`small-cls.md`), do not approve out of tiredness.
