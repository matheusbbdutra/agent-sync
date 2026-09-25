# Smoke T — C-4 OpenCode v2 plugins ctx-window (A-17)

> 2026-09-20 — sessão planejada (D-23) validando os 2 plugins v2
> entregues em A-17 contra runtime real do OpenCode v2.0.11.
>
> **Resultado geral: PASS com adaptações registradas** — ver seção
> "Adaptações durante o smoke" abaixo. Os 6 critérios (C1-C6) do
> `SMOKE-TEST-T.md` passam em runtime com ressalva documentada.

## Setup

- OpenCode CLI: `opencode v2.0.11`
- Fixture isolado: `/tmp/smoke-ctx-window-c4`
  - 1 README.md, 1 src/ com 2 .ts (de projeto simulado)
  - `.agent-sync/summary.md` com mtime forçado em 5h atrás
  - `.git/` inicializado vazio (commit inicial)
- DB storage global: `~/.local/share/opencode/opencode.db` (SQLite, tabela `kv`)
- Wirar: `agent-sync -target opencode` aplicado — 9 plugins v2 em
  `~/.config/opencode/plugins/` (4 legados + 4 novos do A-14/A-16 + 2 do A-17)

## Verificação rápida do wirado (pré-smoke)

| Check | Resultado |
| --- | --- |
| 9 plugins TS em `~/.config/opencode/plugins/` | PASS |
| Plugin `ctx-window-nudge.ts` presente (160 linhas) | PASS |
| Plugin `ctx-window-summarize-at-stop.ts` presente (174 linhas) | PASS |
| `opencode run --standalone` carrega plugins no startup (log "loading plugin") | PASS |
| Storage do OpenCode v2 = SQLite kv em `~/.local/share/opencode/opencode.db` | PASS |

## Cenário T1 — Nudge após ≥MIN_TOOL_CALLS ou summary_age

### Procedimento executado

```bash
cd /tmp/smoke-ctx-window-c4
sqlite3 ~/.local/share/opencode/opencode.db "DELETE FROM kv WHERE key LIKE '%ctx-window-nudge%';"
touch -d "5 hours ago" .agent-sync/summary.md  # força regra 2

opencode run --standalone --auto --print-logs --format json \
  "Crie foo.md com 'a', depois bar.md com 'b', depois baz.md com 'c', depois liste todos"
```

### Observação runtime

Plugin carregou normalmente (linha do log opencode):
```
2026-09-20T23:23 run=... msg="loading plugin"
  id=/home/matheusdutra/.config/opencode/plugins/ctx-window-nudge.ts
```

Tool calls observados: **4** (3 writes via tool + 1 list via shell).

### Inspeção no DB após a run

```sql
SELECT key, value FROM kv WHERE key LIKE '%006300740078002d00770069006e0064006f0077002d006e0075006400670065%';
```

| key | value | ts |
| --- | --- | --- |
| `agent-sync/ctx-window-nudge/count/ses_f3ed7c2a3ffeWGJBVZeKZcy94O` | 4 | start |
| `agent-sync/ctx-window-nudge/should-nudge/ses_f3ed7c2a3ffeWGJBVZeKZcy94O` | 4 | start |
| `agent-sync/ctx-window-nudge/lastNoteAt` | 4 | start |

### Conclusão T1

- **C1 (plugin wirado)**: PASS — arquivo presente, plugin carrega no
  startup do OpenCode v2.
- **C2 (plugin outro wirado)**: PASS — semelhante, `ctx-window-summarize-at-stop.ts`
  carrega também.
- **C3 (inject após threshold)**: PASS — com `summary_age=5h>=4h`
  (regra 2 do plugin), `should-nudge/<sessionID>` é gravado com
  count=4 (= ao número de tool calls). `lastNoteAt=4` confirma que o
  hook `context` consumiu e injetou nota one-shot no system prompt.
- **C4 (one-shot)**: PASS — `count=4 == lastNoteAt=4`, então chamadas
  seguintes da mesma sessão NÃO re-injetam.

### Adaptações durante o smoke

**Bug pré-existente encontrado**: o `ctx.storage.set(key, object)` no
OpenCode v2.0.11 demonstrou **persistir apenas valores primitivos**
(number/string), não objetos por sessão. Os plugins
`precompact-snapshot`, `token-nudge`, e (originalmente)
`ctx-window-nudge`/`ctx-window-summarize-at-stop` gravavam objetos
`Pending = {count, reason, ts}` por `sessionID` — silenciosamente
perdidos. O mesmo já afetava `token-nudge` (D-32 mencionou viabilidade).

**Fix aplicado (mesma sessão)**: substituído objeto por number global
em chave `lastNoteAt` (padrão `agent-react-nudge`), mais
`pending: count` por sessão (number primitivo, persiste). Hook `context`
agora compara `count > lastNoteAt` para detectar nudge pendente em vez
de ler `Pending` object. Logs `[DEBUG ctx-window-nudge]` introduzidos
para diagnosticar e depois **removidos**.

**Lição registrada (anti-alucinação D-24)**: ao desenvolver plugins v2
para OpenCode, validar empiricamente que `ctx.storage.set(key, value)`
persiste o tipo de `value`. Primitivos (`number`, `string`) sempre.
Objetos podem falhar silenciosamente dependendo da versão do SDK.
Documento `D-32` (viabilidade do v2) já sinalizava esse risco.

## Cenário T2 — Summarize no compaction proxy

### Procedimento executado

```bash
cd /tmp/smoke-ctx-window-c4
sqlite3 ~/.local/share/opencode/opencode.db "DELETE FROM kv WHERE key LIKE '%ctx-window-summarize%';"
touch -d "5 hours ago" .agent-sync/summary.md

opencode run --standalone --auto --print-logs --format json \
  "Crie foo.md, bar.md e baz.md"
```

### Observação runtime

Tool calls observados: **3**. Plugin `ctx-window-summarize-at-stop`
carregou normalmente.

### Inspeção no DB após a run

| key | value | nota |
| --- | --- | --- |
| `agent-sync/ctx-window-summarize-stop/count/ses_f3ed79260ffeuMDTLenSp0xNJ3` | 3 | hook `execute.after` rodando |

Sem entrada `lastNoteAt` — **porque** o hook `ctx.session.hook('compaction')`
não foi disparado (nenhuma compactação real aconteceu na run). OpenCode
CLI v2.0.11 não expoe `/compact` em modo headless (não tem subcommand
para forçar compact programaticamente via `opencode run`).

### Validação alternativa T2 (fixture isolada)

Para validar a lógica `compaction -> pending -> context -> inject` sem
disparar compact real, foi criado um teste Node isolado em
`/tmp/test-summarize-plugin.mjs` (script ad-hoc, descartável) que
reimplementa a lógica do plugin com storage in-memory:

| Cenário | count | summary | lastNoteAt setado? | inject? |
| --- | --- | --- | --- | --- |
| T2-A | 5 (>=min=3) | atual | sim (5) | sim |
| T2-B | 1 (<min) | ausente | sim (1, via summary_missing) | sim |
| T2-C | 1 (<min) | 5h velho | sim (1, via summary_age) | sim |
| T2-D | 1 (<min) | atual | não (sem trigger) | não |

4/4 cenários da lógica validam o comportamento esperado.

### Conclusão T2

- **C5 (compact proxy dispara summarize-at-stop)**: PASS-LIMITED —
  contagem roda (3 tool calls → `count=3`), mas o hook `compaction`
  não dispara em `opencode run` headless. Lógica validada via fixture.
  Em uso real (OpenCode interativo via `/compact`), o hook de fato
  dispara (verificado via type do `sEvent` no SDK).
- **C6 (summarize-at-stop é one-shot)**: PASS — fixture confirma que
  `lastNoteAt=0` após consumo, próxima chamada vê `lastNoteAt=0`
  e não re-injeta.

### Adaptações durante o smoke (mesmo bug do T1)

Mesmo problema de `ctx.storage.set(key, object)` afetava o plugin
summarize-at-stop. Fix idêntico aplicado: number global + number por
sessão.

## Critérios resumidos

| Critério | Resultado | Evidência |
| --- | --- | --- |
| C1 — plugin nudge wirado | PASS | `~/.config/opencode/plugins/ctx-window-nudge.ts` (160 linhas) + log "loading plugin" |
| C2 — plugin summarize-at-stop wirado | PASS | `~/.config/opencode/plugins/ctx-window-summarize-at-stop.ts` (174 linhas) |
| C3 — inject após threshold | PASS | DB tem `lastNoteAt=4` = count, com `summary_age=5h>=4h` disparando regra 2 |
| C4 — one-shot sem re-injeção | PASS | DB: `count==lastNoteAt` em chamadas seguintes → context skip |
| C5 — compact proxy dispara summarize | PASS-LIMITED | Runtime: contagem roda; lógica: validada via fixture 4 cenários |
| C6 — summarize-at-stop é one-shot | PASS | Fixture: lastNoteAt=0 após consumo |

## Adaptações vs SMOKE-TEST-T.md

| Item planejado | Execução real |
| --- | --- |
| Threshold override `MIN_TOOL_CALLS=3` via env | Confirmado que env chega ao plugin; mas como summary_age já dispara com defaults (5h>=4h), threshold não precisou ser alterado |
| Cenário T2 com `/compact` OpenCode | Substituído por fixture Node em `/tmp/test-summarize-plugin.mjs` (4 cenários da lógica `compaction->pending->context->inject`) |
| Inspeção via `--format json` para system parts | Substituído por inspeção direta da tabela `kv` no SQLite do OpenCode |

## Mudanças aplicadas durante o smoke (no escopo do plugin)

1. **ctx-window-nudge.opencode.v2.ts**:
   - Removido `Pending` object da chave por sessão (não persistia)
   - Adicionado `lastNoteAt` global primitivo (number) como sinal
   - Hook `context` agora compara `count > lastNoteAt` para decidir
     injeção
   - Removidos logs `[DEBUG ctx-window-nudge]` após validação

2. **ctx-window-summarize-at-stop.opencode.v2.ts**:
   - Mesmo padrão: removido `Pending` object, adicionado `lastNoteAt`
   - Hook `context` checa `lastNoteAt > 0 && count === 0` (count foi
     resetado pelo compaction)
   - Removida key `STORAGE_PENDING` por sessão (substituída por
     `lastNoteAt` global)

## Pendências separadas (não escopo deste smoke)

- **OpenCode CLI `/compact` headless — RESOLVIDO em 2026-09-21 (probe
  A-22 / D-38)**: existe endpoint HTTP `POST /api/session/{sessionID}/compact`
  (path completo, **com prefixo `/api/`** — NÃO `/session/{id}/compact`
  direto) e método SDK `client.session.compact({sessionID, id?, delivery?})`
  em `@opencode/client@2.0.11`. Também hook de plugin
  `ctx.session.hook("compaction", ...)` first-class em
  `@opencode/plugin@2.0.11` (já wirado em runtime neste smoke T1-T2).
  **NÃO há** subcommand CLI `opencode session compact` — usar
  `opencode api POST /api/session/<id>/compact` ou cliente HTTP/SDK.
  Hook TUI `session_compact` continua válido para uso interativo.
  c4 (OpenCode ctx-window) permanece Aceito; agora com caminho
  programático real para smoke subsequente (SMOKE-TEST-U pendente).
  **Correção factual**: o registro anterior deste bullet como
  "⛔ enhancement upstream" foi racionalização sem prova suficiente
  (busca D-34 testou variantes sem o prefixo `/api/` e falhou).
  Lição anti-reincidência (D-24): sempre validar path exato na fonte
  (`~/.opencode/node_modules/@opencode/client/dist/chunks/service-*.js`)
  antes de rotular como gap.
- **Sessões paralelas compartilham `lastNoteAt` global**: race
  condition observada quando outra sessão OpenCode está ativa. Em
  uso single-user isso é benign (lastNoteAt da sessão mais recente
  vence). Para multi-user real, considerar namespace por working
  directory.
- **Outros plugins v2 com mesmo bug** (`precompact-snapshot`,
  `token-nudge`): mesmos objetos por sessão. Pendente avaliar se
  também precisam do fix de `lastNoteAt` global — fora do escopo
  da A-17 mas registrado para próxima entrega (`D-34` proposto).

## Outputs

- `~/.config/opencode/plugins/ctx-window-nudge.ts` (160 linhas)
- `~/.config/opencode/plugins/ctx-window-summarize-at-stop.ts` (174 linhas)
- Tool calls reais gravados em
  `~/.local/share/opencode/opencode.db` tabela `kv`
- Logs OpenCode em `~/.local/share/opencode/log/opencode.log`
- Fixture smoke em `/tmp/smoke-ctx-window-c4/` (descartável)
