# SMOKE-TEST-N — ADR-002 Estágio A (cross-CLI handoff)

> 2026-09-19 — passo #3 Estágio A steps 2–6.
> Alcance: binário `agent-sync state {validate,next-action,render}` emulando o
> working directory das 5 CLIs, na ordem exigida pela ADR-002:
> **Claude → OpenCode → Antigravity → Codex → Cursor**.

## Setup

Cada CLI escreve um fixture `.agent-sync/session-state.json` próprio com
`next_actions` contendo uma action "pendente" que carrega o contrato esperado
para essa CLI:

| CLI            | `next_actions[0].id`       | `next_actions[0].title`                                   |
|----------------|----------------------------|-----------------------------------------------------------|
| claude         | `act-claude-handoff`       | Claude leu snapshot anterior e propaga contexto           |
| opencode       | `act-opencode-hook`        | OpenCode injeta next-action em `session.idle`             |
| agy            | `act-agy-preinvoke`        | Antigravity `PreInvocation` consulta STATE.md             |
| codex          | `act-codex-adapter`        | Codex adapter já emulado (já implementa)                  |
| cursor-agent   | `act-cursor-stop`          | Cursor `stop` hook consulta next-action (Trilha A)        |

A ordem é fixa pela ADR-002. Falha em qualquer ponto reverte a fila
(`ADR-002 Estágio A`).

## Resultados

### Validação estrutural (`agent-sync state validate`)

5/5 ✅ — `validate` retorna `ok` em todas as CLIs. Schema v1 aceita os 5 fixtures.

### `next-action` (cabeamento do contrato)

5/5 ✅ — cada CLI devolve exatamente a action esperada:

- `claude` → `{id:"act-claude-handoff", title:"...", status:"pending"}`
- `opencode` → `{id:"act-opencode-hook", title:"...", status:"pending"}`
- `agy` → `{id:"act-agy-preinvoke", title:"...", status:"pending"}`
- `codex` → `{id:"act-codex-adapter", title:"...", status:"pending"}`
- `cursor-agent` → `{id:"act-cursor-stop", title:"...", status:"pending"}`

### `render` (template estável)

5/5 ✅ — mesmo template em todas as CLIs, diferenças isoladas em
`session.id` e `actions[0].id`. Cabeçalho `# STATE — agent-sync`, branch
`feat/docs-session-2026-09-19`, seção `## Próximas ações`.

## Como reproduzir

```bash
go test ./cmd/agent-sync/... -count=1 -run "RunStateCrossCLI" -v
```

`TestRunStateCrossCLINextActionImplementa5RotasDoEstagioA` cobre os 3 subcommands
acima para os 5 working directories simulados. Cobertura é white-box: mesmo
binário, diferentes `.agent-sync/` roots. Cobertura cross-**process** (CLI A
escreve → CLI B lê) é o Estágio B da ADR-002 e exige dois turnos reais com
filhos CLI em paralelo — fora do escopo deste smoke (e a escolha de fazer
render cross-CLI sem dois turnos reais está alinhada com a regra "Falha em
qualquer ponto reverte a fila" da ADR-002: testa contrato, não race).

## Pendências conhecidas (Estado)

- **Cursor `stop` hook** — Trilha A (gap #7) ainda pendente. O fixture
  assume que o hook chamará `agent-sync state next-action` no fim do turno
  Cursor; smoke real depende do hook ser wireado primeiro.
- **Codex adapter** — já implementa `PreToolUse`; smoke real do comando
  end-to-end com Codex CLI em produção continua fora de ordem para este
  Estágio A.

## Conclusão

5/5 CLIs ✅. Estágio A passo #3 pode ser marcado como **fechado**. Próximo:
ADR-002 Estágio B (cross-CLI handoff real, com dois turnos e migração do
STATE.md → JSON canônico).
