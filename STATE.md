# STATE — agent-sync

> Fonte de verdade para retomar o trabalho entre sessões/compactions.

## Tarefa atual — delegate-run watch (passo 4)

- `delegate-run watch <id>`: poll até `done` (exit 0) / `failed` (1) / timeout (3).
- `--fail-on-stall` → exit 2 se log idle ≥ stall threshold.
- `--interval` / `--timeout` (ou env `AGENT_SYNC_DELEGATE_WATCH_*`).

## Plano de delegação (completo)

1. Skill print/session + matriz de permissão
2. `delegate-run` (tmux + log + manifesto)
3. Contrato `DELEGATE_RESULT` + `result`
4. `watch` / stalled poll

## Próximos passos

1. Usar na prática (OpenCode print / agy session).
2. Não commitiar sem pedido explícito.
3. Fora de escopo ainda: embedding no memory-mcp; heurística automática de modelo.
