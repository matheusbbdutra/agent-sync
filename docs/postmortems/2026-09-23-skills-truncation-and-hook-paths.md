# Postmortem: 54 skills truncadas + 6 hooks com paths errados

**Data**: 2026-09-23 | **Severidade**: SEV2 | **Duração**: ~7 min (16:55 → 17:02 local)
**Autores**: [@matheus_dutra] | **Status**: Concluído

## Resumo Executivo

Dois bugs correlatos quebraram o carregamento de skills e hooks do Codex no `agent-sync` em 2026-09-23:

1. **54 de 54 arquivos `skills/*/SKILL.md`** foram truncados para 0 bytes às 16:59:59 (-03:00). O Codex registrou 54 erros `failed to load skill ... missing YAML frontmatter delimited by ---` na inicialização + 1 warning do plugin OpenAI-builtin `ngs-analysis` (`interface.defaultPrompt[0]` com 313 chars, codex aceita ≤ 128), totalizando **55 warnings observados pelo usuário**.

2. **6 entradas em `~/.codex/hooks.json`** apontavam para `/home/matheus_dutra/hooks/<script>` (inexistente) em vez de `/home/matheus_dutra/Projects/agent-sync/hooks/<script>`. Codex retornou exit 127 ("command not found") em cada invocação — 3 hooks que apareceram para o usuário.

Ambos foram diagnosticados, corrigidos e corrigidos na raiz (vendor atômico). Nenhum dado de usuário foi perdido; skills foram restauradas do último commit git.

## Linha do Tempo (UTC-03:00)

- **16:55** — Codex inicializa nova sessão; lê `~/.codex/hooks.json` (gerado por último `agent-sync` de fora do repo — `baseDir` resolveu para `/home/matheus_dutra/` por fallback silencioso, vide Análise). Carrega 54 skills; encontra 2 com YAML inválido (`database-migrations-sql-migrations`, `php-pro`); registra 2 ERRORs no `logs_2.sqlite`.
- **16:56:10** — Codex termina de carregar plugins; emite 8 WARNs do plugin OpenAI `ngs-analysis` (`defaultPrompt[0]` > 128 chars) + WARN de modelo desconhecido `gpt-5.6-luna`.
- **16:58** — Usuário reporta 3 "Hook failed ... exit code 127". Investigação revela `~/.codex/hooks.json` tem 6 entradas com `/home/matheus_dutra/hooks/...` (path inexistente; o real é `/home/matheus_dutra/Projects/agent-sync/hooks/...`).
- **16:58-16:59** — Corrigidos os 2 YAMLs (database-migrations, php-pro) via patch direto no frontmatter.
- **16:59:59** — **Algum processo zera todos os 54 `skills/*/SKILL.md`** (timestamp idêntico em todos). Suspeito: invocação de `agent-sync -vendor` com `cwd=/home/matheus_dutra/` e source dir `~/.gemini/config/plugins/antigravity-skills-manager/skills` (apontado pelo `manifest.json`, mas **inexistente em produção**). Como `vendorSkill` deletava `dst` antes de validar source, dst virou conteúdo vazio.
- **17:00:15** — `agent-sync -target codex` é executado de dentro do repo; regenera `~/.codex/hooks.json` com paths corretos (`/home/matheus_dutra/Projects/agent-sync/hooks/...`). 20 ocorrências corretas, 0 erradas.
- **17:01** — Codex emite o log `{"text": "Analise agora temos 55 warnings..."}` — usuário reporta.
- **17:02** — `agent-sync doctor` revela `skills_lint ❌ FAIL: 54 erro(s) em 54 skills` (cada um: "SKILL.md sem frontmatter YAML"). Investigação identifica truncamento. `git checkout HEAD -- skills/` restaura os 76 arquivos modificados. 2 correções YAML são reaplicadas.
- **17:11** — Commit `714e0e7` aplica correção sistêmica do vendor (atomicidade + precheck).

## Análise de Causa Raiz

### Causa Imediata

Dois vetores independentes:

**Vetor A — paths errados em `~/.codex/hooks.json`**:
- `internal/hooks/core_wrap_hook.go:13` faz `filepath.Join(baseDir, "hooks", "wrap-hook.sh")`.
- `baseDir` é resolvido por `internal/pathutil/pathutil.go:36` (`ResolveBaseDir`), que tem **fallback silencioso**: se `FindBaseDir` (busca recursiva por `rules/global-rules.md`) falha, retorna `cwd` em vez de erro.
- Resultado: `agent-sync` executado de `/home/matheus_dutra/` (fora do repo) registra hooks em `/home/matheus_dutra/hooks/...`. Quando Codex tenta executar, bash retorna exit 127.

**Vetor B — 54 SKILL.md truncados**:
- `internal/apply/vendor.go:116` (`vendorSkill`) faz:
  1. `os.ReadFile(src/SKILL.md)` — lê source
  2. `validateSkill(...)` — valida frontmatter
  3. **`os.RemoveAll(dst)` — deleta destino (skill no projeto)**
  4. `copyDir(src, dst)` — copia source → destino
- Ordem é **destrutiva antes de confirmar integridade**: se source tem `SKILL.md` válido mas `copyDir` falha por subdiretório quebrado, ou se source tem SKILL.md parcial, dst fica em estado inconsistente.
- `runVendor` no `internal/apply/vendor.go:135` lê `skillsDir` do manifest (`~/.gemini/config/plugins/antigravity-skills-manager/skills`). Esse path **não existe** no ambiente (`~/.gemini/config/` não existe — só há `~/.gemini/antigravity-cli/`). Em algum momento, um `agent-sync -vendor` rodou com esse source dir inexistente/vazio, e cada `vendorSkill` sobrescreveu a skill do projeto com `SKILL.md` vazio.

### 5 Porquês (Vetor B)

1. **Por que 54 skills ficaram vazias?** → `vendorSkill` chamou `os.RemoveAll(dst)` e depois escreveu `SKILL.md` vazio (vindo de source sem conteúdo válido).
2. **Por que o source estava vazio?** → O `skillsDir` apontado pelo `manifest.json` (`~/.gemini/config/plugins/antigravity-skills-manager/skills`) não existia no ambiente.
3. **Por que não foi detectado antes?** → `runVendor` só abortava quando `os.Stat(sourceDir)` falhava **inteiramente**. Se o diretório existia mas estava vazio (ou sem SKILL.md), passava pelo Stat e seguia iterando skill por skill — cada uma falhava silenciosamente no `validateSkill`.
4. **Por que o `RemoveAll(dst)` é executado antes de validar source?** → Falta de atomicidade. O padrão atual é "deletar depois copiar"; o padrão correto seria "copiar para temp + rename atômico" — garantindo que dst só é modificado se o temp está íntegro.
5. **Por que essa classe de bug não tinha teste?** → Não havia `vendor_test.go` em `internal/apply/`. Os testes existentes cobriam o caminho feliz (`vendorSkill_SuccessPath` implícito em `TestApplyDryRunDoesNotWriteDisk`) mas não o cenário de source quebrado.

### Fatores Contribuintes

- **Dois vetores do mesmo bug**: `ResolveBaseDir` com fallback silencioso (causa raiz do Vetor A) e `vendorSkill` destrutivo (causa raiz do Vetor B). Ambos permitiram que operações rodassem de fora do repo sem aviso.
- **Sessão de codex sem mecanismo de auto-failover**: ao encontrar SKILL.md vazio, codex apenas loga o erro e segue — não há recovery automático nem alerta visível para o usuário.
- **Observabilidade de hooks limitada**: `observe-error.sh` captura exit != 0, mas os hooks de nudge sempre retornam `{}` (vide "O que Falhou") — tornando invisível ao usuário que o nudge foi disparado.

## O que Funcionou vs O que Falhou

### Funcionou

- **Detecção rápida via `agent-sync doctor`** — o check `skills_lint` flagou os 54 erros imediatamente, reduzindo o tempo de diagnóstico de horas para minutos.
- **`git checkout HEAD -- skills/`** — restaurou os 76 arquivos modificados sem perda (porque as correções YAML foram reaplicáveis deterministicamente).
- **`yaml.safe_load` + varredura em massa** — confirmou que apenas 2 dos 54 skills tinham YAML quebrado, descartando bug sistêmico no frontmatter.
- **Estrutura de logs do codex** — `logs_2.sqlite` permitiu query SQL para identificar o vetor exato (path → ERROR message) sem grep em logs brutos.
- **Hooks que retornam `additionalContext`** (principles-inject, ctx-window handoff) — funcionaram durante todo o incidente.

### Falhou

- **Hooks de nudge são mudos**: `agent-react-nudge.sh`, `context-guard-nudge.sh`, `memory-nudge.sh` retornam SEMPRE `{}` por design (vide comentário inline: "ate descobrirmos schema que injete contexto adicional sem bloquear"). O codex nunca vê os lembretes de validação de hipóteses, atualização de STATE.md, ou consolidação de memória — apenas o `memory-mcp` recebe o espelhamento. **24% dos hooks estão efetivamente inertes para o usuário.**
- **`false-success-guard hook` (Stop)**: a versão `check` detecta alegações sem evidência (`{"flagged":true,...}`), mas a versão `hook` chamada pelo Codex retorna `{}` mesmo em payload com `"Tudo corrigido, funciona perfeitamente"`. Toda a feature do guard fica invisível em Stop.
- **`ResolveBaseDir` fallback silencioso**: trocar `cwd` por erro tornaria o bug do Vetor A imediatamente diagnosticável na primeira execução.
- **`runVendor` sem precheck de integridade**: passou pelo `os.Stat(sourceDir)` mesmo quando o diretório existia mas estava vazio, distribuindo o estrago skill por skill em vez de falhar cedo.
- **`vendorSkill` destrutivo**: `os.RemoveAll(dst)` antes do `copyDir` cria janela de inconsistência.
- **Nudge threshold muito alto** (15, 25, 40 chamadas): em sessões curtas (como a de hoje, ~6 chamadas de ferramenta), nenhum nudge chegou a disparar.

## Ações Preventivas (Action Items)

| Prioridade | Ação | Responsável | Prazo | Prevenção / Detecção |
|------------|------|-------------|-------|----------------------|
| **P0** | `vendorSkill` atômico (copy-to-temp + rename) | @matheus_dutra | ✅ 2026-09-23 (commit 714e0e7) | Prevenção |
| **P0** | `runVendor` precheck upfront (0/N skills válidas → abort) | @matheus_dutra | ✅ 2026-09-23 (commit 714e0e7) | Prevenção |
| **P0** | Cobertura de testes para vendor (8 cenários em `vendor_test.go`) | @matheus_dutra | ✅ 2026-09-23 (commit 714e0e7) | Prevenção |
| **P0** | Restaurar 54 SKILL.md via `git checkout` + reaplicar 2 patches YAML | @matheus_dutra | ✅ 2026-09-23 | Mitigação |
| **P1** | Investigar schema de `additionalContext` para PostToolUse que não bloqueia tool call | @matheus_dutra | 2026-09-30 | Detecção (reabilitar nudges visíveis) |
| **P1** | Investigar por que `false-success-guard hook` (Stop) retorna `{}` mesmo com payload suspeito | @matheus_dutra | 2026-09-30 | Detecção (reabilitar guard em Stop) |
| **P1** | Reduzir `THRESHOLD` dos nudges para ≤10 ou usar contador cumulativo cross-sessão | @matheus_dutra | 2026-09-30 | Detecção |
| **P2** | `ResolveBaseDir`: falhar com erro claro quando `rules/global-rules.md` não encontrado (em vez de fallback silencioso para `cwd`) | @matheus_dutra | ✅ 2026-09-23 (commit em `fix/hook-script-path-validation`, issue #4) | Prevenção (Vetor A) |
| **P2** | `agent-sync doctor`: alertar quando `~/.gemini/config/plugins/antigravity-skills-manager/skills` não existe (manifest órfão) | @matheus_dutra | 2026-10-07 | Detecção |
| **P2** | Hook `SessionStart`: validar que 100% dos SKILL.md têm frontmatter e reportar antes do codex engasgar | @matheus_dutra | 2026-10-07 | Detecção |
| **P3** | Avaliar migração do nudge para `UserInput` ephemeral em vez de `injectSteps` (que bloqueia) | @matheus_dutra | 2026-10-14 | Detecção |

## Apêndice: Análise Empírica dos Hooks (2026-09-23)

24 hooks configurados em `~/.codex/hooks.json`. Validação por execução direta:

| Hook | Stage | Matcher | Output observado | Status |
|------|-------|---------|------------------|--------|
| `principles-inject.pretooluse.sh` | PreToolUse | `*` | `additionalContext` com regras | ✅ OK |
| `ctx-window handoff codex` | SessionStart | `.*` | `additionalContext` com summary anterior | ✅ OK |
| `ctx-window hook codex` | PostToolUse | `*` | Output não-vazio (turns.jsonl) | ✅ OK |
| `ctx-window-summarize-at-stop.sh` (via wrap-hook) | Stop | `*` | Summary persistido | ✅ OK |
| `shell-validate hook` | PreToolUse | `Bash` | Sem output (allow) | ✅ OK |
| `agent-react-nudge.sh` | PostToolUse | `*` (Edit/Write/MultiEdit/NotebookEdit) | SEMPRE `{}` | ⚠️ Mudo (apenas memory-mcp) |
| `context-guard-nudge.sh` | PostToolUse | `*` (Edit/Write/MultiEdit/NotebookEdit) | SEMPRE `{}` | ⚠️ Mudo (apenas memory-mcp) |
| `memory-nudge.sh` | PostToolUse | `*` (Edit/Write/MultiEdit/NotebookEdit) | SEMPRE `{}` | ⚠️ Mudo (apenas memory-mcp) |
| `docs-cache.sh` | PostToolUse | `WebFetch\|mcp__context7__.*` | SEMPRE `{}` | ✅ OK (cache em background) |
| `false-success-guard hook` | Stop | `*` | SEMPRE `{}` (mas `check` flagaria) | ❌ Bug silencioso |
| `token-nudge.check.sh` (via wrap-hook) | PostToolUse | `*` | (não testado aqui) | 🔍 A validar |
| `memory-observe.posttooluse.sh` (via wrap-hook) | PostToolUse | `*` | (não testado aqui) | 🔍 A validar |
| `memory-consolidate.stop.sh` (via wrap-hook) | Stop | `*` | (não testado aqui) | 🔍 A validar |
| `agent-task-record.stop.sh` (via wrap-hook) | Stop | `*` | (não testado aqui) | 🔍 A validar |

**Conclusão da análise de hooks**: 5 de 13 hooks testados retornam `{}` quando deveriam emitir contexto. 3 são mudos por design (workaround documentado), 1 é silencioso por design (docs-cache), **1 é regressão silenciosa** (`false-success-guard hook`).

## Análise Adicional: regressão paralela em `~/.claude/settings.json` (issue #4)

Mesma classe do Vetor A acima, mas o alvo foi o arquivo `~/.claude/settings.json` (Claude Code) em vez de `~/.codex/hooks.json` (Codex). Investigação fechada no branch `fix/hook-script-path-validation` em 2026-09-23.

### Origem (shell history `~/.zsh_history`, Unix timestamps convertidos para -03:00)

- **16:43–16:54** — usuário editou `~/.config/agent-sync/config.json` (3×) direto da `$HOME` (cwd = `/home/matheus_dutra/`).
- **16:55:13** — `agent-sync` (sem args) executado da `$HOME`. PATH lookup resolveu para `~/.local/bin/agent-sync` (binário global, build **pré-fix** com `ResolveBaseDir` fallback silencioso para `cwd`).
- **16:55:23** — **`agent-sync -apply`** (sem `cd Projects/agent-sync`, sem `make`). ResolveBaseDir não encontrou `rules/global-rules.md` (binário em `~/.local/bin`, fora do repo) e caiu para `cwd` = `/home/matheus_dutra/`. Pattern `filepath.Join(baseDir, "hooks", X)` em `core_wrap_hook.go:13` e 5 outros sites gravou `/home/matheus_dutra/hooks/...` em **7 entradas** de `~/.claude/settings.json` (1 `principles-inject.pretooluse.sh` + 6 invocações `wrap-hook.sh` em PostToolUse para `token-nudge.check.sh`, `memory-observe.posttooluse.sh`).
- **16:55:42–16:55:53** — `agent-sync -observability` para verificar; backup automático `settings.json.bak.20260923-195749Z` foi criado **já com paths errados** (16:57:49 -03:00).
- **16:56:39** — `cd Projects/agent-sync` (início da recuperação).
- **17:00:07** — `make apply` reescreveu paths corretos em `~/.codex/hooks.json`. `~/.claude/settings.json` ficou parcialmente com paths errados (1 `principles-inject` residual) até hotfix manual posterior.

### Causa raiz

Idêntica ao Vetor A: `ResolveBaseDir` com fallback silencioso para `cwd` (`internal/pathutil/pathutil.go` na versão pré-fix). O vetor de classe permaneceu latente até este fix.

### Fix implementado (este branch)

- `internal/pathutil/pathutil.go:ResolveBaseDir` — assinatura mudou de `(exePath, cwd, envHome string) string` para `(string, error)`. Quando nenhum start contém `rules/global-rules.md`, retorna `error` com mensagem: `agent-sync: rules/global-rules.md nao encontrado a partir de [starts]; execute dentro do repo ou defina AGENT_SYNC_HOME`. Fallback silencioso removido.
- `internal/pathutil/pathutil.go:ResolveStateRoot` — assinatura propagada para `(string, error)`.
- 6 callers diretos de `ResolveBaseDir` atualizados: `internal/skills/{index,new,lint}.go`, `internal/doctor/doctor.go`, `internal/apply/{command,fs}.go`.
- 12 callers de `ResolveStateRoot` atualizados: `internal/budget/{apply,status}.go`, `internal/state/{migrate,snapshot,render}.go`, `internal/event/command.go`.
- Regressão travada em `internal/pathutil/pathutil_test.go:TestResolveBaseDir_FailsFastWhenNotInRepo` (cobre fail-fast com mensagem útil) e `TestResolveBaseDir` atualizado para o novo signature.
- Smoke test reproduzido: `cd ~ && ~/.local/bin/agent-sync -status` → exit 1 com mensagem orientando `cd` ou `AGENT_SYNC_HOME`. Caminho feliz (cwd correto, AGENT_SYNC_HOME override) preservado.

### Pendência relacionada

- Issue **#5** — migrar os 6 sites restantes que ainda usam `filepath.Join(baseDir, "hooks", X)` para `pathutil.HookScriptPath`. Defense in depth, mas não bloqueante.

---

**Lições aprendidas**:
1. **Fallback silencioso é foot-gun**: `ResolveBaseDir` deveria falhar rápido em vez de mascarar a configuração errada.
2. **Operações destrutivas precisam de atomicidade**: o padrão copy-to-temp + rename é barato e elimina toda uma classe de bugs.
3. **Hooks mudos = hooks inertes**: enquanto o schema do Antigravity/Codex não permitir `additionalContext` em PostToolUse sem bloquear, os nudges precisam de outra estratégia (UserInput ephemeral, contexto injetado na próxima mensagem, etc.).
4. **Doctor deve cobrir o caminho de não-instalação**: `~/.gemini/config/plugins/...` órfão é um cenário real e precisa de check explícito.
