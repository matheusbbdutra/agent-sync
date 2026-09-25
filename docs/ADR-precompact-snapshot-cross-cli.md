# ADR: PreCompact cross-CLI com snapshot estruturado (Claude + Codex + OpenCode + Antigravity; Cursor observacional)

**Status**: Aceito
**Data**: 2026-09-20
**Decisor**: Matheus Dutra
**Tags**: ai-agent, hooks, precompact, ctx-window, cross-cli, snapshot, typesafe-principles

> Vinheta: `D-28` ("C-1 PreCompact cross-CLI — decisão após verificação Codex") já
> confirmou a viabilidade da matriz 5×N. Esta ADR é a especificação completa
> do mecanismo (entradas, saídas, decisão por CLI, schema do snapshot e
> registro do gap do Cursor como limitação aceita).
>
> Atualização 2026-09-21: status promovido `Proposto` → `Aceito` após A-26
> (bump minor schema session-event.json + false-success-guard reconhece
> tool "compact" no transcript — entrega D-39). Decisão 5 implementada.

---

## Contexto

O agent-sync mantém um snapshot do estado da sessão em
`.agent-sync/session-state.json` (ADR-002) e um histórico append-only em
`.agent-sync/session-event.jsonl` (ADR-003). Ambos sobrevivem à compactação
porque ficam em disco. O que **não** sobrevive de forma determinística é a
**decisão que o harness tomou imediatamente antes da compactação**:

1. **Hoje, compactar é opaco.** O `PreCompact` do Claude Code, o
   `PreCompact` do Codex e o `experimental.session.compacting` do OpenCode v2
   disparam, mas o agent-sync não intercepta — não há registro de quais
   arquivos foram tocados nos últimos N turnos, qual era o token budget
   restante, quais decisões recentes precisam ser preservadas na nova janela.

2. **`false-success-guard` (ADR-harness-trace-guard) precisa de um evento
   `compact` confiável para virar hook "padrão de fim de turno".** Hoje o
   `false-success-guard` opera no `Stop` (Cursor, Claude) ou equivalente; uma
   compactação é exatamente o momento onde o modelo declara "concluí" sem
   ter concluído. Sem hook de PreCompact wirado, a compactação escapa da
   malha de detecção.

3. **Trilha C (`ctx-window`) precisa de um sinal de "compactando agora"**
   para congelar o sumário semântico incremental e gerar a versão final.
   Sem isso, o sumário só roda no `Stop`, tarde demais — o modelo já estará
   operando na janela compactada.

4. **Cobertura cross-CLI assimétrica.** Cada CLI tem um evento de
   compactação próprio com mecânica distinta (JSON output, exit code, callback
   observacional). Padronizar o **payload do snapshot** (`precompact-snapshot`)
   é o que torna a malha uniforme.

---

## Decisão

Adotar a estratégia **PreCompact cross-CLI com snapshot estruturado**,
padronizando o payload (schema `tools/jsonschema/schemas/precompact-snapshot.json`)
e despachando a decisão `allow | block | advise_only` por CLI conforme a
matriz abaixo. **Cursor fica de fora como observacional** — registrado
explicitamente como limitação aceita (seção "Limitações conhecidas").

### Decisão 1 — Schema do snapshot

Schema versionado conforme ADR-001 fechado (`additionalProperties: false`,
enums onde fizer sentido, única exceção registrada — `details` admite
`additionalProperties: true` para evolução por CLI):

```jsonc
{
  "schema_version": "1.0",
  "ts": "2026-09-20T17:00:00Z",         // ISO 8601 UTC
  "actor": "claude",                     // enum: claude|codex|opencode|cursor|agy|agent-sync
  "cli_version": "1.0.50",               // string livre; versão do harness
  "compaction_kind": "auto",             // enum: manual|auto|native
  "session_id": "sess-...",              // opcional
  "trigger": "ctx_threshold_reached",    // opcional, string livre
  "snapshot": {
    "ts": "2026-09-20T17:00:00Z",
    "tokens_in": 124500,                 // int, opcional
    "tokens_out": 31200,                 // int, opcional
    "files_touched": ["cmd/agent-sync/hooks.go"],
    "decisions_recent": ["D-28", "D-29"],
    "open_questions": ["B-2"],
    "actions_pending": ["A-13", "A-14"]
  },
  "decision": "block",                   // enum: allow|block|advise_only
  "decision_reason": "open_question B-2 ainda não respondida", // opcional
  "details": {                           // ÚNICA exceção a additionalProperties:false
    "cursor_compaction_mode": "auto",    // exemplo de detalhe livre
    "codex_continue_value": false        // espelha o campo nativo da CLI
  }
}
```

**Por que esse shape:**

- `compaction_kind` enum fecha o universo — não queremos `"forced"` vs
  `"aggressive"` vs `"user-requested"` sem decisão explícita.
- `decision` enum é a **interface pública entre o hook e o harness**: o
  agent-sync diz `block` e a CLI cancela a compactação (Claude via exit 2;
  Codex via JSON `continue: false`); `advise_only` deixa compactar mas
  registra o aviso; `allow` é o padrão.
- `details: { additionalProperties: true }` é a única exceção registrada ao
  princípio "fechado por padrão" da ADR-001 — mesma justificativa da ADR-003
  (evolução de payloads por CLI sem versionar schema a cada bump).

### Decisão 2 — Matriz de cobertura por CLI

Verificada em 2026-09-20 e registrada em `D-28`. Status por CLI:

| CLI       | Evento nativo                    | Mecanismo de block          | Mecanismo de injeção | Status |
| --------- | -------------------------------- | --------------------------- | -------------------- | ------ |
| Claude    | `PreCompact` (claude.com/docs)   | `exit 2` + stderr           | stdout JSON opcional | ✅ |
| Codex     | `PreCompact` (learn.chatgpt.com) | JSON `{"continue":false}`   | stdout JSON          | ✅ |
| OpenCode v2 | `experimental.session.compacting` (opencode.ai) | callback async (cancelar via `output.context.push` ou flag interno) | `output.context.push` | ✅ |
| Antigravity | `PreInvocation` (proxy)         | exit code + flag interno    | stdout JSON          | 🟡 (proxy; estender) |
| Cursor    | `preCompact` (cursor.com/docs)   | **sem block nativo**        | observacional apenas | ⛔ gap aceito |

**Por que cada estado:**

- **Claude (✅):** doc oficial confirma `PreCompact` aceita exit 2 + stderr
  para bloquear. Wirado via `syncHookCommandAtEvent(... "PreCompact")` no
  padrão já existente em `cmd/agent-sync/hooks.go:147` (`syncCtxCompactHook`).
- **Codex (✅):** doc oficial `learn.chatgpt.com/docs/hooks#precompact`
  (verificada em 2026-09-20, 5/5 checks) confirma JSON output com
  `continue: false` bloqueia. Matcher aceita `manual|auto`.
- **OpenCode v2 (✅):** `D-26` já registra `experimental.session.compacting`
  como callback. Plugin v2 (não v1) injeta via `output.context.push()`.
- **Antigravity (🟡):** Antigravity não tem `PreCompact` documentado.
  `PreInvocation` é o proxy mais próximo — disparado antes de cada tool call
  (incluindo chamadas internas que disparam compactação). Estender o
  `syncAntigravityPreInvocation` existente para enviar o snapshot quando o
  payload da invocação sinalizar compactação.
- **Cursor (⛔):** doc oficial (`cursor.com/docs/hooks`) lista `preCompact`
  como observacional — **não documenta mecanismo de block**. Hook já wirado
  pelo `syncCtxCompactHook` (cursor.go:156) apenas observa; vai virar
  seção "Limitações conhecidas".

### Decisão 3 — Wirar

**Hook Go (`cmd/agent-sync/hooks.go`)** — nova função
`syncContextSnapshotHook` que registra `PreCompact` em Claude Code, Codex e
via proxy Antigravity. Cursor fica sem wirar novo (já tem observacional).

```go
// syncContextSnapshotHook registra o hook PreCompact em CLIs que suportam
// block. Cursor fica de fora (observacional; ver ADR Decisão 4).
func syncContextSnapshotHook(baseDir string, target TargetCLI) error {
    if target.HooksSettingsPath == "" || target.HooksEvent == "" {
        return nil
    }
    command := "ctx-window snapshot " + target.AgentKind
    // Claude Code + Codex: PreCompact direto
    if target.HooksFormat == "claude" || target.HooksFormat == "codex" {
        return syncHookCommandAtEvent(baseDir, target, ctxSnapshotHookName,
            command, "*", "PreCompact", nil)
    }
    // Antigravity: PreInvocation como proxy
    if target.HooksFormat == "antigravity" {
        return syncAntigravityPreInvocation(target, ctxSnapshotHookName,
            command+" preinvocation")
    }
    // Cursor: observacional já wirado em syncCtxCompactHook (não duplicar)
    return nil
}
```

**Subcommand CLI novo:** `agent-sync state snapshot` (dentro do grupo
`state`, análogo a `state next-action` registrado em `D-15`). Lê
`.agent-sync/session-state.json` + últimos N eventos do
`session-event.jsonl`, monta `precompact-snapshot.json` conforme schema,
aplica regras de decisão (`block` se há `B-N` ativo há >7 dias sem
movimento; `advise_only` se há `Q-N` em aberto; `allow` caso contrário),
e escreve em stdout (Claude/Codex) ou via JSON estruturado (Antigravity).

**Plugin OpenCode v2:** `hooks/precompact-snapshot.opencode.v2.ts` no
pattern `Plugin.define` + `setup`. Registra callback em
`experimental.session.compacting`, monta snapshot análogo ao subcommand
CLI, decide `allow|block|advise_only` e usa `output.context.push()` para
injetar no contexto compactado.

**Script de wirar:** o hook Go chama `ctx-window snapshot <actor>` que
internamente decide a CLI pelo `<actor>`. Wirar uma única vez para
Claude/Codex/Antigravity.

### Decisão 4 — Limitações conhecidas (gap do Cursor registrado)

**Cursor não tem mecanismo de block de compactação documentado.** O hook
`preCompact` está wirado (via `syncCtxCompactHook`) mas opera apenas como
observacional — emite log, atualiza `.agent-sync/session-event.jsonl` com
kind `blocker` se detectar compactação bloqueante, mas **não cancela a
compactação**.

**Consequência assumida:** uma compactação automática do Cursor durante
trabalho de ctx-window pode descartar decisões recentes sem que
`false-success-guard` consiga sinalizar. Mitigação atual: o usuário
detecta via inspeção manual pós-compact (comparar `state render` antes/
depois) ou via `session-event.jsonl` que continua gravando eventos mesmo
após compact.

**Por que aceitamos (não abrimos ADR de feature):**

1. **Custo de descobrir e wirar block nativo no Cursor é alto e incerto.**
   Não há doc oficial. Engenharia reversa exigiria patch no harness.
2. **Cobertura cross-CLI fica em 4/5 (80%)** — mesma proporção que o
   agent-react-nudge já tem em outras trilhas (D-26 v2 só cobre OpenCode).
3. **Detecção post-hoc funciona** porque o `state render` + diff manual
   é determinístico — basta disciplina do usuário.
4. **Reabrir quando:** Cursor publicar doc de block nativo ou o agent-sync
   passar a depender criticamente do block (ex.: `false-success-guard`
   virar gate, não advisory).

---

## Consequências

### Positivas

1. **Detecção de compactação destrutiva em 4/5 CLIs.** O `false-success-guard`
   ganha um novo gatilho (`precompact_snapshot` kind `blocker` no
   session-event) confiável para Claude/Codex/OpenCode/Antigravity.
2. **Sumário semântico do `ctx-window` roda no momento certo.** O
   `trilha C` finalmente tem sinal de "compactando agora" — não mais tarde
   demais no `Stop`.
3. **Audit trail completo.** Cada compactação vira 1 linha no
   `session-event.jsonl` com `kind: precompact_snapshot` (kind novo,
   exige bump **minor** do schema — registrado na Decisão 5).
4. **Schema versionado e fechado.** `additionalProperties: false` no
   payload principal mantém princípio da ADR-001; única exceção é
   `details`, idêntica à ADR-003.

### Negativas / Riscos

1. **Antigravity em proxy (`PreInvocation`) gera overhead** — o hook é
   disparado a cada tool call, não só em compactação. Mitigação: filtro
   no script para só repassar quando payload sinalizar compactação
   (`if [ "$event_kind" != "compact" ]; then exit 0; fi`).
2. **Codex precisa de JSON output estrito** — qualquer saída não-JSON
   quebra o parsing do harness. Mitigação: `agent-sync state snapshot`
   sempre emite `{"continue": ...,"stopReason":"..."}` válido, nunca texto
   puro.
3. **OpenCode v2 tem ABI instável** (v2.0.11 verificada em D-26; pode
   mudar). Mitigação: `experimental.session.compacting` é estável
   documentado; se mudar, smoke SMOKE-TEST-S detecta.

### Neutras

1. **Schema bump minor (1.0 → 1.1).** Adição do enum kind
   `precompact_snapshot` no `session-event.json` (consumido pela nova
   ADR). Registrado na Decisão 5.

---

## Decisão 5 — Schema bump e kind novo

- `session-event.json` ganha enum `kind: [..., "precompact_snapshot"]`
  (minor bump 1.0 → 1.1; não-breaking — `additionalProperties: true` em
  consumers garante tolerância).
- `agent_tasks.json` (já bump em D-22): sem mudança.
- Novo schema `precompact-snapshot.json` (v1.0) — referência principal
  para o snapshot, **não** para o evento do log (esse segue
  `session-event.json`).

---

## Evidências e referências

- `D-28` — verificação Codex + matriz 5×N.
- `D-26` — OpenCode v2 plugin pattern + `experimental.session.compacting`.
- `D-15` — `agent-sync state next-action` (analogia ao novo `state snapshot`).
- `D-22` — `agent_tasks` schema base (referência para fechar enums).
- `ADR-001` — schema fechado por padrão (referência do princípio).
- `ADR-003` — `session-event.jsonl` (referência do append-only).
- `ADR-harness-trace-guard` — `false-success-guard` ganha novo gatilho.
- Doc oficial Codex: `learn.chatgpt.com/docs/hooks#precompact` (5/5 checks 2026-09-20).
- Doc oficial Claude: `claude.com/docs/hooks` (PreCompact + exit 2).
- Doc oficial OpenCode: `opencode.ai/docs/plugins/` (v2 callback + context hook).
- Doc oficial Cursor: `cursor.com/docs/hooks` (preCompact observacional).

---

## Próximos passos se aceita

1. Criar `tools/jsonschema/schemas/precompact-snapshot.json` (Fatia 2 de A-14).
2. Implementar `syncContextSnapshotHook` em `cmd/agent-sync/hooks.go` (Fatia 3).
3. Implementar `agent-sync state snapshot` subcommand (Fatia 3).
4. Implementar plugin `hooks/precompact-snapshot.opencode.v2.ts` (Fatia 3).
5. Escrever `docs/SMOKE-TEST-S.md` + rodar 3 cenários (Fatia 4).
6. Bump minor schema `session-event.json` (Decisão 5).
7. Atualizar `STATE.md` com decisão D-30+ quando aceito.
