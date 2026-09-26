# ADR — Wiramento completo de hooks + MCPs em Cline (A-80 rev. 2)

- **Status**: Aceito
- **Data**: 2026-09-25 (rev. 1) · 2026-09-26 (rev. 2, A-80.4: cobertura v2 + gaps)
- **Decisor**: agente + usuário (ses_atual)
- **Fonte**: `docs/investigations/cline-hooks-contract.md` (engenharia reversa do
  binário `cline` 3.0.65 + 5 probes reais), A-79 (wiramento parcial), A-74 (MCP)
- **Tags**: cline, hooks, mcp, plugin, wiramento, A-80

## Contexto (o que mudou desde a rev. 0)

A rev. 0 propunha "18 hooks virados como arquivos em `~/.cline/hooks/<EventName>`".
A investigação empírica derrubou essa premissa:

| Fato verificado | Evidência |
|---|---|
| `~/.cline/hooks` **não é executado** pelo CLI | loader de arquivos só é criado se houver config-extension com capability `hooks`; a lista de config-extensions do CLI é fixa em `["rules","skills","plugins"]` |
| probe com `--hooks-dir` + `TaskStart`/`PreToolUse` que gravam arquivo | 0 execuções, com tool calls reais no turno |
| plugin com `hooks.beforeTool/afterTool/beforeRun/afterRun` | todos os callbacks executaram |
| MCP via `cline mcp add` | 4 servers gravados em `cline_mcp_settings.json` (schema `transport`), idempotente |

Consequência: o wiramento do A-79 (e o `TestWiradoRealClineHook` original)
validava apenas a execução manual do script — **nunca houve integração real**.

## Decisão

**Wirar via Cline Plugin (AgentPlugin)** — a rota suportada pelo CLI — mantendo
o motor de tradução/execução em Go:

1. **Engine Go** (`internal/hooks/cline_bridge.go`):
   `agent-sync hook cline --event=<E> --base-dir=<repo>` lê o payload Cline no
   stdin, normaliza para o contrato Claude/Codex (`session_id`, `tool_name`,
   `tool_input` com `command` sintetizado de `commands[]`, `tool_response`,
   `hook_event_name`), executa os scripts agent-sync mapeados para o evento,
   mergeia contexto/cancel e responde no contrato Cline
   (`{cancel, context, cancelReason}`). Aceita tanto nomes de arquivo
   (`PreToolUse`) quanto nomes internos do runtime (`tool_call`).
2. **Adapter JS** (`cline-plugin/index.js`, ~160L): plugin com
   `manifest.capabilities=["hooks"]` que mapeia `beforeTool`/`afterTool`/
   `beforeRun`/`afterRun` para o engine e devolve `{appendContext}` ou
   `{skip, reason}` (bloqueio por-tool, melhor que o `cancel` do contrato de
   arquivo, que aborta o run).
3. **Wirer Go** (`internal/hooks/apply_cline.go`): instala/atualiza o plugin em
   `~/.cline/plugins/_installed/local/agent-sync-hooks-<hash>/package/` +
   agregador `package.json` (idempotente), grava `agent-sync-config.json` com
   `baseDir` absoluto e caminho absoluto do binário, e remove shims inertes de
   A-79/A-80.1-v1.
4. **MCPs** (`scripts/setup-mcp.sh`): `upsert_cline()`/
   `upsert_cline_http()` delegam para `cline mcp add --yes` (a CLI é dona do
   schema) — cobre `context7`, `docs`, `memory`, `code-graph`.

### Cobertura v1 (A-80.1)

- `PreToolUse` (`tool_call`): bash-rm-guardian, context-guard, memory-nudge,
  agent-react, principles-inject, secret-guard.
- `PostToolUse` (`tool_result`): docs-cache, ctx-window-nudge, secret-guard,
  memory-observe, token-nudge.
- `TaskStart` (`agent_start`): memory-prune-session-start.
- `TaskComplete` (`agent_end`): memory-consolidate.

### Cobertura v2 (A-80.4)

Hooks que o v1 deixou de fora por supostamente dependerem de transcript, mas
que na prática toleram o payload normalizado:

| Hook | Evento | Observação |
|---|---|---|
| `ctx-window-summarize-at-stop.sh` | `TaskComplete` | usa contador local + idade do `summary.md`; transcript é opcional no script |
| `agent-task-record.stop.sh` | `TaskComplete` | `cli=cline` via `AGENT_SYNC_AGENT_KIND`; `model=unknown` e tokens nulos (sem transcript) — grava em `.agent-sync/agent_tasks.jsonl` |
| `ctx-handoff` (`ctx-window handoff cline`) | `TaskStart` | spec por `command`; default do CLI emite `hookSpecificOutput.additionalContext` |

O executor do bridge aceita `script` (arquivo em `<baseDir>/hooks`) **ou**
`command` (linha shell via `bash -c`), e prepende o diretório do próprio binário
`agent-sync` ao `PATH` dos hooks — o `PATH` do processo do Cline não inclui
`~/.local/bin` de forma confiável, e `agent-task-record`/`ctx-handoff` dependem
de resolver `agent-sync`/`ctx-window`.

### Gaps aceitos (não wiráveis no runtime de plugin)

- **`false-success-guard`** (`Stop`): só age com `transcript_path` no payload
  (ver `tools/cmd/false-success-guard/hook.go`: sem transcript devolve `{}`).
  O runtime de plugin do Cline não expõe transcript nem `last_assistant_message`
  → wirar seria no-op. Mesma raiz do `tokens: null` do `agent-task-record`.
- **`precompact-snapshot`**: o runtime de plugin do Cline não expõe `PreCompact`
  (o loader de arquivos mapeia o evento para `undefined`) → mesmo gap aceito já
  registrado para o Cursor (`ADR-precompact-snapshot-cross-cli`, Decisão 4).
- ✅ **`token-nudge` inerte no Cline — RESOLVIDO (A-82)**: o script passa
  `-actor cline` para `agent-sync budget nudge`, e o enum `actor` de
  `token-budget-status.json` não tinha `cline`, então a validação falhava, o
  stdout ficava vazio e o script saía `exit 0` (no-op silencioso) — o nudge de
  contexto nunca era injetado. Corrigido pela revisão da ADR-003 (Decisão 2):
  o core enum ganhou `cline` e passou a aceitar o escape hatch `cli:<slug>`
  (adição deixou de exigir MAJOR bump). Validado:
  `agent-sync budget nudge -actor cline` retorna `should_nudge`.
- 🔴 **Agents custom no Cline (A-83) — gap aceito**: o Cline CLI v3.0.65 **não
  tem superfície de agents definidos pelo usuário**. Evidência: o validador de
  config-extensions aceita somente `rules|skills|plugins` (rejeita `hooks`,
  `workflows`, `agents`); não há `cline agent`; não há símbolos de loader de
  agents; e dois probes reais (`~/.cline/agents/<n>.md` **e** `agents/` no root
  do plugin, este último o campo `agents` do formato Agent Plugin) não
  expuseram nenhum agente ao modelo ("there is no agent registry tool").
  Os agentes do Cline são **dinâmicos**: `spawn_agent` (system prompt em
  runtime) e `team_member` (spawn/shutdown com `agentId`/`rolePrompt`), com
  `agentKind` derivado de `teamRole ∈ {lead, teammate}`. Por isso
  `target.Cline.AgentsDir` é vazio (gerar arquivos ali seria falso positivo,
  como o A-79) e os agentes especialistas do agent-sync chegam ao Cline pela
  camada de **Skills** (54 em `~/.cline/skills`, incluindo `agent-delegate`,
  `agent-learn`, `agent-react`). Se uma versão futura expuser agents, o probe
  deste ADR é o protocolo de revalidação.
- **`agent_tasks.json` (`cli`)**: enum também ficou sem `cline` e **quebrava a
  telemetria** (`budget write` rejeitava `cli=cline`); corrigido no A-80.4
  (aditivo, sem bump — o schema de `cli` não está coberto pela regra de MAJOR da
  ADR-003, que é específica do `actor` de `session-event`).
- **`false-success-guard`/tokens por transcript**: se uma versão futura do Cline
  expuser transcript (ou habilitar capability `hooks` para config-extensions,
  permitindo os hooks por arquivo), basta revogar o gap — o bridge já aceita os
  nomes de arquivo (`PreToolUse`) e os de plugin (`tool_call`).

## Critérios de aceite (verificação)

| # | Critério | Resultado |
|---|---|---|
| 1 | Hooks disparam em sessão real do Cline | ✅ counter `conv_...` criado + modelo cita `appendContext` injetado (principles-inject) |
| 2 | Wiramento idempotente (sem warnings de diretório-como-JSON) | ✅ `syncClineHooks` + early-return em `syncHookCommandAtEvent`; testes verdes |
| 3 | 4 MCP servers wirados no schema do Cline | ✅ `cline_mcp_settings.json` com `context7`/`docs`/`memory`/`code-graph`, diff vazio em 2 execuções |
| 4 | Sem regressão nas outras 5 CLIs | ✅ `go test ./...` (raiz + tools) + `go vet` verdes |
| 5 | Falha de hook nunca derruba o run | ✅ bridge/adapter capturam erro por script e respondem no-op |

## Consequências

**Positivas**: Cline passa a ter hooks agent-sync reais (nudge de contexto,
memória, principles, secret-guard, bash-rm-guardian) + os 4 MCPs; bloqueio
por-tool disponível (`skip`) sem abortar o run; nada de Bun/Python no runtime.

**Negativas/limitações**: adapter JS é obrigatório (a API de plugin do Cline é
JS); o nome do módulo precisa ser único (`agent-sync-hooks` — `agent-sync` não
carrega, causa não isolada); 2 hooks ficam permanentemente sem cobertura no
runtime de plugin (`false-success-guard`, `precompact-snapshot` — gaps aceitos
acima); `agent-task-record` grava `tokens: null`/`model: unknown`; plugins do
Cline são globais (não há escopo por projeto).

## Refs

- `docs/investigations/cline-hooks-contract.md` (contratos + evidências)
- `internal/hooks/cline_bridge.go`, `internal/hooks/apply_cline.go`,
  `cline-plugin/`, `scripts/setup-mcp.sh`
- A-79 (falso positivo), A-74 (audit_removal/MCP), A-73 (foundation)
