# context-engineering

A record of what `ctx-window` **measures and decides today** in agent-sync, with the exact point where the
behavior diverges from the book's recommendation. It is not a design proposal: every number below comes from
`--help`, from a repo `file:line` or from a command that was run (command cited alongside).

It complements `context-window-strategy.md`, which describes the protocol (K, summarizer, summary format).
Here the subject is the meter, the thresholds and the gaps.

## 1. The measured unit is a character, not a token

- `Session.EstimatedChars()` sums `len(Content) + len(Role) + 32` per turn (`tools/cmd/ctx-window/store.go:90-96`).
  There is no tokenizer in the path: no threshold in the binary knows the model's vocabulary.
- The compaction threshold is **fixed at 200 chars** (`compactAtThreshold`, `store.go:172-181`). There used to be
  the knob `AGENT_SYNC_CTX_COMPACT_AT`, removed on 2026-11-16 together with the `on-tool-call` subsystem: it did not
  reach the live flow and only served as a test seam.
- Practical effect: 200 chars ~ 50 tokens, the order of magnitude of a single tool call. Measured on the local
  sessions: `~/.cache/agent-sync/ctx-window` has 321 sessions, 273 with `turns.jsonl`, and **264 of them (96,7%)
  exceed 200 bytes** (`find . -name turns.jsonl -size +200c`). The default does not separate a healthy session from
  a full session.
- Consequence in the score: `ctx-window quality 007c7baa-...` returned
  `0 (w=25%) 5924/200 chars (over threshold)` and `grade: F`. `context_fill` zeroes the signal when
  `fill >= 2*threshold` (`quality.go:114-132`).
- The only point in the repo that actually reads tokens is the Claude nudge, which reads
  `message.usage.input_tokens|cache_creation_input_tokens|cache_read_input_tokens` from the transcript
  (`claude_usage.go:13-24`), with a 2 MiB tail (`transcriptTailBytes = 2 << 20`).

## 2. Real thresholds and environment variables

| Decision | Variable / constant | Default | Where |
| --- | --- | --- | --- |
| Size of the working memory | `AGENT_SYNC_CTX_K` | 5 turns | `store.go:423-431` |
| Summary budget | `AGENT_SYNC_CTX_BUDGET` | 1000 tokens | `store.go:434-442` |
| Claude nudge (once per session) | `AGENT_SYNC_CTX_NUDGE_TOKENS` | 150000 tokens | `claude_usage.go:14` |
| Antigravity nudge (preinvocation) | `AGENT_SYNC_CTX_NUDGE_INVOCATIONS` | 30 invocations | `hook.go:315-321` |
| Post-tool nudge (`ctx-window-nudge.sh`) | `..._MIN_TOOL_CALLS` / `..._MAX_AGE_HOURS` | 80 / 4h | `hooks/ctx-window-nudge.sh:22-23` |
| Mirror cadence to `memory-mcp` | `..._MIRROR_THRESHOLD` | 25 | `hooks/ctx-window-nudge.sh:75` |
| Guard nudge (injects `additionalContext`) | `AGENT_SYNC_NUDGE_THRESHOLD` | 120 (modulo) | `hooks/context-guard-nudge.pretooluse.sh:13,40` |
| Auto-summarize on Stop | `..._SUMMARIZE_MIN_TOOL_CALLS` / `..._MAX_AGE_HOURS` | 100 / 4h | `hooks/ctx-window-summarize-at-stop.sh:23-24` |
| Tool result archiving | `AGENT_SYNC_CTX_ARCHIVE_AT` | 4096 bytes | `archive.go:31,36-45` |
| Automatic archiving (opt-in) | `AGENT_SYNC_CTX_AUTO_ARCHIVE` | off (requires `1`) | `archive.go:210-214` |
| Loop detection | `AGENT_SYNC_CTX_LOOP_WINDOW` / `..._LOOP_MIN` | 10 / 3 | `store.go:107-170` |
| Disabling the ctx nudge | `AGENT_SYNC_CTX_WINDOW_NUDGE_DISABLE` | — | `hooks/ctx-window-nudge.sh:17-20` |

Readings that `--help` misleads about:

- `AGENT_SYNC_CTX_BUDGET` **truncates nothing**. `s.Budget` is only interpolated in the prompt text
  (`summarize.go:170`, `buildSummarizePrompt`); there is no cut by budget in the LLM output.
- The `ctx-window-nudge.sh` nudge never injects context: it emits `{}` on both paths (non-trigger and trigger),
  due to an Antigravity schema limitation (`hooks/ctx-window-nudge.sh:96-105`). The only point that injects is
  `context-guard-nudge.pretooluse.sh`, via `additionalContext` (and via stderr on Antigravity).

## 3. What each subcommand decides

- `on-tool-call-llm` / `hook <cli>`: records the turn with input truncated at 16000 chars after `redactSecrets`
  (`summarize.go:336-346`) and **never compacts** — the comment in `hook.go:60-63` says this explicitly.
- `compact`: forces the local heuristic without an LLM (`main.go:148-176`). `AppendVersionedSummary` writes
  `summary.md` + `summary_v{N}.md` and increments `Version`, **without touching `s.Turns`** (`store.go:301-315`).
  Therefore `beforeChars == afterChars` and the gain entry comes out with `saved_chars = 0` (`main.go:162-176`).
- `quality`: 7 signals with weights 25/15/15/15/10/10/10 (`quality.go:36-42`), grades `S>=90, A>=80, B>=70, C>=55,
  D>=40, otherwise F` (`quality.go:80-93`). `compaction_depth` drops 20 points per version (`quality.go:134-148`):
  a session with 5+ compactions does not recover S even if the content is clean.
- `activity`: classifies the dominant mode from the last 10 tool calls (`activity.go:24`), weights by substring of
  the tool name (`activity.go:26-40`).
- `skeleton`: skeleton of `.py/.ts/.js/.go` cached by fingerprint (mtime+size+hash), but only from 8 KiB upwards —
  below that it returns the whole file with `reduction_pct = 0` (`skeleton.go:29-31,44-54`).
- `archive`/`expand`: archives above 4096 bytes keeping only a 500 chars preview (`archive.go:27-32`).

## 4. State layers and what each one preserves

The binary keeps four independent layers. None reconciles the others: there is no single source of truth between
them.

- **Working memory (turns).** Each tool's input is redacted by `redactSecrets` **before** the cut at 16000 chars,
  so as not to leave a secret split in half (`summarize.go:336-346`). Loop detection reduces each turn to 512 chars
  with whitespace collapsed (`store.go:140-150`): a difference of newline does not hide a real loop, but content
  beyond 512 chars does not take part in the comparison.
- **Heuristic summary.** 6 categories by bilingual regex over the available turns (`heuristic.go:24-33`,
  `HeuristicExtract`). It never fails and never calls an LLM; in exchange, it only preserves a sentence that matches
  a marker.
- **Summary by LLM (`summarize`).** Assembles the prompt with the redacted history (`formatHistoryForPrompt`,
  `summarize.go:365-372`) and interpolates `{BUDGET_TOKENS}` without truncating the answer. In `auto` mode, it uses
  the heuristic while `turns <= 20` **and** `EstimatedChars <= 24000`; above that it goes to the agent
  (`summarize.go:194-218`; ~6k tokens according to the code's own comment).
- **Archive.** A result above 4096 bytes becomes `archive/<id>.txt` with a 500 chars preview in the turn
  (`archive.go:27-32`), retrievable via `expand --list|--search <q>|<id>`.
- **`memory-mcp`.** Mirror of the summary only with `AGENT_SYNC_CTX_REMEMBER=1` — default off because the table is
  shared between machines and synchronized (`summarize.go:249-257`).

Derived rule: when reanchoring, say which layer you are reading from. `ctx-window show` (turns + summary) and
`expand` (archived original) can disagree, and the regex summary has no way to recover what the working memory has
already lost in the 16000 chars cut.

## 5. Real hook routing (3 verified gaps)

1. **Auto-compaction by threshold no longer exists (resolved).** The Go hook and the OpenCode plugin call
   `on-tool-call-llm` (`hook.go:132`, `hooks/ctx-compact.v2.ts:136`), which never compacted. The `on-tool-call`
   subsystem — the only one that compared against the threshold — and the knob `AGENT_SYNC_CTX_COMPACT_AT` were
   removed on 2026-11-16. Compaction now only happens by explicit `compact` or `summarize`.
2. **`ctx-window-summarize-at-stop.sh` never summarizes.** Line 86 calls
   `"$BIN" summarize -root "$root"`, but `summarize` only accepts `--cli`, `--model`, `--timeout`
   (`summarize.go:108-110`). Ran: `ctx-window summarize -root /tmp/x` prints
   `flag provided but not defined: -root` and exits with `exit=1`. Since the call is inside an `if`, the Stop summary
   fails silently.
3. **Compaction is invisible in the metric.** `quality` reports `7 compactions (deep)` and `gain` for the same
   session answers `(no compaction events recorded yet)`. There is no way to audit what the compaction discarded.

## 6. Where the behavior diverges from the book's recommendation

| Recommendation (book) | Current behavior | Divergence |
| --- | --- | --- |
| Count tokens with the model's tokenizer; ~4 chars/token in English and ~2 in code (Prompt Engineering, p.42, p.148) | Everything in chars; `--max-tokens` of `repo-map --summary` counts **words** (`len(strings.Fields(line))`, `tools/internal/repomap/repomap.go:1148-1156`) and `--brief` uses `len(data)/4` (`repomap.go:789-791`) | The same flag has two different units in the same binary; neither is a token |
| The summary should be ~1/10 of the original and the cost is dominated by the input text (Prompt Engineering, p.128) | Local heuristic by regex extracts 6 categories (`heuristic.go:24-33`), with no target ratio and no measurement of the relative size; `BUDGET` exists but does not cut | There is no compression control: the summary can end up the size of the original |
| Nothing critical in the middle; instruction and format at the ends (Prompt Engineering, p.135-136; AI Engineering, p.238-239) | No mechanism positions content: the ctx hook emits `{}` and the summary is written to disk, outside the prompt | No position control — recency depends on the CLI, not on the binary |
| Sliding window with overlap (`doc_stride`) so as not to lose the answer at the end (NLP with Transformers, p.199) | `s.Turns` grows and is never truncated; `K` only affects the display (`store.go:90-96`, `store.go:301-315`) | There is no storage sliding window nor overlap: the "context" accumulates and only the summary represents the past |
| Curation per turn: which tools, which artifacts, how far back to go (Prompt Engineering, p.196-199) | Fixed decision: K turns, archive opt-in by size | There is no per-turn decision; only two static defaults |
| Attention cost ~O(n^2); structure the input instead of stacking it (NLP with Transformers, p.369) | The entry threshold is chars of recorded turn, with no model of the provider window | No threshold expresses the model's real window; there is no signal for "how much of the provider budget I have already spent" |
| The compaction threshold must be calibrated by measurement, not by guesswork (AI Engineering, p.238: measure with NIAH on the private data) | Default 200 chars, without calibration; 96,7% of the local sessions exceed it | An uncalibrated default produces `grade: F` as the normal state |

Where the binary **is aligned** with the book it is worth recording, so as not to "fix" what works:

- `--brief` truncation is explicit: it cuts calls/importers/symbols in blocks of 8/8/10 and marks the rest with
  `"... (N mais)"` [`"... (N more)"`] (`repomap.go:789-813`) — it respects the warning that truncating without warning
  loses information (NLP with Transformers, p.51).

## 7. Actionable rules

- Treat `EstimatedChars` as a rough proxy: to decide "does it fit?", measure tokens in the CLI before comparing with
  the model's window.
- There is no auto-compaction by threshold: the threshold is fixed (200 chars) and only `quality` uses it to score.
  Compact with `ctx-window compact <session>` or `summarize` when you decide to.
- Do not count on the Stop auto-summarize as a safety net: the call passes a nonexistent flag and fails.
- Do not read `quality` as context health: `compaction_depth` is monotonically punitive and `context_fill` is
  saturated by the 200 chars default.
- Write down the essentials before compacting: the heuristic summary preserves nothing that does not match the 6
  regexes of `heuristic.go`.
- For `repo-map --max-tokens`, assume words in `--summary` and chars/4 in `--brief`; if you need a token, measure
  after generating.

## 8. Quick verification

```bash
ctx-window doctor                          # summarizer, K, budget, ollama
ctx-window quality <session>               # 7 signals + grade
ctx-window gain <session>                  # what the compaction claimed to have saved
ctx-window activity --json <session>       # dominant mode in the last 10 tool calls
ctx-window summarize -root .               # must fail: summarize does not accept -root
```

## Sources

- Prompt Engineering for LLMs (Berryman & Ziegler) — Ch. 2 "Counting Tokens" p.42; Ch. 5 "Prompt Content"
  p.116-121, "Summarization" p.127-130; Ch. 6 "Assembling the Prompt" p.135-136 (position), p.148-151
  (inertness/counting); Ch. 8 "Context for Task-Based Interactions" p.196-199.
- AI Engineering (Chip Huyen) — Ch. 5 "Context Length and Context Efficiency" p.238-239 (context order,
  NIAH on the private data).
- NLP with Transformers (Tunstall et al.) — Ch. 2 "From Text to Tokens" p.51; Ch. 7 "Question Answering"
  p.199 (sliding window with overlap); Ch. 11 "Sparse Attention" p.369 (quadratic cost).
