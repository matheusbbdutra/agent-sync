# Smoke Test U — Compact OpenCode runtime via HTTP (A-28(i)) + token-nudge Antigravity dedicado (A-23)

> 2026-09-30 — executado em sessão Cline (D-116). Dois cenários independentes:
> **U1** fecha o sub-item (i) de A-28 (smoke real do endpoint
> `/api/session/{id}/compact`, cujo probe estático foi A-22/D-38);
> **U2** é o smoke dedicado de A-23 (C-3 Antigravity tokens, wirado 🟡 em D-32).

## TL;DR

| Cenário | Resultado | Ressalva |
|---|---|---|
| U1 — compact OpenCode runtime | **PASS** | summarização LLM não executada (sem provider autenticado) |
| U2 — token-nudge Antigravity (script + wiramento) | **PASS** | e2e em sessão `agy` real não executado → A-23 segue 🟡 |
| U3 — bug actor-vocab Antigravity (achado do U2) | **FIXED + VERIFIED** | `AGENT_SYNC_AGENT_KIND=antigravity` era rejeitado pelo schema → nudge morto em silêncio |

## Cenário U1 — POST /api/session/{id}/compact em runtime

**Contexto**: A-22 (2026-09-21) provou estaticamente que o endpoint existe
(`service-5mmhmmk7.js:397-404`). Faltava prova runtime. Ambiente:
`opencode v2.0.15` (probe original: v2.0.11), background service ativo.

**Procedimento e evidência** (2026-09-30):

```bash
# 1. Criar sessão via API (path com prefixo /api/, como previsto em A-22)
opencode api POST /api/session -d '{"title":"smoke-u-test"}'
# → 200: {"data":{"id":"ses_f0db621f6ffeXOfmjS8lrTJkLK",...,"title":"smoke-u-test"}}

# 2. Disparar compact programaticamente
opencode api POST /api/session/ses_f0db621f6ffeXOfmjS8lrTJkLK/compact -d '{}'
# → 200: {"data":{"id":"msg_0f24a9752001e0gkublwEc7A5z",
#         "sessionID":"ses_f0db621f6ffeXOfmjS8lrTJkLK",
#         "type":"compaction","payload":{},"delivery":"steer"}}

# 3. Cleanup
opencode api DELETE /api/session/ses_f0db621f6ffeXOfmjS8lrTJkLK
```

**Veredito**: o endpoint é chamável em runtime via `opencode api` (background
service), retorna mensagem `type:"compaction"` com `delivery:"steer"` —
assinatura idêntica à do SDK descoberta no probe A-22
(`client.session.compact({sessionID, delivery:'steer'|'queue'})`).

**Ressalva honesta**: `opencode auth list` → `No authenticated integrations`.
Sem provider, a **sumarização LLM em si não foi executada** — o smoke prova
roteamento + contrato de resposta, não o efeito de resumo. Fechar isso exige
sessão com provider autenticado (registrar como limitação, não como falha).

## Cenário U2 — token-nudge Antigravity (A-23)

**Objetivo**: smoke dedicado do contrato `ADR-token-nudge-contract.md` para
Antigravity (🟡 desde D-32), no nível script + wiramento.

**Wiramento real verificado** (`~/.gemini/config/hooks.json`):

```json
"agent-sync-token-nudge": { "PostToolUse": [ { "matcher": "*", "hooks": [ {
  "command": "AGENT_SYNC_AGENT_KIND=antigravity .../wrap-hook.sh PostToolUse agent-sync-token-nudge .../hooks/token-nudge.check.sh",
  "timeout": 10, "type": "command" } ] } ] }
```

**Procedimento e evidência** (transcripts sintéticos; parser exige substring
`"type":"assistant"` — `internal/budget/status.go:258`):

```bash
# HIGH: 905k tokens vs janela 200k, threshold 80
echo '{"session_id":"smoke-a23-high","transcript_path":"/tmp/a23-high.jsonl"}' \
  | AGENT_SYNC_AGENT_KIND=agy AGENT_SYNC_TOKEN_NUDGE_BIN=$PWD/bin/agent-sync \
    bash hooks/token-nudge.check.sh
# → {"hookSpecificOutput":{"hookEventName":"PostToolUse",
#    "additionalContext":"[agent-sync] Contexto em 100% de utilizacao
#    (trigger=tokens_in>=threshold). Considere rodar 'ctx-window summarize'..."}}
# exit=0

# LOW: 1.2k tokens → no-op silencioso, exit=0 (nenhum output)

# Decisão direta da CLI (schema token-budget-status v1.0):
./bin/agent-sync budget nudge -actor agy -session-id smoke-a23-high \
  -transcript /tmp/a23-high.jsonl -threshold 80
# → tokens_in=900000, tokens_total=905000, context_window=200000,
#   utilization_pct=100, should_nudge=true, trigger="tokens_in>=threshold"
# -actor antigravity (vocabulário normalizado A-82) produz o mesmo resultado.
```

**Achado importante**: o hook emite `hookSpecificOutput.additionalContext`
(schema PostToolUse), **não** `injectSteps+ephemeralMessage` — ou seja, não é
afetado pelo bug de schema do Antigravity PreInvocation corrigido em
`d1c1e60`/`75aba04` (2026-09-29), que fazia o Gemini CLI interpretar a saída
como "tool call denied by pre-tool hook".

**Veredito**: contrato de nudge funciona ponta-a-ponta no nível
payload→CLI→decisão→output JSON, e o wiramento real existe no host. **A-23
não fecha como ✅ ainda**: falta sessão `agy` real (e2e) confirmando que o
host entrega `transcript_path` no payload PostToolUse e renderiza o
`additionalContext` — o formato de transcript do Antigravity segue não
documentado oficialmente (gap aceito na ADR).

## Cenário U3 — BUG real achado pelo U2: actor-vocab divergente (fix D-116)

**Sintoma**: o wiramento real (`~/.gemini/config/hooks.json`) exporta
`AGENT_SYNC_AGENT_KIND=antigravity` — valor de `target.AgentKind`
(`internal/target/target.go:69`, também consumido por `ctx-window hook`, que
usa o vocabulário "antigravity"). Mas o enum `actor` dos schemas
`token-budget-status`/`agent_tasks` usa **"agy"** (`tools/actorvocab`).
Resultado antes do fix:

```console
$ agent-sync budget nudge -actor antigravity ...
❌ token-budget-status: schema invalido: ... at '/actor': 'anyOf' failed
$ echo $?   # via hook: erro engolido (2>/dev/null || true) → silent no-op
```

Ou seja: **o token nudge do Antigravity estava morto em runtime** — o hook
saía 0 sem nunca nudar, exatamente a classe de falha silenciosa que o
`actorvocab` (A-82) foi criado para prevenir (2 vocabulários divergentes).

**Fix (fonte única do vocabulário, 3 pontos)**:

1. `tools/actorvocab/actorvocab.go`: `aliases` (`antigravity`→`agy`,
   `claude-code`→`claude`) + `CanonicalActor()`, hookado em `NormalizeActor`
   e `NormalizeCLI`.
2. `internal/budget/status.go` (`BuildTokenBudgetStatus`): normaliza actor
   antes de montar/validar o payload.
3. `internal/budget/tasks.go` (`AppendAgentTask`): normaliza `cli` antes do
   append no `agent_tasks.jsonl`.

Testes novos: `TestCanonicalActorAliases`, `TestBuildTokenBudgetStatus_NormalizaActorAntigravity`,
`TestAppendAgentTaskNormalizaAliasAntigravity`. Assert pré-existente
`NormalizeActor("claude-code") = ("", false)` ajustado via caminho C'
(mudança intencional de contrato, comentada com D-116 no teste).

**Verificação pós-fix** (2026-09-30):

```console
$ go test ./...            # root: todos ok, 0 FAIL
$ (cd tools && go test ./...)  # tools: 23 ok, 0 FAIL
$ make install
$ ~/.local/bin/agent-sync budget nudge -actor antigravity \
    -session-id retest-1 -transcript /tmp/a23-high.jsonl -threshold 80
#   "actor": "agy", "should_nudge": true   (exit 0; retestado 2×)
$ echo '{"session_id":"...","transcript_path":"/tmp/a23-high.jsonl"}' \
  | AGENT_SYNC_AGENT_KIND=antigravity ... bash hooks/token-nudge.check.sh
# → {"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":
#    "[agent-sync] Contexto em 100% de utilizacao ..."}}  exit=0
```

## Impactos nos trackers

- **A-28(i)**: fechado por U1 (sub-item (iv) "livre escolha" segue aberto).
- **A-23**: progresso registrado; status 🟡 mantido com evidência de
  script+wiramento+fix U3; pendência residual = e2e em sessão agy real.
- **A-24/A-25 (Cursor)**: não tocados (dependem de release/feature request).
- **Bug fix D-116**: actor-vocab Antigravity normalizado na fonte
  (`actorvocab.CanonicalActor`) — beneficia também `agent_tasks.jsonl`
  (telemetria de budget do Antigravity, que gravaria `cli:"antigravity"`
  fora do vocabulário canônico).
