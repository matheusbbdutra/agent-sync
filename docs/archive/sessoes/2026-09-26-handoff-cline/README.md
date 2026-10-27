# Handoff A-80/A-87 — Cline (arquivado em 2026-10-27)

> **MOVIDO PARA ARCHIVE em 2026-10-27**: bloco manual que vivia no fim de `STATE.md`
> (linhas 256-525). Ficava fora do ledger canônico e era apagado a cada
> `agent-sync state render`, exigindo recolar à mão. Pendência aberta nº 5 (higiene
> pós-merge) foi verificada e fechada em 2026-10-27: 6/6 hooks resolvem para
> `/home/matheus_dutra/Projects/agent-sync/hooks/` e não há resíduo do worktree
> `a73-audit-removal-go`. Conteúdo abaixo preservado byte a byte.

## Handoff — A-80/A-80.4 Cline (atualizado em 2026-09-26)

> **Merge concluído**: `worktree-a73-audit-removal-go` -> `main` em `7cac536`
> (A-73 [done]..A-80 [done]); conflito só em `STATE.md`, resolvido pela versão
> da branch. `make install` completo rodado (todos os binários de `bin/`
> alinhados em `~/.local/bin`).

> Memória durável (sobrevive a worktree/CLI): kinds `task_completed` (A-80),
> `hypothesis_validated` (hooks de arquivo inertes), `open_question` (nome do módulo),
> `decision` (atenção sobre memory.db concorrente), `action` (pendências) + páginas
> `handoff/index` e `handoff/A-80-cline-hooks-mcp`
> (`agent-sync memory read-page handoff/A-80-cline-hooks-mcp`; `memory lint` verde).

### O que ficou pronto

- A-80 [done] em 2 iterações: rev. 0 (hooks por arquivo) foi **descartada** após a
  investigação empírica provar que `~/.cline/hooks` é inerte no CLI v3.0.65; rev. 1
  wirou via **Cline Plugin** (`cline-plugin/` + `agent-sync hook cline`) e 4 MCPs
  (`upsert_cline` no `scripts/setup-mcp.sh`).
- Commits na branch (locais): `e074fe8`, `7c3b40f`, `afc4d09`, `afa0911`.
- Evidência do smoke real: `/tmp/agent-sync-memory-nudge/conv_1790386423728_q0468tf.count`
  + raciocínio do modelo citando o `appendContext` injetado pelo principles-inject.
- **Correção de registro**: o A-79 (`~/.cline/hooks/PreToolUse` + `TestWiradoRealClineHook`)
  era falso positivo — validava execução manual do script, sem passar pela CLI.

### A-80.4 (concluído em 2026-09-26)

- `clineHookSpec` ganhou campo `command` (linha shell via `bash -c`) além de
  `script`; o executor prepende o dir do binário `agent-sync` ao `PATH` dos
  hooks (gotcha: `~/.local/bin` não está no `PATH` do processo do Cline).
- `TaskComplete`: + `ctx-window-summarize-at-stop.sh` e
  `agent-task-record.stop.sh` (telemetria `cli=cline` em
  `.agent-sync/agent_tasks.jsonl`; `tokens: null`/`model: unknown` por ausência
  de transcript). Validado end-to-end com o binário instalado.
- `TaskStart`: + `ctx-handoff` (`ctx-window handoff cline`); `ctx-window handoff`
  agora aceita `cline` (`handoffCLIs` map).
- **Bug de schema encontrado e corrigido**: o enum `cli` de
  `tools/jsonschema/schemas/agent_tasks.json` nunca recebeu a 6ª CLI — o
  `agent-task-record` morria em `jsonschema validation failed` (mascarado por
  `>/dev/null 2>&1`) e **não gravava telemetria**. Adicionado `cline` (aditivo).
- **Gap aceito**: `false-success-guard` (só age com `transcript_path`) e
  `precompact-snapshot` (runtime de plugin não expõe `PreCompact`) — mesma
  classe do gap já aceito no Cursor. Ver ADR rev. 2.
- **Gap novo (requer decisão — MAJOR bump)**: `token-nudge` é **inerte no Cline**.
  O schema `token-budget-status.json` tem `actor` como enum fechado (sem `cline`);
  `budget nudge -actor cline` falha, o script sai `exit 0` e o nudge nunca é
  injetado. A ADR-003 (Decisão 2) exige MAJOR bump + migration para novo CLI no
  enum `actor` (afeta também `session-event.json` e `precompact-snapshot.json`).
- Commits: `d495e1f` (test fix do smoke), `fc5e3b4` (bridge v2),
  `3eccfa1` (docs), + commit do fix de schema/telemetria.
### A-82 / A-83 (concluído em 2026-09-26)

- **A-82 — `actor` extensível sem quebrar (decisão do usuário: opção 1).**
  Raiz: a lista de CLIs estava **duplicada em 6 lugares** (4 schemas JSON +
  `actorToAgent` em `internal/event/store.go` + `agentEnum` em
  `tools/cmd/memory-mcp/main.go`) com **2 vocabulários divergentes**
  (`claude`/`agy` nos schemas; `claude-code`/`antigravity` no memory-mcp), e a
  6ª CLI (Cline) ficou de fora de dois deles.
  Implementação: fonte única `tools/actorvocab`; os campos `actor`/`cli` agora
  são `anyOf: [enum core, pattern '^cli:[a-z][a-z0-9-]{0,31}$']` → **adicionar
  CLI é aditivo** (sem MAJOR bump; revisão da ADR-003 Decisão 2); `agentEnum` do
  memory-mcp derivado do registry; `actorToAgent` normaliza
  `claude`→`claude-code`/`agy`→`antigravity` num só lugar; help strings
  derivadas do registry.
  Guardas: `tools/jsonschema/jsonschema_actorvocab_test.go` (paridade
  schema↔registry + escape hatch), `tools/actorvocab/actorvocab_test.go`,
  `internal/event/store_test.go::TestActorToAgentNormalizaVocabularioDoMCP`.
  Validado: `budget nudge -actor cline` → `should_nudge`; `state snapshot -actor
  cline` → ok; bare desconhecido → rejeitado; `cli:<slug>` → aceito.
- **A-83 — agents custom no Cline: gap aceito (não entregar pela metade).**
  Evidência: config-extensions do CLI aceita só `rules|skills|plugins`; sem
  `cline agent`; sem loader de agents; 2 probes reais (`~/.cline/agents/<n>.md` e
  `agents/` no root do plugin) **não** expuseram agentes ao modelo. Agents do
  Cline são dinâmicos (`spawn_agent`, `team_member`/`teamRole`). Ação:
  `target.Cline.AgentsDir` **zerado** (o apply não emite mais o warning "tipo de
  agente desconhecido" nem entrega diretório vazio); os agentes especialistas
  chegam ao Cline via **Skills** (54 sincronizadas). Guarda:
  `internal/target/target_test.go::TestClineNaoDeclaraAgentsDir`.
### A-84 — wiramento único do plugin Cline (Pendência 4 fechada)

- **Problema**: dir do plugin = `_installed/local/agent-sync-hooks-<sha256(baseDir)[:12]>`,
  então apply de outro `baseDir` **não substitui** — cria um 2º plugin. O CLI do
  Cline carrega **todos** os dirs de `_installed/local` (`u9()` empurra todo
  subdir; `i9()` usa Map com **chave = path**, sem dedup por nome) → cada hook
  rodaria 2× (appendContext duplicado, 2 linhas de telemetria/task, summarize em
  dobro). E `cline-plugin/index.js` engole erro do bridge → plugin apontando para
  `baseDir` morto vira **no-op silencioso**.
- **Solução**: `internal/hooks/canonical_repo.go` +
  `pruneOrphanClinePluginDirs` em `apply_cline.go` (detalhes na pendência 4).
  Escape hatch `AGENT_SYNC_ALLOW_BASEDIR=1` mantém a coexistência deliberada (sem
  podar) e avisa que hooks podem rodar 2×.
- **Decisão de design**: "último apply manda" (opção a do handoff) **não** era o
  comportamento real — o hash no nome sempre acumulou. Optou-se por canônico
  explícito + poda, não por dir fixo (opção A), para permitir testar um worktree
  sem sequestrar o plugin global.
- **Hermeticidade**: testes isolam `XDG_CONFIG_HOME` (`isolateConfigHome`) — o
  config real do usuário (com `turso.token`) **não** é tocado pela suíte;
  confirmado porque o `make apply` real depois ainda disse "primeiro apply".

### A-85 — smoke visível do wiramento Cline (TaskStart)

- **Motivação** (ses_atual, 2026-09-26): usuário abriu Cline, rodou 1 tool call
  e reportou "não consta nenhum hook nele". Investigação empírica provou que o
  wiramento estava **operacional** (4 hooks escrevendo em `/tmp/agent-sync-*/`),
  mas a saída era **silenciosa** — `principles-inject` é one-shot e
  `memory-nudge`/`ctx-window-nudge` só injetam contexto a cada N calls
  (`count % THRESHOLD == 0`). Sem feedback visível no turno, era impossível
  distinguir "wiramento quebrado" de "wiramento silencioso".
- **Solução**: hook `cline-wiramento-smoke.sh` wirado em `TaskStart` que
  (1) escreve `/tmp/agent-sync-cline-wiramento/<sid>.log` com timestamp +
  lista completa de hooks wirados + paths de artefatos visíveis, e
  (2) injeta `[agent-sync Cline wiramento] hooks wirados via plugin
  (A-80/A-80.4/A-84): PreToolUse=6 PostToolUse=5 TaskStart=3 TaskComplete=3.
  Artefatos visiveis em /tmp/agent-sync-*/. MCP=4 servers, skills=54, agents=0
  (gap aceito). Smoke log: <path>` no `additionalContext` do TaskStart.
  One-shot por sessão via flag `<sid>.seen` (subsequentes injetam só o resumo).
- **Implementação**:
  - `hooks/cline-wiramento-smoke.sh` (novo, 70 linhas): bash puro, padrão
    `memory-prune-session-start.sh:1-88` (opt-out `AGENT_SYNC_WIRAMENTO_SMOKE=0`,
    `STATE_DIR=${TMPDIR:-/tmp}/agent-sync-cline-wiramento`, dedup via `.seen`).
  - `internal/hooks/cline_bridge.go:117` — adicionada spec
    `{event: "TaskStart", script: "cline-wiramento-smoke.sh", name: "cline-wiramento-smoke"}`
    após `memory-prune-session-start.sh` e `ctx-window handoff cline`.
  - `make build && make install && agent-sync -apply -target=cline` (idempotente).
- **Validação 4 camadas** (ses_atual, 2026-09-26):
  1. **Script isolado**: `bash hooks/cline-wiramento-smoke.sh < payload` →
     stdout JSON válido com tag `[agent-sync Cline wiramento]` + cria `.log` e
     `.seen` em `/tmp/agent-sync-cline-wiramento/`. ✅
  2. **Bridge manual**: `agent-sync hook cline --event=TaskStart` →
     `{"cancel":false,"context":"[agent-sync Cline wiramento] hooks wirados..."}`
     + `smoke-A85-bridge.log` com 18 linhas. ✅
  3. **Cline real**: `cline --cwd ... "Responda SMOKE-A85-OK se vir a tag"` →
     modelo ecoou `SMOKE-A85-OK` e citou literalmente o conteúdo do contexto
     injetado (incluindo o path do log). Host criou
     `conv_1790446909364_xvx5e8e.{log,seen}`. ✅
  4. **Wiramento colateral validado**: `bash-rm-guardian` (PreToolUse wirado
     em paralelo) **bloqueou** meu `rm -rf /tmp/agent-sync-cline-wiramento`
     com mensagem `"tem refs ativas no repo. 3 active unknown-literal-hit"` —
     prova empírica de que o PreToolUse do bash-rm-guardian está wirado e
     protegendo refs do repo contra deleção acidental. ✅
- **Como o usuário valida daqui pra frente**: abrir o Cline normalmente. Na
  **primeira resposta** do modelo, esperar ver literalmente a tag
  `[agent-sync Cline wiramento] hooks wirados via plugin...`. Se NÃO
  aparecer, o wiramento realmente está quebrado (não é mais artefato
  silencioso). Log persistente em `/tmp/agent-sync-cline-wiramento/<sid>.log`
  para auditoria.
- **Decisão de produto**: opt-in default ON; opt-out via
  `AGENT_SYNC_WIRAMENTO_SMOKE=0`. Justificativa: custo zero (idempotente
  one-shot), ganho de UX alto (elimina "wiramento invisível" como classe de
  confusão). Refs: A-80/A-80.4/A-84 (cobre a totalidade do wiramento Cline).
  Origem: ses_atual, 2026-09-26.





### Achado que vale reler antes de mexer

- Hooks por ARQUIVO em `~/.cline/hooks` são inertes no CLI v3.0.65 (loader gateado por
  capability `hooks` ausente); o wiramento real é via Cline Plugin. O A-79 era falso
  positivo. Evidências: `docs/investigations/cline-hooks-contract.md`.
- Gotcha do plugin (memória `open_question`): exportar `name: "agent-sync"` no módulo do
  plugin **não carrega** (nenhum callback executa); `agent-sync-hooks` funciona. Causa
  raiz não isolada (suspeita: reserva/dedup por nome).
- Ferramenta (memória `decision`): **não** abrir `~/.cache/agent-sync/memory.db` com
  sqlite3/python externo enquanto o `memory-mcp` pode escrever — nesta sessão isso fez
  `memory add` falhar com `memory-mcp saiu com erro: exit status 1` e uma gravação
  reportar sucesso sem persistir. Usar sempre a CLI (`agent-sync memory ...`).


### Pendências (ordem sugerida)

1. ~~**Merge/push** `worktree-a73-audit-removal-go` -> `main`~~ **[done 2026-09-26]**
   (`7cac536`; conflito só em `STATE.md`).
2. ~~**A-80.4** (nova): estender `clineHookSpecs`~~ **[done 2026-09-26]** — ver
   seção "A-80.4" acima (gaps aceitos para `false-success-guard`/`precompact`).
3. ~~**`make install`** completo~~ **[done 2026-09-26]** (todos os binários de
   `bin/` alinhados em `~/.local/bin`; smoke `TestSmokeClinePluginAdapter` verde).
4. ~~**Decidir sobre path injection**~~ **[done 2026-09-26 → A-84]** — decisão do
   usuário: **opção D** = repo canônico + poda de órfãos. Implementado em
   `internal/hooks/canonical_repo.go` + `apply_cline.go`:
   - o 1º apply registra o canônico em `~/.config/agent-sync/config.json`
     (`"repo"`, merge preservando `turso`/`summarizer`/`projects`, modo 0600);
   - apply de outro `baseDir` **avisa e não adota** (nem cria nem poda),
     escape hatch `AGENT_SYNC_ALLOW_BASEDIR=1`;
   - no caminho canônico, plugins nossos de outros baseDirs são **podados**
     (`pruneOrphanClinePluginDirs`), e canônico obsoleto se auto-cura.
   Evidência real (`make apply`): órfão `agent-sync-hooks-deadbeef0000` removido
   com nota ℹ️, config real ganhou `repo` sem perder campos, 2º apply silencioso
   (idempotente). 13 testes em `canonical_repo_test.go` /
   `apply_cline_prune_test.go`. Resumo técnico abaixo em "A-84".
5. Higiene pós-merge: ~~rodar `make apply` do repo principal~~ + conferir
   `~/.codex/hooks.json` / `~/.claude/settings.json` apontam para paths válidos
   (o apply da sessão A-80 usou `baseDir` = worktree `a73-audit-removal-go`).
6. ~~**A-81**: `memory add` sensível a escrita concorrente no `memory.db`~~
   **[done 2026-09-26]** — `event.recordEventWithRetry`
   (`internal/event/store.go`) faz até 3 tentativas com backoff de 150ms em
   falhas transitórias (orçamento total dividido; binário ausente não re-tenta) e
   devolve erro explícito com o número de tentativas. Regra mantida: **não** usar
   SQL externo (sqlite3/python) concorrente no `memory.db` — usar a CLI
   (`agent-sync memory ...`). Limitação conhecida: "sucesso sem persistir" (RPC
   ok) não é coberto pelo retry. Testes em `internal/event/store_test.go`.
7. ~~**Decidir MAJOR bump do enum `actor`**~~ **[done 2026-09-26 → A-82]** — decisão
   do usuário: **core enum + escape hatch namespaced `cli:<slug>`**, que torna a
   adição de CLI **aditiva** (sem MAJOR bump) e revisa a ADR-003 Decisão 2.
   Implementado com fonte única em `tools/actorvocab` (alimenta os 4 schemas, o
   `agentEnum` do memory-mcp e as help strings), teste de paridade dos schemas e
   normalização do vocabulário do memory-mcp (`claude`→`claude-code`,
   `agy`→`antigravity`). Consequência: **`token-nudge` no Cline deixou de ser
   inerte** (`budget nudge -actor cline` validado). Gaps que sobraram: apenas
   `false-success-guard` e `precompact-snapshot` (sem transcript/evento).

### Como revalidar rápido

```bash
cd /home/matheus_dutra/Projects/agent-sync        # main, pós-merge
go vet ./... && go test ./... && (cd tools && go test ./...)
go test ./internal/hooks/ -run 'Cline|PrependedPath' -v
(cd tools && go test ./cmd/ctx-window/ -run Handoff -v)
make install                                      # revalida os smokes do plugin Cline
ls ~/.cline/plugins/_installed/local/              # A-84: deve listar UM só agent-sync-hooks-<hash>
python3 -c "import json,os;print(json.load(open(os.path.expanduser('~/.config/agent-sync/config.json'))).get('repo'))"
timeout 60 cline -t 120 "Liste os arquivos .md da raiz usando run_commands e responda so o total."
ls /tmp/agent-sync-memory-nudge/ | tail -3         # counter conv_... novo = hooks rodaram
```

### Ponteiros

- `docs/ADR-cline-hooks-mcp-wiramento.md` (Aceito rev. 2, com cobertura v2 + gaps)
- `docs/investigations/cline-hooks-contract.md` (contratos + 5 probes + gotchas)
- `internal/hooks/cline_bridge.go`, `internal/hooks/apply_cline.go`, `internal/hooks/canonical_repo.go`, `cline-plugin/index.js`
### A-87 [done] — Plugin Cline em TypeScript (migração 1:1 do JS)

- **Origem**: ses_atual (2026-09-27). Usuário identificou mistura
  desnecessária de linguagens nos wiramentos cross-CLI: Cline JS
  (`cline-plugin/index.js`), OpenCode TS (`hooks/*.v2.ts`), Claude Code +
  Cursor + Antigravity + Codex em shell. Decisão verbal: **opção A** —
  host define tipo, Go fica motor, plugin Cline migra JS→TS, shell vira
  modo legacy explícito nos 4 hosts sem SDK TS.
- **ADRs Aceitos** (movidos de `docs/adr-staging/` para `docs/`):
  1. `docs/ADR-cline-plugin-ts-first.md` (A-87, Aceito) — migração 1:1
     do `cline-plugin/index.js` para `index.ts`, build `tsc` →
     `dist/index.js`, bridge Go intacto, tipagem via
     `cline-plugin/types.ts` (cópia local, mesmo padrão dos `.v2.ts`
     que copiam `PluginContext` de `@opencode/plugin`).
  2. `docs/ADR-linguagem-hooks-por-host.md` (A-88, Aceito) — política
     geral: Cline/OpenCode v2 → TS; Claude Code/Cursor/Antigravity/
     Codex → shell thin adapter ≤30 LOC; motor Go nunca vira handler;
     1 fonte de verdade por hook.
- **Refs**: `ADR-cline-hooks-mcp-wiramento.md` (Aceito rev. 3),
  `ADR-opencode-v2-ts-runtime.md` (Aceito, paralelo TS em OpenCode),
  `ADR-trilha-c-cobertura-cross-cli.md` (regra 5×N que embasa a
  política), `PADRAO-HOOKS-CLIS.md` (matriz de equivalência entre
  CLIs), `cline-plugin/index.js:1-160` (origem da migração).
- **Nota de numeração**: A-86 já existia (2026-09-26, chmod + parse
  taskId em 3 hooks — linha 14 deste STATE). A-87 = presente migração;
  A-88 = política geral; A-89 = ADR filha de tipos compartilhados.
- **Próximos passos** (tasks no shared list):
  1. ~~Sprint 0~~ [done]: revisão e promoção dos 2 ADRs.
  2. Sprint 1 [in_progress, branch `feature/cline-plugin-ts-first`]:
     portar `cline-plugin/index.js` → `index.ts`, build `tsc`, smoke
     paridade 1:1 com bridge Go.
  3. Sprint 2 (opcional): `hooks/_shared/types.ts` para tipos comuns
     Cline+OpenCode (ADR filha A-89, não decidido).
  4. Sprint 3: aplicar a política a 1 hook novo (piloto end-to-end
     TS-first).
  5. Sprint 4: declarar shell como "modo legacy" para hosts não-TS;
     consolidar Edit/Write/MultiEdit/NotebookEdit no
     `~/.claude/settings.json` num único entry com matcher amplo
     (ADR filha separada).
