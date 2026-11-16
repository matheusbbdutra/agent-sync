# token-economics

Measured cost of the three reduction tools (`repo-map`, `ast-outline`, `trace-strip`) and of the
equivalent `ctx-window` subcommands, with the point where optimizing stops paying off. All numbers
below were measured in the agent-sync repo itself (526 files, 1778 symbols, `repo-map --summary`) and the
command is next to each line. There is no provider bill estimate: what is measured here is output
size (which becomes a token in the prompt) and wall clock.

## 1. Unit of charge

- What reaches the model is token, not byte. Book reference: ~4 chars/token in English and ~2 in
  code/keys, and the count **is not additive** between snippets (Prompt Engineering, p.148; p.42).
- Consequence for the tables below: dividing bytes by 4 underestimates the cost of code. A cut of
  2.242 bytes from `repo-map --focus` over a `.go` costs closer to 1.100 tokens than to 560.
- No tool in the repo counts real tokens: `EstimatedChars` adds up chars (`store.go:90-96`) and the
  `--max-tokens` of `repo-map --summary` counts words (`repomap.go:1148-1156`). Mark it as a divergence
  whenever comparing with the provider budget.
- Order of magnitude of one turn: one tool call becomes ~16000 chars at most (`summarize.go:336-346`), which
  is already ~4k tokens per large file read — more expensive than all the optimized outputs of this page added up.

## 2. Measured cost per operation

Run with `--cache-dir /tmp/p4bench/cache` so as not to write cache into the repo.

| Operation | Time | Output | Equivalent input | Gain |
| --- | --- | --- | --- | --- |
| `repo-map --update` (cold cache) | 413 ms | — | 526 files scanned | — |
| `repo-map --update` (no change) | 64 ms | — | empty delta | 6,5x faster than cold |
| `repo-map --summary` (default 1000) | 33 ms | 4.288 B | — | -93% vs no limit |
| `repo-map --summary --max-tokens 500` | 43 ms | 2.180 B | — | -96% vs no limit |
| `repo-map --summary --max-tokens 2000` | 34 ms | 8.429 B | — | -85% vs no limit |
| `repo-map --summary --max-tokens 0` | 33 ms | 57.442 B (~14k tokens) | — | **none** |
| `repo-map --focus <file>` | 31 ms | 2.242 B | 17.620 B (`store.go`) | -87% |
| `repo-map --brief <file>` | 29 ms | 3.685 B | 17.620 B | -79% |
| `ast-outline <file>` | 2 ms | 1.276 B / 39 lines | 17.620 B / 592 lines | -93% |
| `ctx-window skeleton <file>` | 3 ms | 2.001 chars, `reduction_pct 88` | 17.620 B | -88% |
| `trace-strip` (synthetic stack trace) | 4 ms | 6 lines kept | 8 lines | 2 lines hidden |

Cost facts the table hides:

- `--summary --max-tokens 0` is the only path that **worsens** the context: 57.442 B (~14k tokens) in 1236 hubs
  in this repo. It is not optimization, it is dumping — that is why the default is 1000, with a truncation warning on the last
  line. Never pass `0` out of curiosity.
- The `repo-map` cache weighs 2.887.723 B on disk. It costs disk, not context — the savings are in
  `--focus`/`--brief`, not in the cache.
- `--max-tokens` counts `len/4` both in `--summary` and in `--brief` (`repomap.go:789-791`). Before,
  `--summary` counted words and delivered ~2,4x the requested value; fixed on 2026-11-16.
- `ast-outline` only uses a real AST in Go (`astoutline.go:25-27`); `.py/.js/.ts/.php` fall back to regex
  (`astoutline.go:29-35`). The 93% gain does not guarantee a correct skeleton outside Go.
- `skeleton` only saves above 8 KiB (`skeletonMinBytes`, `skeleton.go:29-31`); below that it returns the
  whole file with `reduction_pct 0`, at the cost of one tool call.
- `expand` rehydrates the original from the archive: the gain of archiving is temporary and only holds while
  nobody asks for the content back (`--help` of `ctx-window archive|expand`; limits in `archive.go:27-32`).
- Each tool call costs up to 16000 chars in the working memory, regardless of the size of the file read
  (`summarize.go:336-346`). The opt-in `AUTO_ARCHIVE` swaps the large result for a 500-char preview and
  keeps the rest on disk (`archive.go:27-32,210-214`): it is the only lever that cuts cost **by number of
  tool calls**, not by read size.

## 3. When cache pays off

- **`repo-map` artifact cache: yes, from the second read on.** Cold 413 ms, warm 64 ms. If the
  session will do a single search, `--update` is pure cost.
- Divergence with the `SKILL.md` of this skill: it announces a "<20ms" cache; measured here in the agent-sync repo,
  `--summary` took 33 ms and `--update` with no changes 64 ms. Use the measured numbers, not the one from the SKILL.
- **Skeleton cache: yes, by fingerprint.** `skeleton` writes by mtime+size+hash with `from_cache` in the
  JSON (`skeleton.go:31,44-54`): the second call on the same file does not re-parse.
- **Prompt cache: does not exist in the repo.** The book treats prompt caching as the big gain — invariant
  segments processed once, with "up to 90% cost savings" and "up to 75% latency reduction"
  (AI Engineering, p.463-464; p.481: do not cache a query tied to a user; p.482: semantic cache has
  fragile components). agent-sync caches **file and skeleton**, not prompt prefix: the system
  prompt and the rules are resent on every turn without explicit cache. Divergence, and the one of highest
  absolute value in the cost per session.
- The book's criterion for caching: the segment repeats on every call (p.463). Rules + loaded skill +
  STATE.md satisfy that and today are not cached by any layer of the repo.

## 4. Point of diminishing returns

- A big cut is free; a fine cut is expensive. Same file: 17.620 B -> 2.242 B (`focus`, -87%) ->
  1.276 B (`ast-outline`, -93%). The final 6 percentage points (87% -> 93%) save 966 B (~240 tokens)
  and cost one extra tool call.
- In `--summary`, the first cut removes 94% (81.308 -> 4.638 B). Going from 500 to 2000 "tokens" multiplies
  the output by 4,2 (4.638 -> 19.575 B) with identical latency (43 ms vs 34 ms, both dominated by the cache).
  Extra headroom buys detail linearly, not savings.
- Reference of acceptable ratio for a summary: the summary must stay at ~1/10 of the original, and the cost of
  summarizing is dominated by the input text (Prompt Engineering, p.128). Applied to the free `--summary`:
  target ~8 KB. The 2000-word cut (19,5 KB) is already above that; 500 words (4,6 KB) is below and
  is the safe point.
- Fixed cost per call dominates below ~40 ms: `ast-outline` (2 ms) and `trace-strip` (4 ms) are so
  cheap that the bottleneck becomes the number of calls, not the size of each one.

## 5. What is not worth optimizing

- **`trace-strip` on text that is not a stack trace.** It only discards lines that match 3 regexes
  (`node_modules|vendor/|site-packages|/usr/local/go|/lib/python`, `internal/process|runtime/asm_|runtime/proc.go`,
  `org.springframework|jakarta.|javax.`) and cuts at `-max 25` (`tracestrip.go:12-16`). In a common log the gain is zero.
- **`ast-outline` on a 40-line file.** The cost of deciding is already larger than the read; the gain grows with the
  size of the file.
- **`skeleton` on a file < 8 KiB**: returns everything (`skeleton.go:29-31`).
- **`repo-map --update` in a single-read session**: 413 ms and 2,9 MB to save one search.
- **Quantization / INT8 / distillation.** The book recommends dynamic INT8 as the first step (~4x less
  memory) and choosing the smallest tolerable precision, re-evaluating the metric on the same test set (NLP with
  Transformers, p.255-256). It does not apply here: the repo does not run local transformer inference — the cost is
  API token. Optimizing local compute does not reduce the bill. Deliberate divergence, not technical debt.
- **Optimizing `quality`, `gain`, `activity`**: they are metadata of a few KB. The gain is noise.

## 6. Generation and latency: what the repo controls

- **Stop sequences: not controlled.** The book requires passing `stop` with a logical terminator (`\n#`, `\nclass`,
  `\ndef`) and cancelling the stream (Prompt Engineering, p.162-163). `knownCLIs` only defines the binary, static args
  and model flag — without `stop` (`summarize.go:71-80`). Every generation of `summarize` runs until the model stops.
  Divergence marked.
- **Latency: single-shot measurement, not distribution.** The book requires TTFT and TPOT separated at p50/p90/p99 plus
  goodput (`total = TTFT + TPOT x output tokens`; the average misleads) (AI Engineering, p.433-435). The times
  of this page are **one sample** of wall clock; there is no percentile, there is no prefill/decode separation. Do not cite
  these numbers as an SLO.
- **Existing telemetry:** only the MCP of `repo-map` has a metric channel (`--telemetry-file` or
  `AGENT_SYNC_GRAPH_TELEMETRY=1`, per `repo-map --help`). There is no token telemetry per session.
- **Savings tracking:** `ctx-window gain <session>` reads `gain.jsonl` with `before_chars/after_chars/
  saved_chars` and `saved_tokens = chars/4` (`gain.go:15-24`). Since compaction does not change `s.Turns`
  (`store.go:301-315`), the `saved_chars` of a compaction is 0 — do not use `gain` to prove compaction savings.

## 7. Actionable rules

- Always pass `--max-tokens` in `repo-map --summary`; 500 is the measured point of best gain/headroom ratio.
- Prefer `--focus` over `--brief` when you want short text for the model: 2.242 B against 3.685 B in the same file.
- Use `ast-outline` as a localization step and then read the exact range; do not treat the skeleton as a
  substitute for the file outside Go.
- Do not run `repo-map --update` for one read; run it when there are 3+ structural searches in the session.
- Measure before optimizing: `reduction_pct` is in the `skeleton` JSON — decide by the number, not by the feeling.
- Do not present the wall clock of one sample as a performance guarantee; for that the percentiles are missing.

## 8. Quick verification

```bash
repo-map --summary | wc -c                                  # 4,3k (default 1000 tokens)
repo-map --summary --max-tokens 500 | wc -c                 # 2,2k
repo-map --focus tools/cmd/ctx-window/store.go | wc -c      # 2,2k
ast-outline tools/cmd/ctx-window/store.go | wc -lc          # 1,2k / 39 lines
ctx-window skeleton --json <file> | grep reduction_pct      # 0 below 8 KiB
ctx-window gain <session>                                   # what compaction accounted for
```

## Sources

- Prompt Engineering for LLMs (Berryman & Ziegler) — Ch. 2 "Counting Tokens" p.42; Ch. 5 "Summarization"
  p.128 (summary ratio and cost dominated by the input); Ch. 6 "More on Inertness" p.148 (chars per token,
  non-additive count); Ch. 7 "Recognizable Start and End" p.162-163 (stop sequences).
- AI Engineering (Chip Huyen) — Ch. 9 "Inference Performance Metrics" p.433-435 (TTFT/TPOT, percentiles,
  goodput) and "Inference Service Optimization" p.460, 463-464 (prompt caching); Ch. 10 "Step 4 Caches"
  p.480-482 (what not to cache).
- NLP with Transformers (Tunstall et al.) — Ch. 8 "Making Models Faster with Quantization" p.255-256
  (INT8 ~4x, smallest tolerable precision, re-evaluate on the same test set).
