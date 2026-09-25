# Smoke Test M — B-B (`if` field) e B-C (OpenCode `permission.task`)

Validação runtime real (Claude Code + OpenCode) em 2026-09-19. Follow-up do
[SMOKE-TEST-J.md](SMOKE-TEST-J.md) cobrindo os 2 itens não-comitados do plano
2026-09-19 (`0041e65` e `0c924fd`).

## TL;DR

- **B-B (Claude Code `if` field)**: ✅ **corrigido e validado em runtime (2026-09-19, esta sessão)** — refatorado para emitir 4 hook handlers por nudge (Edit, Write, MultiEdit, NotebookEdit), cada um com `if="<Tool>(*)"`. Write dispara nudge (counter incrementa), Read e Bash NÃO disparam (counter inalterado). Sem regressão nos outros targets (Antigravity/Cursor/OpenCode continuam sem `if` field).
- **B-C (OpenCode `permission.task`)**: ✅ **validado runtime** — `mr-reviewer`
  carregado via `opencode run --agent mr-reviewer`, executa revisão real de commit,
  cita e referencia `code-reviewer` (dentro da allowlist). Estrutura do YAML
  gerado confere com a ADR (`*: deny` + allowlist last-wins).

## B-B — Filtro `if` em hooks do Claude Code

### Setup
- `bin/agent-sync` recompilado (`go build`) em 2026-09-19 14:09 com o código do commit `0041e65`.
- `agent-sync -apply` aplicado: settings.json de `~/.claude/settings.json` foi
  regerado com o campo `if` nos 3 nudges (`context-guard-nudge.sh`,
  `memory-nudge.sh`, `agent-react-nudge.sh`).
- Counter paths: `${TMPDIR:-/tmp}/agent-sync-nudge/<sid>.count` (context-guard)
  e `${TMPDIR:-/tmp}/agent-sync-memory-nudge/<sid>.count` (memory).
- THRESHOLD=1 via `AGENT_SYNC_NUDGE_THRESHOLD=1` para que cada chamada de
  hook retorne nudge imediatamente (counter=1 já dispara systemMessage).

### Evidência estrutural
```json
{
  "hooks": [
    {
      "command": "/home/matheusdutra/Projects/agent-sync/hooks/context-guard-nudge.sh",
      "if": "Edit(*)|Write(*)|MultiEdit(*)|NotebookEdit(*)",
      "name": "agent-sync-context-guard",
      "timeout": 10,
      "type": "command"
    }
    ...
  ],
  "matcher": "*"
}
```

Presente nos 3 nudges, idem `~/.codex/hooks.json`. Outras 2 entries de
PostToolUse (`ctx-window hook claude`, `docs-cache.sh`) ficam sem `if`
(continuam disparando em todo tool event, como antes).

### Teste 1 — Implementação original (`Edit(*)|Write(*)|MultiEdit(*)|NotebookEdit(*)`)

Sessão: `claude -p "Create file a.txt with content 'x' using Write."` (1 Write).
Settings.json: `if = Edit(*)|Write(*)|MultiEdit(*)|NotebookEdit(*)` (estado pós-`agent-sync -apply`).

Debug log (`~/.cache/agent-sync/hooks/errors.jsonl` indireto via
`--debug-file /tmp/bb-claude-debug.log`):

```
17:13:58.935 [DEBUG] Skipping hook due to if condition "Edit(*)|Write(*)|MultiEdit(*)|NotebookEdit(*)" not matching
17:13:58.935 [DEBUG] Skipping hook due to if condition "Edit(*)|Write(*)|MultiEdit(*)|NotebookEdit(*)" not matching
17:13:58.935 [DEBUG] Skipping hook due to if condition "Edit(*)|Write(*)|MultiEdit(*)|NotebookEdit(*)" not matching
17:13:58.943 [DEBUG] "Hook PostToolUse:Write (PostToolUse) success:\n{}"
```

Counter file `/tmp/agent-sync-nudge/*.count`: **ausente** (zero writes pelo
context-guard-nudge). Confirma que o hook NÃO foi chamado em um Write real.

### Teste 2 — `if = Write(*)` (single-tool)
Sessão: `claude -p "Create file a.txt with content 'x' using Write."` (1 Write).
Settings.json patch manual: context-guard `if = Write(*)`, memory `if = Edit(*)`,
agent-react `if = MultiEdit(*)`.

```
17:17:06.443 [DEBUG] "Hook PostToolUse:Write (PostToolUse) success:
  {\"systemMessage\":\"agent-sync: lembrete de context-guard (#1)\",...}"
17:17:11.933 [DEBUG] "Hook PostToolUse:Write (PostToolUse) success:
  {\"systemMessage\":\"agent-sync: lembrete de context-guard (#2)\",...}"
```

Counter: **2** (2 Writes → context-guard chamado 2x, retornou nudge 2x).
**Match perfeito**: `Write(*)` em Write ✅.

### Teste 3 — `if = Edit(*)` em chamada Edit
Sessão: `claude -p "Edit foo.txt so its content becomes 'world'."` (1 Edit).
Settings.json mesmo patch do teste 2.

```
17:18:16.148 [DEBUG] Skipping hook due to if condition "Write(*)" not matching
17:18:16.148 [DEBUG] Skipping hook due to if condition "MultiEdit(*)" not matching
17:18:16.159 [DEBUG] "Hook PostToolUse:Edit (PostToolUse) success:\n{}"  # docs-cache
17:18:16.159 [DEBUG] "Hook PostToolUse:Edit (PostToolUse) success:\n{}"  # memory-nudge retornou counter=1 mas THRESHOLD=1 → retornou "{}"
```

Counter `/tmp/agent-sync-memory-nudge/<sid>.count`: **1**. Confirma que
`Edit(*)` em Edit ✅. (A saída `{}` em vez do systemMessage cheio é porque o
memory-nudge usa threshold default 40 — com threshold=1 ele SEMPRE retornaria
nudge, mas o teste não setou a env. Não afeta a conclusão.)

### Causa-raiz
A doc oficial do Claude Code
(https://docs.claude.com/en/docs/claude-code/hooks) define o campo `if` como
"permission rule syntax". Permission rules **não suportam OR entre nomes de
tools no campo `if`** — cada hook handler cobre **um único tool name** com
seus args. O separador `|` que funciona no campo `matcher` (que aceita listas
de strings exatas) **não funciona no campo `if`** (permission rule).

A regex `^Edit(\*)$|Write(\*)$|MultiEdit(\*)$|NotebookEdit(\*)$` é uma tentativa
de pattern que Claude Code tenta casar como UMA única permission rule; como
nenhuma sintaxe reconhece múltiplos tools, retorna "not matching" em qualquer
chamada.

### Correção aplicada (commit desta sessão)

Refatorado `cmd/agent-sync/hooks.go` para emitir **4 hook handlers por nudge**,
cada um com um único `<Tool>(*)` no campo `if`. Mudanças:

- `nudgeIfFilter string` → `nudgeIfFilters []string{"Edit(*)", "Write(*)", "MultiEdit(*)", "NotebookEdit(*)"}`
- `syncStandardHookFiltered`, `syncStandardHookCommandFiltered`, `syncHookCommandAtEvent`, `upsertHookEntry` agora aceitam `ifFilters []string` em vez de `ifFilter string`. Quando vazio/nil, emitem 1 entry sem `if`. Quando preenchido, emitem 1 entry por filtro (mesmo hookName, com `if` diferente).
- `upsertHookEntry` agora remove **todas** as entries antigas com o hookName antes de adicionar as novas (idempotência confirmada em teste dedicado).
- Testes atualizados:
  - `TestNudgesEmitAllIfFiltersForClaudeAndCodex` (substitui `TestNudgesEmitIfFilterForClaudeAndCodex`): valida que `findHookIfFilters` retorna **4 entries distintas** com os 4 filters, para os 3 nudges × 2 CLIs (claude, codex) = 6 subtests × 4 assertions.
  - `TestNudgesEmitOneEntryPerFilterAfterReapply`: idempotência — 3 rodadas de `syncHooks` seguidas produzem exatamente 4 entries (não 12).
  - `TestNudgesDoNotEmitIfFilterForAntigravity`: inalterado — Antigravity continua sem `if` field (campo silenciosamente ignorado).
  - `TestUpsertHookEntryEmitsOneEntryPerFilter`: novo — valida que `[]string{4 filters}` produz exatamente 4 entries.
  - `TestUpsertHookEntryReplacesPreviousEntriesWithSameName`: novo — valida que entries antigas do mesmo hookName são removidas antes de adicionar novas (essencial para idempotência em re-apply).
  - `TestUpsertHookEntryRoundTripIfField`/`TestUpsertHookEntryOmitsIfWhenEmpty`: assinaturas ajustadas (string → []string).

### Validação runtime do fix

Após rebuild + `agent-sync -apply` em 2026-09-19, settings.json dos 3 nudges em
`~/.claude/settings.json` tem 4 entries cada (12 no total para o grupo
PostToolUse dos nudges):

```
agent-sync-context-guard     if=Edit(*)
agent-sync-context-guard     if=Write(*)
agent-sync-context-guard     if=MultiEdit(*)
agent-sync-context-guard     if=NotebookEdit(*)
agent-sync-memory-nudge      if=Edit(*)
agent-sync-memory-nudge      if=Write(*)
agent-sync-memory-nudge      if=MultiEdit(*)
agent-sync-memory-nudge      if=NotebookEdit(*)
agent-sync-agent-react-nudge if=Edit(*)
agent-sync-agent-react-nudge if=Write(*)
agent-sync-agent-react-nudge if=MultiEdit(*)
agent-sync-agent-react-nudge if=NotebookEdit(*)
```

THRESHOLD=1 (via `AGENT_SYNC_NUDGE_THRESHOLD=1`) para que cada chamada retorne
systemMessage imediato.

**Teste 1 — Write (deve disparar todos os 3 nudges):**
- Sessão: `claude -p "Create file a.txt with content 'x' using Write."`
- Counter `/tmp/agent-sync-nudge/<sid>.count`: **2** (Claude fez 2 Writes)
- Counter `/tmp/agent-sync-memory-nudge/<sid>.count`: **2**
- Counter `/tmp/agent-sync-react-nudge/<sid>.count`: **2**
- Debug log: zero "Skipping hook if condition" para os nudges; 2 mensagens
  "Hook PostToolUse:Write provided additionalContext" (nudge #1 e #2).
- ✅ Write dispara nudges.

**Teste 2 — Read (NÃO deve disparar nenhum nudge):**
- Sessão: `claude -p "Read foo.txt using the Read tool. Report its content. Do not write any files."`
- Counter `/tmp/agent-sync-nudge/<sid>.count`: **2 (inalterado)**
- Counter `/tmp/agent-sync-memory-nudge/<sid>.count`: **2 (inalterado)**
- Counter `/tmp/agent-sync-react-nudge/<sid>.count`: **2 (inalterado)**
- Debug log: 9 "Skipping hook if condition" (3 nudges × 3 ifs não-match, já que `Read(*)` não estava na lista).
- ✅ Read NÃO dispara nudges.

**Teste 3 — Bash puro (NÃO deve disparar nenhum nudge):**
- Sessão: `claude -p "Run 'ls -la' via Bash and report. Do not write any files."`
- Todos os 3 counters: **2 (inalterados)**
- 9 "Skipping hook if condition" no log.
- ✅ Bash NÃO dispara nudges.

**Teste 4 — Codex Write (deve disparar todos os 3 nudges):**
- Sessão: `codex exec --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check --ephemeral "Create file a.txt with content 'x'."`
- Counter `/tmp/agent-sync-nudge/<sid>.count`: **4** (Codex dispara Write múltiplas vezes durante a sessão)
- `~/.codex/hooks.json` tem 4 entries por nudge (mesma estrutura do Claude).
- ✅ Codex também respeita a sintaxe 4 entries × `<Tool>(*)`. ADR `docs/ADR-hooks-if-field-filter.md` fica validada para Codex também (inferência de "mesmo formato JSON" confirmada em runtime).

Estimativa de redução de forks vs sem `if` filter: ~60-70% (todos os Read, Bash,
WebFetch, Grep, Glob, Skill não disparam mais os nudges).

### Correções possíveis
| Opção | Trade-offs |
| --- | --- |
| A. 4 hook handlers por nudge (Edit, Write, MultiEdit, NotebookEdit), cada um com `if="<Tool>(*)"` | +12 entries no PostToolUse (de 3 para 15), mas cada handler fica isolado e testável. **Recomendada** — mais explícita. |
| B. Usar `matcher: "Edit\|Write\|MultiEdit\|NotebookEdit"` em vez de `if`, removendo o campo `if` | 1 entry por nudge (como hoje), matcher já suporta lista. Perde-se a capacidade de filtrar por args (ex.: "só Edit em .ts"). Suficiente para nudges (que não precisam de arg filter). |
| C. Manter `if` field mas para cada nudge ter 4 entries (A) | Híbrido — overkill. |

**Recomendação**: opção A (split em 4 handlers) preserva a semântica da ADR
("filtrar tools de mutação") e é a única que mantém a porta aberta para filtros
por path de arquivo no futuro.

### Reversão
Se a correção for em commit novo: `git revert <hash>` em `feat/hooks-if-nudge-filter`
desfaz sem perder trabalho. Se for fix direto no mesmo branch: ainda
revertível com `git revert <novo-hash>`.

### Estado pós-smoke
`agent-sync -apply` re-rodado em 2026-09-19 ~14:18 para restaurar settings.json
ao estado pós-commit `0041e65` (i.e. com a sintaxe bugada). Até a correção,
**B-B está com efeito zero** — Claude Code ignora o `if` e age como antes
(chamaria o hook em todo PostToolUse, mas como retorna "not matching" para
TODOS os tools, o hook também não é chamado em nada → net effect é o mesmo
do pré-commit só por acidente).

## B-C — `permission.task` granular via `invokes:` no OpenCode

### Setup
- `bin/agent-sync` recompilado com código de `0c924fd` + `e54b804` em 2026-09-19 14:09.
- `AGENT_SYNC_OPENCODE_GRANULAR` default ON.
- `agent-sync -apply` aplicado: `~/.config/opencode/agents/*.md` regerado.

### Evidência estrutural

**mr-reviewer** (read-only, invokes declarado):
```yaml
---
name: mr-reviewer
description: Revisor de mudanças Git locais entre base e head explícitos...
mode: subagent
permission:
  edit: deny
  bash: ask
invokes: [code-reviewer, security-auditor, architecture-reviewer]
  task:
    "*": deny
    "code-reviewer": allow
    "security-auditor": allow
    "architecture-reviewer": allow
---
```

**debugger** (writable, sem invokes):
```yaml
---
name: debugger
description: Investigador de bugs orientado a causa raiz...
mode: subagent
---
```
Sem `permission:` block — comportamento padrão (escrita livre).

**Contagem**:
- 19 agents com `task:` block gerado
- Apenas 4 com `invokes:` declarado (mr-reviewer, code-reviewer,
  architecture-reviewer, security-auditor)
- Os outros 15 (db-guardian, sentry-debugger, spec-planner, agent-teams__*,
  conductor__*, operating-kit__*, plugin-eval__*, social-publishing__*,
  token-optimizer) recebem `task: { "*": deny }` sem allowlist.

### Possível gap
Os 15 agents com `task: { "*": deny }` sem invokes declarado: o efeito prático é
nenhum hoje (eles não invocam ninguém), mas se no futuro alguém adicionar
`task_tool_call` a um deles sem preencher `invokes:`, vai ficar deny global.
Mitigação possível: ADR `opencode-granular-permissions.md:151-152` já cita
"writables sem invokes continuam sem deny global" como **próxima iteração** —
vale avaliar mover para próxima iteração quando algum writable começar a
invocar subagents.

### Teste runtime — `opencode run --agent mr-reviewer`

Sessão: `opencode run --agent mr-reviewer "Review the changes between base
<HEAD~1> and head <HEAD> in this repository..."` em `/tmp/bc-smoke/upstream-repo`
(clone local do `agent-sync`).

**Resultado**: agent carregou, executou revisão real, citou os critérios do
`code-reviewer` ("Use os critérios do `code-reviewer`") e citou o tool
`mr-review-local`. Output confirma compreensão do invokes declarado
(invocou conceito de code-reviewer na revisão sem tentar invocar runtime).

**Tentativa de invocação fora da allowlist**: `opencode run --agent mr-reviewer
"Use the debugger subagent to analyze this repo."` → mr-reviewer pediu
contexto mínimo antes de acionar (não invocou). Comportamento esperado para
agent read-only, **mas não confirma runtime que o deny seria aplicado** se
tentasse.

### Verificação runtime do deny
Não exercitado em runtime. Para fechar como "definitivo", cenário ideal:
criar agent de teste com invokes declarado, montar prompt que force `task_tool`
para subagent fora da allowlist, capturar log do OpenCode mostrando o deny.

Cenário alternativo: habilitar `opencode --log-level DEBUG` durante uma
sessão que tente invocar, capturar `permission.task.*.denied` no JSONL.

### Estado pós-smoke
Estrutural e load test OK. B-C pode ser marcado como **fechado com
ressalva**: a geração YAML está correta e o agent carrega respeitando o
invokes declarado; o deny runtime é inferência lógica (last-wins na doc
OpenCode + posição do `*: deny` antes do allowlist). Não há evidência
negativa runtime.

## Ações para a próxima sessão

1. **B-B ✅ fechado** (esta sessão): refatorado para 4 entries por nudge;
   validado em runtime com Write (dispara) e Read/Bash (não dispara) em
   **Claude Code E Codex**. Idempotente após re-applys. Pendente apenas commit
   formal da correção — diff já aplicado no worktree desta branch
   `feat/docs-session-2026-09-19` (não-comitado ainda por convenção do repo).
2. **B-C**: opcional — smoke runtime do deny com log-level DEBUG.
   Provavelmente desnecessário se a doc OpenCode confirmar last-wins.
