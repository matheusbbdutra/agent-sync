# SMOKE-TEST-P — ADR-003 Implementação (event log append-only)

> 2026-09-19 — passo #5 do STATE. ADR-003 status: Proposto → **Implementado**
> (aguarda smoke em **sessão planejada** com ≥2 cenários distintos, ≥20 eventos
> e mix de kinds para mover para "Aceito" — ADR critério de aceite atualizado
> por D-23, ver `docs/SMOKE-TEST-Q.md`).

## Setup

Implementação completa da ADR-003 entregue em 6 commits na branch
`feat/docs-session-2026-09-19`:

| # | Commit | Escopo |
|---|---|---|
| 1 | `7d8341a` feat(event-schema): session-event v1.0 + lib jsonschema | Schema JSONL + 14 testes verdes |
| 2 | `55d6f4d` feat(session-event): Append/Read/Stats + rotação 10MB | Struct + Append atômico + Read filtrado + Stats + rotação |
| 3 | `0b9ec61` feat(state-write): hook emite diff em WriteSessionState | Integração: snapshot → eventos |
| 4 | `aaec826` feat(event-cli): subcommands read/tail/stats wirados | CLI surface |
| 5 | `e740893` feat(agent-delegate): consome event stats | Skill atualizada |

Pré-requisitos cumpridos (do critério de aceite):

- ✅ Schema publicado e validado via `tools/jsonschema/` (ADR-001)
- ✅ `Append` funciona em worktree de teste com rotação em 10MB
- ✅ `Read` filtra corretamente por `--kind`, `--since`, `--last N`
- ✅ Hook em `WriteSessionState` emite diff: snapshot antigo vs novo → lista de eventos derivados
- ⏳ Smoke em sessão planejada (≥2 cenários distintos, ≥20 eventos, mix de kinds, simulação de compactação) — **pendente**; protocolo em `docs/SMOKE-TEST-Q.md`, cenários em `docs/smoke-evidence/PACOTE-CENARIOS.md`
- ✅ `agent-delegate` atualizado para consumir `event stats`

## Resultados dos testes automatizados

### Schema (`tools/jsonschema/jsonschema_event_test.go`)

14 testes verdes:

- `TestLoadSessionEvent` — schema carrega via embed.FS
- `TestValidateSessionEventHappyPath` — payload válido passa
- `TestValidateSessionEventMissingRequired` × 4 subtests — `ts`/`kind`/`ref`/`title` faltando rejeitados
- `TestValidateSessionEventKindEnumInvalid` — `kind: "thought"` rejeitado
- `TestValidateSessionEventActorEnumInvalid` — `actor: "gpt-5"` rejeitado (enum fechado)
- `TestValidateSessionEventRefPatternInvalid` × 4 subtests — `X-1`/`0-1`/`decision-1`/`D` rejeitados
- `TestValidateSessionEventRefPatternValid` × 5 subtests — `D-1`/`A-42`/`B-100`/`Q-7`/`S-3` aceitos
- `TestValidateSessionEventDetailsAcceptsArbitraryKeys` — `details` aceita chaves arbitrárias (exceção ADR-001 registrada)
- `TestValidateSessionEventDetailsCanBeAbsent` — `details` é opcional
- `TestValidateSessionEventAdditionalPropertyRejected` — root com chave desconhecida rejeitado
- `TestValidateSessionEventActorCanBeAbsent` — `actor` é opcional
- `TestValidateSessionEventSessionIDCanBeAbsent` — `session_id` é opcional
- `TestValidateSessionEventSchemaVersionInvalid` — `schema_version: "v1"` rejeitado pelo pattern
- `TestValidateSessionEventRoundTripJSON` — round-trip via MarshalIndent + Unmarshal preserva validade

### Lib (`cmd/agent-sync/session_event_test.go`)

11 testes verdes:

- `TestAppendEventCreatesFile` — arquivo criado com 1 linha terminada em `\n`
- `TestAppendEventRejeitaSchemaInvalido` — kind fora do enum rejeitado, arquivo não criado
- `TestAppendEventMultiplosPreservaAppendOnly` — 5 eventos preservam ordem
- `TestReadEventsFiltraPorKind` — filtro `decision` retorna só decisions
- `TestReadEventsFiltraPorLast` — `--last 3` retorna os 3 mais recentes
- `TestReadEventsFiltraPorSince` — `--since <ts>` retorna eventos >= ts
- `TestReadEventsPulaLinhaInvalida` — linha corrompida no meio é pulada (Limite 4)
- `TestStatsEventsContaPorKindEActor` — total + by_kind + by_actor corretos
- `TestStatsEventsLogVazioRetornaZero` — log vazio retorna map vazio, não nil
- `TestAppendEventRotacionaEm10MB` — padding `SessionEventRotateBytes-200` força rotação; `.1` criado; log atual contém só o evento novo
- `TestReadEventsLogInexistenteRetornaVazio` — log ausente retorna slice vazio (não erro)

### Hook (`cmd/agent-sync/session_state_events_test.go`)

5 testes verdes:

- `TestWriteSessionStateEmiteStateRenderNaPrimeiraEscrita` — primeiro write emite 1 state_render
- `TestWriteSessionStateEmiteEventosParaItemsNovos` — segundo write com `D-99`/`A-99`/`B-99` emite eventos por item
- `TestWriteSessionStateNaoEmiteParaItemsInalterados` — segundo write idêntico NÃO emite per-item (só state_render)
- `TestWriteSessionStateDetectaMudancaDeStatusEmAction` — flip de status emite evento com `details.status: "done"`
- `TestWriteSessionStateFalhaEmEventNaoAbortaWrite` — falha de validate no snapshot NÃO cria arquivo parcial

### CLI (`cmd/agent-sync/event_cli_test.go`)

9 testes verdes:

- `TestRunEventReadListaTodosEventos` — read imprime JSONL parseável
- `TestRunEventReadComFiltros` — `kind` + `last` combinados
- `TestRunEventReadSinceInvalidoFalha` — since não-RFC3339 falha estruturado
- `TestRunEventStatsRetornaJSONEstruturado` — stats retorna JSON com by_kind correto
- `TestRunEventStatsLogVazioRetornaZeroValido` — log vazio retorna zero estruturado
- `TestRunEventCommandDespachaSubcommands` — read/stats/help despacham corretamente
- `TestRunEventCommandSubcommandDesconhecido` — erro estruturado para subcommand inválido
- `TestRunEventUsageListaSubcommands` — usage lista read/stats/tail/-last/-kind/-since
- `TestSplitLineBasico` — helper de tail
- `TestReadEventsFiltraPorKindEPeriodoCombinados` — kind + since combinados

## Como exercitar manualmente

```bash
# Setup
cd /path/do/projeto
mkdir -p .agent-sync

# Adicionar evento manual (subcommand 'event log' nao wirado nesta sessao;
# eventos vem do hook em WriteSessionState, ou via AppendEvent programatico)

# Estado A: dispara state_render + 3 events (decisions/action/blocker novos)
agent-sync state write -from <payload.json> -root .
agent-sync event read -root .

# Filtrar
agent-sync event read -kind decision -last 5 -root .
agent-sync event stats -actor agent-sync -root .
agent-sync event read -since 2026-09-19T18:00:00Z -root .
```

## Limitações registradas na ADR (aceitas, fora de escopo desta entrega)

1. Não substitui o transcript do LLM (prosa livre continua no harness de cada CLI)
2. Rotação descarta — `.1` substitui anterior; sem `.2`/`.3` (ADR separada futura)
3. `actor` enum fechado — novo CLI exige MAJOR bump do schema
4. Sem proteção contra corrupção parcial — `Read` pula linhas inválidas + log warning (mitigação aceita)
5. `details: additionalProperties: true` quebra princípio "fechado por padrão" ADR-001 (exceção registrada)

## Pendência: smoke em sessão planejada

Para mover ADR-003 de "Implementado" para "Aceito" falta o critério:

> Smoke real em sessão planejada com ≥2 cenários distintos, ≥20 eventos,
> mix de kinds (não só `state_render`), simulação explícita de compactação,
> e verificação de que `event read` permite re-orientação após perda
> de contexto.

**Por que sessão planejada** (em vez de ≥30min × 2 sessões reais)? Ver D-23:
cobertura sistemática de cenários é mais rigorosa que uso real improvisado;
permite forçar compactação mid-task, erros de tool, retries; dados
reproduzíveis. Tempo típico: 15-20min em vez de ≥60min.

**Protocolo completo:** `docs/SMOKE-TEST-Q.md`.
**Pacote de cenários para você rodar:** `docs/smoke-evidence/PACOTE-CENARIOS.md`.

**Como rodar** (resumo):

1. Crie um projeto temporário (`mkdir /tmp/smoke-adr-003 && cd $_`).
2. Inicialize agent-sync (`agent-sync state write` com payload mínimo).
3. Siga o pacote de cenários (≥2 deles).
4. Rode `agent-sync event read -last 50 -root .` para inspecionar.
5. Simule compactação (limpar contexto / nova sessão) e re-oriente via log.
6. Colete evidência via `scripts/collect-smoke-evidence.sh -session 1 -duration 15 -root . -compact yes -out docs/smoke-evidence/adr-003-sess-1.md`.
7. Edite os placeholders C1-C6 no YAML.

Quando o smoke passar:

1. Atualizar `docs/ADR-session-event-jsonl-append-only.md`: status → **Aceito**.
2. Atualizar `STATE.md` (via `agent-sync state write` + `state render`): `A-11` → done, `B-4` removido.
3. **Commit**:
   ```bash
   git add docs/ADR-session-event-jsonl-append-only.md STATE.md .agent-sync/session-state.json docs/smoke-evidence/
   git commit -m "docs(adr-003): aceita após smoke planejado (≥2 cenários, ≥20 eventos)"
   ```

## Conclusão

5/6 critérios de aceite verdes via testes automatizados. Pendente apenas
smoke planejado (gate de aceitação reduzido de 60min para 15-20min via D-23).
ADR-003 **Implementado** — pronto para uso imediato, "Aceito" após smoke.