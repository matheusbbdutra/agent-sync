# prompt-craft

Rules of form and of contract for writing and reviewing the prompt of an application. Assumes `SKILL.md` loaded.
`p.N` markers come from the source digest (ch. 6 of *Prompt Engineering for LLMs* starts on p.133). Token
budget, per-turn context order and compaction are **not** here: they are in `context-guard` and
`token-saving-toolkit`.

## 0. Writing flow

Six steps, in order. Skipping one of them is what produces a prompt that "works on my machine".

1. **Classify the answer** and choose the archetype (§1) — p.137, p.140, p.143.
2. **Write the instruction as a verifiable statement** with the reason attached (§2) — p.101-102.
3. **Close the output contract** with a start and a testable terminator (§6) — p.161-162.
4. **Decide few-shot by the trigger**, naming the implicit rule (§3) — p.111.
5. **Declare position, importance and dependency** of each element (§5) — p.151.
6. **Run the 4-criteria gate** of `SKILL.md` — p.81-82.

## 1. Document archetype

Before writing, decide **what type of document the answer is** and copy that form. The choice criterion
is the shape of the desired answer, not the taste of whoever writes it (p.137, p.140, p.143).

| Archetype | Section | When it serves | Typical failure |
| --- | --- | --- | --- |
| Advice conversation | p.137 | multi-turn dialogue, the answer is the next turn of an assistant | analytical task with a rigid boundary; the model opens back-and-forth clarification |
| Analytic report | p.140 | analysis with a declared scope and named sections | implicit scope; the model extrapolates what was outside |
| Structured document | p.143 | output that must be parsed (XML/YAML/JSON/code) | fragile terminator: indentation is not testable by substring (p.161-162) |

- **Declare the boundary as an explicit negation in the body**: "This report focuses solely on novels, excluding
  self-help books." (p.140).
- **A report respects a limit better than a dialogue**: "LLMs tend to respect such clear boundaries more
  consistently in reports than in dialogues" (p.140) — if there is prohibited content, use a report with a
  `Scope` section, not a conversation.
- **Long prompt in Markdown with an index**: sections `# Ideas` / `# Analysis` before `# Conclusion` guide the
  conclusion (p.141).
- **Sentinel section as the end**: `# Further Reading` at the end of the document gives the model a predictable
  stopping point (p.141). The cut by `stop` itself is an API call, see `token-saving-toolkit`.
- **Do not invent syntax**: the prompt format must look like a document that exists in training corpora
  (p.81). `#`, code fence, numbered list and heading are already known motifs.

## 2. Verifiable instruction

- **Positive instead of negative**: "Ask for positives instead of negatives and dos instead of don'ts."
  (p.102). Rewrite "do not invent numbers" as "cite only the numbers that appear in the context".
- **Anchor the command in a reason**: "Bolster your command with a reason." (p.102) — "answer in at most
  3 bullets because the output is read in a terminal".
- **Avoid the absolute** (p.102): "never" / "always" in an ambiguous context produces a silent partial
  violation; replace them with a testable condition.
- **Ask for clarification before producing**: "Clarification creates consistency." (p.101) — if the request
  admits more than one reading, the prompt requires the model to ask before the answer.
- **The prompt must condition to a useful answer**, not only restate the request: "the prompt must
  condition the model to generate a completion that is actually helpful" (p.82).
- **All relevant information in the prompt**: criterion 2 of the gate — the model must not need to guess
  (p.81). A missing datum is not compensated with a stronger instruction.

## 3. Few-shot with a trigger

Use an example only when there is an implicit rule to demonstrate. The trigger is: **name the rule before pasting
the example**.

- **No nameable rule, no few-shot**: "Use few-shot prompting if you have relevant examples that
  illustrate an aspect of what you want the model to do that is otherwise unobvious." (p.111).
- **Format only → minimal example**: few-shot is "particularly suited to quickly and easily demonstrating
  just the expected output format and nothing more" (p.106).
- **Pattern is compulsion**: "LLMs have a compulsion to continue patterns" (p.104) — everything that appears in
  the example becomes a rule, including what you did not want to fix.
- **Respect the real distribution**: "If you have an idea what the distribution is, don't stray too far away
  from it." (p.108).
- **Cover the classes and the non-trivial exceptions**: one example per relevant class, including the edge
  case the prompt must get right (p.108).
- **Shuffle the order**: "Your prompt examples are not randomly ordered unless you consciously shuffle
  them." (p.109) — happy path first and error after creates a spurious pattern.
- **Keep the level of context**: a gigantic example paired with a short question (or the opposite) changes what
  the model imitates; shrinking too much removes the reasoning that was to be demonstrated (p.109).
- **The example format follows the source document** (p.81, p.149): code in a code fence, table in
  Markdown — the model has already seen these motifs in training.

| Situation | Few-shot? | Basis |
| --- | --- | --- |
| non-obvious output format | yes, minimal examples | p.106 |
| specific persona or style | yes, real and short examples | p.111 |
| subtle boundary between classes | yes, one per class | p.108 |
| rule already written in text and obeyed | no | p.111 |
| short budget and simple format | no; describe the format in words | p.106 |

## 4. Inertness and snippet format

Inertness is keeping the pasted content **inert**: the snippet must be read as data of its own scope, not
as a continuation of the instruction.

- **The model uses everything it receives**: "the model often feels compelled to use every bit of information it
  gets" (p.116) — a snippet that does not serve the answer displaces the answer.
- **Pasting is not adding tokens**: "Pasting strings together doesn't mean the arrays of tokens just get
  concatenated" (p.148). The boundary between two snippets can fuse tokens (1+1 → 1) or break into three
  (1+1 → 3), so the cost of the prompt is not the sum of the costs of the pieces.
- **Separate elements by whitespace** and do not trust direct string concatenation (p.148).
- **Start the element with a space**: "prefer prompt elements that start with a space rather than ending with
  one" (p.149) — an element that ends with a space/newline contaminates its neighbour.
- **Snippet format is contract**: the block must look like the source document (heading, fence, list), and
  not a proprietary format the model has never seen (p.81).
- **Inertness does not replace delimitation** of the start/end of the answer: see §6.

## 5. Position, importance and dependency

Model the prompt as a set of declared elements, not as a string assembled in the order in which the code
was written.

- **Three relations per element**: "has to account for three ways in which the elements relate to each
  other: position and ordering, importance, and dependency" (p.151). For each element declare a fixed
  position, an importance tier and which other elements it depends on.
- **Recency dominates**: "The closer a piece of information is to the end of the prompt, the more impact it
  has on the model" (p.135); the middle is where the instruction is ignored ("Valley of Meh", p.135).
- **Sandwich**: instruction at the start, context in the middle, refocus at the end — the refocus names what to do
  now (p.135-136).
- **Dependency is coupling**: the snippet and the instruction that explains how to use it travel together; cutting
  one without the other leaves the pair broken (p.151). Requirements and incompatibilities are part of the
  declaration of the element.
- **The ceiling is the window minus the answer**: the element budget is "your context window size minus the
  tokens needed for the model's response" (p.153). The assembly algorithm and the truncation policy are in
  `context-guard`; here the rule is only to declare position and dependency before cutting.

## 6. Parseable output contract

- **Define the start of the answer**: a recognizable start before writing the contract (p.161). The preamble
  can be long — "having a long preamble is a virtue, not a vice" (p.158) — as long as a marker exists that
  separates the preamble from the answer.
- **Choose the terminator by the end test**: the table of p.161 classifies the formats by the column "Test
  for end is test for substring" (Yes/No). Choose the format that answers Yes for your case; indentation
  is not a terminator testable by substring (p.161-162).
- **End testable by substring**: "you can determine the end by checking for specific substrings" (p.162)
  — section heading, code fence and numbered item are valid terminators; "any heading" is not
  (the parser needs the exact literal).
- **Close the input block with an unlikely marker** so the model knows where the output starts
  (AI Engineering, p.243).
- **Declare the exact format**: JSON keys, integer versus fractional, absence of preamble
  (AI Engineering, p.243).
- **Prefer the most economical example format** when the performance is equivalent: 38 versus 27
  tokens in the cited case (AI Engineering, p.242).
- **One contract per answer**: if the output has two parts (label + justification), the contract must
  order both; see §7.

## 7. Classification with disjoint options

- **Unique first token per option**: "you need to make sure each option starts with a unique token"
  (p.165). `North America` versus `Northeast Asia` share a token prefix and the probabilities of the
  two options add up, making the model choose the alternative it considers worse.
- **Prohibit multiple alternatives**: "avoid waffling answers that include several alternatives" (p.165).
- **Enumerated with the label before the justification**: `1. [negative | positive | neutral], 2. [explanation]`
  (p.165) — the first token carries the decision and the justification comes after.
- **A unique label is a prompt fix, not a model fix**: if the classes cannot start with distinct
  tokens, rewrite the labels before blaming the model (p.165).
- **Run the 4-criteria gate** after touching the labels; a new option can break criterion 3 (p.81-82).

## 8. Logprobs as a confidence gate

- **logprob is not probability**: "To convert a logprob to a standard probability, you use the exp
  function" (p.163).
- **Aggregate the first tokens**: "averaging the probabilities ... of early tokens in the completion is
  predictive of overall quality" (p.164). The simple mean of the first tokens is already a predictive signal.
- **Act by threshold, not by intuition**: "Include warnings when the model struggles more than usual"
  (p.164) — the options are not to show, to warn, to retrieve more context or to escalate to a better model
  (p.163-164).
- **Gate ≠ truth**: a low logprob indicates hesitation of the model, and a high logprob does not validate the
  content. Never use that number to justify the answer; use it to decide whether it is displayed with a warning
  (p.163-164).
- **The threshold is product policy**: deciding between showing, warning or escalating is the choice of whoever
  operates the feature; comparing two policies by score is the role of `llm-eval` (p.163-164).

## 9. Review diagnosis

Symptoms that show up in prompt review and the rule that solves them. If the symptom is not here, measure
before touching: comparing versions by score is `llm-eval`.

| Symptom | Probable cause | Rule | Evidence |
| --- | --- | --- | --- |
| the answer restates the problem and does not solve it | the prompt positions the model as a "problem poser" | write the beginning of the answer | p.136 |
| the analysis extrapolates what was asked | implicit scope | report archetype + explicit negation | p.140 |
| the text rambles and does not end | missing stopping point | sentinel section + testable terminator | p.141, p.161-162 |
| the model continues the input document instead of answering | missing end marker of the input block | close the input with an unlikely marker | AI Eng. p.243 |
| output with two alternatives in the same answer | options with a common token prefix | unique first token per option | p.165 |
| the parser captures extra content | delimiter not testable by substring | literal heading/fence instead of indentation | p.161-162 |
| few-shot copied literally into the output | the example carries an unintentional pattern | trigger of §3 and minimal examples | p.104, p.111 |
| the answer changes when reordering snippets | non-inert snippet or broken dependency | separation by whitespace and snippet+instruction pair | p.148, p.151 |
| the answer does not use the data you pasted | data far from the end or outside the refocus | recency + sandwich | p.135-136 |
| a wrong correction displayed with confidence | no policy about logprob | threshold gate | p.163-164 |

## Sources

- Berryman & Ziegler, *Prompt Engineering for LLMs* — ch. 4 Designing LLM Applications (p.81-82); ch. 5
  Prompt Content (p.101-102, p.104, p.106, p.108-109, p.111, p.116); ch. 6 Assembling the Prompt
  (p.135-137, p.140-141, p.143, p.147-149, p.151, p.153); ch. 7 Taming the Model (p.158, p.161-165).
- Huyen, *AI Engineering* — ch. 5 Prompt Engineering: Prompt Engineering Best Practices (p.240-243).
