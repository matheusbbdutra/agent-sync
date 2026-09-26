# ADR — Wiramento completo de hooks + MCPs em Cline (A-80 rev. 1)

- **Status**: Aceito
- **Data**: 2026-09-25 (rev. 1)
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

### Cobertura v1

- `PreToolUse` (`tool_call`): bash-rm-guardian, context-guard, memory-nudge,
  agent-react, principles-inject, secret-guard.
- `PostToolUse` (`tool_result`): docs-cache, ctx-window-nudge, secret-guard,
  memory-observe, token-nudge.
- `TaskStart` (`agent_start`): memory-prune-session-start.
- `TaskComplete` (`agent_end`): memory-consolidate.

### Fora do escopo (A-80.4 candidato)

- hooks dependentes de transcript (`ctx-window` summarize, `ctx-handoff`,
  `agent-task-record`, `false-success-guard`) e `precompact-snapshot`
  (o Cline não expõe PreCompact no runtime de plugin);
- telemetria Cline em `.agent-sync/agent_tasks.jsonl`.

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
carrega, causa não isolada); hooks de transcript ficam sem cobertura v1;
plugins do Cline são globais (não há escopo por projeto).

## Refs

- `docs/investigations/cline-hooks-contract.md` (contratos + evidências)
- `internal/hooks/cline_bridge.go`, `internal/hooks/apply_cline.go`,
  `cline-plugin/`, `scripts/setup-mcp.sh`
- A-79 (falso positivo), A-74 (audit_removal/MCP), A-73 (foundation)
