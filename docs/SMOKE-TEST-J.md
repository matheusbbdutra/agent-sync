# Smoke Test J — Resultados

Validação manual + automatizada em `/tmp/agent-sync-smoke/<cli>/` em 2026-09-19.

## TL;DR

**5/5 CLIs passaram** com pytest 3/3. Zero falha de hook em `~/.cache/agent-sync/hooks/errors.jsonl`. **Gap D validado em produção real** (receipts gerados por codex).

## Por CLI

### claude (`-p --dangerously-skip-permissions`)
- Arquivos: `email_validator.py`, `test_email_validator.py`, `.venv/`
- Pytest: 3/3 PASS
- Extras: precisou criar venv (sem pytest no Python do sistema)
- Tokens: 564k input (sessão interativa desta conversa)

### codex (`exec --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check --add-dir ... --ephemeral`)
- Arquivos: `email_validator.py`, `test_email_validator.py`
- Pytest: 3/3 PASS (via `uv run --with pytest`)
- **Gap D em produção**: `receipts/receipts.jsonl` (7 entradas) + `review-receipts/receipts.jsonl` (7 entradas)
- Tokens: 19.971

### cursor-agent (`-p --force --trust`)
- Arquivos: `email_validator.py`, `test_email_validator.py`
- Pytest: 3/3 PASS

### agy (`--print="..." --dangerously-skip-permissions`)
- Rodado manualmente pelo usuário
- Arquivos: `email_validator.py`, `test_email_validator.py`, `__pycache__/`, `.pytest_cache/`
- Pytest: 3/3 PASS

### opencode (`run --pure`)
- Arquivos: `email_validator.py`, `test_email_validator.py`, `__pycache__/`, `.pytest_cache/`
- Pytest: 3/3 PASS (via `uv run --with pytest`)
- Sessão registrada no SQLite: `ses_f46013d0cffeYjG4FjfMWng2aD`
- Tokens: 51.356 input / 1.172 output
- **Correção aplicada**: `~/.config/opencode/opencode.json` tinha `"model": "minimax/MiniMax-M3"` (provider errado); corrigido para `"minimax-coding-plan/MiniMax-M3"` (provider+model corretos do `auth.json`). Backup em `opencode.json.bak.pre-smoke-1789826825`.

## Hooks observados

- `~/.cache/agent-sync/hooks/errors.jsonl`: 0 eventos novos durante smoke (1 evento antigo de `set 19 13:01` não relacionado)
- 5 plugins OpenCode carregados: `ctx-compact.ts`, `agent-react-nudge.ts`, `memory-nudge.ts`, `docs-cache.ts`, `context-guard-nudge.ts`
- 2 plugins Codex adaptados via Gap D: `protect-mcp`, `review-agent-governance` (apontam para `codex-protect-mcp-adapter.sh` em vez de `npx protect-mcp`)

## Pendência L (handoff Antigravity via PreInvocation)

- **Fundida em J**: código já está correto e coerente com a doc oficial
- Não é item de trabalho separado — linha do smoke manual futuro

## Pendência K (nudge de tokens por CLI)

- **Codex**: ✓ confirmado (rollouts JSONL)
- **OpenCode**: ✓ confirmado (SQLite `~/.local/share/opencode/opencode.db`)
- **Cursor + Antigravity**: 🔭 tracking-only em [#2](https://github.com/matheusbbdutra/agent-sync/issues/2)

## Estado consolidado

Pendência J pode ser marcada como **fechada 2026-09-19** — 5/5 CLIs validadas, gap D validado em produção, wrap-hook.sh estável em todas as 5.
