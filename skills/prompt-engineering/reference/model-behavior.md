# model-behavior

Mechanical behaviour of the model that changes the **usage decision** of prompt and context. It is not an
architecture lesson: each section ends on the practical consequence. The decoding playbook (greedy, beam,
temperature, top-k/top-p) stays in [generation-params](generation-params.md); window budget and compaction stay in
`context-guard`.

`p.N` markers come from the source digest, not from the printed page: in the digest of *Prompt Engineering for LLMs*
the marker is ~14-15 pages above the printed number (ch. 6 printed "123" = marker p.133). A citation without a
book prefix is from *Prompt Engineering for LLMs*; `NLPwT` = *Natural Language Processing with
Transformers*; `AI Eng.` = *AI Engineering*.

## 1. The model sees tokens, not text

The text enters the model as a sequence of token IDs, produced by a four-stage pipeline —
normalization, pretokenization, tokenizer model, postprocessing (NLPwT p.114). Changing the tokenizer is changing
the meaning of the IDs: "switching the tokenizer is like shuffling the vocabulary" (NLPwT p.57), and "you can't
mix and match tokenizers and models" (p.42).

What looks the same may not be:

- **A space before the element changes the token.** The source recommendation is "prefer prompt elements that start
  with a space rather than ending with one" (p.149) — the initial space is part of the token boundary, not
  decoration. Pasting two strings also does not concatenate the token lists: the table of p.148 shows junctions
  1+1➜1 and 1+1➜3, because "pasting strings together doesn't mean the arrays of tokens just get concatenated".
- **A common word can come out split.** "Christa" becomes `Chr` + `##ista` (NLPwT p.117); a tokenizer from another
  domain does not know a common word of yours (T5 never saw "sex", CamemBERT does not know "being") and splitting
  it "will increase the input sequence length" (NLPwT p.330).
- **Accent, emoji and symbol do not have a guaranteed token of their own.** Coverage of all of Unicode is guaranteed by the
  byte alphabet, with 256 symbols (NLPwT p.333); coverage is not the same as one token per character — what
  falls outside the vocabulary is sliced, and the sequence length goes up (NLPwT p.330).
- **Notation and code cost more per character.** The source rule of thumb is ~4 characters per token in
  English and ~2 in code/keys (p.42).

Cost and limit are measured in tokens, not in characters: "so you need to count tokens to stop that from
happening" (p.42). Counting letters, reversing a string or aligning character by character requires the representation that the
model does not receive (NLPwT p.117; p.148); it is a derivation from the mechanics, not a measurement in the digest.

**Consequence for usage:** two templates that look the same can cost and perform differently — measure
with the model's tokenizer before comparing versions, start each element with a space (p.149), and send
character counting/reversal/alignment to code, not to the prompt.

## 2. The model completes a document, it does not obey a command

The model was trained to continue text: "don't stray far from the path upon which the model was trained"
(p.81) and "make your prompts follow the patterns and motifs seen in training data" (p.257). That is why the
unit of decision is the **type of document** the prompt imitates, not the strength of the imperative.

Restating the request is not enough: "the prompt must condition the model to generate a completion that is
actually helpful" (p.82). And everything that is in the document becomes an instruction in practice — "LLMs have a compulsion to
continue patterns" (p.104) and "the model often feels compelled to use every bit of information it gets"
(p.116).

The shape of the document also governs how much the model respects a boundary: a declared limit is respected with
more consistency in a report than in a dialogue (p.140).

**Consequence for usage:** choose the document archetype by the shape of the desired answer before
decorating the instruction — report for a rigid scope, conversation for multi-turn, structured document for
parsed output (`prompt-craft` §1, p.137, p.140, p.143). A stronger instruction does not fix a wrong archetype.

## 3. Autoregressive and without revision

Generation is one token at a time: "Generating text requires at least one forward pass per generated token"
(NLPwT p.160), and the source chapter calls it "One Token at a Time" (p.44). Each chosen token becomes the
condition of the next, so the beginning of the answer anchors the rest — that is why writing the start changes
the behaviour: "change your perspective from problem poser to problem solver and begin writing the answer
for the model" (p.136).

There is no internal draft: "language models have no internal monologue" (p.192) — reasoning that is not in the
text does not happen. And the probability of the first tokens predicts the quality of the whole: "averaging
the probabilities ... of early tokens in the completion is predictive of overall quality" (p.164).

**Consequence for usage:** when the model restates the request instead of solving it, write the start of the
answer in the prompt (p.136); treat the start as an anchor, not as a disposable preamble; read the logprob of the
first tokens as a risk signal, not as truth (`prompt-craft` §8, p.163-164).

## 4. Tokenizer determinism vs model sampling

The tokenizer applies a fixed rule: the same string, in the same tokenizer, produces the same IDs, and the round-trip
`decode(encode(x)) == x` returns the text "without ambiguity" when the whitespace is preserved
(NLPwT p.114-115). The output of the model has no such guarantee:

- **Greedy** always takes the token of highest probability, a single path (NLPwT p.150).
- **Sampling** draws from the distribution, and temperature rescales the logits before the softmax: "When T ≪ 1, the
  distribution becomes peaked ... when T ≫ 1, the distribution flattens out" (NLPwT p.155). With T=2 the output
  becomes gibberish (NLPwT p.155).
- **Truncating the tail** (`top_k`/`top_p`) reduces the draw space, it does not eliminate it (NLPwT p.158-159); the
  declared trade-off is coherence versus diversity (NLPwT p.156).

Therefore, the same input does **not** guarantee the same output: with any temperature above zero, tail tokens
enter the sampling (NLPwT p.155-156). One run is not evidence — "You'll have no automated way of
knowing whether any change is an improvement or a regression" (p.232); judging by visual inspection "won't be
sufficient for application iteration" (AI Eng. p.139); and a score that oscillates between 90% and 70% on resampling
shows an unstable pipeline, not an unstable model (AI Eng. p.227).

**Consequence for usage:** evaluate with a versioned suite (5-20 examples covering the real range already give a signal,
p.232), pin the sampling and version the prompt outside the code with metadata — model, date, schema, sampling — in a
catalog of versions (AI Eng. p.253, p.255), and never conclude "it improved" from a single run.

## 5. Hallucination is the default, not the exception

The output is the most probable continuation of the document, not the truest one. The cost of this shows up even with the
source at hand: in summarization, GPT-2 "hallucinates" facts (NLPwT p.167). When the question has no answer
in the material, the natural behaviour is to fill in: the rewriting model "should acknowledge that this query
isn't solvable instead of hallucinating a name" (AI Eng. p.290).

They are two distinct modes, and both need their own test: factual consistency **local** (against the
provided context) and **global** (against open knowledge) (AI Eng. p.186); the real triggers are "queries
that involve niche knowledge" and "queries asking for things that don't exist" (AI Eng. p.187).

**Consequence for usage:** require a citation of the excerpt that supports each statement and write the absence as a
valid output in the contract — "if there is no evidence in the context, answer NOT FOUND". Treat local
consistency as a check against the pasted context (AI Eng. p.186) and include in the eval set a question whose answer does not
exist (AI Eng. p.187).

## 6. The context window is finite and expensive

The window cost grows quadratically: self-attention "naively scales like O(n²), where n is the length of the
sequence" (NLPwT p.369), and the maximum context limit comes "due to the quadratic memory requirements of the
attention mechanism" — 512 tokens in most BERT, 4.096 in BigBird, 8× larger (NLPwT p.84, p.370). The
real budget is "your context window size minus the tokens needed for the model's response" (p.153).

Inside the window, position matters: "The closer a piece of information is to the end of the prompt, the more
impact it has on the model" (p.135), and the middle is where the instruction is lost ("Valley of Meh", p.135); hence the
pattern instruction at the start, context in the middle, refocus at the end (p.135-136).

Irrelevant context is not neutral: the model "feels compelled to use every bit of information it gets"
(p.116) — "if you retrieve snippets, retrieve the right ones" (p.117). And truncating is not a neutral solution: cutting
text can remove "crucial information" that was at the end (NLPwT p.51).

**Consequence for usage:** order the context by importance instead of by order of arrival, keep the
instruction at the start and the refocus at the end, and omit a snippet that does not serve the answer — the model will use it
anyway (p.116). Counting, order and compaction are budget: `context-guard`.

## 7. What this does NOT explain

- **Reasoning capability is not read in the architecture.** The derivation of self-attention and feed-forward
  (NLPwT p.61-73) stays out of this file; from it only the O(n²) cost is used (NLPwT p.369), which is about
  the price of context, not about answer quality.
- **A public benchmark does not predict your task.** "Public benchmarks will help you filter out bad models, but
  they won't help you find the best models for your application" (AI Eng. p.221), and contamination is common — the
  GPT-3 had "13 benchmarks with at least 40% in the training data" (AI Eng. p.220), which is what the source
  defines as "data contamination happens when a model was trained on the same data it's evaluated on"
  (AI Eng. p.218). A score in MCQ is also not a score in generation: "MCQs test the ability to differentiate good
  responses from bad responses (classification), which is different from the ability to generate good
  responses" (AI Eng. p.184).
- **Nothing here authorizes choosing a model by size.** The choice is confirmed in your eval set and without nailing it into the
  code: "don't bake your choice into your code too firmly" (p.169).

**Consequence for usage:** use this file to predict *behaviour* (tokenization, completion,
anchoring, variability, hallucination, window cost) and use your evaluation suite to decide *which
model and which prompt serve* (`llm-eval`).

## Sources

- Berryman & Ziegler, *Prompt Engineering for LLMs* — ch. 2 Understanding LLMs: Counting Tokens p.42; One
  Token at a Time p.44; ch. 4 Designing LLM Applications: p.81-82; ch. 5 Prompt Content: p.104, p.116-117;
  ch. 6 Assembling the Prompt: p.135-137, p.140, p.143, p.148-149, p.153; ch. 7 Taming the Model:
  p.163-165, p.169; ch. 8 Conversational Agency: p.192; ch. 10 Evaluating LLM Applications: p.232; ch. 11
  Looking Ahead: p.257.
- Tunstall, von Werra & Wolf, *Natural Language Processing with Transformers* — ch. 3 Transformer Anatomy:
  p.84 (context limit), p.61-73 (attention derivation, discarded); ch. 4 Multilingual NER: p.114-117;
  ch. 5 Text Generation: p.150, p.155-160; ch. 6 Summarization: p.51, p.167; ch. 10 Training from Scratch:
  p.330, p.333; ch. 11 Future Directions: p.369-370.
- Huyen, *AI Engineering* — ch. 2 Understanding Foundation Models: Sampling p.105, The Probabilistic Nature
  of AI p.127, Hallucination p.129; ch. 3 Evaluation Methodology: p.139, p.227; ch. 4 Evaluate AI Systems:
  p.184, p.186-187, p.218, p.220-221; ch. 5 Prompt Engineering: p.253, p.255; ch. 6 RAG and Agents: p.290.
- All the numbers are digest markers, not printed pages.

## What not to use from this file

Do not infer capability by model size, by benchmark score or by "larger context": nothing here measures
your domain. Do not use the mechanism to justify a smarter prompt — architecture explains cost and
variability, it does not guarantee reasoning. To decide between two candidates, the evidence is your eval set
(`llm-eval`), not the chapters cited.
