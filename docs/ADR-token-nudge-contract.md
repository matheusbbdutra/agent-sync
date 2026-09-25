# ADR: Contrato de token nudge cross-CLI (Claude ✅ Codex ✅ OpenCode ✅ Antigravity 🟡 Cursor 🟡)

**Status**: Aceito (promovido por D-64 em 2026-09-23; schema `token-budget-status.json` v1.0 + `agent-sync budget nudge` + plugin OpenCode v2 + wirar cross-CLI feitos; Antigravity 🟡/Cursor 🟡 aceitos como gap)
**Data**: 2026-09-20
**Decisor**: Matheus Dutra
**Tags**: ai-agent, hooks, postToolUse, ctx-window, cross-cli, tokens, typesafe-principles

> Vinheta: A-16 fecha a Trilha C-3 e resolve B-2 ("Cursor + Antigravity: nudge
> de tokens sem contrato documentado"). A-14 (PreCompact) já entregou a
> matriz cross-CLI para compactação; esta ADR é o complemento para o
> **estado de tokens em tempo real durante o turno**.

---

## Contexto

O `ctx-window-nudge.sh` atual (linha 22-46 de `hooks/ctx-window-nudge.sh`)
**não consulta tokens** — opera puramente por heurística:

1. `count >= MIN_TOOL_CALLS` (default 80) → nudge.
2. `summary.md` com idade >= `MAX_AGE_HOURS` (default 4h) → nudge.
3. `summary.md` ausente → nudge.

**Três limitações conhecidas:**

1. **Heurística não detecta pressão real de contexto.** Sessão típica com
   5 tool calls de patches grandes pode já estar em 90% do context window
   sem atingir `MIN_TOOL_CALLS`. Conversely, sessão com 100 tool calls
   triviais (Read de arquivos curtos) pode estar em 20% do contexto e
   receber nudge desnecessário.

2. **`ctx-window-summarize-at-stop.sh` (A-2) e `precompact-snapshot`
   (A-14) já gravam tokens** (via `agent-task-record.stop.sh` e
   `state snapshot`). Mas o **nudge em postToolUse** continua cego aos
   tokens — perde a janela para sugerir summarização **antes** do
   usuário atingir o threshold de compactação.

3. **B-2 registrado** ("Cursor + Antigravity: nudge de tokens sem
   contrato documentado"). Ações wiradas hoje nesses CLIs usam o mesmo
   `ctx-window-nudge.sh` proxy — não há contrato formal de como cada CLI
   expõe tokens durante o turno.

---

## Decisão

Adotar **contrato de token nudge cross-CLI** com 3 pilares:

1. **Schema canônico** `tools/jsonschema/schemas/token-budget-status.json`
   v1.0 — payload do status de tokens lido pelo hook antes de decidir
   nudge.
2. **Fonte por CLI** (matriz Decisão 2) — cada CLI declara como o hook
   extrai tokens em postToolUse.
3. **Regra de decisão** (Decisão 3) — `should_nudge = true` quando
   `tokens_in >= threshold` OU heurística legada dispara.

### Decisão 1 — Schema do payload

Schema versionado conforme ADR-001 fechado (`additionalProperties: false`):

```jsonc
{
  "schema_version": "1.0",
  "ts": "2026-09-20T22:30:00Z",          // ISO 8601 UTC
  "actor": "claude",                      // enum: claude|codex|opencode|cursor|agy|agent-sync
  "session_id": "sess-...",               // string
  "model": "claude-sonnet-4.5",           // string (opcional; codex/opencode nativo, claude ausente)
  "tokens_in": 124500,                    // int >= 0
  "tokens_out": 31200,                    // int >= 0
  "tokens_total": 155700,                 // int >= 0 (tokens_in + tokens_out, computado)
  "context_window": 200000,               // int >= 0 (model context window)
  "utilization_pct": 77,                  // int 0-100 (tokens_total / context_window * 100)
  "threshold_pct": 80,                    // int 1-100 (limite para nudge; default 80)
  "should_nudge": true,                   // bool (computado pela regra de decisao)
  "trigger": "tokens_in>=threshold",      // enum: tokens_in>=threshold|tool_calls>=min|summary_age>=max|summary_missing
  "details": {                            // UNICA excecao a additionalProperties:false
    "transcript_lines_parsed": 42,        // exemplo de detalhe operacional
    "transcript_path": "/tmp/.claude/..." // exemplo de caminho interno
  }
}
```

**Por que esse shape:**

- `tokens_in`/`tokens_out`/`tokens_total`/`context_window`/`utilization_pct`
  formam o **bloco fechado** — campos que todas as fontes precisam
  fornecer (Claude/Codex via transcript parse, OpenCode via SDK).
- `threshold_pct` configurável via `AGENT_SYNC_TOKEN_NUDGE_THRESHOLD`
  (default 80; mesmo default heurístico legado).
- `should_nudge` é **computado** — não vem da fonte; o hook Go (CLI
  `budget nudge`) ou o script bash (postToolUse) decide.
- `details: { additionalProperties: true }` é a única exceção
  registrada, mesma justificativa da ADR-003 (evolução por CLI).
- **Ausência justificada**: `cost_estimate` não entra — A-15 já entrega
  cost em `AgentTask` separado. Schema de token-budget-status é sobre
  **decisão de nudge**, não telemetria financeira.

### Decisão 2 — Matriz de fontes por CLI

| CLI       | Fonte de tokens em postToolUse                  | Status |
| --------- | ----------------------------------------------- | ------ |
| Claude Code | Parse JSONL em `transcript_path` (campo `usage.input_tokens`/`output_tokens`) | ✅ verificado |
| Codex     | Parse JSONL em `transcript_path` + `model` nativo no payload | ✅ verificado |
| OpenCode v2 | `client.session.tokens()` via SDK injetado no Plugin context (`ctx.client`) | ✅ viabilidade a confirmar |
| Antigravity | Parse JSONL em `transcript_path` (assumindo mesmo padrão) | 🟡 gap aceito (D-32 separado) |
| Cursor    | Hook nativo NÃO documentado; **fallback**: ler `transcript_path` se disponível | 🟡 gap aceito (D-32 separado) |

**Por que cada estado:**

- **Claude Code (✅):** doc oficial `claude.com/docs/hooks` (verificada
  em 2026-09-20 para A-15) confirma `PostToolUse` recebe
  `transcript_path` mas **NÃO** tem campo `usage` no payload. Tokens vêm
  do parse do JSONL.
- **Codex (✅):** idem Claude Code; **acrescenta** campo `model` nativo
  no payload (Codex-specific extension).
- **OpenCode v2 (✅):** SDK OpenCode (v2.0.11) injeta `client: OpencodeClient`
  no Plugin context. Método `client.session.tokens({sessionID})` retorna
  `{input, output, total}` — **a verificar em smoke runtime** antes de
  promulgar a ADR (viabilidade em D-32).
- **Antigravity (🟡):** docs Antigravity não listadas/verificadas;
  assumir mesmo padrão JSONL com `transcript_path`. Se payload divergir,
  tokens ficam 0 e `should_nudge = false` (fallback para heurística
  legada). Aceito como gap até verificação oficial.
- **Cursor (🟡):** doc `cursor.com/docs/hooks` lista eventos mas
  `transcript_path` não documentado oficialmente. Se ausente, tokens
  ficam 0; heurística legada cobre. Gap aceito.

### Decisão 3 — Regra de decisão (`should_nudge`)

```
should_nudge = (utilization_pct >= threshold_pct)
            OR (heuristica_legada_dispara)

onde heuristica_legada_dispara = (tool_calls >= MIN_TOOL_CALLS)
                              OR (summary_age_hours >= MAX_AGE_HOURS)
                              OR (summary_missing)
```

**Implementação:** subcommand CLI `agent-sync budget nudge` (dentro do
grupo `budget`) lê último `AgentTask` (status=completed, mais recente)
+ invoca heurística legada (`ctx-window-nudge.sh` internamente? não —
  replicar lógica em Go é mais limpo). Decide e imprime JSON
  `token-budget-status.json` em stdout.

**Hook bash `token-nudge.check.sh`** wirado em `postToolUse` de Claude
Code, Codex, Antigravity, Cursor:

1. Lê payload `postToolUse` (tool_response, transcript_path).
2. Extrai tokens via parse de `transcript_path` (mesmo código do A-15).
3. Chama `agent-sync budget nudge -actor <cli> -transcript <path>`.
4. CLI devolve `token-budget-status.json` em stdout.
5. Se `should_nudge = true`, emite JSON
   `{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"[agent-sync] tokens X% (>=threshold Y%)..."}}`.
6. Exit 0 sempre (postToolUse não bloqueia).

### Decisão 4 — Plugin OpenCode v2 (viabilidade D-32)

Plugin `hooks/token-nudge.opencode.v2.ts`:

```ts
await ctx.tool.hook("execute.after", async (event) => {
  if (event.status !== "completed") return
  const tokens = await (ctx.client as any).session.tokens({
    sessionID: event.sessionID,
  })
  if (!tokens) return
  const total = tokens.input + tokens.output
  const threshold = Number(process.env.AGENT_SYNC_TOKEN_NUDGE_THRESHOLD || 80)
  const utilPct = Math.floor(total / tokens.contextWindow * 100)
  await ctx.storage.set(`agent-sync/token-nudge/${event.sessionID}`, {
    ts: new Date().toISOString(),
    tokens_in: tokens.input,
    tokens_out: tokens.output,
    context_window: tokens.contextWindow,
    utilization_pct: utilPct,
    threshold_pct: threshold,
    should_nudge: utilPct >= threshold,
  })
})
await ctx.session.hook("context", async (sEvent) => {
  const cached = await ctx.storage.get(
    `agent-sync/token-nudge/${sEvent.sessionID}`,
  )
  if (!cached || !cached.should_nudge) return
  sEvent.system.push({ type: "text", text: `[agent-sync] tokens ${cached.utilization_pct}% >= ${cached.threshold_pct}%; considere ctx-window summarize.` })
})
```

**Risco (registrado em Consequências):** ABI do SDK OpenCode v2 pode
mudar entre minor versions; hook silencioso se `client.session.tokens()`
mudar de assinatura. Mitigação: smoke runtime em sessão real antes de
promulgar D-32; fallback para heurística legada se método ausente.

### Decisão 5 — Limitações conhecidas

**Antigravity e Cursor**: tokens em tempo real NÃO verificados
oficialmente. Aceito como gap até verificação em D-32:

- Documentar heurística legada como **rede de segurança** (tool calls
  + summary age).
- Schema `token-budget-status.json` aceita `tokens_in = 0` sem erro
  (schema não exige > 0).
- Hook bash emite nudge quando heurística legada dispara, **mesmo se
  tokens não foram extraídos**.

**OpenCode v2**: viabilidade do `client.session.tokens()` confirmada em
D-26 (verificação parcial) mas não smoke runtime. Registrar como
**pendente verificar** — se método não existir, plugin vira
**silent no-op** e heurística legada cobre.

---

## Consequências

### Positivas

1. **Nudge baseado em pressão real de contexto.** Sessão com 90% do
   context window recebe nudge mesmo com 5 tool calls.
2. **Sessão com 100 tool calls triviais NÃO recebe nudge.** Reduz
   fadiga de notificação (causa raiz de nudges ignorados).
3. **Schema fechado consistente** com ADR-001 e ADR-003 (`details`
   única exceção).
4. **OpenCode v2 ganha canal de tokens** — fecha gap de cobertura
   cross-CLI que motivou B-2.
5. **Reaproveita infra A-15** (`agent-task-record.stop.sh` já grava
   tokens em `AgentTask`) — schema `token-budget-status` pode ser
   derivado do último AgentTask sem novo parse.

### Negativas / Riscos

1. **Parse de transcript em hook bash é caro.** Cada `postToolUse`
   dispara `tail + jq` no transcript JSONL. Em sessão típica (100+
   tool calls) são 100+ parses. Mitigação: cache em
   `$TMPDIR/agent-sync-token-nudge/<session_id>.last` para evitar
   re-parse; invalida a cada `N` chamadas (configurável).
2. **Antigravity/Cursor podem não expor `transcript_path`.** Hook
   silencioso vira heurística legada; aceite documentado.
3. **OpenCode v2 ABI instável** — `client.session.tokens()` pode
   mudar entre minor versions. Mitigação: silent fallback + smoke
   runtime gate.

### Neutras

1. **Schema `token-budget-status.json` é novo** (não existia antes).
   Adição não-breaking ao `tools/jsonschema/schemas/`.
2. **Subcommand CLI novo**: `agent-sync budget nudge` (dentro do grupo
   `budget` existente).
3. **Plugin v2 novo**: `hooks/token-nudge.opencode.v2.ts`.

---

## Próximos passos se aceita

1. Criar `tools/jsonschema/schemas/token-budget-status.json` (Fatia 2 de A-16).
2. Implementar `agent-sync budget nudge` em `cmd/agent-sync/budget_cli.go` (Fatia 3).
3. Criar `hooks/token-nudge.check.sh` + wirar em 4 CLIs (Fatia 4).
4. Criar `hooks/token-nudge.opencode.v2.ts` (Fatia 5).
5. Validar viabilidade do OpenCode v2 em smoke runtime (D-32 separado).
6. Atualizar `STATE.md` com decisão D-32+ quando aceito.

---

## Evidências e referências

- `D-30` — A-14 PreCompact cross-CLI (matriz 5×N).
- `D-31` — A-15 Budget tracking write path (parse de transcript como fonte).
- `D-26` — OpenCode v2 plugin pattern.
- `D-22` — A-9 budget tracking (lib jsonschema + CLI).
- `ADR-001` — schema fechado por padrão.
- `ADR-003` — `session-event.jsonl` append-only.
- `ADR-harness-trace-guard` — `false-success-guard` (referência de hook cross-CLI).
- Doc oficial Claude: `claude.com/docs/hooks` (PostToolUse input schema).
- Doc oficial Codex: `learn.chatgpt.com/docs/hooks` (Common input fields).
- Doc oficial OpenCode: `opencode.ai/docs/plugins/` (v2 Plugin context).
