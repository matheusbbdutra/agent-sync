---
name: social-context
description: >
  Interview the user in ten questions or fewer and produce a social-context.md file that
  captures who they are, who they are talking to, what they post about, and what they
  refuse to post. Covers LinkedIn, X (Twitter), Instagram, Threads, TikTok, and YouTube —
  the interview decides which of those platforms actually matter. Use when the user says
  "set up my social context", "onboard me", "set up my socials",
  or "create my social profile".
  Every other skill in this collection reads social-context.md for brand voice
  and setup, so this is the skill to run first. Re-running it updates the existing file
  section by section and never overwrites the user's manual edits without asking.
license: MIT
metadata:
  version: 0.1.0
  category: Foundation
  topics:
    - foundation
    - planning
  examplePrompt: "Set up my social context — I'm a solo founder selling a developer tool"
---

Interview the user, then write a `social-context.md` at the project root that every other
social skill can read.

## Context

Check for an existing `social-context.md` at the project root, then `.agents/social-context.md`.
If one exists, this run is an **update**: read it fully before asking anything, skip questions
it already answers well, and confirm only what looks stale. If neither exists, this is a fresh
interview — say so and start immediately.

## Workflow

1. Set expectations in one line: "Up to 10 quick questions, multiple choice where I can —
   say 'skip' to any." Then ask the first question in the same message. Do not wait for
   permission to begin.
2. **Who are you and what do you sell?** One open question, captured as the `## Positioning`
   section. If the answer is vague ("a startup"), push once for the one-sentence version a
   stranger would understand: product, category, and who pays.
3. **Audience.** Ask who they most need to reach, offering choices:
   - (a) buyers/customers
   - (b) peers in their field
   - (c) potential hires
   - (d) investors
   - (e) a niche community
     Follow with one sharpening question: "Describe one real person in that audience — job
     title, what they scroll past, what stops them." Capture that person, not a demographic.
4. **Platforms.** Ask which platforms they're active on — LinkedIn, X, Instagram, Threads,
   TikTok, YouTube etc. — and, for each chosen one, _why_ (where the audience is vs. where they're
   comfortable). If they pick more than three, warn once that two done consistently beats
   five done sporadically, and ask them to rank. Record a primary and a secondary.
5. **Pillars.** Ask for 3–5 recurring themes they can post about for a year without running
   dry. If they stall, propose pillars derived from their answers so far — build-in-public
   progress, opinions on their industry, teaching their craft, customer stories,
   behind-the-scenes — and let them pick and edit. Each pillar gets a name and a one-line
   description.
6. **Tone sliders.** Present three sliders as multiple choice (1–5), all in one message:
   - funny ↔ serious
   - spicy ↔ safe
   - personal ↔ brand
     Then ask for one account they admire and one they'd hate to sound like — those two
     anchors are worth more than the sliders.
7. **Cadence reality check.** Ask how many posts per week they will _actually_ ship, not
   aspire to. If they say daily, ask what they shipped last month. Record the honest number
   per platform.
8. **Goals.** Ask what a win looks like in 90 days, offering: reach/awareness, inbound leads,
   hiring pipeline, community, or personal brand. Force a single primary goal; secondary
   goals are allowed but labeled.
9. **Red lines.** Ask what they will never post: competitors they won't name, topics that are
   off-limits (politics, salaries, client names), formats they hate (dance trends, engagement
   bait), words they'd never use. Get at least three concrete items.
10. **Write the file.** Assemble `social-context.md` using the sections in the Quality bar
    table below, in that order, with those exact headings — other skills locate sections by
    these headings, so never rename or reorder them.
11. **Fresh run:** write the whole file. **Update run:** regenerate section by section, diff
    each against the existing file, and where the existing text contains detail you didn't
    collect this session (manual edits, rules added by hand), show the old and
    new versions side by side and ask which to keep. Never silently drop a line the user
    wrote by hand.
12. Before showing the file, check every section against the Quality bar; fix any that fail
    its "Must contain" bar, and turn any section you can only fill with an invented answer
    into a placeholder instead. Then show the file and ask for one correction pass on the
    highest-stakes sections: "Read the Never and Voice sections — anything wrong?" Apply
    fixes and finish.

## Quality bar

| Section     | Must contain                                                                             |
| ----------- | ---------------------------------------------------------------------------------------- |
| Positioning | One-sentence statement: product, category, and who pays — a stranger could draft from it |
| Audience    | One named persona sketch (role, pain, what stops their scroll) — not just a demographic  |
| Platforms   | Primary and secondary marked, with a one-line "why" for each                             |
| Pillars     | 3–5 named pillars, each with a one-line description                                      |
| Voice       | The three slider values plus the admire/avoid anchor accounts                            |
| Cadence     | A realistic posts-per-week number per platform                                           |
| Goals       | Exactly one primary goal; secondaries labeled as secondary                               |
| Never       | At least 3 concrete, checkable red lines                                                 |

Additional rules:

- Every entry must be specific enough that a stranger could draft a post from the file alone.
  "Professional but approachable" fails; "explains like a senior engineer at lunch, no
  corporate verbs, swearing allowed but rare" passes.
- Skipped questions produce a `_Not captured yet — rerun this interview to fill in._`
  placeholder under the relevant heading, never an invented answer.
- The file is plain markdown: no YAML frontmatter, no HTML, headings exactly as listed.

## Deliverable

`social-context.md` written to the project root (or updated in place, edits preserved),
followed by a short summary in chat: the one-line positioning, primary platform, primary
goal, the pillar list, and any sections left as placeholders with a note that re-running
this skill fills them in.
