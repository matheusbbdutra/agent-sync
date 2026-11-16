# Metrics and judge — exact, rubric, bias and baseline

How to decide **what** to measure before running the harness, and when the number comes from code
(exact) and when it comes from a judge. Pages cited from the source digest; the behavior of
`eval-run` corresponds to the code in `tools/internal/evalrun/score.go`.

## 1. Choose the metric by task type, not by what is easy to compute

- Knowledge/classification → close-ended task with a declared random baseline; open
  generation → verification by judge/rule; code/SQL → executable functional correctness (pass@k,
  with `k` declared). "MCQs test the ability to differentiate good responses from bad
  responses (classification), which is different from the ability to generate good responses"
  (p.184); "The final score, called pass@k, is the fraction of the solved problems out of all
  problems" (p.149).
- Choose the metric by purpose and by cost asymmetry, and anchor the threshold in the cost
  of the error: "The purpose of the model will determine which evaluation metrics are most
  important" (p.112); "metrics that are robust to noise, such as mean absolute error" (p.113);
  "RMSLE is less responsive to outliers than RMSE, and it tends to penalize underestimations
  slightly more than overestimations" (p.115).
- Threshold is a declared parameter, not a tool default: "The best confidence threshold
  will depend on your business needs and goals, so there is no single correct answer here"
  (p.154).
- Rare class destroys the aggregate metric: "the PR curve tends to be a stronger metric in the
  case of unbalanced datasets" (p.147); "accuracy quickly becomes unreliable on its own"
  (p.145).

## 2. Exact (deterministic) — what it proves and what it does not prove

- In the harness, exact = `must_match` (case-insensitive substring), `must_not_match` and
  `json_schema` (JSON parse + level-1 `required`). It does not cost token, does not vary between runs and
  is the only signal you can use in a CI gate without a statistical baseline.
- Exact is **necessary and insufficient**. Compare with the EM/F1 lesson: "EM = 1 if the
  characters in the predicted and ground truth answers match exactly" (p.216) — EM underestimates
  ("about 6000 hours" vs "6000 hours" → EM 0, F1 0.8, p.216) and F1 overestimates ("about 6000
  dollars" → EM 0, F1 0.4, p.217). "Relying on just the F1-score is thus misleading … tracking
  both metrics is a good strategy" (p.217).
- Translating to the harness: `must_match` is loose (substring), `json_schema` is rigid
  (field presence), neither of them measures content quality. Whenever the suite only has
  exact, declare that in the report — `pass_rate` measures form, not correctness.
- Exact serves to lock what cannot vary: output format, section presence, expected skill
  name, absence of prohibited term. Do not use judge to check field presence.

## 3. Trivial baseline before any metric on generated text

- Always include an idiotic heuristic and report side by side; if it wins, the model number
  is not evidence. Baseline from the book: "the first three sentences" (p.143) — and in the
  corresponding table the baseline scores ROUGE-1 0.303571 against 0.187500 of gpt2 (p.173).
- Prefer a metric with internal tokenization over a homemade implementation: "it expects the text to
  already be tokenized … SacreBLEU … is the preferred metric for benchmarking" (p.170).
- Automatic text metric is a proxy: "human judgment remains the best measure" (p.183). In the
  harness, that means: judge/regex never replaces inspection of the sample that failed.

## 4. Rubric — write it before measuring, and validate it with a human

- Creating the guideline is the most important step of the pipeline: "Creating a clear evaluation
  guideline is the most important step of the evaluation pipeline" (p.223).
- Validate the rubric with humans before trusting it as a metric: "Validate your rubric with
  humans… If humans find it hard to follow the rubric, you need to refine it" (p.224). A rubric
  that a human does not apply consistently does not generate an interpretable score.
- Declare what the system **must not** do, not only what it must do. A rubric without negative scope
  approves "correct but useless" (p.223).
- In the harness: `judge.rubric` is short, actionable free text — criteria, scale and what
  disqualifies. If the rubric needs more than one paragraph, it is not ready yet.

## 5. Anatomy of the judge prompt

- Three mandatory parts: (1) the task, (2) the detailed criteria, (3) the scoring
  system with examples per grade — "1. The task… 2. The criteria… 3. The scoring system"
  (p.162).
- Prefer classification over a continuous number: "AI judges work better with classification than
  with numerical scoring systems" (p.162). If you need a number, use a short discrete scale
  (1–5) with meaning per grade, as in the book.
- An opaque judge is an invalid judge: "Do not trust any AI judge if you can't see the model and the
  prompt used for the judge" (p.165). In the harness this is structural — the judge prompt is the
  `judge-cmd` you version, and `--model` is recorded in the record.
- Harness normalization and threshold: `normalized = score / max_score`, approved when
  `normalized >= threshold`, with `threshold` in `(0,1]`. This prevents comparing
  different scales as if they were the same metric: Table 3-4 of ch. 3 lists 1-5 for MLflow,
  0/1 for Ragas and YES/NO for LlamaIndex under the same label "faithfulness", and the scoring
  system declared in the judge prompt is what defines the scale (p.162).

## 6. Judge bias — assume it before interpreting delta

- Cost multiplies: generating + evaluating per criterion makes several calls per item — "you'll do
  twice as many GPT-4 calls… increase your number of API calls four times" (p.166). Define the
  judge sampling rate and record it.
- Position bias: "Many AI models have first-position bias" (p.167) — when comparing two
  responses, reorder and repeat; never compare in fixed order.
- Verbosity bias: the judge prefers a long response "regardless of their quality" (p.167). A gain
  in `pass_rate` accompanied by an increase in length is suspect until proven otherwise.
- Self-bias and spot-checking: reviewing by sample "might fail to catch some failures" (p.166).
  Operational conclusion: judge serves for aggregate signal and monitoring, not for approving
  an individual case without inspection.

## 7. Mandatory baseline

- Without a baseline, `pass_rate` is a loose number. Comparison is only worth it with suite and target frozen
  (equal hashes); any change in one of the two makes the delta incomparable — and the binary
  exits with `3` precisely to prevent improper interpretation.
- Choose criterion and success target **before** seeing the number: "satisfactory performance is
  something that should be dictated by the business needs and should be agreed upon before
  starting any ML engagement" (p.27).
- Do not choose the winner by the set you report on: "the final model you choose may be
  biased toward the validation dataset since you specifically choose the model that performed
  the best on the validation dataset" (p.264).
- When adding judge to a suite that used to be exact only, the baseline changes ruler: run the
  baseline again in the same configuration before comparing. A `judge` case without `--judge-cmd`
  stays `skipped` (never `pass`), and a `--judge-cmd` that breaks aborts the run with `2`.

## 8. Where the judge does not enter

- Format and schema: use `json_schema` (level-1 `required`) — deterministic and cheap.
- Term presence/absence: `must_match`/`must_not_match`.
- Item count, size, order: explicit rule or human review; a 1–5 scale judge
  does not measure magnitude.
- Safety and policy: if a prohibited case passes, the cost of FP and FN is not symmetric —
  treat it as a rigid gate (`must_not_match`), with the error cost declared (p.147, p.154).

## 9. Worked example (real routing suite)

Exact case — the request is the stimulus, the expected skill name is the rule:

```json
{"id":"roteamento-pt-01","slice":"pt-br","weight":1,"input":{"kind":"prompt","text":"audite o repo e liste os débitos técnicos"},"expect":{"must_match":["repo-auditor"]}}
```

Running it against `skills/repo-auditor/SKILL.md` the case passes (the file contains its own name);
running it against another SKILL.md, it fails. That is: `must_match` of a name proves **which artifact
was selected**, not that the routing decided well — for that the output must be the decision of the
router, not the document.

Negative case — the same input with a prohibited term separates confusing neighbors:

```json
{"id":"roteamento-neg-09","slice":"negativo","weight":2,"input":{"kind":"prompt","text":"audite as vulnerabilidades de segurança deste serviço"},"expect":{"must_match":["security-auditor"],"must_not_match":["repo-auditor"]}}
```

Judge case — when the answer is free text and no substring proves correctness:

```json
{"id":"roteamento-aberto-10","slice":"aberto","input":{"kind":"prompt","text":"resuma o que este agente fez nesta sessão"},"judge":{"rubric":"Cita um arquivo alterado e uma pendência concreta?","max_score":5,"threshold":0.6}}
```

Without `--judge-cmd`, this case exits `skipped` (never `pass`). With a judge that returns
`{"score":5}`, `5/5 = 1.0 >= 0.6` → `pass`; with `{"score":2.5}`, `0.5 < 0.6` → `fail`.

## 10. Checklist before trusting a judge

- [ ] The rubric was written before the first run and was not adjusted to the result.
- [ ] The scale is discrete and each grade has meaning (p.162).
- [ ] The judge prompt/model is visible and versioned (p.165).
- [ ] Judge sampling and cost are declared (p.166).
- [ ] Order and length were controlled in the comparison (p.167).
- [ ] There is a baseline with identical suite and target; delta is only read when `comparable`.
- [ ] There is a trivial or exact baseline alongside to prove the judge adds signal (p.143,
      p.173).

## Sources

- Tunstall, von Werra, Wolf, *Natural Language Processing with Transformers* (O'Reilly, 2022)
  — summarization/metrics ch.: p.143, p.170, p.173, p.183; QA ch.: p.216–217. Digest
  `analysis/nlp-transformers.md` (U12, U18).
- Stripling & Abel, *Low-Code AI* — ch. 1: p.27; ch. 4: p.112–115; ch. 5: p.127, p.145,
  p.147, p.154; ch. 8: p.264. Digest `analysis/low-code-ai.md` (U1–U8).
- Huyen, *AI Engineering* — ch. 3 (Exact Evaluation p.149, AI as a Judge p.162, Limitations
  p.165, Biases p.166–167), ch. 4 (Generation Capability p.184, Evaluation Criteria p.223–224,
  Design Your Evaluation Pipeline p.221). Digest `analysis/ai-engineering.md` (U3, U5, U8, U9).
- Binary behavior: `tools/internal/evalrun/score.go` (`ScoreCase`, `ExecJudge`,
  normalization `score/max_score`) and `tools/cmd/eval-run/main.go` (exit codes).
