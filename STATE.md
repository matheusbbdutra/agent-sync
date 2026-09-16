# STATE — agent-sync

> Fonte de verdade para retomar o trabalho entre sessões/compactions.

## Meta atual

- Skill `agent-react` + hook `agent-react-nudge` + regra hipótese ≠ fato.
- Status: concluído (commit pendente de push se desejado)

## Decisões tomadas

- Nudge (não gate): threshold 15, `AGENT_SYNC_REACT_NUDGE_THRESHOLD`
- Garantia semântica plena exige camada extra (ex. `stop`/`afterAgentResponse` no Cursor) — não implementada ainda
- Hook global Cursor = `~/.cursor/hooks.json` (local); Cloud Agents só leem `.cursor/hooks.json` do repo

## Próximos passos

1. Push se desejado
2. Se barulho: subir threshold ou filtrar só tools de mutação
3. Avaliar hook `stop` no Cursor para reforço no fim do turno

## Bloqueios / perguntas abertas

- OpenCode: nudge best-effort até issue upstream #13574
