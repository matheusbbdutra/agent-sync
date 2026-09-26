# Investigação — Contrato de hooks + MCPs do Cline v3 (A-80)

- **Status**: concluída (2026-09-25)
- **Método**: engenharia reversa do binário `cline` 3.0.65 (Bun bundle) + probes
  reais no host (hooks de arquivo, plugin, MCP, smokes)
- **Refs**: A-79 (wiramento parcial), A-80 (ADR-cline-hooks-mcp-wiramento.md),
  D-106, `internal/hooks/cline_bridge.go`, `cline-plugin/index.js`

## 1. Achado crítico: hooks por arquivo são INERTES no CLI

O A-79 concluiu que `~/.cline/hooks/PreToolUse` era a interface de hooks do
Cline. O binário tem de fato esse loader (`LW`/`wp0`/`i90`):

- diretório: `--hooks-dir` (default `~/.cline/hooks`); a flag só faz
  `process.env.CLINE_HOOKS_DIR = <path>`;
- descoberta: `readdir(dir)` + `extname ∈ {"", .sh, .bash, .zsh, .js, .cjs,
  .ts, .py, .ps1}` e nome-base (lowercased) ∈ enum de eventos;
- enum de eventos: `TaskStart, TaskResume, TaskCancel, TaskComplete,
  TaskError, PreToolUse, PostToolUse, UserPromptSubmit, PreCompact,
  SessionShutdown` (PreCompact mapeia para `undefined` → ignorado);
- execução: shebang decide o interpretador (`bash`, `node`, `bun run`,
  `python3`, `pwsh`); payload JSON no stdin; stdout inteiro (ou a última linha
  `HOOK_CONTROL\t<json>`) precisa ser JSON do contrato
  `{cancel, cancelReason, context|contextModification, errorMessage, review,
  overrideInput}`; `cancel:true` → `applyStopControl()` → `throw` → **aborta o
  run** (não existe deny por-tool no contrato de arquivo).

**Mas o loader só é criado se existir config-extension com capability
"hooks":**

```js
R50 = ["rules", "skills", "plugins"];
Gd0(Z, Q) => new Set(Z.extensions ?? R50).has(Q);
configExtensions = R50.filter((cap) => Gd0(config, cap));
// ...
G0 = tH(configExtensions, "hooks") ? sE({...file hooks...}) : undefined;
```

Como o CLI nunca inclui "hooks" em `configExtensions`, `G0` é sempre
`undefined` e **nenhum arquivo de `~/.cline/hooks` é executado**.

### Evidência empírica (4 probes)

| Probe | Resultado |
|---|---|
| `cline --hooks-dir /tmp/probe "..."` com `TaskStart`/`PreToolUse` que gravam em `probe.log` | `probe.log` **não** criado (0 execuções), mesmo com `editor`/`run_commands` no turno |
| `cline -v` com os shims wirados em `~/.cline/hooks` (A-80.1-v1) | log mostra os **estágios** `[hook:agent_start]`, `[hook:tool_call]`, `[hook:tool_result]` mas nenhum script roda |
| Cline Plugin com `hooks.beforeTool/afterTool/beforeRun/afterRun/onEvent` | **todos** os callbacks executaram (log do plugin com toolName/input) |
| `cline plugin install/uninstall` + layout `_installed` | plugin é descoberto varrendo `~/.cline/plugins/_installed/<kind>/<nome>-<hash>/package/` |

**Conclusão**: o A-79 validou apenas a execução manual do script (`bash
~/.cline/hooks/PreToolUse`) — nunca houve integração real com o Cline. O
`TestWiradoRealClineHook` do A-79 era um falso positivo (não passava pela CLI).

## 2. Rota suportada: Cline Plugin (AgentPlugin)

`cline plugin install <path>` copia para
`~/.cline/plugins/_installed/local/<nome>-<hash>/`:

```
<install>/package.json      {"name":...,"private":true,"cline":{"plugins":[{"paths":["./package/index.js"]}]}}
<install>/package/          conteúdo do pacote (index.js + package.json + plugin.json)
```

O CLI descobre plugins varrendo `_installed` — por isso o wiramento em Go pode
escrever esse layout direto (sem depender do binário `cline` no apply).

Interface do plugin (validada em runtime):

```js
module.exports = {
  name: "agent-sync",
  manifest: { capabilities: ["hooks"] },   // sem "hooks", os callbacks são ignorados
  hooks: { beforeTool, afterTool, beforeRun, afterRun, onEvent },
};
```

Contrato **interno** dos hooks (diferente do contrato de arquivo):

| Callback | Retorno aceito | Efeito |
|---|---|---|
| `beforeTool` | `{appendContext}` \| `{skip:true, reason}` \| `{input}` \| `{policy}` \| `{stop,reason}` | injecta contexto / **bloqueia a tool** / override input / policy / aborta run |
| `afterTool` | `{appendContext}` \| `{result}` | injecta contexto / substitui resultado |
| `beforeRun`/`afterRun`/`onEvent` | `{appendContext}` / `{stop}` | contexto no início; os dois últimos são observacionais |

Payload entregue aos callbacks (observado): `beforeTool({snapshot:{agentId,
conversationId,runId,iteration,status}, tool, toolCall:{toolCallId,toolName},
input})` e `afterTool({..., result:{output,isError}, startedAt, endedAt,
durationMs})`.

Diferença importante: no plugin o bloqueio é `skip` (por-tool), então um deny
de secret-guard **não** precisa abortar o run (como aconteceria com
`cancel:true` no contrato de arquivo).

## 3. MCPs: schema e config

- config: `~/.cline/data/settings/cline_mcp_settings.json` (env
  `CLINE_MCP_SETTINGS_PATH`), schema `{"mcpServers":{...}}` com `transport`:

```json
{
  "mcpServers": {
    "docs":       {"transport": {"type": "stdio", "command": "/path/docs-mcp"}},
    "code-graph": {"transport": {"type": "stdio", "command": "/path/repo-map", "args": ["--mcp"]}},
    "context7":   {"transport": {"type": "streamableHttp", "url": "https://mcp.context7.com/mcp"}}
  }
}
```

- escrita suportada: `cline mcp add <nome> --yes [--transport http <url>] --
  <cmd> [args...]` e `cline mcp remove <nome>` (idempotente com remove+add).
  `scripts/setup-mcp.sh` ganhou `upsert_cline()` e `upsert_cline_http()`.
- validado empiricamente com `CLINE_MCP_SETTINGS_PATH` isolado: 4 servers
  (`context7`, `docs`, `memory`, `code-graph`) gravados no schema acima +
  idempotência confirmada (diff vazio entre duas execuções).

## 4. Wiramento implementado (A-80)

| Peça | Caminho | Papel |
|---|---|---|
| Engine (Go) | `agent-sync hook cline --event=<E> --base-dir=<repo>` (`internal/hooks/cline_bridge.go`) | traduz payload Cline ↔ scripts agent-sync, roda os scripts do evento, mergeia contexto/cancel |
| Adapter (JS) | `cline-plugin/index.js` → `~/.cline/plugins/_installed/local/agent-sync-<hash>/package/` | mapeia callbacks do plugin para o engine e devolve `{appendContext}`/`{skip}` |
| Wirer (Go) | `internal/hooks/apply_cline.go` | instala/atualiza o plugin (idempotente) e remove shims inertes de A-79/A-80.1-v1 |
| MCP | `scripts/setup-mcp.sh` `upsert_cline*()` | 4 servers via `cline mcp add` |

Eventos cobertos pelo bridge v1 (payload compatível): `PreToolUse`
(bash-rm-guardian, context-guard, memory-nudge, agent-react, principles-inject,
secret-guard), `PostToolUse` (docs-cache, ctx-window-nudge, secret-guard,
memory-observe, token-nudge), `TaskStart` (memory-prune-session-start) e
`TaskComplete` (memory-consolidate).

Cobertura v2 (A-80.4): `TaskStart` ganha `ctx-handoff`
(`ctx-window handoff cline`, spec por `command`) e `TaskComplete` ganha
`ctx-window-summarize-at-stop.sh` e `agent-task-record.stop.sh` (telemetria
`cli=cline` em `.agent-sync/agent_tasks.jsonl`, com `tokens: null` — sem
transcript). O executor aceita `script` ou `command` e prepende o dir do binário
`agent-sync` ao `PATH` dos hooks.

Gaps aceitos (runtime de plugin não os suporta): `false-success-guard` (exige
`transcript_path`) e `precompact-snapshot` (sem `PreCompact`).

## 5. Refs

- `internal/hooks/cline_bridge.go`, `cline-plugin/index.js`,
  `internal/hooks/apply_cline.go`, `scripts/setup-mcp.sh`
- ADR: `docs/ADR-cline-hooks-mcp-wiramento.md`
- Testes: `internal/hooks/cline_bridge_test.go`,
  `internal/hooks/apply_cline_test.go`, `internal/hooks/smoke_real_a80_test.go`,
  `tools/cmd/repo-map/wirado_real_test.go`

## 6. Gotchas observados (evidência) e smoke real

### 6.1 O nome do módulo "agent-sync" não carrega

Com `module.exports.name = "agent-sync"` os hooks **não** são chamados (nenhum
registro em `/tmp/agent-sync-*`), tanto para o pacote escrito pelo wirer quanto
para o instalado por `cline plugin install`. Renomeando para
`agent-sync-hooks` (única mudança relevante; binário e manifest iguais) os
callbacks passam a executar. Causa raiz não isolada (suspeita: dedup/reserva de
nome no loader de plugins do CLI); o plugin usa `agent-sync-hooks` e o wirer
escreve esse mesmo nome.

Outro gotcha: o binário `agent-sync` **não está no PATH** do processo do Cline
em todos os ambientes — por isso o wirer grava o caminho absoluto do
executável em `agent-sync-config.json` (`{"baseDir":...,"bin":...}`); o
adapter só cai no PATH como fallback.

### 6.2 Smoke real (2026-09-25, `cline` 3.0.65, provider opencode-go)

```bash
cd <repo> && cline -t 150 "Liste os arquivos .md da raiz usando run_commands e responda so o total."
```

Evidência coletada:

1. `/tmp/agent-sync-memory-nudge/conv_1790386423728_q0468tf.count` criado às
   22:33:46 — id `conv_...` é o conversation id interno do Cline, provando que
   os scripts rodaram dentro da sessão real (o bridge mapeia `taskId`/
   `snapshot.conversationId`).
2. O raciocínio do modelo no log cita o contexto injetado pelo hook:
   *"The hook context reminds principles. I already ran the command..."* — prova
   de que `appendContext` alcançou o modelo (principles-inject).
3. Tool call real (`run_commands`) com `tool_input.command` sintetizado a partir
   de `{"commands":[...]}` (o Cline não usa `command` singular).

Reprodução do smoke de contrato (sem LLM, determinístico):

```bash
go test ./internal/hooks/ -run 'TestSmokeCline(BridgeEndToEnd|PluginAdapter)' -v
cd tools && go test ./cmd/repo-map/ -run TestWiradoRealClineHook -v
```
