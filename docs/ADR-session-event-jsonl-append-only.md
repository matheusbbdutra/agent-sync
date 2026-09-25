# ADR: `session-event.jsonl` append-only como histórico da sessão

**Status**: **Aceito** (smoke em sessão planejada concluído em 2026-09-19 — ver `docs/smoke-evidence/adr-003-sess-1.md`, critérios D-23 todos pass)
**Data**: 2026-09-19
**Decisor**: Matheus Dutra
**Tags**: state, observability, jsonl, append-only, deepseek-inspired, typesafe-principles

## Contexto

`session-state.json` (ADR-002) é **snapshot do estado atual**: ele diz "qual a próxima ação, qual decisão está tomada, qual blocker está aberto". Mas tem um limite fundamental: **não preserva história**.

Quando o contexto estoura e o LLM reescreve o estado na compactação, o que existia entre turnos se perde. Hoje isso significa:

1. **Compactação destrói histórico.** O `session-state.json` mantém só o último snapshot; tudo entre o último e o anterior evapora. Decisões intermediárias, ações concluídas, dúvidas que foram respondidas — tudo perdido.
2. **`agent-delegate` não tem insumo para calibrar.** O skill decide qual CLI/modelo usar por heurística textual. Sem histórico de "nas últimas 10 decisões, Claude consumiu X tokens", a calibragem é cega.
3. **Debugging é replay manual.** "Quando essa decisão foi tomada?" exige grep no transcript do LLM, que é grande, ruidoso, e some na compactação.
4. **Auditoria forense é fraca.** Para incidentes (decisão ruim que apareceu depois), não há log determinístico de "quem/quando/por quê".

O DeepSeek Harness documenta (em <https://deepseek-code.com/>, lido em 2026-09-19) que seu "traceable session state" usa **append-only event logs** como abstração primária. É o oposto complementar do nosso snapshot: snapshot = estado atual derivado; event log = histórico bruto do qual snapshot pode ser reconstruído.

A typesafe.ai manifesto reforça o ponto: o estado tem que ser primitiva consumível por outros componentes, não prosa reinterpretada. Hoje `session-state.json` cumpre isso para "agora"; precisamos cumprir para "antes".

## Decisão

Estabelecer `session-event.jsonl` como **log append-only** ao lado do `session-state.json`. Cinco decisões de shape:

### Decisão 1 — Localização e formato

```
.agent-sync/
├── session-state.json     # snapshot (ADR-002)
└── session-event.jsonl    # histórico append-only (NOVA)
```

- **JSONL**, uma linha JSON por evento.
- **Append-only**: nenhuma mutação de linhas passadas. Apenas `O_APPEND` writes.
- Mesmo diretório do snapshot para evitar fragmentação.

### Decisão 2 — Schema mínimo do evento

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://agent-sync.local/schemas/session-event.json",
  "title": "SessionEvent",
  "version": "1.0",
  "type": "object",
  "additionalProperties": false,
  "required": ["schema_version", "ts", "kind", "ref", "title"],
  "properties": {
    "schema_version": { "const": "1.0" },
    "ts": { "type": "string", "format": "date-time" },
    "kind": { "enum": ["decision", "action", "blocker", "open_question", "state_render"] },
    "ref": { "type": "string", "pattern": "^(D|A|B|Q|S)-[0-9]+$" },
    "title": { "type": "string", "minLength": 1 },
    "actor": { "enum": ["user", "claude", "codex", "opencode", "cursor", "agy", "tool", "agent-sync"] },
    "session_id": { "type": "string" },
    "details": { "type": "object", "additionalProperties": true }
  }
}
```

Notas:

- **`ref` tipado**: `D-N` para decisão, `A-N` para action, `B-N` para blocker, `Q-N` para open_question, `S-N` para state_render. Mesmo namespace do snapshot (cross-ref válido).
- **`actor` enum fechado por CLI do agente-sync**: mais um CLI exige bump MAJOR + migration. Aceitável: é um enum pequeno e estável.
- **`details` é `additionalProperties: true`**: única exceção ao princípio da ADR-001. Justificativa: cada kind pode carregar payload específico sem versionar schema a cada evolução. Consumers devem tolerar campos desconhecidos (forward-compatible).
- **`session_id` opcional**: presente quando evento vem de hook/sessão; ausente para eventos gerados por `agent-sync state write` manual.

### Decisão 3 — Quem escreve, quando

| Origem | Quando | Como |
|---|---|---|
| `agent-sync state write` | Após cada write bem-sucedido do snapshot | Hook no `WriteSessionState` Go: emite evento `state_render` + diff contra estado anterior, emitindo 1 evento por mudança detectada (`decision`/`action`/`blocker`/`open_question`) |
| `SessionStart` hook (5 CLIs) | No início de cada sessão | Emite `state_render` se log estiver vazio para esse `session_id` |
| User manual | Via subcommand `agent-sync state log <event>` | Para correções/adições manuais |
| `agent-delegate` skill (futuro) | Quando delega task | Emite evento `action` com kind=`delegated`, details=`{cli, model, tokens}` |

### Decisão 4 — Append-only com rotação

- **Append-only** enforçado no código: `os.OpenFile(path, O_APPEND|O_WRONLY|O_CREATE, 0644)` + write. Sem truncate, sem `Seek(0)`.
- **Rotação por tamanho**: quando arquivo atinge 10MB, rotaciona para `session-event.jsonl.1` (mantém 1 rotação). 10MB escolhido porque cabe ~50k eventos em payload típico — cobertura de ~1 mês de uso intenso.
- **Rotação por tempo**: alternativa futura (1 mês calendário). Não nesta ADR.
- **Sem compactação**: log é append-only plain text; `gzip` opcional em hook de pós-sessão, ADR separada.

### Decisão 5 — Read API

Novos subcommands:

| Subcommand | O que faz |
|---|---|
| `agent-sync event read [--last N] [--kind K] [--since TS]` | Imprime eventos (JSONL por padrão, texto opcional) |
| `agent-sync event tail` | `tail -f` equivalente para stream live |
| `agent-sync event stats` | Conta eventos por kind/actor em janela (insumo para `agent-delegate`) |

Implementação: scan linear do JSONL. Para logs <10MB é instantâneo. Se virar problema, índice binário ADR separada.

## Consequências

### Positivas

- **Histórico preserva entre compactações.** Log fica em disco; LLM pode re-ler via `event read --last N` quando precisa de contexto.
- **`agent-delegate` ganha insumo real.** Pode ler `event stats --actor claude --last 100` para ver custo histórico, então calibrar modelo por classe de tarefa.
- **Debugging vira replay determinístico.** `event read --kind decision --ref D-1` retorna o snapshot exato do momento da decisão.
- **Auditoria forense viável.** Para investigar decisão ruim: `event read --since <deploy>` mostra tudo que aconteceu desde o deploy suspeito.
- **Tiposafe-style: histórico como primitiva.** Outros tools (dashboards, alertas, eval sets) consomem o JSONL sem reinterpretar transcript do LLM.
- **Insumo futuro para eval harness.** Eventos viram regression cases automaticamente.

### Negativas / trade-offs

- **Crescimento ilimitado sem rotação.** Mitigado pela Decisão 4 (10MB + 1 rotação). Trade-off explícito: prefere simplicidade sobre retenção longa.
- **Append-only + enum `actor`**: novo CLI exige MAJOR bump. Aceitável: enum pequeno, mudança rara.
- **Race em writes concorrentes.** Mesmo tmpfile único por PID+nanoTimestamp da ADR-002 aplica aqui. Sem lockfile necessário.
- **`details: additionalProperties: true` quebra princípio fechado.** Trade-off explícito: ganha flexibilidade para evolução por kind; mitiga via convention "consumers devem tolerar campos desconhecidos".
- **Sem replay de transcript do LLM.** Event log captura decisões/ações, não toda a prosa entre elas. Para replay completo, transcript JSONL do harness continua sendo source of truth (sob outra ADR).

## Decisões revisadas

(nenhuma — ADR em estado Proposto na primeira iteração.)

## Evidência / Implementação

Esta ADR é Proposta — sem código ainda. Quando aceita:

| Arquivo | Mudança |
|---|---|
| `schemas/session-event.json` | Schema do evento (1.0) |
| `cmd/agent-sync/session_event.go` | Struct `SessionEvent` + `Append()` / `Read()` / `Stats()` |
| `cmd/agent-sync/session_event_test.go` | Append idempotente + read filtrado + rotação |
| `cmd/agent-sync/session_state.go` | Hook em `WriteSessionState` para emitir diff como eventos |
| `cmd/agent-sync/main.go` | Subcommand `event read/tail/stats` wirado |
| `skills/agent-delegate/SKILL.md` | Adicionar linha: "consultar `agent-sync event stats --actor X --last N` antes de delegar" |
| `docs/SCHEMA-CHANGELOG.md` | Entrada inicial para `session-event` |

### Critério de aceite (Proposto → Aceito)

- Schema publicado e validado via `tools/internal/jsonschema/` (ADR-001).
- `Append` funciona em worktree de teste com 1000 eventos gerados; rotação dispara em 10MB.
- `Read` filtra corretamente por `--kind`, `--since`, `--last N` (todos com testes).
- Hook em `WriteSessionState` emite diff correto: snapshot antigo vs novo → lista de eventos derivados.
- Smoke real em **sessão planejada** com ≥2 cenários distintos, ≥20 eventos gerados, mix de kinds (não só `state_render`), simulação explícita de compactação, e verificação de que `event read` permite re-orientação após perda de contexto. Protocolo em `docs/SMOKE-TEST-Q.md`; pacote de cenários em `docs/smoke-evidence/PACOTE-CENARIOS.md`.
- `agent-delegate` atualizado para consumir `event stats` (mínimo: 1 leitura em smoke test).

## Limites conhecidos

1. **Não substitui o transcript do LLM.** Event log captura decisões/ações estruturadas; prosa livre entre elas continua no transcript JSONL do harness (sob controle de cada CLI).
2. **Rotação descarta.** Quando rotaciona, o arquivo `.1` substitui o anterior. Sem histórico arqueológico. Mitigação futura: ADR de retenção com `.1`, `.2`, `.3` etc.
3. **`actor` enum é fechado.** Novo CLI no agent-sync exige MAJOR bump do schema. Aceitável: agente-sync adiciona CLI raramente (último foi Antigravity, meses atrás).
4. **Sem proteção contra corrupção parcial.** Write parcial mid-event (e.g., disco cheio) deixa linha inválida. Mitigação: validação na leitura (pula linhas que falham `json.Valid` + log warning).
5. **Schema `details` é `additionalProperties: true`.** Quebra o princípio "fechado por padrão" da ADR-001. Justificativa registrada na Decisão 2; revisar se aparecer abuso.

## Próximos passos

1. **Aceitar a ADR** (revisão do usuário).
2. **Schema + validator** (pré-requisito: ADR-001 implementada com `tools/internal/jsonschema/`).
3. **PoC `session_event.go`** com `Append`/`Read` + round-trip test.
4. **Hook em `WriteSessionState`** para emitir diff.
5. **Subcommands** em `main.go`.
6. **Smoke em sessão planejada** (ver `SMOKE-TEST-Q.md` — ≥2 cenários, ≥20 eventos, compactação simulada, recuperação validada).
7. **Mover ADR para Aceito** após smoke.

## Referências

- **DeepSeek Harness** (<https://deepseek-code.com/>, lido em 2026-09-19): "Traceable session state — Stores sessions as append-only event logs so the context shown to a model can be traced, resumed, forked, searched, and replayed." Esta ADR reaproveita o conceito sem adicionar DSH como dependência ou CLI.
- **ADR relacionada**: `docs/ADR-session-state-json-canonico.md` — snapshot canônico; este ADR estende com histórico append-only. Direção inversa: snapshot deriva de log, não log deriva de snapshot (na prática ambos coexistem; rebuild de snapshot a partir do log é ADR futura).
- **ADR relacionada**: `docs/ADR-schema-output-versionado.md` — mesma filosofia (contrato fechado + versionado); este ADR é exceção parcial via `details: additionalProperties: true`, justificada na Decisão 2.
- **Tipificação segura**: typesafe.ai manifesto (lido em 2026-09-19) — "make intelligence composable": histórico como primitiva consumível por `agent-delegate`, dashboards e debug tools sem reinterpretar transcript do LLM.
- **Sessão 2026-09-19**: decisão de "explorar o conjunto de ferramentas do DeepSeek Harness reaproveitáveis com bons ganhos" (registrada no STATE.md desta sessão). Das 7 ferramentas DSH analisadas, apenas event log foi selecionada para adoção.