---
name: token-saving-toolkit
description: "High-performance tools (repo-map, ast-outline, trace-strip) for drastic token reduction and structural navigation."
---

# Token-Saving Toolkit & Data Guardians

This skill instructs the agent to use the native high-performance utilities installed in `~/.local/bin` to filter unnecessary context and preserve tokens.

## Available Utilities

### 1. `repo-map`
Structural and relational map of the repository with deterministic incremental cache (<20ms):
- **When to use:** When starting tasks, investigating architecture or discovering dependencies without dumping whole files into the context.
- **Native multi-language support:** Go (`.go`), TypeScript/JavaScript (`.ts`, `.tsx`, `.js`, `.jsx`), PHP (`.php`) and Python (`.py`).
- **Main commands:**
  - `repo-map --focus <path/file>`: Textual subgraph around the file (declared symbols, imports and calls/importers).
  - `repo-map --summary`: Top structural hubs of the project (symbols most referenced in the repository). Limited to ~1000 tokens by default: when it cuts, the last line says how many hubs were left out. Use `--max-tokens N` to adjust or `--max-tokens 0` to see everything (1236 hubs ≈ 14k tokens in this repo — do not do that out of curiosity).
  - `repo-map --update`: Silently re-indexes only the modified files (delta).

### 2. `ast-outline`
Generates the file skeleton (classes, methods, interfaces, functions with line numbers) instead of reading the whole file.
- **When to use:** Always before reading files with more than 100 lines.
- **Command:** `ast-outline <file_path>`
- **Native support:** Go (via native AST), Python, TypeScript, JavaScript, PHP and generic fallback.
- **Savings:** Reduces from ~2.500 tokens to ~150 tokens per inspection.

### 2. `trace-strip`
Filters stack traces and extensive logs, hiding internal frames of frameworks, runtime and vendors (`node_modules`, `vendor/`, `site-packages`, Spring/Jakarta, etc.), keeping only the exact point of the failure in the application.
- **When to use:** When analyzing failures, test logs or exceptions in the terminal.
- **Command:** `trace-strip [file_or_pipe]`
- **Example:** `cat error.log | trace-strip` or `trace-strip error.log`

### 3. `db-guardian`
Guardian of database and API queries:
- Blocks destructive mutations (`INSERT`, `UPDATE`, `DELETE`, `DROP`, `ALTER`, etc.) unless expressly authorized.
- Adds defensive limits (`LIMIT 20`) automatically if the query has no pagination.
- Alerts against `SELECT *`.
### 4. `git-diff-summary`
Summarizes unified Git diffs, extracting modified, added or deleted files with status `[M]`/`[A]`/`[D]`, header of the changed function and count of added/removed lines (`+`/`-`), avoiding dumping giant raw diffs into the context.
- **When to use:** When inspecting changes before commits or in code reviews.
- **Command:** `git-diff-summary [file_or_pipe]` (or with no arguments to inspect the current `git diff`).
- **Example:** `git-diff-summary` or `git diff main | git-diff-summary`

## Recommended Resolution Flow with Low Token Consumption

1. **Orientation:** Use `repo-map --focus <file>` or `repo-map --summary` to understand the neighborhood and dependencies without reading multiple files.
2. **Mapping:** Use `ast-outline <file>` to discover on which lines the desired function is.
3. **Focus:** Use `view_file` specifying `StartLine` and `EndLine` in the identified excerpt.
4. **Debug:** Pass the error output through `trace-strip` to keep in the context only the root cause and the useful stack trace.
5. **Review:** Use `git-diff-summary` to validate the impact of the code changes without polluting the history.
