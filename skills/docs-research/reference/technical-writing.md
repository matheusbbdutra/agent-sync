# Technical writing

How to write the research artifact — summary, reference, handoff, PR description. It deals with
the structure, syntax and vocabulary of the text.

Out of scope: how to search, choose the official source, version the query and separate verified from
inference (that is in the `SKILL.md` of this skill) and search operators
([search-specialist](search-specialist.md)).

## 1. Pyramid: conclusion first, argument after, data last

- **First sentence = main idea + what you need from the reader** (p.119, p.296).
- **Then the arguments that support it, and only then the data** (p.119-121).
- The book's example puts the request before the context: "We need to reschedule the project kick-off
  meeting. Can you let me know ASAP" (p.296).
- **Anti-pattern this prevents:** text in chronological order of discovery, with the request in the last
  paragraph (p.119-121).
- **Application to the research artifact:** open with the finding and the decision it asks for; search
  method, attempts and discards come later.
- Applies to email, PR description, reference and slide deck — the book applies the same structure to
  all four (p.119).

## 2. Syntax: action verb, one reason per sentence, 3 to 5 sentences per paragraph

- **Replace weak verb with action verb:** `be`, `happen`, `there is`/`there are` do not say what
  happens (p.123).
- **One reason per sentence:** "Your sentences should have only one reason to exist" (p.124).
- **One reason per paragraph**, with 3 to 5 sentences: "The sweet spot is at around three to five
  sentences" (p.124).
- **Practical test:** if the sentence needs two "and"/"but" to link independent ideas, they are two
  sentences.
- **Why it matters in agent instruction:** an ambiguous instruction is interpreted in different ways
  by the executor (p.123-124).
- **Application:** skill instructions, subagent prompt, error message and repo doc follow the
  same syntax (p.123).

## 3. One term per concept, acronym defined on first occurrence

- **Pick one term and always use it:** "Pick one and stick to it when you mean the same thing"
  (p.125).
- **If you use two similar terms, declare the difference** (p.125). Without that, the reader — or the
  agent — treats them as two things.
- **Define every acronym on first occurrence** or in a glossary/legend; do not assume that the
  audience knows it (p.116, p.118, p.125).
- **Do not alternate between long and short form** of the same name (p.116-118).
- **Direct application:** tool, hook, skill and command name; log messages; title of a reference
  section. Two names for the same concept make the reader (or the agent) treat them as two
  things (p.125).

## 4. Minimum checklist for a technical document

Every technical document states, at the beginning: (p.127)

- [ ] **key points** — what the reader takes away if they stop here.
- [ ] **scope** — what the document covers.
- [ ] **nonscope** — "State the nonscope" (p.127).
- [ ] **intended audience** — "State the intended audience" (p.127).
- [ ] **prerequisites / required reading** (p.127).

- **Anti-pattern this prevents:** the reader consumes the whole document to find out it was not
  for them (p.127).
- **Declared nonscope** prevents exactly that consumption: the reader discards the document early, instead of
  finding out at the end that it was out of scope (p.127).

| Item | Test question | Evidence |
| --- | --- | --- |
| key points | if the reader stops at the first section, do they know the result? | p.127 |
| scope | is what this document covers written in one sentence? | p.127 |
| nonscope | is what it does **not** cover written? | p.127 |
| audience | is it stated whom the document was written for? | p.127 |
| prerequisites | is what to read before stated? | p.127 |

## 5. Application to the `docs-research` artifact

| Artifact section | Rule | Evidence |
| --- | --- | --- |
| First line | main finding + decision/question for the reader | p.119, p.296 |
| Opening paragraph | argument that supports the finding | p.119-121 |
| Scope | what was investigated and what stayed out | p.127 |
| Audience | who should read it and what they need to know before | p.127 |
| Body | one term per concept, acronym defined | p.116-118, p.125 |
| Paragraphs | 3 to 5 sentences, one reason each | p.124 |
| Data | after the conclusion and the arguments | p.119 |

- The document must not open with the search method: method is supporting data, not conclusion (p.119).
- Library, flag and command name follow the single vocabulary rule (p.125); verification of the
  name in the official source is already in `SKILL.md`.

## 6. Before and after

| Before | After | Rule applied |
| --- | --- | --- |
| "I searched several sources and in some the behavior appears as deprecated, but the official doc still describes the old signature, and there is an open issue, so maybe it is better to confirm." | "The old signature still appears in the official doc; confirm the version before using it. The open issue was not accepted." | conclusion first (p.119) |
| "The framework has a plugin system that is configured via file and that also allows extension by code, which gives flexibility." | "The framework configures plugins via file. Extension by code exists, but does not replace the configuration." | one reason per sentence (p.124) |
| "See the PRM (the manager) and the PRM's prm-handler (the handler)." | "See `prm-handler`, the handler of the PRM (package manager)." | one term per concept + acronym defined (p.125) |
| "The endpoint changed and now returns 404 and not 200, but it worked before, so the migration may have broken it." | "The endpoint returns 404 in the new version; the migration is the probable cause." | one reason per sentence (p.124) |
| "I investigated the topic; this document covers the streaming API." | "The streaming API accepts `--follow`. This document covers only streaming; batch is out of scope." | conclusion first + nonscope (p.119, p.127) |

## 7. Common errors

| Error | Correction | Evidence |
| --- | --- | --- |
| Request in the last paragraph | move it to the first sentence | p.119-121 |
| Sentence with two ideas linked by "and" | split into two sentences | p.124 |
| Paragraph of 10+ sentences | cut to 3-5 or split | p.124 |
| Two names for the same component | pick one and declare the difference from the other | p.125 |
| Acronym without definition | define on first occurrence | p.116, p.125 |
| Document without nonscope | declare what it does not cover | p.127 |
| Search method opening the text | move method to after the finding | p.119-121 |

## 8. Review in 60 seconds

1. Does the first sentence state the finding and what you want from the reader? (p.119, p.296)
2. Does each sentence have one reason? Does each paragraph have 3-5 sentences? (p.124)
3. Is there a single name per concept? Is every acronym defined? (p.116-118, p.125)
4. Does the beginning declare key points, scope, nonscope, audience and prerequisites? (p.127)

## Sources

- Communication Patterns — A Guide for Developers and Architects (Jacqui Read) — ch. 7, Technical
  writing (p.113-127): Acronym Hell (p.116, p.118); Structured Writing / Minto (p.119-121, with the
  email example on p.296); Syntax of Technical Writing (p.123-124); Audience Empathy (p.125);
  document checklist (p.127).
- Scope note: the audience units and abstraction levels of the same book are on p.20-28
  (ch. 1) and were not used here because they fall outside the requested chapter; "big picture first" is on
  p.63-65 (ch. 4).
