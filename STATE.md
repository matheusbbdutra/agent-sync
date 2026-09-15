# STATE — agent-sync

> Fonte de verdade para retomar o trabalho entre sessões/compactions.

## Meta atual

- Hook `agent-react-nudge` (validação de hipóteses) nas 5 CLIs.
- Status: concluído

## Decisões tomadas

- Nudge (não gate): threshold 15, env `AGENT_SYNC_REACT_NUDGE_THRESHOLD`
- Mesmo padrão de context-guard/memory; OpenCode best-effort
- Hipótese ≠ fato já em global-rules + skill agent-react

## Estado do repositório

- Branch: `main`
- Mudanças não commitadas: agent-react skill, hooks, wiring Go, READMEs, STATE.md, global-rules

## Arquivos-chave

- `hooks/agent-react-nudge.{sh,cursor.sh,antigravity.sh,opencode.ts}`
- `cmd/agent-sync/hooks.go` / `cursor.go` / `main.go`
- `skills/agent-react/SKILL.md`

## Próximos passos

1. Commit quando o usuário pedir
2. Observar ruído do threshold 15 na prática

## Bloqueios / perguntas abertas

- Nenhum

## Contexto para reancorar

- Regras: PT-BR; hipótese ≠ fato; nudge ≠ garantia
- Verificação: `grep agent-react ~/.cursor/hooks.json` + `go test ./cmd/agent-sync/`
