# generation-params

Decoding parameters: how the probability distribution of the model becomes text. Choose by task type
and by cost, not by a default inherited from a tutorial. Basis: ch. 5 (Text Generation) of *Natural
Language Processing with Transformers*, sections `Greedy/Beam Search` (p.148-153) and `Top-k and Nucleus
Sampling` (p.156).

Context: any display policy based on confidence is `prompt-craft` §8 (logprobs); window and context cost
is `context-guard`.

## 1. Decision order

Decide in this order, stopping at the first item that solves the task:

1. **Is the output verifiable and must it be deterministic?** (arithmetic, extraction, short factual answer)
   → greedy, or greedy combined with beam (p.159-160).
2. **Is the output open text and repetitive with greedy?** → beam with an n-gram repetition penalty, or
   sampling (p.150, p.153-154).
3. **Does the open output need controlled diversity?** → truncate the tail (`top_k` / `top_p`) and only then
   adjust temperature (p.155-159).
4. **Always**: pin `max_new_tokens` coherent with the largest real case; there is no decoder that "ends by
   itself" for free (p.160).

There is no universal choice: "there is no universally 'best' decoding method" (p.159-160).

## 2. Greedy

- **What it does**: at each step takes the token of highest probability; a single path, without exploration.
- **Minimal cost**: one forward pass per generated token (p.160).
- **Structural defect**: "one of the main drawbacks with greedy search decoding: it tends to produce
  repetitive output sequences" (p.150) — in free text the model enters a sentence loop.
- **Where it serves**: deterministic task, short answer, output you are going to parse (p.159-160).
- **It is not fixed with temperature**: temperature rescales the distribution before the softmax (p.155); in
  greedy the choice is the token of highest probability, so rescaling does not change the chosen token. To
  change the behaviour in free text you have to change the decoder (p.150, p.153).

## 3. Beam search

- **What it does**: keeps `num_beams` simultaneous paths and picks the sequence of highest joint
  probability; there is no draw between candidates, so the same input tends to the same result (p.153).
- **Cost**: "Generating text requires at least one forward pass per generated token, and even more if we
  use beam search" (p.160) — beam multiplies the compute by path kept.
- **It does not eliminate repetition**: "beam search also suffers from repetitive text" (p.153). Beam improves
  the choice of the sequence, not the diversity.
- **Where it serves**: translation, summarization and a precise task in which you want the most probable
  sequence, not the most creative one (p.159-160).
- **Careful with a large beam**: you pay N× compute for diminishing gain; justify `num_beams` by measurement,
  not by default (p.160).

## 4. Repetition: n-gram penalty

- **Solution for repetition of a deterministic decoder**: "One way to address this is to impose an n-gram
  penalty" (p.153).
- **Measured effect**: with `no_repeat_ngram_size=2` the log-prob falls from −55.23 to −93.12 and the repetition
  stops (p.154) — the text becomes less probable and less repetitive; that is exactly the accepted trade-off.
- **Cost of the penalty**: blocking a repeated n-gram reduces the continuation space. n=2 is the value measured
  in the book (p.154); a larger n prohibits more sequences and starts to veto legitimate repetition of technical
  terms (p.153-154).
- **Real alternative**: switch to sampling with truncation when the task accepts variation (p.155-159).

## 5. Temperature

- **What it does**: rescales the logits before the softmax — "When T ≪ 1, the distribution becomes peaked ...
  when T ≫ 1, the distribution flattens out" (p.155).
- **Effect by range**: low T concentrates mass on the probable tokens; high T spreads mass and starts to
  sample improbable tokens — with T=2 the output becomes gibberish (p.155).
- **The trade-off**: "a trade-off between coherence (low temperature) and diversity (high temperature)"
  (p.156). There is no "correct" T; there is a T coherent with the task.
- **Common error**: raising temperature seeking creativity without truncating the tail — the gain comes from
  the tail tokens entering the sampling, and that is what produces an invented word (p.155-156).
- **Correct order**: truncate first (`top_k`/`top_p`), adjust temperature afterwards (p.158-159). Temperature
  alone does not remove the improbable tail you wanted to exclude.

## 6. top-k and top-p (nucleus)

| Parameter | Cut criterion | Strength | Limit |
| --- | --- | --- | --- |
| `top_k` | keeps the k most probable tokens | simple, predictable | a fixed cut ignores the shape of the distribution: "that fixed cutoff might not be very satisfactory" (p.158) |
| `top_p` | keeps the smallest set that sums to mass p | adapts the number of candidates per step | depends on a calibrated `p`; in a flat distribution it keeps many tokens |

- **`top_k` avoids the tail**: "top-k sampling is to avoid the low-probability choices by only sampling from
  the k tokens with the highest probability" (p.158).
- **Combining the two**: "`top_k=50` and `top_p=0.9` corresponds to ... probability mass of 90%, from a pool
  of at most 50 tokens" (p.159) — the candidate ceiling comes from k, the mass comes from p.
- **Without truncation there is no control**: with any temperature above 0, improbable tail tokens enter the
  sampling (p.155-156).
- **Calibrate p and k together**: touching one changes the effect of the other; record the pair used per task
  (p.159).

## 7. Choice by task

| Task | Decoder | Parameter that matters | Evidence |
| --- | --- | --- | --- |
| arithmetic, extraction, short factual answer | greedy or greedy + beam | determinism; no sampling | p.159-160 |
| parseable output (JSON, label) | greedy/beam | no variation desired | p.159-160 |
| translation, summarization | beam | `num_beams` justified by measurement | p.153, p.160 |
| short open text with repetition | beam + `no_repeat_ngram_size` | n-gram penalty | p.153-154 |
| creative open text | sampling with `top_k`/`top_p` + temperature | truncated tail mass | p.155-159 |
| open chat/dialogue | truncated sampling | coherence versus diversity | p.156 |

Rule of thumb: the easier it is to verify the output automatically, the closer to deterministic the
decoder must stay (p.159-160).

## 8. Cost per generated token

- **Latency floor**: "Generating text requires at least one forward pass per generated token, and even
  more if we use beam search. This makes text generation computationally demanding" (p.160).
- **Budget = input tokens + output tokens × passes per token**: the term that scales per
  request is output tokens × passes per token, that is, `max_new_tokens` × `num_beams` (p.160).
- **Practical consequence**: cutting `max_new_tokens` and avoiding beam in a task that does not need it reduces
  cost directly; the output cost is what scales per request (p.160).
- **Measure before fixing**: the decoder choice is a decision per task; comparing the quality of two
  decoders requires a baseline and an eval set, outside the scope of this skill (p.159-160).

## 9. Diagnosis by symptom

| Symptom in the output | Probable cause | Action | Evidence |
| --- | --- | --- | --- |
| sentence loop, the text repeats | greedy (or beam) in open text | change the decoder; n-gram penalty | p.150, p.153-154 |
| repetition even with beam | beam also repeats | `no_repeat_ngram_size` or sampling | p.153-154 |
| invented word, incoherent text | high T without tail truncation | lower T and truncate before with `top_k`/`top_p` | p.155-156 |
| text always the same, without variation | low T / greedy | raise T with a truncated tail, if the task accepts diversity | p.156 |
| bad candidates even with a high k | a fixed cut does not follow the distribution | use `top_p` (or combine with `top_k`) | p.158-159 |
| an occasional good answer lost | 1 path only | beam with a justified k | p.153, p.160 |
| cost/latency above the predicted | high `num_beams` or `max_new_tokens` | reduce beam and the token ceiling | p.160 |

## Sources

- Tunstall, von Werra & Wolf, *Natural Language Processing with Transformers* (Revised Edition) — ch. 5
  Text Generation: `Greedy/Beam Search` (p.148-153; greedy on p.150, beam and n-gram penalty on
  p.153-154), temperature (p.155-156), `Top-k and Nucleus Sampling` (p.156-159), generation cost (p.160).
