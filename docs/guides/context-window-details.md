# Context window strategy (detalhes)

Detalhes operacionais do `context-window-strategy`. Referência rápida
no README, conteúdo completo aqui.

## Skill + ferramenta

- **Skill**: `skills/context-window-strategy/SKILL.md`.
- **Tool**: `tools/cmd/ctx-window/` com subcommands `show`,
  `summarize`, `hook <cli>`, `set-k`, `doctor`, `benchmark`.

## Objetivo

Evitar o *lost in the middle* (modelos ignorando decisões iniciais em
context bloated) e eliminar desperdício de tokens com auto-compaction
frequente.

## Paper de referência

Baseado em *"Less Context, Better Agents: Efficient Context Engineering
for Long-Horizon Tool-Using LLM Agents"* (Lodha, Pahlavikhah Varnosfaderani,
Chakraborty, Mithal — 2026,
[arXiv:2606.10209v1](https://arxiv.org/html/2606.10209v1)).

**Resultados**: restringir a janela ativa aos últimos 5 tool calls +
sumário incremental (C4) atinge **91.6%** de task completion vs **71.0%**
do contexto integral (C2), com redução de **63.9%** em tokens.

## Como funciona

- **Continuous recording (zero LLM extra)**: hooks capturam argumentos
  + outputs truncados em cada tool execution. Index local dos últimos
  $K$ passos (default $K=5$).
- **On-demand summarization**: `ctx-window summarize` invoca modelo
  ativo para produzir resumo YAML 6-seções (*decisions,
  active_hypotheses, artifacts, resolved_errors, next_steps,
  constraints*) em `<projectRoot>/.agent-sync/summary.md`
  (auto-.gitignored).
- **Auto session handoff**: ao abrir nova sessão, sumário e memória
  recente são restaurados no contexto inicial via `SessionStart`
  (Claude/Codex) ou `sessionStart` (Cursor).

## Comandos úteis

```bash
ctx-window summarize                # salva em .agent-sync/summary.md
ctx-window show [session]           # inspeciona working memory
ctx-window set-k <session> <N>     # ajusta K (default 5)
ctx-window doctor                   # verifica config + cache
```

Ver [`docs/ADR-context-window-strategy.md`](../ADR-context-window-strategy.md)
para o design completo.
