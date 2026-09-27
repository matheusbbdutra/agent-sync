# ADR — Plugin Cline em TypeScript (host define linguagem, Go fica motor)

- **Status**: Aceito
- **Data**: 2026-09-27
- **Decisor**: agente + usuário (ses_atual, decisão verbal "host define o tipo, mantemos Go como motor, opção A")
- **Fonte**: `docs/ADR-cline-hooks-mcp-wiramento.md` (Aceito rev. 3), `cline-plugin/index.js` (adapter atual), `docs/ADR-opencode-v2-ts-runtime.md` (paralelo OpenCode), `docs/PADRAO-HOOKS-CLIS.md`
- **Tags**: cline, hooks, typescript, plugin, A-87, host-definitive-language

## Contexto (verificável)

1. `cline-plugin/index.js:1-160` — adapter atual é **JS puro** (160 LOC), faz `execFileSync("agent-sync", ["hook","cline",...])` para o bridge Go. Mapeia `beforeTool/afterTool/beforeRun/afterRun` do plugin Cline para o engine e devolve `{appendContext}` ou `{skip, reason}`.
2. `docs.cline.bot/sdk/plugins` (overview lido 2026-09-27) — Cline aceita **TypeScript** via `pluginPaths: ["/abs/path/plugin.ts"]` ou `cline plugin install`; o objeto `AgentPlugin` carrega `manifest.capabilities` e o callback `hooks: { beforeTool, afterTool, beforeRun, afterRun, beforeModel, afterModel, onEvent }`.
3. `hooks/*.v2.ts` (13 arquivos, repo-map cache) — plugins OpenCode v2 já são TS, usam `@opencode/plugin`, pattern `Plugin.define({id, setup})`. Wiramento TS-first em OpenCode já é o estado-da-arte.
4. `cline-plugin/package.json:1-9` — `"name": "cline-plugin-agent-sync"`, `"main": "index.js"`. Sem `tsconfig.json`, sem `dist/`, sem deps TS no `cline-plugin/`.
5. `internal/hooks/cline_bridge.go` (referenciado em ADR-80) — engine Go permanece: `agent-sync hook cline --event=<E> --base-dir=<repo>` aceita payload Cline no stdin, normaliza, executa scripts agent-sync, responde `{cancel, context, cancelReason}`. **Não muda**.
6. `~/.claude/settings.json` (lido 2026-09-27) — wiramento Claude Code usa **shell** com 8 entries só para 2 hooks (Edit/Write/MultiEdit/NotebookEdit). Shell é o único mecanismo aceito por Claude Code, Cursor, Antigravity, Codex.
7. `docs/PADRAO-HOOKS-CLIS.md:21-28` — matriz 5×N já formaliza a separação: cada CLI tem superfície de hooks diferente; o agente-sync normaliza via bridge.

### Diagnóstico

- Cline hoje = plugin JS fino + bridge Go (já funciona, ADR-80 Aceito).
- OpenCode hoje = 13 plugins `.v2.ts` standalone wirados direto em `~/.config/opencode/plugins/`.
- Claude Code / Cursor / Antigravity / Codex hoje = shell adapters (gap aceito pela matriz 5×N).
- **Mistura existente**: dois wiramentos TS (Cline JS, OpenCode TS) + shell wiramento nos outros 4 CLIs. O usuário identificou que essa mistura merece política explícita.

## Decisão

### 1. Host define a linguagem do hook

Política formal, alinhada à matriz 5×N do `PADRAO-HOOKS-CLIS.md`:

| Host | Linguagem de hook | Mecanismo de wiramento |
|---|---|---|
| **Cline** | TypeScript | `cline plugin install` / `pluginPaths` apontando para `dist/index.js` |
| **OpenCode v2** | TypeScript | Plugins wirados em `~/.config/opencode/plugins/` |
| **Claude Code** | Shell thin adapter | `~/.claude/settings.json` → shell → bridge Go |
| **Cursor** | Shell thin adapter | `~/.cursor/hooks.json` → shell → bridge Go |
| **Antigravity** | Shell thin adapter | `~/.gemini/config/hooks.json` → shell → bridge Go |
| **Codex** | Shell thin adapter | `~/.codex/hooks.json` → shell → bridge Go |

### 2. Plugin Cline vira TS (migração 1:1)

Trocar `cline-plugin/index.js` por `cline-plugin/index.ts`:

- **Build**: `tsc -p cline-plugin/tsconfig.json` → `cline-plugin/dist/index.js` (Cline executa JS, não TS).
- **Wiramento**: `cline plugin install` aponta para `dist/` (ou `pluginPaths: [..., "dist/index.js"]`).
- **Mantém contrato interno**: `module.exports = { name, manifest, hooks: { beforeTool, afterTool, beforeRun, afterRun } }` — **não muda a forma**, só o tipo.
- **Mantém bridge Go intacto**: `callBridge(event, payload)` chama o mesmo `agent-sync hook cline` que já roda em produção.
- **Tipagem**: declarar `HookContext`, `HookResult`, `AgentPlugin` no próprio `cline-plugin/types.ts` (cópia local, mesmo padrão dos `.v2.ts` que copiam `PluginContext` de `@opencode/plugin`). Não importar `@cline/sdk` — subpath exports podem mudar entre versões (gotcha idêntico ao já documentado em `ADR-opencode-v2-ts-runtime.md:36`).

### 3. Go é o motor, não o handler

- **Motor** (mantém Go): `internal/hooks/cline_bridge.go`, `internal/hooks/apply_cline.go`, `internal/hooks/canonical_repo.go`, `internal/event/*`, `internal/state/*`, `internal/agentmemory/*`, `tools/cmd/ctx-window/*`, `tools/cmd/audit-removal/*`.
- **Handler** (passa a TS onde o host aceita): adapter Cline, plugins OpenCode v2.
- **Regra**: nunca escrever handler de hook em Go. Toda lógica de evento vive no motor (binário CLI); o adapter só mapeia payload.

### 4. Hook novo nasce em TS

- Default para Cline e OpenCode v2: TS com tipo declarado.
- Default para Claude Code / Cursor / Antigravity / Codex: shell ≤30 LOC que só faz `jq` + `exec agent-sync hook ...`. Sem lógica.
- Cada hook tem **1 fonte de verdade** (TS ou shell), não ambos.

### 5. Shell como "modo legacy" explícito

- Wiramento shell existente em Claude Code / Cursor / Antigravity / Codex vira **modo legacy aceito** (não será migrado para TS).
- Justificativa: nenhum desses 4 CLIs expõe SDK/plugin TS no escopo do agente-sync (ver `PADRAO-HOOKS-CLIS.md`). Reescrever shell em TS não traz ganho de manutenibilidade porque o host não entende TS ali.
- Consolidação paralela (ADR filha futura): reduzir duplicação Edit/Write/MultiEdit/NotebookEdit no `~/.claude/settings.json` via `matcher` mais amplo — fora do escopo deste ADR.

## Consequências

### Positivas

- **Consistência**: Cline e OpenCode compartilham paradigma TS-first; shell fica como modo legacy explícito nos 4 CLIs que não têm SDK.
- **Tipos**: `HookContext`, `HookResult`, `AgentPlugin` declarados em `cline-plugin/types.ts` eliminam casts manuais e dão autocomplete.
- **Build reproduzível**: `tsc` produz JS deterministicamente; o `cline plugin install` aponta para artefato versionado, não source TS.
- **Reversibilidade trivial**: revert deste commit remove `dist/` e `tsconfig.json`, volta a wirar `index.js` direto.
- **Anti-overengineering**: Go não é reescrito (motor fica intacto); bridge não muda; cobertura cross-CLI não regride.

### Negativas / limitações

- **+1 toolchain**: TypeScript agora é pré-requisito para desenvolver o plugin Cline (Node ≥ 20 + `tsc`). Mitigação: TS já é pré-requisito para OpenCode v2 (ADR-opencode-v2-ts-runtime.md Aceito), então a máquina que desenvolve agent-sync já tem.
- **+1 build step**: `tsc -p cline-plugin/` antes de `make install`. Pode virar target do Makefile (`make cline-plugin-build`).
- **Gap cross-CLI não fechado**: Claude Code / Cursor / Antigravity / Codex continuam shell-only. Sem mudança aqui.
- **Tipos manuais**: `cline-plugin/types.ts` precisa ser sincronizado com o schema real do Cline SDK. Se a doc oficial não listar tipos (os 404s em `/sdk/plugins/writing-plugins` indicam doc escassa), o jeito é copiar o `.d.ts` do runtime empacotado ou engenharia reversa (já feita em `cline-hooks-contract.md`).

### Trade-offs assumidos

- TS não é requisito para o **runtime** do Cline (só para desenvolvimento). O usuário final roda `dist/index.js` puro.
- Build não automatizado em CI ainda — fica como follow-up (`make cline-plugin-build`).
- Plugin Cline continua **global** (não há escopo por projeto no Cline CLI v3.0.65), mesmo gap já aceito no ADR-80.

## Critérios de promoção (Proposto → Aceito)

| # | Critério | Como medir |
|---|---|---|
| 1 | Plugin TS wirado em `~/.cline/plugins/_installed/local/agent-sync-hooks-<hash>/package/` e carregado pelo CLI | `ls` confirma; smoke empírico `cline -t <prompt>` gera counter em `/tmp/agent-sync-memory-nudge/` |
| 2 | Bridge Go executado com mesmo payload de antes (paridade 1:1) | Diff do stdout de `agent-sync hook cline --event=tool_call --base-dir=...` antes/depois deve ser vazio para inputs idênticos |
| 3 | `index.ts` compila sem warnings (`tsc --noEmit`) | `tsc --noEmit` exit 0 |
| 4 | `go test ./...` + `go vet ./...` verdes (motor intacto) | Suite completa |
| 5 | Wiramento idempotente em 2 execuções (sem warnings de diretório-como-JSON) | Mesmo critério já validado no ADR-80 |
| 6 | Sem regressão nos 4 CLIs shell-only | `agent-sync -target claude` + `cursor` + `antigravity` + `codex` continua wirando shell |

## Reversibilidade

- **Reverter migração TS**: `git revert <commit>` → plugin volta a ser `index.js` (commit original). Bridge Go intacto, wiramento volta ao estado Aceito do ADR-80.
- **Reverter toda a estratégia**: voltar `module.exports = ...` para JS em `cline-plugin/index.ts`, deletar `tsconfig.json` e `dist/`. Cline passa a rodar JS novamente.
- **Build quebrado por upgrade de Cline**: se nova versão do Cline mudar shape do `AgentPlugin`, ajustar `cline-plugin/types.ts` + lógica no mesmo PR. Bridge Go absorve variações (já provado em ADR-80).

## Não escopo

- Reescrever Go em TS (rejeitado por anti-overengineering — Go sustenta telemetria, ctx-window, repo-map, audit).
- Migrar shell adapters de Claude Code / Cursor / Antigravity / Codex para TS (gap aceito por host).
- Substituir `agent-sync hook cline` por implementação TS pura do bridge (rejeitado — bridge precisa de libsql, go-libsql, fs walks que já existem em Go).
- Adicionar `tsc` ao Makefile `setup` (pode ser tarefa filha A-90).
- Shared `hooks/_shared/types.ts` entre Cline e OpenCode (pode ser ADR filha A-89 se você quiser evitar drift de tipos).

## Refs

- `docs/ADR-cline-hooks-mcp-wiramento.md` (Aceito rev. 3) — plugin Cline atual via AgentPlugin
- `docs/ADR-opencode-v2-ts-runtime.md` (Aceito) — paralelo TS-first em OpenCode
- `docs/ADR-trilha-c-cobertura-cross-cli.md` (Aceito) — regra 5×N que justifica "host define linguagem"
- `docs/PADRAO-HOOKS-CLIS.md` — matriz de equivalência entre CLIs
- `cline-plugin/index.js:1-160` — adapter atual (origem da migração)
- `cline-plugin/package.json:1-9` — manifest atual
- `internal/hooks/cline_bridge.go` — engine Go (não muda)
- `docs/investigations/cline-hooks-contract.md` — engenharia reversa do Cline runtime
