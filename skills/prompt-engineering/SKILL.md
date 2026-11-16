---
name: prompt-engineering
description: "Use when writing or reviewing the prompt of an LLM application: system message, template, agent persona, few-shot and output contract. Construction and review of prompt — document archetype, verifiable instruction, parseable format, classification options and quality gate. Does not measure answer quality; measurement is llm-eval."
---

# Prompt Engineering

Prompt is document, not command. The work is to decide **what type of document the model must continue** and to close the acceptance criterion before publishing. Measuring quality and comparing versions by score stays in `llm-eval`; window budget and compaction stay in `context-guard` and `token-saving-toolkit`.

## Use this skill when

- Write or review a system message, a prompt template, an agent persona or an instruction message.
- Decide whether a few-shot set is necessary and how to assemble it.
- Define the output format that code will parse.
- Review a prompt that has no explicit acceptance criterion — apply the 4-criteria gate below.
- Take the prompt out of a string in the code and version it.
- Choose generation parameters (`temperature`, `top_p`, `top_k`, beam) coherent with the task.

## Do not use this skill when

- The question is "is this output good?" or "is version A better than B?" → `llm-eval`.
- The problem is a full window, compaction, session drift or STATE.md → `context-guard`.
- The problem is reducing tokens for reading a file/log or cutting generation with a stop sequence → `token-saving-toolkit`.
- The object is the tool spec, the multi-step workflow or the reason/act loop → `mcp-advisor`, `agent-delegate`, `agent-react`.
- The topic is prompt injection, irreversible action or untrusted content → `security-auditor`.

## Instructions

1. **Choose the document archetype** by the shape of the desired answer and do not invent a format: reuse motifs that exist in training corpora (`#`, code fence, lists). "Don't stray far from the path upon which the model was trained" — p.81. Detail in [prompt-craft](reference/prompt-craft.md).
2. **Write the instruction as a verifiable statement**: ask for what to do, not what to avoid, and anchor it with the reason — "Ask for positives instead of negatives and dos instead of don'ts." / "Bolster your command with a reason." — p.102.
3. **Require clarification before the answer** when the request admits more than one reading: "Clarification creates consistency." — p.101.
4. **Add few-shot only if there is a non-obvious aspect** (format, persona, label distribution) that the examples carry — p.111.
5. **Write the beginning of the answer in the prompt** when the model insists on restating the problem: "change your perspective from problem poser to problem solver" — p.136.
6. **Close a recognizable start and end** in the parseable output and choose a delimiter whose termination is testable by substring — p.161-162.
7. **In classification, guarantee a unique first token per option** and prohibit an answer with several alternatives — p.165.
8. **Treat logprob as a confidence gate**, not as truth: derive a threshold to warn, retrieve more context or escalate the model — p.163-164.
9. **Version the prompt outside the code**, with metadata (model, date, application, input/output schema, sampling) and a catalog that allows different versions per application — p.253, p.255.
10. **Pin the generation parameters** per task type; see [generation-params](reference/generation-params.md).

## Review gate: the 4 criteria (p.81-82)

Reject the prompt that fails any one of them:

- [ ] **Looks like a document from the training set** — recognizable format, without proprietary syntax (p.81).
- [ ] **Contains all the relevant information** — nothing the model must guess (p.81).
- [ ] **Leads to a completion that solves the problem** — not merely restating the request: "the prompt must condition the model to generate a completion that is actually helpful" (p.82).
- [ ] **Has a natural stopping point** — the end of the answer is testable by substring (p.161-162).

Apply the gate before publishing and after every template change. Ordering context by importance, counting tokens and deciding what goes into the window are steps of `context-guard`; this gate evaluates form and contract.

## References

- [prompt-craft](reference/prompt-craft.md) — document archetypes, few-shot with a trigger, inertness, snippet position/dependency, parseable output contract, classification with disjoint options, logprobs.
- [generation-params](reference/generation-params.md) — greedy vs beam, temperature, top-k/top-p, repetition, cost per generated token.
- **`bin/prompt-lint`** (Go, `tools/cmd/prompt-lint`) — mechanical lint of the structural defects of the prompt: `negation` (prohibition without a positive alternative, p.102), `no-terminator` (prompt > 800 chars without an end marker, p.161-162), `undeclared-output-format` (p.143), `non-disjoint-classes` (option contained in another or without a single-choice instruction, p.165) and `placeholder-leak` (`{{...}}`, `<PLACEHOLDER>`, TODO/FIXME that would go literally to the model). Command: `prompt-lint check SKILL.md --severity error` (`--json`, `--quiet`, `-` for stdin). Exit codes: `0` no finding, `1` with a finding ≥ `--severity`, `2` usage or read error. It does not evaluate semantic quality — measuring quality and comparing versions is `llm-eval`.

## Sources

- Berryman & Ziegler, *Prompt Engineering for LLMs* — ch. 4 Designing LLM Applications (p.81-82); ch. 5 Prompt Content (p.101-102, p.111); ch. 6 Assembling the Prompt (p.136, p.161-162); ch. 7 Taming the Model (p.163-165).
- Huyen, *AI Engineering* — ch. 5 Prompt Engineering: Prompt Engineering Best Practices (p.240-243) and Organize and Version Prompts (p.253-255).
