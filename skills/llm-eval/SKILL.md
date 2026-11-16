---
name: llm-eval
description: "Use when measuring quality with a number: run eval-run, compare pass_rate against baseline and detect regression of a skill or prompt. Measures an existing artifact; does not write a prompt nor invent a rubric."
---

# LLM Eval — measure what already exists

**Measurement** skill. Scope: validate suite, run, read report and decide whether the number is
trustworthy. Writing or rewriting a prompt, skill or rubric is another task — here you measure the
artifact in the state it is in.

## Use this skill when

- Someone says "it improved"/"it got worse" and you need a number with baseline.
- Before and after editing `SKILL.md`, `AGENTS.md`, prompt or template.
- You need a regression gate in CI (passed/did not pass with declared criterion).
- You need to decide whether a case fits an exact rule (`must_match`) or requires judge.

## Do not use this skill when

- There is no suite yet: first define the cases (format in
  `reference/eval-harness.md`).
- The question is "how do I write this prompt better" — that is not measurement.
- The artifact is code and the criterion is deterministic: use a normal test (`go test`, e2e).

## Flow

```bash
# 0. binary (make build generates bin/eval-run)
make build

# Real example suite, to see the format in use:
#   tools/cmd/eval-run/testdata/smoke.jsonl

# 1. validate the suite before running
bin/eval-run suite check <your-suite>.jsonl

# 2. baseline (before the change)
bin/eval-run run --suite <your-suite>.jsonl --target skills/repo-auditor/SKILL.md \
  --model baseline-manual

# 3. edit the artifact ...

# 4. same suite, same target, against the baseline
bin/eval-run run --suite <your-suite>.jsonl --target skills/repo-auditor/SKILL.md \
  --baseline <run_id from step 2> --tolerance 0.02

# 5. read what was recorded
bin/eval-run report                 # last run
bin/eval-run list --limit 10        # history
```

## How to read the report

| Signal | Meaning |
| --- | --- |
| exit `0` | no failure and, if there was a baseline, comparable |
| exit `1` | at least one case in `fail` |
| exit `2` | invalid suite, missing target, broken judge, wrong usage |
| exit `3` | `suite_hash` or `target_hash` diverges from the baseline: **do not compare** |

- `scored` + `skipped` = `cases`. `skipped` is a case without an applicable rule (e.g.: `judge`
  block without `--judge-cmd`); **never** read it as a hit.
- `pass_rate` of a slice is weighted by `weight` and uses only `scored` cases in the denominator.
- `delta_pass_rate` and `regressions` only exist when `comparable=true`. Regression = drop
  above `--tolerance` (default `0.02`), measured by slice.
- Read `by_slice` before the total: a stable total with a slice dropping is regression.

## What is a trustworthy number

- **Equal hashes.** Without `suite_hash` and `target_hash` identical to the baseline there is no delta; there
  is only exit `3`.
- **Delta larger than the sample.** A small change needs more cases: each 3× reduction in the
  difference asks for ~10× the samples.
- **No slice dropping** with the total equal.
- **Gain does not come from format.** If `pass_rate` rose together with the artifact size, with
  file name or with section header, treat it as suspect until inspecting one sample
  that passed.
- **Auditable judge.** A judge score is only worth it with a visible rubric, `max_score` declared and
  `--model` recorded. A judge without a visible prompt is not a metric.

## Common errors (all of them invalidate the measurement)

- Editing the suite and comparing against an old baseline (becomes exit `3`, but the number has already changed ruler).
- Running once and calling it a result: one execution does not distinguish signal from noise.
- Reporting only the total and ignoring the slice that dropped.
- Counting `skipped` as success to "close green".
- Trusting a judge whose rubric was adjusted after seeing the result.

## Reference

- `reference/eval-harness.md` — real JSONL schema, frozen split, slice, baseline, regression gate,
  record fields and binary flags.
- `reference/metrics-and-judge.md` — exact vs judge, rubric, judge bias, mandatory
  baseline.
