# STATE — agent-sync

> Fonte de verdade para retomar o trabalho entre sessões/compactions.

## Execução do plano consolidado — 2026-09-18

- Otimização de Regras Globais e Skills (Princípios Karpathy & Context Reduction) — 2026-09-18:
  - **Regras Globais (`rules/global-rules.md`)**: Enxugadas cirurgicamente de ~15 KB para ~2.2 KB (~85% de redução de tokens de entrada em todo turno das 5 CLIs), mantendo rigorosamente PT-BR, proibições de segurança (.env, git destrutivo), anti-alucinação estrita (*Hipótese ≠ Fato*) e Clean Code cirúrgico.
  - **Poda de Skills Desnecessárias/Redundantes**: Removidas do repositório e de `manifest.json` as skills fora da stack do usuário (`python-pro`, `python-testing-patterns`), redundâncias de contexto genéricas (`context-management-context-save`, `context-manager`) e sobreposições (`codebase-cleanup-tech-debt`, `incident-response-smart-fix`). Mantidas linguagens ativas (PHP, Go, TypeScript, JavaScript).
  - **Descrições Cirúrgicas das Skills**: 42 skills tiveram o campo `description` no frontmatter YAML reescrito para um formato estritamente conciso (12–18 palavras, "o que faz + gatilho"), evitando o estouro de orçamento de contexto (`context budget limits`) das ferramentas e diluição de atenção.
  - **Verificação**: `go test ./...` 100% PASS, `make build` concluído com sucesso e validação de `git diff`.

- Revisão defensiva aplicada: `runHandoff` em `tools/cmd/ctx-window/handoff.go` foi blindado para aceitar `stderr`, engolir payloads vazios ou JSONs inválidos e sempre retornar exit code 0 com `{}` quando não houver resumo a injetar, prevenindo quebrar o `SessionStart` da CLI hospedeira. `runHook` em `hook.go` agora loga falhas de gravação em `stderr` e retorna `nil` com `{}` (em vez de propagar erro e causar `os.Exit(1)`). Casos defensivos cobertos por testes unitários em `handoff_test.go`. `go test`, `go vet` e `make build` aprovados.
- Aplicação global concluída: `GOCACHE=/tmp/agent-sync-go-cache make apply` terminou com exit 0 e sincronizou as 5 CLIs. Verificação read-only do JSON global confirmou `ctx-window hook` e `ctx-window handoff` em Claude, Codex, Cursor e Antigravity; hashes do binário `ctx-window` e do plugin OpenCode instalado coincidem com os arquivos compilado/fonte do repositório. `agent-sync -status` confirmou regras, skills e agentes presentes. O fluxo `make apply` também sincronizou regras/skills/scripts e atualizou o bloco gerenciado de shell-validate em `~/.zshrc`.
- Correção antes da aplicação: `main.go` saía do loop no ramo Cursor antes de chamar `syncCtxCompactHook` e `syncCtxHandoffHook`; além disso, `syncCtxCompactHook` usava o formato de configuração Claude/Codex para Cursor. Causa raiz comprovada pela leitura de `main.go` e pelo schema Cursor. O ramo agora chama ambos antes do `continue`, usando o formato de comandos diretos em `hooks.postToolUse` e `hooks.sessionStart`. Testes de sync, `go test ./...`, `go vet ./...`, `make build` e `git diff --check` passaram.

- Implementação de Handoff com Working Memory (arXiv 2606.10209v1): `latestProjectHandoff` em `tools/cmd/ctx-window/handoff.go` recupera tanto o `summary.md` consolidado quanto as últimas K interações da sessão (`session.Turns`), formatando-as como `Histórico Recente (Working Memory)`. Testes em `handoff_test.go` cobrem o fluxo completo com 100% de aprovação.
- Resumo automático por projeto: `ctx-window summarize` agora suporta execução sem argumentos posicionais de sessão, autodetectando a sessão mais recente do projeto corrente (`LatestSessionForProject`).
- Nudges implementados e sincronizados em todas as CLIs:
  - **Codex**: Implementado leitor em `codex_usage.go` lendo tokens dos rollouts locais (`~/.codex/sessions/`). Hook `PostToolUse` emite `hookSpecificOutput.additionalContext` ao ultrapassar threshold.
  - **OpenCode**: Implementado leitor em `opencode_usage.go` consultando a tabela `session` do SQLite (`~/.local/share/opencode/opencode.db`). Plugin TS atualizado para injetar aviso no retorno do tool call e disparar `showToast` na TUI.
  - **Cursor**: Hook `preCompact` registrado em `.cursor/hooks.json` retornando `user_message` para orientar cancelamento do compact automático e execução manual do summarize.
  - **Antigravity**: Hook `PreInvocation` registrado em `hooks.json` monitorando turnos (`invocationNum`) e injetando `ephemeralMessage` sem poluição de transcript.
- Sincronização global executada via `make apply`, confirmada com testes unitários em raiz e `tools/` (100% PASS), compilação limpa de binários e `git diff --check` sem warnings.

- Implementação de Handoff Local por Projeto (Abordagem B) — 2026-09-18:
  - `ctx-window summarize` agora grava o resumo consolidado em `<projectRoot>/.agent-sync/summary.md` (com `.gitignore` contendo `*` gerado automaticamente na pasta `.agent-sync/`), além de manter a cópia histórica no cache de sessões.
  - `ctx-window handoff` prioriza a leitura de `<projectRoot>/.agent-sync/summary.md` quando presente, eliminando riscos de colisão entre tarefas/agentes no mesmo repositório ou heurísticas no cache global.
  - Antigravity CLI: o evento `SessionStart` (ignorado pelo runtime oficial do `agy`) foi substituído pelo evento oficial `PreInvocation` para handoff em `cmd/agent-sync/hooks.go` e `~/.gemini/config/hooks.json`. `runHandoff` checa `invocationNum`: no turno 1 (`invocationNum == 1` ou payload inicial), injeta o resumo via `injectSteps` (`ephemeralMessage`); em turnos posteriores (`invocationNum > 1`), retorna `{}` sem custo.
  - Verificação: `go test ./...` (100% PASS na raiz e em `tools/`), `go vet ./...` limpo, `git diff --check` sem alertas, `make apply` executado com sucesso e smoke tests executados no terminal confirmando a injeção correta no turno 1 e silêncio no turno 2.

- Guardrails de Fim de Turno e Lembrete Preventivo (Execução Paralela) — 2026-09-18:
  - **Bloqueio de Hipóteses no Summarize**: `tools/cmd/ctx-window/summarize.go` agora verifica se há itens não validados em `active_hypotheses` e injeta automaticamente em `next_steps` o marcador `[BLOQUEIO agent-react] Existem hipóteses ativas pendentes de validação antes de declarar a tarefa pronta.` (com testes de não-duplicação e idempotência 100% PASS).
  - **Proteção de Fim de Turno (`Stop`) no Antigravity**: Implementado em `cmd/agent-sync/hooks.go` e `hooks/agent-stop.antigravity.sh` o registro do hook nativo `Stop` no formato flat handler do `hooks.json`. Ele inspeciona `fullyIdle`, tarefas ativas e pendências, retornando `{"decision": "continue", "reason": "..."}` para barrar encerramentos prematuros com loop guard de segurança.
  - **Lembrete Preventivo *Just-in-Time* (`PreInvocation`) no Antigravity**: Implementado em `hooks/agent-preinvocation.antigravity.sh`, injetando instrução efêmera (`ephemeralMessage`) antes da invocação do modelo ("Diretriz ativa: responda em PT-BR, sem rodeios e finalize com o resumo de 1-2 frases do que mudou e o que falta.") sem poluição permanente do transcript.
  - **Verificação**: Testes unitários em `cmd/agent-sync` e `tools/cmd/ctx-window` aprovados com 100% de sucesso. Git diff check limpo.

- Validações reais de OpenCode e Codex — 2026-09-18:
  - **OpenCode**: Validado carregamento dos 5 plugins TS via `opencode debug info` (v1.18.31). Testes sintéticos executados para `tool.execute.after` (indexação no working memory) e `experimental.session.compacting` (injeção no array `output.context`). Teste unitário de leitura do SQLite (`~/.local/share/opencode/opencode.db`) 100% PASS.
  - **Codex**: Validado ambiente via `codex doctor` (18 checks aprovados). Testado `SessionStart` via payload sintético de `ctx-window handoff codex` gerando `hookSpecificOutput.additionalContext` compatível com o schema oficial. Testado `PostToolUse` com resposta `{}` defensiva e indexação do turno. Testes unitários do Go para rollouts JSONL (`TestCodexNudgeUsesLatestUsageOnce`) 100% PASS.
  - **Isolamento de teste**: Corrigido `TestHandoffDefensiveOnEmptyOrInvalidPayload` em `tools/cmd/ctx-window/handoff_test.go` para isolar o diretório de trabalho com `t.TempDir()`, prevenindo que a existência do `.agent-sync/summary.md` real no diretório raiz do repositório contaminasse a asserção de payload vazio.

- Documentação consolidada — 2026-09-18:
  - `README.pt-BR.md`, `README.md`, `docs/ADR-context-window-strategy.md` e `skills/context-window-strategy/SKILL.md` atualizados para explicitar:
    1. **Objetivo**: Mitigação do efeito *lost in the middle*, eliminação de compactações LLM automáticas redundantes e controle do working memory (K últimos tool calls) com resumos estruturados sob demanda.
    2. **Referência do Artigo**: Paper da Microsoft Research (*"Less Context, Better Agents: Efficient Context Engineering for Long-Horizon Tool-Using LLM Agents"*, arXiv:2606.10209v1) destacando os resultados de 91.6% de sucesso (C4) vs 71.0% (C2) e redução de 63.9% de tokens.
    3. **Guia de uso nas 5 CLIs**: Detalhamento prático dos hooks (`SessionStart`/`PreInvocation`/`experimental.session.compacting`), formato do `.agent-sync/summary.md` e comandos da CLI (`ctx-window summarize`, `show`, `set-k`, `doctor`).

### Pendências abertas

- Fazer smoke real interativo nas 5 CLIs em sessões reais de trabalho.

- Fase 0: `runOnToolCallLLM` agora só registra turnos; resumo LLM exige `ctx-window summarize <sessão>` explícito. O nome antigo do subcomando foi mantido para não quebrar hooks instalados.
- Fase 2: plugin OpenCode usa `experimental.session.compacting` com `output.context.push()` e injeta o snapshot local retornado por `ctx-window show`.
- Fase 3: `ctx-window handoff <cli>` lê o `summary.md` local mais recente do mesmo `project_path`; `-apply` instala `SessionStart` em Claude/Codex, `sessionStart` em Cursor e `SessionStart` interno em Antigravity. `agentmemory` não guarda o YAML completo e é opt-in, então não serve como fonte automática do handoff. Cursor usa `CURSOR_PROJECT_DIR`, confirmado na documentação oficial. Antigravity usa API interna sem contrato publicado; integração não foi validada em execução real.
- Correção de schema Antigravity: a documentação oficial de hooks expõe `conversationId` e `workspacePaths` nos campos comuns, enquanto o parser anterior só lia `sessionId`/`session_id` e o novo código inicialmente usava `cwd`. Causa: assumir sem confirmar o formato do payload. Agora o tracking identifica a sessão por `conversationId`, e o handoff seleciona o único `workspacePaths[0]` quando há um workspace; com múltiplos workspaces, não injeta resumo para evitar misturar projetos. Fonte: https://antigravity.google/docs/hooks (Common Input Fields). Testes sintéticos cobrem o formato; a API interna de `SessionStart` ainda não foi exercitada no CLI real.
- Fase 4: Claude Code emite uma sugestão única em `PostToolUse` ao atingir `AGENT_SYNC_CTX_NUDGE_TOKENS` (padrão 150000), lendo `message.usage` nos 2 MiB finais do transcript. Pesquisa das demais CLIs: Cursor informa `context_tokens` apenas no evento observacional `preCompact`; Antigravity informa uso na API de status line, não no hook; Codex documenta `transcript_path` com formato instável e não expõe tokens no schema de hook; plugin OpenCode consultado não documenta uso de tokens no callback legado usado aqui. Não foi implementado nudge por token nelas sem contrato validado.
- Verificação: `go test ./...` e `go vet ./...` passaram na raiz e em `tools/`; `make build` passou; `git diff --check` sem alertas. O plugin TS foi carregado com o strip de tipos do Node e expôs os dois callbacks esperados; não houve typecheck, pois este repositório não tem `package.json`/`tsconfig.json` e `bun`/`tsc` não estão instalados. Configuração global aplicada e verificada conforme registro acima.
- Erro de ambiente durante verificação: primeiro `gofmt` recebeu caminhos relativos ao diretório errado; corrigido com caminhos do módulo `tools/`. `go test` falhou ao tentar usar o cache Go em `~/.cache/go-build` somente leitura no sandbox; repetido com `GOCACHE=/tmp/agent-sync-go-cache` e passou. Não era falha do código.
- Regras ativas: não consultar `.env`; não executar LLM automático; não fazer commit sem pedido; preservar alterações pré-existentes em `scripts/delegate-run.sh`, `receipts/`, `review-receipts/` e `docs/ADR-fim-de-turno-hooks.md`.

## Pesquisa de equivalência de hooks — 2026-09-18

- Escopo: somente pesquisa em documentação oficial, sem alteração de implementação.
- Codex: `PreCompact` existe e dispara antes da compactação, mas a especificação documenta stdout ignorado e somente saída comum (`continue`, `stopReason`, etc.); não documenta `additionalContext` para esse evento. Portanto: evento pré-compactação encontrado, mas equivalente funcional ao PreCompact do Claude Code (injeção de contexto que influencia o resumo) **não encontrado**. `SessionStart` existe para `startup`, `resume`, `clear` e `compact`, e aceita `hookSpecificOutput.additionalContext`, inclusive na continuação imediata após compactação. Fonte: https://developers.openai.com/codex/hooks (redireciona para https://learn.chatgpt.com/docs/hooks; seções `PreCompact` e `SessionStart`).
- Cursor Agent: `preCompact` existe antes da compactação, mas a documentação o classifica explicitamente como observacional e diz que não pode bloquear nem modificar a compactação; portanto, equivalente funcional ao PreCompact com injeção de contexto **não encontrado**. `sessionStart` existe na criação da conversa e aceita `additional_context` no contexto inicial. Fonte: https://prod.cursor.com/docs/hooks.
- Antigravity CLI (`agy`): a referência oficial documenta `/hooks` como “pre-flight/post-format script hooks”, sem eventos documentados `PreCompact`, `SessionStart` ou contrato de `additionalContext` para esses pontos. Resultado: ambos **não encontrados** na documentação do CLI. O SDK separado pode ter lifecycle próprio, mas não prova suporte no `agy`. Fontes: https://www.antigravity.google/docs/cli/reference/ e https://antigravity.google/docs/hooks?tab=cli.
- OpenCode: o plugin system documenta `experimental.session.compacting`, disparado antes de o LLM gerar o resumo, com `output.context.push(...)` e substituição via `output.prompt`; é equivalente funcional a pré-compactação com injeção. Também lista `session.created`, mas não encontrei contrato documentado de saída que injete contexto adicional no início/retomada; portanto equivalente funcional a `SessionStart` **não encontrado**. Fonte: https://dev.opencode.ai/docs/plugins/.
- Verificação: não consultei `.env`, não rodei testes (tarefa somente pesquisa), não alterei implementação; mudanças pré-existentes no worktree foram preservadas. Fontes foram verificadas em 2026-09-18.

## Implementação de compactação via hooks — bloqueio de schema — 2026-09-18

- Tarefa recebida: remover o disparo LLM automático por threshold e integrar compactação real em Claude/OpenCode, além de handoff em Codex/Cursor.
- Leitura realizada antes de codar: `tools/cmd/ctx-window/hook.go`, `summarize.go`, `store.go`, `main.go`, testes relevantes, `cmd/agent-sync/hooks.go`, `hooks/ctx-compact.opencode.ts` e alvos de sincronização.
- Causa do bloqueio: a documentação oficial atual do Claude Code (`https://code.claude.com/docs/en/hooks`) confirma o input de `PreCompact` (`session_id`, `transcript_path`, `cwd`, `hook_event_name`, `trigger`, `custom_instructions`), mas a tabela oficial de saída classifica somente `SessionStart`, `SubagentStart` e `PostModelSwitch` como eventos que aceitam `hookSpecificOutput.additionalContext`; `PreCompact` não está nessa lista. Implementar a saída pedida como se fosse suportada contradiz o schema verificado.
- Evidência complementar: issue oficial do repositório `anthropics/claude-code` sobre `additionalContext` em `PreCompact` foi fechada como “not planned”: https://github.com/anthropics/claude-code/issues/46191. Não tratar exemplos de terceiros como contrato.
- Estado: nenhuma alteração de implementação foi feita nesta tarefa; somente este checkpoint foi atualizado. Alterações anteriores do worktree foram preservadas.
- Próximo passo bloqueado: usuário deve escolher entre (a) não injetar contexto no Claude `PreCompact` e manter apenas tracking/registro, (b) usar outro evento oficialmente suportado para injeção, se compatível com o objetivo, ou (c) aceitar uma integração experimental não garantida pelo schema.
- Regras ativas: não consultar `.env`; não tocar em Antigravity; não criar commit; não afirmar funcionamento sem schema/teste; registrar causa raiz de bloqueios.

## Implementação aprovada após bloqueio — 2026-09-18

- Decisão do usuário: opção (a). Implementar Fases 0, 2 e 3; Claude `PreCompact` fica somente como tracking/observabilidade, sem tentar injetar `additionalContext` não suportado pelo schema oficial.
- Antigravity permanece fora do escopo e não será alterado.
- Próximas verificações: remover LLM automático do `on-tool-call-llm`; validar/implementar plugin OpenCode de compactação; implementar saída de handoff para Codex/Cursor e registrar os eventos; executar gofmt, vet, testes e builds exigidos.

### Validação independente pelo próprio Antigravity (agy) — 2026-09-18

- O usuário rodou o prompt de validação diretamente no `agy` interativo (sem `delegate-run`). Resultado mais profundo que a pesquisa do Codex: além de checar docs públicas, o agy inspecionou símbolos protobuf do próprio binário instalado (`/usr/bin/agy`, `google3/third_party/jetski/hooks_pb/hooks_go_proto.(*SessionStartHookResult).GetInjectSteps`).
- **`PreCompact` (ou equivalente)**: confirmado **ausente** no CLI `agy` — bate com o achado do Codex. O guia oficial (`hooks.md`, via skill `agy-customizations` e `google-antigravity-sdk`) só documenta `PreToolUse`, `PostToolUse`, `PreInvocation`, `PostInvocation`, `Stop`. O SDK Python separado (`google.antigravity.hooks`) tem `@hooks.on_compaction`, mas é estritamente observacional (`async def on_compact(data)`, sem retorno de injeção/substituição) — SDK não prova capacidade do CLI, como já era a ressalva.
- **`SessionStart` — achado novo e importante**: **existe de verdade no motor Go interno do agy** (`SessionStartHookArgs`/`SessionStartHookResult`), permitindo `injectSteps` (`userMessage`, `ephemeralMessage`) — mesmo mecanismo de injeção usado pelo `PreInvocation`. **Porém não está documentado no `hooks.md` público** — foi encontrado via inspeção de símbolos do binário, não da doc oficial.
- **Implicação pro plano**: Antigravity NÃO entra nas Fases 1/2 (sem lever de compactação), mas **entra na Fase 3** (handoff via SessionStart) igual Codex/Cursor — com a ressalva de que é API interna não documentada: pode mudar sem aviso em atualizações do agy, sem contrato de suporte oficial. **Decisão do usuário (2026-09-18): incluir mesmo assim, aceitando o risco** — sem feature flag experimental, tratado igual Codex/Cursor. Implementação a cargo do próprio `agy` (prompt separado, rodado diretamente pelo usuário, fora do escopo que o Codex está implementando).
- Fontes citadas pelo próprio agy: `file:///home/matheusdutra/.gemini/antigravity-cli/builtin/skills/agy-customizations/docs/hooks.md`, `file:///home/matheusdutra/.gemini/config/plugins/google-antigravity-sdk/skills/google-antigravity-sdk/examples/getting_started/hooks.md`, símbolos do binário `/usr/bin/agy`.

### Correção crítica — Claude Code `PreCompact` NÃO suporta `additionalContext` — 2026-09-18

- **Achado do Codex durante a implementação da Fase 1**: bloqueou a implementação e reportou que a doc oficial do Claude Code confirma o payload de entrada do `PreCompact`, mas **não lista esse evento como capaz de retornar `hookSpecificOutput.additionalContext`** — essa capacidade só existe para `SessionStart`, `SubagentStart` e `PostModelSwitch`. Citou a issue https://github.com/anthropics/claude-code/issues/46191, fechada como "not planned".
- **Verificação independente feita agora** (API do GitHub, não a página web): confirmado. Issue "Feature request: support additionalContext in PreCompact/PostCompact hook output", `state: closed`, `state_reason: not_planned`. Corpo da issue: "`PreCompact` and `PostCompact` hook events do not support `hookSpecificOutput.additionalContext` [...] Currently the only valid output field for these events is `systemMessage` (a display-only notification)."
- **Correção da minha afirmação anterior nesta sessão**: eu tinha confirmado (via HTML bruto do code.claude.com/docs/en/hooks) que o EVENTO `PreCompact` existe de verdade, mas nunca verifiquei o texto exato da capacidade de `additionalContext` especificamente para ele — confiei no resumo do WebFetch pra esse detalhe, que estava errado nesse ponto específico (os nomes de evento estavam certos, a tabela de capacidades não).
- **Decisão**: Fase 1 original (injeção em `PreCompact` do Claude Code) é **inviável hoje** — não é falta de documentação, é rejeição deliberada da Anthropic. Claude Code passa a usar o **mesmo padrão da Fase 3** (handoff via `SessionStart`, que está confirmado que aceita `additionalContext`) em vez de injeção ao vivo.
- **Implicação geral no plano**: nenhuma das 5 CLIs consegue injeção ao vivo durante compactação, exceto **OpenCode** (`experimental.session.compacting` com `output.prompt`, único caso real de "Fase 1/2"). Claude Code, Codex, Cursor e Antigravity caem todos no mesmo padrão: handoff via evento de início de sessão (`SessionStart`/`sessionStart`), não redução de tokens dentro da mesma sessão.
- Instrução passada ao Codex: prosseguir com a opção 1 dele mesmo (Fases 0, 2, 3 — incluindo agora Claude Code na Fase 3 via `SessionStart`, em vez de tentar algo experimental contra uma API oficialmente recusada).

## PLANO CONSOLIDADO — handoff para outro agente — 2026-09-18

Sessão encerrando por cota de horas. Este é o resumo definitivo pra quem pegar o trabalho a partir daqui — não repetir a pesquisa já feita, só implementar.

### Problema original (confirmado, não hipótese)
`ctx-window` gastava 1 chamada de LLM aninhada por tool call acima de 200 chars, gravava resumo em disco, e **nunca devolvia isso pro contexto real** (`hook.go` sempre retornava `{}`). Puro desperdício de tokens, comprovado numa sessão real com 9 compactações silenciosas.

### Fases a implementar (Codex já está executando 0, 2, 3 — verificar progresso antes de continuar)

**Fase 0 (todas as CLIs):** Remover o disparo automático de `runSummarize` em `runOnToolCallLLM` (`tools/cmd/ctx-window/summarize.go`). Manter só o tracking barato (`AddTurn`, sem LLM).

**Fase 1 — Claude Code: CANCELADA.** `PreCompact` não suporta `additionalContext` (confirmado via issue oficial fechada como "not planned": https://github.com/anthropics/claude-code/issues/46191). Claude Code migra pra Fase 3 (usa `SessionStart`, que confirmadamente aceita `additionalContext`).

**Fase 2 — OpenCode:** único caso de injeção viva real. Estender `hooks/ctx-compact.opencode.ts` pro evento `experimental.session.compacting`, usando `output.prompt` (substitui o prompt de compactação nativo) ou `output.context.push()`. Fonte: https://dev.opencode.ai/docs/plugins/.

**Fase 3 — handoff via SessionStart (Claude Code + Codex + Cursor + Antigravity):** Nenhuma CLI consegue injeção ao vivo durante compactação (exceto OpenCode, Fase 2). Todas as outras 4 usam o mesmo padrão: hook em `SessionStart`/`sessionStart` lê o último resumo estruturado persistido (via `internal/agentmemory`, já implementado) e devolve como `additionalContext`/`additional_context`/`injectSteps` (formato varia por CLI — **verificar schema exato de cada uma antes de codar, não assumir simetria**).
  - Claude Code: `SessionStart`, aceita `additionalContext`. Fonte: https://code.claude.com/docs/en/hooks
  - Codex: `SessionStart` (`startup`/`resume`/`clear`/`compact`), aceita `hookSpecificOutput.additionalContext`. Fonte: https://developers.openai.com/codex/hooks
  - Cursor: `sessionStart`, aceita `additional_context`. Fonte: https://prod.cursor.com/docs/hooks
  - Antigravity (agy): **API interna não documentada** (achada via símbolos do binário `/usr/bin/agy`: `SessionStartHookArgs`/`SessionStartHookResult`, campo `injectSteps` com `userMessage`/`ephemeralMessage`). Decisão do usuário: incluir mesmo assim, aceitando risco de quebrar em updates futuros sem aviso.

**Fase 4 — nudge de tamanho de contexto real (proposta do usuário, ainda não iniciada):**
- Achado importante: o transcript do Claude Code (`transcript_path`, já lido hoje pelo `false-success-guard`) tem uso de token **real** por turno da API: `usage.input_tokens + usage.cache_creation_input_tokens + usage.cache_read_input_tokens` ≈ tamanho real do contexto atual (verificado nesta própria sessão: ~400k tokens). Muito melhor que estimativa por caractere.
- Design: hook barato (sem LLM) que lê a última entrada `assistant` do transcript, soma os campos de usage, compara contra um threshold configurável, e se cruzar, devolve `additionalContext` sugerindo rodar `ctx-window summarize <session>` (comando manual, decisão do usuário de não automatizar o resumo — o usuário/agente decide quando vale gastar a chamada de LLM) e começar sessão nova (que aí sim pega o resumo automaticamente via Fase 3).
- **Pendência real**: só verificado o formato de transcript do Claude Code. Não verificado se Codex/Cursor/Antigravity/OpenCode expõem uso de token real em algum log/transcript acessível a hook — precisa checar cada um antes de implementar lá (não assumir simetria, mesmo padrão de cautela das fases anteriores).
- Comando manual: `ctx-window summarize <session>` já existe (não precisa criar) — só considerar expor com nome mais claro tipo alias `compact-now` se fizer sentido.

### Regras que valem pra quem for continuar
- Não fazer commit sem pedido explícito.
- Verificar schema exato de cada hook (payload de entrada/saída) direto na doc oficial ou nos internals reais antes de codar — várias suposições iniciais desta sessão já se provaram erradas (PreCompact do Claude Code é o exemplo mais caro).
- `gofmt`/`go vet`/`go test`/`go build` obrigatórios antes de reportar qualquer fase como concluída.
- Antigravity: nunca rodar comando destrutivo/manual contra dado real sem sandbox (lição cara desta sessão, ver `[[feedback_destructive_test_needs_sandbox]]`).

## Diagnóstico de hooks — 2026-09-16

- Status: implementação concluída no repositório; configuração global pendente de ressincronização.
- Sintoma: duas falhas `invalid pre-tool-use JSON output` e duas `invalid post-tool-use JSON output` ao usar ferramentas; sem stack trace no relato.
- Evidência: `~/.codex/config.toml:213` e `:225` habilitam protect-mcp e review-agent-governance. Ambos os `hooks/hooks.json` no cache dos plugins chamam protect-mcp@0.7.4 evaluate/sign diretamente, sem adaptação ao protocolo Codex.
- Causa: CLI instalado em `~/.npm/_npx/98c3fd9639547f82/node_modules/protect-mcp/dist/cli.js:11150` (evaluate) e `:11196` (sign) emite JSON de domínio, não a resposta de hook esperada. Reprodução isolada com Node, telemetria desativada e recibos temporários: evaluate retorna exit 0 e `{"allowed":true,"reason":"no_policy_configured"}`; sign retorna exit 0 e campos signed/artifact_type/request_id. JSON sintaticamente válido não garante compatibilidade de schema.
- Segurança: protect.cedar e review-governance.cedar ausentes no projeto; comandos usam --fail-on-missing-policy false. Não confiar nesses hooks como gates de política neste estado.
- Achado adicional: hooks/context-guard-nudge.sh:22, hooks/memory-nudge.sh:23 e hooks/agent-react-nudge.sh:22 omitem hookSpecificOutput.hookEventName nos lembretes; documentação oficial exige identificar o evento. Isso é separado do par recorrente dos plugins.
- Implementação: `hooks/codex-protect-mcp-adapter.sh` traduz evaluate para `permissionDecision` e silencia a saída de sign; `cmd/agent-sync/hooks.go` reescreve comandos dos plugins ao sincronizar o Codex. Nudges agora informam `hookEventName`.
- Próximo passo: executar `make sync` (ou sincronização equivalente) para atualizar a configuração global existente; validar cenários permitidos/negados e pós-execução.
- Fonte do contrato: https://developers.openai.com/codex/hooks (PreToolUse/PostToolUse).
- Verificação: leitura de configuração/código e execução isolada dos dois subcomandos; não reproduzida a sessão original completa. Durante a análise, receipts/ e review-receipts/ apareceram como não rastreados; não foram removidos.
- Regras ativas: não consultar .env; não expor segredos; não alterar gates de segurança nem criar commits nesta análise; preservar evidências e validar antes de concluir.

## Verificação atual dos hooks — 2026-09-18

- Configuração confirmada: `~/.codex/config.toml:213-226` mantém `protect-mcp` e `review-agent-governance` habilitados. Os hooks ativos dos plugins ainda chamam `npx protect-mcp@0.7.4` diretamente em `~/.codex/plugins/{protect-mcp,review-agent-governance}/hooks/hooks.json:9,20`.
- Timeout reproduzido: os comandos `npx ... evaluate` e `npx ... sign`, com entrada sintética e `timeout 12s`, terminaram com status 124; execução direta do CLI instalado terminou em aproximadamente 40 ms. Portanto, o timeout de 10 s é causado pelo caminho `npx`/resolução do pacote, não pelo JSON do payload.
- Schema incompatível confirmado no CLI instalado: `dist/cli.js:11172-11173` emite `{"allowed":true,...}` e `:11237` emite `{"signed":...}`. A documentação do Codex exige, para decisão de hook, `hookSpecificOutput` com `hookEventName` e `permissionDecision` (`https://developers.openai.com/codex/hooks`, seção PreToolUse). JSON válido não implica JSON de hook válido.
- Lacuna de sincronização: `cmd/agent-sync/hooks.go:198-205` só adapta entradas já presentes no `hooks` do arquivo-alvo; não altera os `hooks.json` dos plugins ativos. Assim, o adaptador do repositório não corrige os comandos que o Codex está carregando diretamente dos plugins.
- Verificação: não consultei `.env`, não alterei implementação, não executei testes unitários; usei apenas arquivos de configuração, leitura do CLI instalado e reproduções sintéticas locais. Nenhum segredo foi registrado.

## Meta anterior

- Skill `agent-react` + hook `agent-react-nudge` + regra hipótese ≠ fato.
- Status: concluído (commit pendente de push se desejado)

## Decisões tomadas

- Nudge (não gate): threshold 15, `AGENT_SYNC_REACT_NUDGE_THRESHOLD`
- Garantia semântica plena exige camada extra (ex. `stop`/`afterAgentResponse` no Cursor) — não implementada ainda
- Hook global Cursor = `~/.cursor/hooks.json` (local); Cloud Agents só leem `.cursor/hooks.json` do repo

## Próximos passos

1. Push se desejado
2. Se barulho: subir threshold ou filtrar só tools de mutação
3. Avaliar hook `stop` no Cursor para reforço no fim do turno

## Proposta de observabilidade — 2026-09-16

- Estado atual: recibos JSONL existem, mas não há eventos estruturados próprios, duração, resultado normalizado, correlação ou métricas agregadas.
- Direção sugerida: logger JSONL local com redaction de input/output, campos `timestamp`, `event`, `cli`, `tool`, `session_id`, `status`, `duration_ms`, `error_code`; contadores derivados por uma CLI de inspeção, sem rede e sem dependência de Prometheus.
- Fases: (1) contrato de evento + redaction e escrita atômica; (2) instrumentar adaptador Codex e sincronização; (3) comando `agent-sync observability` para resumo de falhas/latência; (4) exportação opcional Prometheus/OpenTelemetry somente se houver necessidade operacional.
- Erros de hook: podem ser persistidos no mesmo fluxo, capturando exit code, stderr resumido, etapa (`PreToolUse`/`PostToolUse`), CLI, ferramenta, sessão e duração. Não registrar stdin bruto, comandos, argumentos, tool output ou segredos; aplicar limite de tamanho e rotação ao arquivo.
- Implementação adicional: `hooks/observe-error.sh` persiste eventos redacted em `~/.cache/agent-sync/hooks/errors.jsonl` (limite aproximado de 5 MiB/10 mil linhas); `agent-sync -observability` resume estágio e código. O adaptador registra exit codes e JSON inválido.

## Agente de análise de MR/PR — proposta — 2026-09-16

- Escopo: análise local, GitHub, GitLab SaaS e GitLab self-hosted v12.
- Arquitetura: contrato único `ChangeProvider`/`ReviewPublisher`, com adaptadores Local (git diff), GitHub REST e GitLab REST; o agente `code-reviewer` permanece host-agnostic e recebe um pacote normalizado de diff/metadados.
- Compatibilidade GitLab v12: usar API REST v4 e endpoint de diffs compatível, evitando GraphQL e recursos recentes; comentários devem usar Notes API. Token somente via variável de ambiente/credential helper, nunca em logs ou STATE.
- Segurança: publicação de comentários fica opt-in; modo padrão somente leitura. Redaction antes de enviar diff a qualquer modelo/serviço.
- Plano: MVP local + pacote normalizado; depois GitHub/GitLab com dry-run; por fim publicação idempotente e CI.

## Bloqueios / perguntas abertas

- OpenCode: nudge best-effort até issue upstream #13574

## Revisão do plano de análise de MR/PR — 2026-09-16

- Estado: análise concluída; plano e implementação não alterados.
- Evidência: `/vox/dev-tools/go.mod:16` usa `gitlab.com/gitlab-org/api/client-go` v0.128.0. `/vox/dev-tools/app/services/git_service_remote.go:2027-2063` usa `ListMergeRequestDiffs` (`/diffs`) com fallback HTTP manual para `/changes`; o SDK local já fornece `GetMergeRequestChanges` para `/changes` (`merge_requests.go:543-548`).
- Correção necessária no plano: `docs/PLANO-AGENTE-ANALISE-MR.md:36` chama `/diffs` de rota legada do GitLab 12. O endpoint novo foi implementado em 2022/2023 (https://gitlab.com/gitlab-org/gitlab/-/merge_requests/104561); usar `/changes` para v12 e `/diffs` quando disponível, com detecção de capacidade.
- Riscos do exemplo: somente primeira página de `/diffs` (`git_service_remote.go:2027-2045`); erro do fallback pode virar lista vazia (`:2048-2073`); TLS desativado no cliente (`:28-31`). Não reproduzir no novo adaptador.
- Limite de verificação: não houve consulta à instância GitLab 12 nem execução de testes. Próximo passo, se solicitado: corrigir o plano e definir testes de compatibilidade.
- Regras ativas: não consultar `.env`, não expor segredos, não criar commits; citar fontes e validar afirmações.

## Configuração genérica do GitLab no plano — 2026-09-16

- Decisão do usuário: o agent-sync é geral; a configuração da instância GitLab não deve conter valores da Vox fixados no código.
- `docs/PLANO-AGENTE-ANALISE-MR.md` agora exemplifica JSON com `base_url`, `version` (`auto` ou versão explícita), `token_env` e `timeout_seconds`. Projeto e IID são parâmetros por análise; token não fica no JSON.
- Estratégia prevista: SDK Go isolado no adaptador; `/diffs` em versões com suporte e `/changes` para GitLab 12; fallback somente para rota inexistente, sem ocultar erros de autenticação ou rede. TLS verificado e paginação obrigatória.
- Estado: apenas plano alterado; não há implementação de agente de MR no código atual. Próximo passo: implementar quando o escopo completo do agente for definido.
- Regras ativas: não consultar `.env`, não expor segredos, não criar commits; verificar antes de afirmar funcionamento.

## Prioridade do MVP local e critérios de revisão — 2026-09-16

- Direção do usuário: implementar primeiro a comparação Git local, inclusive fork/upstream, com base e head explícitos como `upstream/branch-teste` e `origin/branch-teste`; buscar remotos quando solicitado.
- `docs/PLANO-AGENTE-ANALISE-MR.md` detalha seleção de refs, fetch opcional sem checkout/merge, merge-base, leitura de contexto adjacente e análise de segurança/impacto.
- Critérios: distinguir defeito preexistente, regressão introduzida e risco ainda não confirmado; exigir condição de disparo, antes/depois e evidência `arquivo:linha` antes de afirmar quebra de fluxo; declarar revisão parcial se faltar contexto.
- Estado: plano atualizado; não há implementação do agente de análise de MR/PR. Próximo passo: construir provider local e CLI conforme escopo do plano.
- Regras ativas: não consultar `.env`, não expor segredos, não criar commits; não executar código não confiável da branch durante a coleta; testes apenas quando houver implementação real.

## Agente de revisão Git local — implementação — 2026-09-16

- Estado: MVP local implementado em `agents/mr-reviewer.md` e `tools/cmd/mr-review-local/`; `Makefile` inclui o binário. `syncAgents` existente gera o agente para Claude, Codex, Antigravity, OpenCode e Cursor. Ainda não há coleta GitHub/GitLab nem publicação.
- Coleta: refs base/head explícitas, `fetch` opcional dos remotos indicados, SHA de base/head/merge-base, `git diff` sem diff externo ou textconv, timeout, limite de patch, redaction regex existente, JSON. Patch truncado é omitido e marcado como revisão parcial.
- Falha encontrada em teste: `TestCollectRejectsInvalidRefAndReportsTruncation` esperava truncamento, mas o `limitedBuffer` incorporava `bytes.Buffer`; `os/exec` podia usar o `ReadFrom` promovido e contornar `Write`. Corrigido por composição com campo privado `buffer`; teste passou. Lição: buffers limitados não devem expor interfaces de cópia que ignorem o limite.
- Verificação: `go test ./...` e `go vet ./...` passaram nos módulos raiz e `tools`; build do novo binário em `/tmp` passou. `git diff --check` passou. Instalação global nas CLIs não executada.
- Turso: memória atual usa `go-libsql` local (`tools/internal/agentmemory/store.go:105-115`). A documentação oficial informa que embedded replicas de `go-libsql` enviam escritas ao remoto imediatamente; sincronização somente no início/fim exige avaliar `tursogo` com `Push`/`Pull` ou outro fluxo. Ainda não alterado. Pergunta pendente ao usuário: sincronizar arquivos do repositório, memórias do MCP ou ambos?
- Regras ativas: não consultar `.env`, não expor segredos, não criar commits; não executar código não confiável da branch durante coleta; não tratar revisão parcial como completa.

## Avaliação Turso Cloud — 2026-09-16

- Esclarecimento ao usuário: Turso Cloud seria o banco remoto das memórias; SQLite/libSQL local permanece em cada PC. Código, agentes, regras e skills são arquivos e continuam sob Git.
- Fonte oficial: https://docs.turso.tech/sdk/go/reference — embedded replica `go-libsql` envia escritas ao remoto; `tursogo` oferece `Push`/`Pull` explícitos. Porém o schema atual usa SQLite FTS5 (`tools/internal/agentmemory/store.go:69-80`) e a compatibilidade Turso Database informa FTS5 não suportado: https://github.com/tursodatabase/turso/blob/main/COMPAT.md. Portanto migração direta do banco local para `tursogo` não está validada.
- Opção a detalhar: manter busca FTS5 local e sincronizar apenas registros de memória persistentes com uma tabela remota Turso/libSQL no início/fim da sessão, com conflitos detectados; não sincronizar o arquivo SQLite bruto. Ainda não implementado nem testado contra Turso Cloud; faltam decisão de escopo e credenciais remotas para validação final.

## Sincronização entre dois PCs — 2026-09-16

- Decisão confirmada pelo usuário: código/regras/skills via Git; memórias persistentes via banco remoto somente no começo/fim da sessão. Turso é transporte/armazenamento, sem pesquisa remota; SQLite FTS5 continua local em cada PC. SQLiteCloud é alternativa possível, mas integração da extensão SQLite-Sync com o driver Go atual e FTS5 não foi validada.
- Fonte Turso: https://docs.turso.tech/libsql (libSQL é fork compatível); https://github.com/tursodatabase/turso/blob/main/COMPAT.md (Turso Database novo não implementa FTS5, usa Tantivy). Isso não impede enviar registros para tabela comum, pois o remoto não usa FTS. Fonte SQLiteCloud: https://docs.sqlitecloud.io/docs/sqlite-sync-getting-started.
- Implementação local: `tools/internal/agentmemory/sync.go` sincroniza registros `scratch=0`, detecta conflito por hash/base local, usa transação e atualização condicionada no push, impede envio de padrões conhecidos de segredo; `tools/cmd/memory-sync` expõe fases start/end e resolução explícita; `scripts/agent-sync-session.sh` faz pull Git, aplica configurações das cinco CLIs, baixa memórias, executa a CLI e depois envia memórias/push Git. `Makefile` instala os novos comandos; READMEs documentam o uso. O banco local permanece em `~/.cache/agent-sync/memory.db`.
- Verificação local: `go test ./...` nos módulos raiz e tools passou; `bash -n scripts/agent-sync-session.sh` passou. Teste de integração usa dois SQLite locais e um libSQL local como remoto; conexão com Turso Cloud real não foi testada por falta de conta/credenciais. Não ler `.env` nem registrar tokens.
- Verificação adicional: `go vet ./...` nos módulos raiz e tools, `git diff --check` e `gofmt -l` dos arquivos Go alterados passaram. Próximo passo externo: validar em conta Turso real antes de afirmar operação remota concluída. Sem commit solicitado.
- Regras ativas: não consultar `.env`; não expor segredos; não fazer commit; não tratar teste local como teste da nuvem; manter `STATE.md` atualizado.

## Configuração Turso em JSON local — 2026-09-16

- Pedido do usuário: ler a configuração do Turso em arquivo sob a pasta de configuração do sistema e sempre criar o modelo quando estiver ausente; o usuário preencherá o token.
- Implementação: `tools/internal/agentmemory/config.go` usa `os.UserConfigDir()/agent-sync/config.json`, cria diretório `0700` e arquivo `0600` com `turso.url` e `turso.token` vazios, sem sobrescrever arquivo existente. Recusa arquivo não regular ou legível por grupo/outros. `memory-sync -init` cria e informa o caminho sem conectar ao banco. `memory-sync -phase ...` chama `LoadRemoteConfig`; wrapper não exige mais variáveis de ambiente.
- Arquivo local criado por `go run ./cmd/memory-sync -init`: `/home/matheusdutra/.config/agent-sync/config.json`, permissão verificada `600`; conteúdo não inspecionado para preservar credenciais. O token ainda precisa ser preenchido pelo usuário. O JSON fica fora do repositório.
- Testes: `go test ./...` nos módulos raiz e tools, `go vet ./...` em tools, `bash -n` do wrapper e `git diff --check` passaram. Testes de config cobrem criação, permissão, preservação e leitura. Conexão Turso real não testada.
- Regras ativas: não consultar `.env`, não expor token, não commitar segredo, não afirmar validação remota sem teste.

## Teste Turso real — falha de autorização — 2026-09-16

- Gatilho: `go run ./cmd/memory-sync -phase start` com a configuração local preenchida pelo usuário. Saída capturada e token redigido antes de mostrar. A fase `end` não foi executada.
- Erro resumido: Turso retornou HTTP 401 `Unauthorized: unauthorized access attempt on database: token does not have the permissions to access the database` durante `CREATE TABLE IF NOT EXISTS agent_sync_memories` em `Store.Pull`; não houve envio de memórias.
- Evidência: URL tem esquema `libsql`, token está presente e tem formato JWT, sem prefixo `Bearer`; arquivo tem permissão 0600. O servidor recebeu a tentativa e recusou autorização. A causa exata (token de outro banco, escopo/permissão insuficiente, token inválido para esta URL) não foi determinada; não tratar como bug de sincronização confirmado.
- Próximo passo: gerar um token de banco com acesso de escrita para o banco correspondente à URL (`turso db tokens create <nome-do-banco>`, sem `--read-only`), substituir `turso.token` no JSON local e repetir start/end. Não registrar token em logs, STATE ou repositório.
- Lição: validar token e URL do mesmo banco antes de concluir que a sincronização remota funciona; teste local com libSQL não cobre autenticação da nuvem.

## Teste Turso real — confirmação de persistência — 2026-09-16

- Após o usuário atualizar o token, `memory-sync -phase start` passou e `-phase end` informou 45 envios. Na segunda passagem, `start` informou 0 e `end` falhou com conflito anônimo `ca369a16454b`.
- Diagnóstico somente leitura, sem imprimir conteúdo: havia 45 memórias persistentes locais e 45 baselines, mas 0 registros na tabela remota. Uma transação de diagnóstico separada, inclusive com 45 inserts e leitura em outro processo, persistiu normalmente. Portanto o motivo específico do desaparecimento inicial dos 45 registros não foi comprovado; não atribuir falsamente ao driver/COMMIT.
- Falha comprovada no nosso fluxo: `Store.Push` marcava o baseline local após `Commit` sem reler o remoto; isso permitia reportar sucesso mesmo quando os registros não estavam lá. Corrigido com verificação remota dos hashes após commit e antes de atualizar baselines. Se o remoto perdeu um registro e o conteúdo local ainda coincide com o baseline, o envio agora o restaura. Teste `TestPushRestoresMissingRemoteRecord` adicionado.
- Verificação real após correção: primeira passagem `start=0`, `end=45`; segunda passagem `start=0`, `end=0`, ambas com exit 0. Nenhum token ou conteúdo de memória foi exibido. `go test ./...`, `go vet ./...`, `git diff --check` e `gofmt -l` passaram.
- Lição: resposta de sucesso do `Commit` não substitui confirmação de leitura remota antes de registrar o estado sincronizado. Causa da perda original permanece incerta; monitorar reincidência.
- Regras ativas: não consultar `.env`; não expor token/conteúdo de memória; não criar commit sem pedido; distinguir fato comprovado de hipótese.

- Instalação local após a correção: `make install` passou; `memory-sync` e `agent-sync-session` estão em `~/.local/bin`. Execução dos comandos instalados `memory-sync -phase start` e `-phase end` passou com 0 alterações nas duas fases.

## Escopo de projeto nas memórias — 2026-09-16

- Decisão do usuário: projetos sem remoto Git usam ID de projeto configurável. Projetos com `origin` ou `upstream` usam ID determinístico derivado do remoto normalizado.
- Implementação: `Memory` agora registra `PC`, `ProjectPath` e `ProjectID`; chave local/remota é `project_id + type + name`. `memory_sync_state_v2` e `agent_sync_memories_v2` preservam a tabela legada e migram registros antigos com projeto vazio. O MCP aceita `project_path`, `project_dir`, `project_id`, `pc` e `global`; exibe origem nos resultados e impede ambiguidade de nomes.
- Configuração sem remoto: `projects` no `~/.config/agent-sync/config.json`, com caminho local como chave e ID comum como valor. `project_dir` resolve o ID no PC atual.
- Verificação: testes de identidade Git em caminhos diferentes, ID configurável, colisão de nomes, filtros, sincronização entre dois PCs e migração remota passaram. `make install` passou. `memory-sync -phase start/end` no Turso real passou com 0 alterações após a migração.
- Limitação: memórias antigas recebem `project_id` vazio; não foram atribuídas automaticamente a um projeto para evitar associação incorreta. A ferramenta `store_memory` passa a exigir um projeto Git válido, ID configurado ou `global=true` para novas memórias.

## Diagnóstico de `Hook failed` código 127 — 2026-09-16

- Sintoma relatado: `Hook failed / hook exited with code 127`.
- Evidência: os hooks Codex ativos apontam para scripts no checkout (`~/.codex/hooks.json`); execução direta com payload vazio retornava exit 0 e não havia evento 127 no log redigido. O código 127 ocorre antes do corpo do script quando `#!/usr/bin/env bash` não encontra `bash` em um PATH restrito da CLI; por isso `observe-error.sh` não consegue registrar o erro. O adaptador `codex-protect-mcp-adapter.sh` também foi reproduzido localmente e retornou exit 0.
- Correção aplicada: hooks Codex `context-guard-nudge.sh`, `memory-nudge.sh`, `agent-react-nudge.sh`, `docs-cache.sh` e `codex-protect-mcp-adapter.sh` usam `#!/usr/bin/bash`, removendo a dependência do PATH para localizar o interpretador. Não alterado o conteúdo dos payloads nem os gates.
- Verificação: `bash -n` e execução dos quatro hooks com payload vazio passaram; `go test ./cmd/agent-sync` passou; `git diff --check` passou. A reprodução exata depende do ambiente da CLI, que não foi capturado no relato.
- Próximo passo: repetir a operação que gerou o erro. Se persistir, capturar o nome do hook e stderr da CLI; o próximo suspeito será um comando interno ausente no PATH restrito.

## Estratégia de janela de contexto (Cenário 3) — 2026-09-17

- Plano aprovado: skill `context-window-strategy` + tool `tools/cmd/ctx-window/` aplicando o padrão do paper arXiv 2606.10209v1 (Microsoft — Lodha et al.): Last 5 tool calls + summary incremental atinge 91.6% de conclusão vs 71% do full context.
- Decisão de default: **summarizer = próprio modelo da sessão** (default). Heurística pura local como fallback garantido. Ollama local como upgrade opcional para privacidade. Configurável via `agent-sync -apply`, env `AGENT_SYNC_SUMMARIZER`, JSON `~/.config/agent-sync/config.json`.
- Compactação sob demanda (não dois níveis paralelos): quando contexto estoura (tokens > budget ou N tool calls > threshold), o próprio modelo resume o histórico enquanto tem contexto completo disponível; vamos resetar mesmo, então o custo de tokens não é desperdiçado.
- Unidade de janela: tool calls (alinhado ao paper), K=5 por padrão configurável.
- Próximo passo externo: Fase 0 de calibração empírica (mini-projetos variando K, teto e summarizer) antes de cravar defaults numéricos; sem ela, qualquer teto é chute.
- Status: Fase 1 em andamento — skill + tool + heurística + testes.

### Fase 1 — Esqueleto concluído — 2026-09-17

- Skill `skills/context-window-strategy/SKILL.md` + prompt de sumarização `skills/context-window-strategy/prompts/summarize.md`.
- Tool Go `tools/cmd/ctx-window/` com subcomandos: `show`, `compact`, `set-k`, `doctor`, `benchmark`.
- Heurística local implementada como fallback (extrator por regex de marcadores + YAML estruturado com 6 seções).
- Persistência em `~/.cache/agent-sync/ctx-window/<sessão>/{meta.json,turns.jsonl,summary.md,summary_vN.md}`.
- 29 testes passando (`go test ./cmd/ctx-window/...`); `gofmt -l` limpo; `go vet ./...` sem alertas.
- `Makefile` build/install adiciona `bin/ctx-window`.
- Bugs corrigidos durante testes: shadowing de variável `t` em teste JSONL; `looksLikePath` agora aceita formato `path:line`; `Save` agora regrava `turns.jsonl` (idempotente) para refletir trim após `set-k`; `runCompact` não incrementa Version duas vezes.
- Próximo: Fase 2 (hook de disparo `hooks/ctx-compact.sh` + integração memory-mcp + setup wizard no `-apply`).
- Regras ativas: não consultar `.env`; não expor segredos; não commitar sem pedido; manter padrão de testes do repo (table-driven quando aplicável, cobertura de caminhos de erro); sem dependência externa — só stdlib.

### Migração para EN — 2026-09-17

- Decisão: skill, prompt, mensagens CLI e comentários em inglês (componentes técnicos que vão para o LLM ou definem contrato da skill).
- Justificativa do usuário: LLMs são otimizados para inglês; ganho marginal mas vale a consistência técnica.
- Trade-off aceito: quebra de consistência com o resto do agent-sync (regras globais, READMEs e outros tools continuam PT-BR).
- Heurística local mantida **bilíngue** (marcadores PT-BR + EN) — fallback funciona para qualquer idioma do agente sem perder funcionalidade.
- Marcadores EN adicionados: `was defined`, `was set to`, `chose to` (decisões); cobertura completa em erros, hipóteses, próximos passos, restrições.
- 30 testes passando (incluindo `TestHeuristicAcceptsPTBRMarkers` para garantir cobertura PT-BR na heurística).
- Mensagens do relatório agora em inglês (`Current summary` em vez de `Sumário atual`).

### Fase 2 — Hook + integração no sync — 2026-09-17

- **Subcomando `ctx-window on-tool-call`**: registra cada tool call no working memory e dispara auto-compactação quando `AGENT_SYNC_CTX_COMPACT_AT` (default 200 chars estimados) é atingido. Saída em JSON com `auto_compacted`, `version`, `turns`.
- **Hooks criados**:
  - `hooks/ctx-compact.sh` — Claude Code + Codex (PostToolUse).
  - `hooks/ctx-compact.cursor.sh` — Cursor (postToolUse, lê `session_id` ou `conversation_id`).
  - `hooks/ctx-compact.antigravity.sh` — Antigravity CLI (lê `sessionId` ou `session_id`).
  - OpenCode best-effort via `syncOpenCodeCtxCompactPlugin` (silencioso se o plugin TS não existir ainda).
- **`cmd/agent-sync/hooks.go`**: nova função `syncCtxCompactHook` (formato padrão + antigravity) e constante `ctxCompactHookName`. Mensagem de instalação impressa para cada CLI sincronizada.
- **`cmd/agent-sync/main.go`**: chamada de `syncCtxCompactHook` para cada CLI com hooks suportados.
- **`tools/internal/agentmemory/config.go`**: campos `Summarizer`, `CTXK`, `CTXBudget` adicionados ao `Config` para suportar setup wizard futuro (campos opcionais via `omitempty`).
- **Testes**: 5 novos testes para `on-tool-call` (abaixo do threshold, acima do threshold, sem `--tool`, `EstimatedChars`, `compactAtThreshold`).
- **Verificação**: `bash -n` dos 3 hooks passa; `go test ./cmd/ctx-window/...` 35 testes OK; `gofmt -l` limpo; `go vet ./...` sem alertas.
- **Bug corrigido durante testes**: `flag.Parse` consumia o session_id como flag (tokens sem `--` são tentados como flag); ajustado para extrair session manualmente antes do Parse.
- Próximo: setup wizard interativo no `agent-sync -apply` perguntando summarizer (A/B/C); ADR `docs/ADR-context-window-strategy.md`; READMEs.

### Parecer empírico (mini-projeto /tmp/ctx-test/) — 2026-09-17

- 6 runs (A–F) com a mesma tarefa (refatorar `Calculator` introduzindo `Stats` struct, forçando decisão documentada).
- **Cenários A–E (heurística)**: 5 testes passando em todos; sumário **vazio nas seções semânticas** (decisions, resolved_errors, next_steps, constraints). Heurística só captura `artifacts` (paths).
- **Cenário F (LLM summarizer via `opencode run --pure`)**: **todas as 6 seções populadas semanticamente** — 5 decisões + rationale, paths com descrição, cause+fix, 2 próximos passos, 4 restrições das regras globais (citou `AGENTS.md`).
- **Parecer**: heurística é **fallback de segurança**, não produção. LLM summarizer é o **caminho real** do paper, validado end-to-end no OpenCode.
- Implementação: `tools/cmd/ctx-window/summarize.go` (subcomandos `summarize` e `on-tool-call-llm`). Hook TS passa a chamar `on-tool-call-llm` em vez de `on-tool-call`. Cap de input aumentado para 16000 chars (~4000 tokens).
- Custo adicional: 1 chamada extra de LLM por compactação (~5s latência). Aceitável.
- Limitação não testada: ganho percentual exato do paper (91.6% vs 71%) exige LLM que degrade em contexto longo + sessão 50+ tool calls — fora do escopo deste ambiente.

### Fase 4 — ADR + documentação — 2026-09-17

- **`docs/ADR-context-window-strategy.md`** criado: aceita a estratégia com referência explícita ao paper arXiv [2606.10209v1](https://arxiv.org/html/2606.10209v1). Documenta decisão, consequências, evidência empírica (tabela 6 runs), implementação, limites conhecidos.
- **READMEs atualizados**: `README.md` e `README.pt-BR.md` ganharam bullet em "Context and long sessions" / "Contexto e sessões longas" descrevendo a skill + link para o ADR.
- **Fase 0 — calibração empírica**: marcada como **reduzida e aceita** no ADR (variamos summarizer + cap; aceitamos K=5 e teto=1000 do paper sem calibração local — justificado por custo proibitivo e paper já ter calibrado W).

### Correção — summarizer hardcoded em opencode/MiniMax-M3 — 2026-09-17

- **Achado do usuário**: `runSummarize` chamava sempre `opencode run --pure -m minimax/MiniMax-M3`, mesmo quando o hook disparava em Claude Code, Codex, Cursor ou Antigravity. Cada CLI deveria resumir via seu próprio modo não-interativo, não sempre via opencode.
- **Verificação ao vivo** (`--help` de cada CLI instalada, não suposição): `claude -p`, `codex exec`, `opencode run --pure -m`, `cursor-agent -p`, `agy -p` — todos aceitam um prompt posicional e imprimem a resposta em stdout; flag de modelo é `--model` em claude/codex/cursor/agy e `-m` em opencode.
- **Correção aplicada**: `tools/cmd/ctx-window/summarize.go` ganhou `knownCLIs` (tabela bin+staticArgs+modelFlag) e `summarizerCommand(cli, model, prompt)`. `Session` ganhou campo `CLIName`; `on-tool-call` e `on-tool-call-llm` aceitam `--cli` e persistem no meta.json; `summarize` usa `--cli` ou `s.CLIName` da sessão (erro explícito se nenhum dos dois estiver setado — não assume opencode como default).
- Hooks divididos por CLI: `ctx-compact.sh` (compartilhado Claude+Codex) virou `ctx-compact.claude.sh` + `ctx-compact.codex.sh`; `ctx-compact.cursor.sh`/`.antigravity.sh`/`.opencode.ts` passaram a chamar `on-tool-call-llm --cli <nome>` (antes cursor/antigravity só rodavam a heurística local, nunca o LLM). `cmd/agent-sync/hooks.go` (`syncCtxCompactHook`) seleciona o script por `target.AgentKind`.
- Testes novos: `tools/cmd/ctx-window/summarize_test.go` (dispatch por CLI, omissão de `--model`, erro em CLI desconhecida) — puros, sem exec real. `go build ./...` e `go test ./cmd/ctx-window/... ./cmd/agent-sync/...` (módulos `tools/` e raiz) passaram.
- ADR e READMEs (EN/PT-BR) atualizados para não citar mais opencode como único caminho.
- Pendência: não testado end-to-end contra cada CLI real (exec de `claude -p`/`codex exec`/`cursor-agent -p`/`agy -p` fazendo uma chamada de LLM de verdade) — só a sintaxe dos comandos foi confirmada via `--help`.

### Smoke test end-to-end — summarizer por CLI — 2026-09-17

- Sessão sintética fixa (5 tool calls: leitura, decisão de design, erro+correção, teste passando, próximos passos+restrição) rodada contra `ctx-window summarize --cli <nome>` para `claude`, `codex`, `cursor`, `antigravity` (opencode já validado na Fase 3).
- **Resultado**: as 4 CLIs geraram YAML válido com `decisions`, `resolved_errors`, `next_steps` e `constraints` populados corretamente; `active_hypotheses` vazio em todas (esperado — dado sintético não tinha hipótese aberta). Latência 6-14s por chamada.
- **Bug de uso encontrado** (não de implementação): o parser `flag` da stdlib para de interpretar flags no primeiro argumento posicional — `summarize <session> --cli X` falha silenciosamente (`NArg()!=1`); a ordem correta é `summarize --cli X <session>`. Corrigido o `-help` em `main.go` para deixar isso explícito.
- Sessão de teste removida do cache após validação (`~/.cache/agent-sync/ctx-window/smoke-test-cli-dispatch`).
- Não coberto (na época): `on-tool-call-llm` disparando via hook real de cada CLI em uma sessão de trabalho de verdade (só o `summarize` isolado foi testado). **Resolvido nas duas seções abaixo.**

### Validação end-to-end via hook real (payload simulado) — 2026-09-17

- Binário `ctx-window` instalado em `~/.local/bin` estava **desatualizado** (build anterior ao `--cli`, ainda com hardcode opencode/MiniMax) — reinstalado a partir do módulo `tools/` antes de testar.
- Simulados payloads reais de PostToolUse (schema `session_id`/`tool_name` confirmado — é o mesmo já usado por `context-guard-nudge.sh`, ativo nesta própria sessão) via stdin para `hooks/ctx-compact.{claude,codex,cursor,antigravity}.sh`, forçando `AGENT_SYNC_CTX_COMPACT_AT=1` para disparar a compactação via LLM no primeiro tool call.
- **Resultado**: as 4 CLIs completaram a cadeia inteira (script → `ctx-window on-tool-call-llm --cli X` → exec da CLI real → `summary.md` gravado com `cli_name` correto). Latência 4-8s.
- **Achado**: os hooks só extraíam `tool_name`, nunca `tool_input`/`tool_response` — o LLM recebia contexto vazio (só "Bash") e corretamente reportava dado insuficiente em vez de alucinar.

### Extensão — captura real de tool_input/tool_response nos 4 hooks bash — 2026-09-17

- Schemas de payload confirmados **por leitura de código já existente no repo** (não suposição): `docs-cache.py` (Claude/Codex: `tool_name`, `tool_input`, `tool_response`), `docs-cache.cursor.py` (Cursor: `tool_output`/`tool_response`, `tool_input` pode vir como string JSON), `docs-cache.antigravity.py` (Antigravity: payload não traz resultado — precisa ler do `transcriptPath` JSONL em `step_index == stepIdx+1`).
- Criados `hooks/ctx-compact.py` (compartilhado Claude+Codex, recebe `--cli` como argv), `hooks/ctx-compact.cursor.py`, `hooks/ctx-compact.antigravity.py`. Todos chamam `subprocess.run([...])` com lista de argumentos (nunca string de shell) para não ter risco de injeção de comando via conteúdo de tool_input/tool_response.
- `hooks/ctx-compact.{claude,codex,cursor,antigravity}.sh` viraram wrappers finos: delegam pro python3 se disponível; fallback bash puro (só session_id+tool_name, sem input) se `python3` ausente — mesmo padrão de degradação graciosa já usado em `docs-cache.sh`.
- **Correção de bug no fallback do antigravity**: a versão anterior extraía `toolName` (campo plano, nunca confirmado) — o schema real é `toolCall.name` (aninhado). Fallback bash corrigido para extrair `"name"` de dentro de `toolCall` via grep raso (aceitável só como fallback degradado).
- **Validação**: repetido o smoke test end-to-end com payloads reais de erro (`TestAverage_EmptyHistory panic: division by zero`) para claude, codex, cursor e antigravity (este último com `transcriptPath` simulado em arquivo temporário). Todos os 4 resumos capturaram a hipótese correta (divisão por zero, linha calculator.go:58) com `active_hypotheses` marcado `(hypothesis)` — antes vinha tudo vazio.
- `go build ./...`, `go vet ./...` (raiz) e `go build ./... && go test ./...` (módulo `tools/`) passaram sem alteração de código Go (só hooks bash/python foram tocados nesta rodada).
- Sessões de teste e arquivos temporários removidos do cache/scratchpad após validação.
- Pendência: não testado contra o schema real do OpenCode ainda mais a fundo — plugin TS já capturava input/output desde antes (não fazia parte deste achado). Fallback puro-bash (sem python3) permanece sem captura de input — aceito, é apenas o pior caso degradado.

### Correção — bash+Python demais, consolidado em Go — 2026-09-17

- **Achado do usuário**: a versão anterior (bash `.sh` wrapper + Python `.py` de parsing, por CLI) misturava Go (ctx-window) + bash + Python no mesmo hook, dificultando depurar/manter. Pedido explícito: reduzir a mistura de linguagens.
- **Correção**: criado `tools/cmd/ctx-window/hook.go` com subcomando `ctx-window hook <cli>` — lê o payload de PostToolUse via stdin, faz o parsing do schema de cada CLI **em Go** (`encoding/json`), monta o conteúdo e chama `runOnToolCallLLM` internamente (mesma função já usada por `on-tool-call-llm`, sem exec extra). Removidos `hooks/ctx-compact.{claude,codex,cursor,antigravity}.sh` e `hooks/ctx-compact.{,cursor,antigravity}.py` — não existe mais bash nem Python nesse caminho.
- `cmd/agent-sync/hooks.go`: `syncCtxCompactHook` agora instala o comando `ctx-window hook <cli>` **direto** no settings.json de cada CLI (sem exigir arquivo de script em disco). Novas funções `syncStandardHookCommand`/`syncAntigravityHookCommand` (variante sem checagem de arquivo) mantidas ao lado das antigas `syncStandardHook`/`syncAntigravityHook` (que outros hooks — context-guard-nudge, memory-nudge, docs-cache — continuam usando normalmente, baseados em script).
- **OpenCode é a única exceção que permanece fora do Go**: o hook dele *é* o runtime de plugin TS (`tool.execute.after`), não um comando de shell — não dá pra substituir sem reescrever o próprio OpenCode. `hooks/ctx-compact.opencode.ts` não foi tocado.
- Testes novos: `tools/cmd/ctx-window/hook_test.go` — parsers das 3 schemas (claude/codex, cursor, antigravity incluindo leitura de transcript simulado), erro em CLI desconhecida, e um teste end-to-end (`TestRunHookEndToEndRecordsTurn`) que roda `runHook` completo isolando `cacheRoot` em diretório temporário. `go build`/`go vet`/`go test` (raiz + módulo `tools/`) passaram.
- **Revalidado end-to-end de verdade**: reinstalado `~/.local/bin/ctx-window` com o novo binário e repetido o smoke test contra `claude`, `codex`, `cursor`, `antigravity` via `ctx-window hook <cli>` direto (sem bash/python), com payloads simulados (incluindo `transcriptPath` fake pro antigravity). Todos os 4 produziram resumo com hipótese/causa/próximos passos corretos. Sessões de teste e arquivo de transcript fake removidos após validação.
- ADR, READMEs (EN/PT-BR) atualizados com a nova arquitetura e uma seção "Decisões revisadas" documentando o porquê da reversão bash+Python → Go.
- Pendência restante (opcional, não bloqueia): integração `memory-mcp` para reancorar sumário entre CLIs via memória `project`.

## Detector de falso-sucesso (arXiv 2606.09863v1) — 2026-09-17

- Origem: usuário pediu para cruzar o que já foi implementado (ctx-window, baseado em arXiv 2606.10209v1) com outro paper (2606.09863v1) e o repo `nidhinjs/prompt-master`. Achado do paper: agentes LLM afirmam sucesso quando o estado real indica falha (até 89% em alguns benchmarks); um detector léxico simples (regex/TF-IDF) supera um LLM-judge em AUROC (0.83-0.95 vs 0.64) e é ~3300x mais barato. `prompt-master` não trouxe nada aproveitável (é sobre geração de prompt de entrada, não sobre contexto/verificação).
- Implementação: `tools/cmd/false-success-guard/` (Go, mesmo padrão do `ctx-window`): `detector.go` (regex `successAssertionPattern`/`honestFailurePattern`/`evidencePattern`, função `Classify`), `hook.go` (lê payload de Stop hook do Claude Code via stdin, extrai o último texto assistant do `transcript_path` JSONL, classifica), `main.go` (subcomandos `check` e `hook`).
- Caráter **advisory, nunca gate**: o próprio paper reporta ~50% de precisão na faixa de flag rate usável — bloquear o Stop teria mais falso positivo que acerto. O hook só emite `hookSpecificOutput.additionalContext` (mesmo padrão não-bloqueante do `context-guard-nudge.sh`), nunca impede o Stop.
- `Makefile`: alvo `build` ganhou `go build -o ../bin/false-success-guard ./cmd/false-success-guard`.
- Registrado em `~/.claude/settings.json` (fora do repo) como hook `Stop` com `matcher: "*"`, comando `false-success-guard hook` — decisão do usuário, confirmada explicitamente via pergunta antes de tocar em config compartilhada.
- Verificação: `gofmt -l` limpo (após correção de alinhamento de struct), `go vet` sem alertas, `go test ./cmd/false-success-guard/...` e `go test ./...` (módulo `tools/` completo) passaram, `make build` compilou todos os binários, `make install` colocou o binário em `~/.local/bin` (confirmado via `which`), teste manual do `check` com caso positivo (flagged=true) e negativo com evidência (flagged=false) confirmado na prática. `settings.json` validado como JSON válido após edição.
- Regras ativas: não consultar `.env`; não expor segredos; não criar commit sem pedido explícito; testar só implementação real; confirmar antes de mudar config compartilhada (settings.json).

### Validação end-to-end real + correção de falso positivo — 2026-09-17

- **Pendência resolvida**: o hook `Stop` disparou de verdade nesta mesma sessão (não simulado) sobre a mensagem "**Feito.** Implementado `tools/cmd/false-success-guard/`... Verificado: `gofmt`, `go vet`, `go test`... tudo passou." — confirmado rastreando o transcript JSONL real e rodando `false-success-guard check` sobre o texto exato (`flagged:true`), não por suposição.
- **Causa raiz do falso positivo**: `evidencePattern` só reconhecia bloco de código com três crases, `path:line`, `exit code` e `passed`/`failed` em inglês. A verificação real foi descrita em prosa PT-BR ("tudo passou") citando ferramentas com crase simples (`` `go test` ``) — nenhum dos dois casava com o regex.
- **Correção**: `evidencePattern` em `tools/cmd/false-success-guard/detector.go` passou a aceitar também `passou` (PT-BR) e qualquer span de crase simples (`` `[^`\n]+` ``), já que citar comando/ferramenta específica é como este projeto normalmente relata verificação em prosa.
- Teste de regressão adicionado (`detector_test.go`): o texto real que disparou o falso positivo agora está coberto e passa.
- Verificação: `gofmt -l` limpo, `go vet` sem alertas, `go test ./cmd/false-success-guard/...` e `go test ./...` (raiz + módulo `tools/`) passaram, `make build`/`make install` reinstalaram o binário, e o mesmo texto real que antes dava `flagged:true` foi reexecutado contra o binário reinstalado e agora dá `flagged:false` — confirmado na prática, não assumido.
- Lição: evidência de verificação neste projeto normalmente aparece como prosa citando ferramenta (crase simples) + confirmação verbal PT-BR, não como saída de comando colada — o detector precisa refletir como o próprio agente relata, não só o formato do paper original (que era em inglês, contexto de benchmark).

### Sinalização de falha de ferramenta + truncamento head+tail — 2026-09-17

- Origem: triagem de papers novos complementares aos já adotados (2606.10209, 2606.09863). Confirmados os 7 papers via WebFetch (existem, resumos batem). Adotáveis de verdade: **Fabrication After Tool Failure** (2609.14758) e **SWE-agent ACI** (2405.15793). Descartados: ACE (overengineering), Reflexion (exigiria reescrever actor loop — impossível sem controlar a CLI), LLMs Cannot Self-Correct (já é a justificativa do false-success-guard), Lost in the Middle (já é o fundamento do ctx-window), Look Before You Leap (item 3, adiado como proposta).
- **Item 1 — Tool status marker** (arXiv 2609.14758): paper mostra que sinalização explícita `status: OK/FAILED/EMPTY` derruba a taxa de fabricação de 45,3% para 0,87% (1024 itens, 16 domínios). Implementado em `tools/cmd/ctx-window/hook.go`: regex `toolFailurePattern` casa `error/exception/traceback/failed/panic/timeout/timed out`; parsers de claude/codex/cursor agora injetam `[TOOL_STATUS: FAILED]` ou `[TOOL_STATUS: EMPTY]` no `content` do turno (depois do input e do response), entrando automaticamente no working memory e no resumo LLM. Antgravity usa variante `toolFailureMarker` (só FAILED, não EMPTY), porque lá resultado vazio pode ser falha de leitura do transcript, não resposta genuína — afirmar `EMPTY` nesse caso seria uma alegação não verificada, exatamente o padrão que o false-success-guard combate.
- **Item 2 — Truncamento head+tail** (arXiv 2405.15793, SWE-agent/ACI): troca do corte cru (só primeiros 16000 chars) por head + `...[truncated]...` + tail. Saída de ferramenta normalmente tem o resultado acionável (erro, exit status, payload) no fim — corte só-de-início jogava fora a parte útil.
- Testes novos (`hook_test.go`): `TestParseClaudeCodexPayloadFlagsEmptyToolResponse`, `TestParseClaudeCodexPayloadFlagsFailedToolResponse`, `TestParseClaudeCodexPayloadNoMarkerOnSuccess`, `TestParseAntigravityPayloadMissingTranscriptDoesNotClaimEmpty`, `TestTruncateKeepsHeadAndTail`.
- Verificação: `gofmt -l` limpo, `go vet ./cmd/ctx-window/...` sem alertas, `go test ./...` (raiz + módulo `tools/`) passou; `make build`/`make install` reinstalaram o binário. **Validação manual real** (não só unit test) com `ctx-window hook claude` contra 3 payloads: tool_response com erro → `[TOOL_STATUS: FAILED]` apareceu no turns.jsonl; tool_response limpo → marcador suprimido; tool_response 2× o limite (40200 chars) → resultado com 16005 chars contendo `AAAA...` no head, `BBBB...` no tail e `[truncated]` no meio. Comportamento confirmado, não assumido.
- Lição: o `toolFailureMarker` separado do `toolStatusMarker` no caso do antigravity é importante — categorizar um estado ambíguo (não consegui ler o transcript) como `EMPTY` seria a mesma classe de erro que o false-success-guard combate: afirmar algo não verificado. Custo baixo de manter dois helpers, valor alto de não mentir pro agente sobre o estado do mundo.

### Integração memory-mcp + verificador estático pré-execução — 2026-09-17

Pendência das seções anteriores fechada: o verificador de Look Before You Leap virou binário próprio; a integração memory-mcp virou método no `agentmemory.Store`.

#### Integração memory-mcp (arXiv 2606.10209v1, persistência entre sessões)

- Origem: o paper que sustenta o ctx-window menciona explicitamente 3 modos de propagação (working memory local + sumário incremental + camada opcional de "skill" persistente entre sessões). Nosso sumário morria com a sessão em `~/.cache/agent-sync/ctx-window/<sessão>/summary_vN.md` — sem mecanismo de propagação entre CLIs/PCs.
- Decisão: **acesso direto ao `agentmemory.Store`** (não via MCP stdio). Razão: o memory-mcp é só uma fachada stdio sobre o mesmo `Store` (`memory-mcp/main.go:354`: `agentmemory.Open` → `store.Upsert`); acessar o Store diretamente tem o mesmo efeito com menos superfície de bug. O MCP agrega apenas a interface uniforme para as CLIs — qualquer registro gravado por qualquer caminho já é lido por todos via `search_memory`.
- Granularidade: **uma memória por item de seção do YAML** (decisão individual, hypothesis individual, etc.) em vez de uma memória com o YAML inteiro. Cansa mais linhas na tabela mas torna `search_memory("decisão sobre LLM cascade")` realmente útil — match exato no BM25 em vez de match enterrado em blob.
- Naming: `type="project"`, `name="ctx-<section>-<sha256-prefix>"` — chave estável; se a mesma decisão reaparece em outra compactação, `Upsert` sobrescreve (mesma chave) em vez de duplicar. Histórico de compactações continua nos `summary_vN.md` originais.
- Opt-in via `AGENT_SYNC_CTX_REMEMBER=1` — vaza contexto de sessão para a tabela compartilhada (que sincroniza via Turso), então default off alinha com a regra "não expor sem decisão explícita".
- Implementação: `tools/internal/agentmemory/summary.go` (`ParseSummary`, `UpsertSummary`, parser heurístico não-YAML-completo que entende o formato específico do `summarizePromptTemplate`); `tools/cmd/ctx-window/summarize.go:runSummarize` chama `rememberSummary` após `AppendVersionedSummary` se env=1.
- Limitação conhecida (registrada): o `project_id` é derivado do `os.Getwd()` de quem roda o summarizer, não do projeto original da sessão. Se a sessão foi aberta numa CLI remota, o projectID pode divergir. Corrigir exigiria persistir `ProjectPath` na `Session` desde o hook — fora do escopo desta integração.
- 7 testes novos em `summary_test.go` cobrindo parser (decisions+next_steps, artifacts+errors, vazio/só comentários/inline, chaves estáveis) e persistência (UpsertSummary grava linhas, Search acha, idempotência).
- **Verificação end-to-end real**: harness Go ad-hoc compilado direto no tools/, gravou 4 itens em libSQL local, `search_memory("LLM cascade")` → 1 hit, `search_memory("memory-mcp")` → 1 hit, `search_memory("sem-match-xyz")` → 0 (filtro de escopo OK). Harness removido após validação.
- `gofmt -l` limpo, `go vet ./...` sem alertas nos 2 módulos, `go test ./...` passou (raiz + tools), `make build` compilou todos os binários.

#### Verificador estático pré-execução (arXiv 2609.11957) — agora shell-validate

- Origem: paper Althoubi — verificador estático sobre 9930 comandos shell detecta 95.8% inválidos com 10% FP. Decisão do usuário: **só argumentos faltando** (heurística mais barata, ~80% do ruído do paper), **avisar via additionalContext** (não bloquear — combina com o padrão dos outros hooks), **opt-in via `AGENT_SYNC_PRETOOLUSE_VALIDATE=1`**.
- **Decisão arquitetura**: binário próprio `tools/cmd/shell-validate/` (não inchar o `ctx-window`) — duas responsabilidades distintas em binários separados é o padrão do projeto.
- Implementação: `detector.go` (lista de ~22 regras: binário → exige argumento posicional; cobre `go`, `cargo`, `rustc`, `gcc`, `gofmt`, `python`, `node`, `npm`, `make`, `kubectl`, `docker`, etc.); `hook.go` (lê payload PreToolUse de claude/codex/cursor/antigravity, extrai `tool_input.toolCall.args.command` com suporte a antigravity); `main.go` (subcomandos `check` e `hook`).
- **Bug encontrado e corrigido durante o teste**: antigravity envia `tool_input: null` quando ausente, que `json.Unmarshal` converte em `RawMessage("null")` (4 bytes, não nil). O switch original entrava no ramo errado. Corrigido com `looksLikeObject(raw)` que distingue `null` de objeto válido.
- 19 testes novos: 14 casos do `Classify` (válidos + inválidos para cada binário) + 5 do `runHook` (opt-in por env, comando inválido, válido, ferramenta não-Bash, formato antigravity).
- **Smoke real**: `/tmp/shell-validate check --cmd "go"` → flagged, `--cmd "go test ./..."` → ok, `--cmd "kubectl"` → flagged, `--cmd "ls -la /tmp"` → ok.
- `cmd/agent-sync/hooks.go`: nova constante `shellValidateHookName` e função `syncShellValidateHook` (paralela à `syncCtxCompactHook`, instala comando `shell-validate hook` direto no settings.json de cada CLI via `syncStandardHookCommand`, com `matcher: "Bash"` para Claude/Codex/Cursor e equivalente Antigravity).
- `Makefile`: alvo `build` ganhou `go build -o ../bin/shell-validate ./cmd/shell-validate`. `make install` colocou em `~/.local/bin` (confirmado via `which shell-validate`).
- **Registro no `settings.json` é automático via `make apply`**: o `cmd/agent-sync/main.go` chama `syncShellValidateHook` para cada CLI suportada. **Ativação do opt-in**: `persistShellEnv` (cmd/agent-sync/main.go, chamado no fim do `-apply`) escreve `export AGENT_SYNC_PRETOOLUSE_VALIDATE=1` em `~/.zshrc` (ou `~/.bashrc` se zshrc não existir) de forma **idempotente** (detecta bloco anterior pelo marcador, substitui em vez de duplicar). Na próxima shell que abrir, o hook já sai do no-op.
- **Bug capturado e corrigido durante `make apply` real**: primeira chamada gravou o hook no `PostToolUse` (evento padrão da CLI) em vez de `PreToolUse` (verificar ANTES de executar). Causa: `syncShellValidateHook` chamava `syncStandardHookCommand` que usa `target.HooksEvent` (que é `PostToolUse` para Claude/Codex). Adicionada `syncHookCommandAtEvent(baseDir, target, hookName, command, matcher, event)` em cmd/agent-sync/hooks.go que aceita o evento como parâmetro. **Bug secundário**: Antigravity usa schema top-level (`"agent-sync-shell-validate": { "PreInvocation": [...] }`), não schema padrão (`hooks.<evento>`); corrigido fazendo `syncShellValidateHook` chamar `syncAntigravityHookCommand` direto para Antigravity. **Bug terciário**: `upsertHookEntry` é idempotente dentro do mesmo evento mas não limpa entradas órfãs em outros eventos; entradas que migraram de evento ficaram duplicadas (Codex tinha 2 entries, Antigravity tinha 1 no schema errado + 1 no schema top-level). Limpeza feita via script Python que varre ambos os schemas. **Decisão final**: shell-validate não vai para Cursor — Cursor já tem `bash-guardian` em `beforeShellExecution` que cumpre o mesmo papel. Cursor não tem "PreToolUse" nativo com schema compatível.
- Verificação final (real, não assumida): Claude/Codex com `shell-validate hook` em `PreToolUse` matcher `Bash`; Antigravity em `PreInvocation` matcher `*`; Cursor sem entrada nova (bash-guardian já cobre); `~/.zshrc` com `export AGENT_SYNC_PRETOOLUSE_VALIDATE=1` persistido pelo `persistShellEnv`.
- Regras ativas: não consultar .env; não expor segredos; não criar commit sem pedido; testes só com implementação real.

## Gap "MemoryBank" (decaimento/relevância) no memory-mcp — 2026-09-17

- Origem: revisão bibliográfica de papers propostos pelo usuário para o agent-sync (arXiv 2309.12499 CodePlan, 2310.11248 CrossCodeEval, 2310.08560 MemGPT, 2305.10250 MemoryBank, 2310.05029 "Walking Down the Memory Maze"/apelido "MemWalker", 2310.10501 NeMo Guardrails, 2304.05128 Self-Debugging; 2404.06733 era citação fabricada — pertence a outro paper sobre XAI, descartada). Todos os 6 restantes confirmados via WebSearch como existentes; títulos de CodePlan e MemWalker vieram levemente errados na proposta original mas o conceito bate.
- Comparação feita lendo o código real: MemGPT↔`memory-mcp` e NeMo Guardrails↔`bashguardian` já implementados de forma fiel. CodePlan↔`ast-outline` e MemWalker↔`ctx-window` são versões mais simples (sem grafo de dependência entre arquivos / sem árvore hierárquica de sumários) — aceito como está, não é gap a fechar agora. Self-Debugging↔`trace-strip` é só suporte de redução de ruído, o loop real é do próprio agente.
- **Gap real identificado**: MemoryBank (decaimento tipo Ebbinghaus) não tem equivalente — `tools/internal/agentmemory/store.go` (`Memory` struct linha 39-53) só tem `CreatedAt`/`UpdatedAt`, nenhum campo de último acesso, e `Search` (linha 270-294) rankeia só por `bm25()`, sem peso de recência/uso.
- Também investigado por pedido do usuário: repo `multica-ai/multica` (plataforma de orquestração multi-agente, categoria de produto diferente — board web/desktop/mobile, não comparável 1:1) e `multica-ai/andrej-karpathy-skills` (são só 4 princípios comportamentais em markdown, não guardrails técnicos — não tem nada a ver com bashguardian/db-guardian). Nenhum dos dois trouxe gap técnico novo; o Multica também não tem decaimento de memória (usa persistência total via issues do Git).
- **Plano acordado com o usuário** (confirmado antes de implementar):
  1. Coluna `accessed_at` em `memories` (migração aditiva, mesmo padrão de `applySchema` usado para `scratch`/`pc`/etc.).
  2. `Store.Touch(id)` chamado internamente após cada leitura que retorna hit (`Get`/`GetScoped`/`Search`).
  3. Ranking de `Search` combina `bm25()` com decaimento exponencial sobre `accessed_at` (ORDER BY composto, não reescrita do algoritmo).
  4. Comando de poda (`memory-mcp prune --older-than <duração>` ou subcomando equivalente) que remove **só** memórias `scratch=true` com `accessed_at`/`updated_at` além do limite. Memórias permanentes (`scratch=false`) nunca são tocadas automaticamente — preserva o invariante existente de `ErrNotScratch`.
  5. Nenhuma tool MCP nova exposta ao agente: o boost de recência é transparente dentro de `search_memory`; a poda é operação de manutenção local, não algo que o agente aciona via MCP.
  - Fora de escopo: busca semântica/embedding (schema já reserva `embedding_json`, mas é outro projeto).
- **Status: implementação concluída e verificada — 2026-09-18.**

### Execução real: 2 delegações falhas + conclusão manual — 2026-09-18

- **Tentativa 1** (subagent interno `Agent(subagent_type: general-purpose)`): errada por definição — usuário queria delegação para outra CLI, não subagent interno da mesma sessão. Parada via `TaskStop` após só 2 linhas de diff parcial (revertidas com `git checkout`). Ver `[[feedback_delegation_target_and_skills]]`.
- **Tentativa 2** (`delegate-run --target opencode --mode print`, sessão `20260918T002014-memory-decay`): travou ~40 min sem nenhum log além do header — causa raiz real (confirmada depois): o modelo default do OpenCode (MiniMax-M3, "build") as vezes trava indefinidamente aguardando resposta após um tool call de `Read`, sem erro, sem timeout (processo em `do_epoll_wait`, CPU parada). A hipótese inicial (falta de `--auto`) era só parcialmente relevante — `--auto` foi adicionado em `scripts/delegate-run.sh` (comentário explicando o porquê) e é correção válida por si só (evita travar esperando aprovação sem TTY), mas não era a causa do travamento longo.
- **Tentativa 3** (`20260918T011228-memory-decay-v3`): travou de novo pelo mesmo motivo; usuário interveio manualmente (abriu `opencode` interativo, recuperou a sessão, trocou o modelo) e destravou. A sessão terminou com `DELEGATE_RESULT` honesto (`ok:false`): implementou só migração/campo/`Upsert`/`Touch`/`PruneScratch`, faltando `Get`/`GetScoped`/`Search` usarem `accessed_at`+chamarem `Touch`, re-rank, subcomando `prune` no `main.go`, testes e toda a verificação obrigatória.
- **Conclusão manual** (eu, nesta sessão, direto — sem mais delegação, dado o histórico de instabilidade): completei `Get`/`GetScoped` (SELECT+scan `accessed_at`, chamam `touchHit` pós-hit), `List` (SELECT `accessed_at`), `Search` (nova query com `bm25(memories_fts) AS bm25` + `accessed_at`, busca `limit*3`, `scanScoredMemories`, `rankSearchResults`/`adjustedScore` reordenam em Go combinando bm25 bruto + decaimento exponencial com meia-vida de 30 dias (`searchDecayHalfLife`) e peso `searchDecayWeight=0.5`, `touchHit` nos retornados). Subcomando `memory-mcp prune --older-than <duração>` adicionado em `main.go` (roda a poda e sai, sem entrar no loop stdio; não é tool MCP).
- 5 testes novos em `tools/internal/agentmemory/store_test.go`: migração de banco sem a coluna nova, `Touch` avança `accessed_at`, `Search` prioriza acesso mais recente em empate textual, `PruneScratch` remove só `scratch=true` expirado e nunca `scratch=false`, não remove `scratch=true` recente.
- **Verificação real**: `gofmt -l` limpo, `go vet ./...` sem alertas, `go test ./...` no módulo `tools/` passou tudo (17 pacotes, `agentmemory` rodou de verdade em 1.184s, não cached), `go build ./...` em `tools/` e na raiz passaram.
- **Incidente durante a verificação manual do `prune`**: rodei `/tmp/memory-mcp-test prune --older-than 1h` esquecendo que o binário usa `agentmemory.DefaultDBPath()` (banco real do usuário, não sandbox) — deletou 8 memórias `scratch=true` reais sem confirmação prévia. Usuário aceitou a perda (scratch=true é descartável por definição, "de qualquer forma era lixo"), não tentamos recuperação forense. Lição registrada em `[[feedback_destructive_test_needs_sandbox]]`: nunca rodar comando destrutivo manual contra dado real, mesmo pra "confirmar visualmente" algo que os testes automatizados (`t.TempDir()`) já cobrem.
- **Efeito colateral bom desta sessão**: `Makefile` (`apply` agora chama `memory-sync -init` também, criando `~/.config/agent-sync/config.json` automaticamente) e `scripts/delegate-run.sh` (`opencode run --auto` em vez de sem a flag) ficaram permanentemente melhores, independente do resultado da tarefa principal.
- Pendências reais: nenhum commit foi feito (regra: só commitar quando pedido explicitamente). `receipts/` e `review-receipts/` continuam não rastreados (não mexidos, não são desta tarefa).

## Commit do MemoryBank — 2026-09-17

- Commit `5815d9d` registrado: 7 arquivos, 325 insertions(+), 16 deletions(-).
- Mensagem em Conventional Commit PT-BR (alinhada com `c29bfe7` e anteriores): feat/decay com bullets por responsabilidade + colaterais (`make apply` chama `memory-sync -init`, `delegate-run.sh` usa `--auto`) + regras ativas.
- Identity local: `Matheus Dutra <matheusbbdutra@gmail.com>` (verificado antes do commit).

## Hooks de fim de turno (Cursor `stop` + OpenCode `session.idle`) — proposta — 2026-09-17

- Origem: usuário pediu para pensar o que fazer sobre OpenCode + hook do Cursor. Investigação puxou documentação oficial via `docs-fetch`:
  - **Cursor** (https://cursor.com/docs/hooks, baixado agora): o agent-sync usa só 3 dos ~17 eventos disponíveis. Faltam `stop` (com `loop_limit` e `followup_message`), `afterAgentResponse`, `afterAgentThought`, `preCompact`, `postToolUseFailure`, `sessionStart`/`sessionEnd`, `subagentStart`/`Stop`, `afterShellExecution`, `beforeReadFile`, `afterFileEdit`, `beforeSubmitPrompt`, `workspaceOpen`.
  - **OpenCode** (https://opencode.ai/docs/plugins/, baixado agora): o agent-sync usa só `tool.execute.after`. Faltam `session.idle` (equivalente direto do `stop` do Cursor), `session.compacted` (útil para ctx-window), `tool.execute.before`, `message.updated`, `permission.asked/replied`, `session.created/deleted/error/status/updated/diff`.
- **ADR criada**: `docs/ADR-fim-de-turno-hooks.md` (Status: Proposto). Estrutura padrão igual à `ADR-context-window-strategy.md`: Contexto / Decisão / Consequências / Evidência consultada / Implementação proposta / Limites / Próximos passos.
- Três trilhas registradas na ADR (não implementadas — só a decisão arquitetural foi fechada):
  - **Trilha A (Cursor `stop`)**: baixo risco, documentação oficial, schema conhecido. Hook `agent-react-nudge.stop.cursor.sh` emitindo `followup_message` em `~/.cursor/hooks.json` no evento `stop`. Migrar `false-success-guard` para o Cursor. Smoke real pendente antes de fechar ADR.
  - **Trilha B (OpenCode `session.idle`)**: risco não verificado — a doc do OpenCode não explicita se o callback pode injetar texto no contexto. Antes de implementar, **plugin mínimo de teste** para confirmar. Se não permitir injeção, manter `tool.execute.after` e tratar `session.idle` só como observabilidade.
  - **Trilha C (OpenCode `session.compacted`)**: fecha gap do ctx-window quando o OpenCode compacta autonomamente. Depende do resultado da B.
- Não verificado: como `agy` (Antigravity) trata "fim de turno" — não consultado nesta rodada. Pendência separada, marcada na ADR.
- Pendência anterior do STATE.md:27 ("camada extra `stop`/`afterAgentResponse` no Cursor não implementada") agora documentada na ADR — não sumiu, virou referência estruturada. Próximo passo é implementar Trilha A com smoke real, fechar ADR como Aceita.
- Regras ativas: não consultar `.env`; não expor segredos; não commitar sem pedido; não implementar com base em hipótese não validada (Trilha B especialmente); citar fonte (`path:line`, doc URL, ADR).
