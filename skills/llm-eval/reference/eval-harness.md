# Eval harness — suite, frozen split, slice and regression gate

Operational contract of `bin/eval-run` (Go, `tools/cmd/eval-run`). Every rule below has a
verifiable page in the source digest; the schema and the binary behavior were copied
from the code and from the real command output, not from memory.

## 1. What the harness measures

- Measures **one artifact** (`--target` file: SKILL.md, prompt, AGENTS.md) against a **frozen
  suite** of cases. The target content is the output applied to each case.
- Deterministic signal first: `must_match`, `must_not_match`, `json_schema` do not depend
  on a model, do not cost token and are reproducible byte by byte.
- Expensive signal later: `judge` only runs when `--judge-cmd` is passed, and only for cases that
  declare a `judge` block. Compose by cost: cheap signal in 100% of the cases, expensive in the
  sample you know how to interpret (p.225 — "a cheap classifier that gives low-quality
  signals on 100% of your data, and an expensive AI judge… on 1% of the data"; p.139).
- Evaluate component and end: one suite per artifact (routing, output format, section
  coverage) besides the end-to-end suite. "You should evaluate the end-to-end output and each
  component's intermediate output independently" (p.221).

## 2. Real case schema (JSONL)

One line = one case, empty lines ignored. Fields exactly as in
`tools/internal/evalsuite/suite.go`:

| Field | Type | Validation rule |
| --- | --- | --- |
| `id` | string | required, non-empty, unique in the suite |
| `slice` | string | required, non-empty |
| `weight` | number | `> 0`; absent/0 becomes `1` |
| `input.kind` | string | label of the stimulus (e.g.: `prompt`) |
| `input.text` | string | stimulus readable by a human |
| `expect.must_match` | []string | substring, case-insensitive, all must appear |
| `expect.must_not_match` | []string | substring, case-insensitive, none may appear |
| `expect.json_schema` | object | compilable JSON Schema; scoring checks level-1 `required` |
| `judge.rubric` | string | required if `judge` exists |
| `judge.max_score` | number | `> 0` |
| `judge.threshold` | number | fraction in `(0,1]` of `max_score` |

Minimum valid example (one line, no breaks):

```json
{"id":"roteamento-pt-01","slice":"pt-br","weight":1,"input":{"kind":"prompt","text":"audite o repo"},"expect":{"must_match":["repo-auditor"]}}
```

Validation (`eval-run suite check`): empty/duplicate id, empty slice, `weight <= 0`, case without
any rule in `expect` or `judge`, `json_schema` that does not compile, `judge` without `rubric`,
`max_score <= 0` or `threshold` outside `(0,1]` — each problem comes with line and field.

## 3. Frozen split

- Generate the suite **once**, version it in git and never re-split: "Splitting the dataset at the
  beginning of each training run is not a good idea… we should split once, and then continue
  using the same training and validation datasets for all our hyperparameter tuning
  experiments" (p.213).
- Practical consequence: the `suite_hash` (sha256 of the file) is the identity of the suite. Editing the
  suite invalidates the historical comparison — and the binary refuses to compare (§6).
- Separate what **chooses** from what **confirms**: use validation to decide between candidates and
  test to report the final number; choosing by the same set you report on produces
  optimistic bias (p.264). In the harness: distinct `.jsonl` suites or distinct `--slice` for the
  two roles.
- Reserve known-error cases and out-of-scope cases: "If you care about something, put a
  test set on it" (p.227).

## 4. Slice

- Every case belongs to a named slice (language, task type, input size, origin).
  Slice is what prevents the average from hiding the tail: "the model may perform much worse on some
  slices of the data than others, and these deficiencies can be lost in the ocean of
  calculating on the entire dataset" (p.314).
- Read `by_slice` before the total. A stable total with a dropping slice is regression, not noise
  (it is the same phenomenon described in p.314–315).
- `--slice <name>` runs only one slice; should the name not exist in the suite, the binary exits `2` instead
  of reporting zero cases.
- A new slice appears in the delta as `new_slice: true` and **does not** enter the global delta calculation
  nor the regression count — comparing different universes would give a number without meaning.

- A new slice appears in the delta as `new_slice: true` and **does not** enter the global delta calculation
  nor the regression count — comparing different universes would give a number without meaning.

## 4.1 Failure mode becomes slice

- Before choosing slices, list the failure modes you want to catch; the metric derives
from them, not from what the tool reports by default (p.487, "design your metrics around these
failures").
- For artifacts with action (routing, skill choice, tool call), enumerate the modes and
measure frequency per mode: invalid tool; valid tool with invalid parameter; valid tool with
wrong value; objective not reached; reflection that declares completion without being (p.318–319).
Each mode is a slice or a `must_not_match` case.
- Rare failure requires its own treatment: with a rare positive class, the aggregate metric is useless and
the decision needs the cost of FP vs FN made explicit (p.127, "equally accurate and useless";
p.145, "accuracy quickly becomes unreliable on its own").

## 5. Baseline

- Without baseline there is no "improved": every number is absolute and not comparable. The append-only
  record (`.agent-sync/eval-runs.jsonl`) is the de facto baseline.
- `--baseline` accepts a records file path (uses the **last** line) or `run_id`
  (searches the current record). `baseline`, `delta` and `comparable` are recorded in the record,
  so the verdict is auditable later.
- Choose the metric and the criterion **before** looking at the number: "satisfactory performance is
  something that should be dictated by the business needs and should be agreed upon before
  starting any ML engagement" (p.27).

## 5.1 Versioning of target and suite

- Treat the suite as a prompt: outside the code, versioned, with a change metadata — "It's good
practice to separate prompts from code" (p.253; versioned catalog in p.255). The `suite_hash`
is the effective version; if the file changes without the version changing, the `pass_rate` history loses
meaning.
- Two artifacts do not share a baseline: different target = different `target_hash` = exit 3.
Compare as separate series, one per artifact.

## 6. Regression gate

- `comparable=false` and exit `3` when `suite_hash` **or** `target_hash` diverges from the baseline.
  Reason: with a different suite or target, the `pass_rate` difference is not attributable to the change
  you want to measure.
- Regression = drop in `pass_rate` **above** `--tolerance` (default `0.02`), by slice.
  Strict comparison with epsilon `1e-9` so that a drop exactly equal to the tolerance does not
  become regression through floating-point error.
- `pass_rate` is weighted by `weight` and calculated only over `scored` cases: `pass_rate =
  Σ weight_pass / Σ weight_scored`. A `skipped` case stays **outside the denominator** — never counts
  as a hit.
- Exit code precedence: `3` (incomparable) wins over `1` (failures), because a delta measured
  against another target must not be interpreted even when there are failures.
- Detect small differences with sample, not with generous tolerance: "for every 3× decrease
  in score difference, the number of samples needed increases 10×" (p.228); unstable
  bootstrap resampling (90% in one, 70% in another) means the suite is too small (p.227).

## 7. How to run (real flags)

```
eval-run suite check <suite.jsonl> [--json]
eval-run run --suite <s> --target <t> [--baseline v] [--tolerance 0.02] \
             [--judge-cmd '<cmd>'] [--slice <name>] [--model <name>] \
             [--record <file>] [--no-record] [--json]
eval-run report [--run <run_id>] [--record <file>] [--json]
eval-run list [--record <file>] [--limit N] [--json]
```

- Default record: `.agent-sync/eval-runs.jsonl` (append-only, matched by `.gitignore`).
  `--no-record` runs without recording (useful in a throwaway experiment).
- `--judge-cmd` is an external process: it receives `{"case":...,"output":...}` on stdin and returns
  `{"score":N,"reason":"..."}` on stdout. No provider SDK, no HTTP. A judge that fails or
  returns invalid stdout aborts the run with exit `2` — a judge error never becomes a verdict.
- Exit codes: `0` ok, `1` failures, `2` usage/config, `3` incomparable (hash).

## 8. Record fields

`schema_version`, `run_id` (`eval-YYYYMMDD-HHMMSS-<4 hex>`), `ts` (RFC3339 UTC), `suite`,
`suite_hash`, `target`, `target_hash` (all hashes `sha256:<hex>`), `model`,
`totals{cases,scored,skipped,pass,fail}`, `by_slice[{slice,cases,scored,skipped,pass,fail,weight_pass,weight_scored,pass_rate}]`,
`baseline{run_id,ts,pass_rate}`, `delta{pass_rate,regressions}`, `comparable`.

The record does not store per-case results: if you need the detail, use the run output
(`--json` brings `results` with the rule, the pattern and the reason for each verdict).

## 9. Suite growth in production

- Sample and save a fraction of the real executions, label by the same criterion as the suite and
add as a case: "Randomly sample and save the data being sent for model predictions…
we might choose to save 1%" (p.316).
- Trigger review by an explicit trigger (time or performance drop), not by complaint:
retrain/alert by threshold and by compared window (p.316; p.310 for trigger by percentage
of reported loss).
- Distinguish input drift from feature→label relationship drift when deciding what to do with the
result: "data drift, a change in the underlying data distribution" vs "concept drift is
where the relationship between the features and the labels change over time" (p.310). In
practice: new slice or different case distribution = input drift; same slice dropping =
relationship changed.

## 10. Checklist before trusting the number

- [ ] `suite_hash` and `target_hash` equal to the baseline ones (otherwise exit 3 and nothing to interpret).
- [ ] `skipped` was looked at: if many cases have no applicable rule, the suite measures little.
- [ ] The total matches `by_slice`; no slice dropped with the total stable.
- [ ] The number is not from a single execution when the delta is small (p.227, bootstrap;
p.228, 3× difference requires 10× sample).
- [ ] If there is `judge`, the judge prompt is visible and auditable (p.165).
- [ ] The gain does not come from a format clue: file name, header, section order
(p.211).

## 11. Shortcuts that invalidate the measurement

- Changing the suite and comparing against an old baseline: it becomes exit `3`, but the habit hides that the
  number changed ruler.
- Counting `skipped` as success: a case without an applicable rule is never evidence of a hit.
- Accepting gain without looking at slice and without looking at what the artifact actually uses — "we need to be
  mindful of what other things are within our images (and examine model explanations)"
  (p.211): file name, header, section order and item count are easy clues
  to learn instead of the real behavior.
- Reporting only the global average when the failure case is rare: a metric dominated by the
  majority class "gets it right" and is useless (p.302, "correct 998 times and wrong only twice… accuracy is
  not a useful evaluation metric").

## 12. Routing evaluation

Routing is the decision "given this request, which skill the agent chooses". The real suite lives in
`tools/cmd/eval-run/testdata/routing.jsonl`: 12 cases, slice `pt-br` (5), `en` (4) and
`negativo` (3), each with a request written as a user would write it and the `name` of the skill that
should be chosen in `expect.must_match`. In the `negativo` cases the `must_match` points to the
correct skill and the `must_not_match` points to the tempting neighbor (`security-auditor` against
`repo-auditor`, `dependency-management-deps-audit` against `repo-auditor`, `prompt-engineering`
against `llm-eval`).

Why this is not measured in offline mode: without `--runner-cmd` the evaluated output is the content of
`--target`, the same text for all cases. `must_match` of a skill name then only proves
that that file was chosen: every `SKILL.md` contains its own name (0 exceptions in 34), and
no file in the tree contains the 12 expected names (the maximum measured is 3, in
`prompt-engineering`). Measured: `routing.jsonl` with `--target skills/repo-auditor/SKILL.md` gives
`pass 1 fail 11`, and `smoke.jsonl` in the same mold gives `pass 3 fail 6 skipped 1` — the result
stops at a set of names the file contains, not at the router decision. Routing is only
measured with the output of each case coming from the router.

The runner contract is the same as `--judge-cmd`: the binary writes `{"case":...}` on stdin and
reads the case output on stdout. `tools/cmd/eval-run/testdata/routing-runner.sh` (bash, thin glue)
extracts `input.text`, assembles a prompt with the catalog (`name` + `description` from the frontmatter of
`skills/*/SKILL.md`), calls the CLI pointed to by `EVAL_CLI` and prints only the chosen id. The CLI is
free: `opencode run`, `claude -p` or anything else. Without `EVAL_CLI` the runner exits with `2`, and
a non-zero exit in the runner is an infrastructure error: the run aborts with exit `2` instead of
marking `fail`.

```bash
# real routing (spends model)
EVAL_CLI='opencode run' ./bin/eval-run run \
  --suite tools/cmd/eval-run/testdata/routing.jsonl \
  --runner-cmd tools/cmd/eval-run/testdata/routing-runner.sh --no-record

# plumbing without model: the fake always answers a fixed id
EVAL_CLI='bash tools/cmd/eval-run/testdata/fake-cli.sh' FAKE_ANSWER=repo-auditor \
  ./bin/eval-run run --suite tools/cmd/eval-run/testdata/routing.jsonl \
  --runner-cmd 'bash tools/cmd/eval-run/testdata/routing-runner.sh' --no-record
```

With `FAKE_ANSWER=repo-auditor` the result is `pass 1  fail 11`: only the case that points
to this skill passes. If the plumbing is broken, the pattern changes (0 pass, all `fail`, or
abort by runner) — it is the plumbing test, not the routing quality test.

Explicit limit: 12 cases do not prove routing quality, they prove regression. They catch
change that pushes the request to the wrong skill, they do not cover the long tail of ambiguous
requests. A new suite here is a new case with a real request (p.316, production sampling), and
a collision between neighboring skills becomes a `negativo` case with larger `weight`.

## Sources

- Huyen, *AI Engineering* (O'Reilly, 2025) — ch. 3 (Evaluation Methodology): p.139, p.147–149;
  ch. 4 (Evaluate AI Systems): p.221, p.225, p.227, p.228; ch. 10 (AI Engineering
  Architecture and User Feedback): p.487. `[[page N]]` marker of the digest
  `analysis/ai-engineering.md`.
- Lakshmanan, Görner, Gillard, *Practical Machine Learning for Computer Vision* (O'Reilly,
  2021) — ch. 5 (Creating Vision Datasets): p.211; ch. 8 (Model Quality and Continuous
  Evaluation): p.264, p.302, p.314–315, p.316. Digest `analysis/practical-ml-cv.md` (U1–U6).
- Stripling & Abel, *Low-Code AI* — ch. 1: p.27; ch. 9 (Next Steps): p.310. Digest
  `analysis/low-code-ai.md` (U1, U8).
- Schema and binary behavior: `tools/internal/evalsuite/suite.go`,
  `tools/internal/evalrun/{score,compare,record}.go`, `tools/cmd/eval-run/main.go` and real output
  of `eval-run --help`.
