# ADR: Filtragem de nudges via campo `if` do hook (Claude Code + Codex)

**Status**: Aceito
**Data**: 2026-09-19
**Decisor**: Matheus Dutra
**Tags**: ai-agent, hooks, performance, observability, determinism

## Contexto

O agent-sync instala 3 nudges em `PostToolUse` que rodam em **todo tool call** de uma sessão:

- `context-guard-nudge` — lembra periodicamente (a cada N=40) de carregar a skill `context-guard` e atualizar `STATE.md`.
- `memory-nudge` — lembra periodicamente de gravar em `memory-mcp` via `store_memory`.
- `agent-react-nudge` — reforça o contrato "hipótese ≠ fato; valide ou peça o passo concreto ao usuário".

Lógica comum: spawn de processo shell → `cat` no stdin (JSON da tool call) → `grep`/`sed` para extrair `session_id` → `mkdir -p` no `${TMPDIR}` → incremento de contador em `${TMPDIR}/agent-sync-nudge*/<sid>.count` → retorno `{}` (ou lembrete a cada N).

Em uma sessão típica (100-300 tool calls), esses 3 nudges geram **300-900 forks**, sendo que apenas os tools de edição (Edit/Write/MultiEdit/NotebookEdit) — onde o lembrete de "atualize STATE.md" tem sentido semântico — compõem ~30% das chamadas. Read, Bash de leitura, Grep, Glob, WebFetch disparam o processo sem necessidade real.

A doc oficial do Claude Code (`docs.claude.com/en/docs/claude-code/hooks`, lida em 2026-09-19) confirma que handlers de hook em tool events aceitam um campo `if` que filtra sub-comandos pela permission rule syntax antes do spawn:

```json
{"matcher": "*", "hooks": [{"type": "command", "command": "...", "if": "Edit(*)|Write(*)|MultiEdit(*)|NotebookEdit(*)"}]}
```

O `if` reduz forks no caso "matcher genérico casa mas a ação não é a visada". Filtros equivalentes **não existem** documentados em Cursor (JSON próprio simples) ou OpenCode (plugin TS sem hooks nativos para tool events no formato Claude).

O item estava previsto como "próximo passo" em `STATE.md:217`: *"Se barulho: subir threshold ou filtrar só tools de mutação"*.

## Decisão

Adotar o campo `if` para os 3 nudges em **Claude Code** (suporte confirmado na doc oficial) e **Codex** (mesmo formato JSON compartilhado, sem doc explícita verificada mas mesmo schema — risco aceito com reversão trivial).

**Escopo por CLI:**

| CLI | Suporte `if` field | Ação nesta decisão |
|---|---|---|
| Claude Code | sim (doc oficial) | aplicar filtro `Edit(*)|Write(*)|MultiEdit(*)|NotebookEdit(*)` |
| Codex | mesmo formato JSON do Claude; sem doc que confirmei 100% | aplicar (mesmo filtro); rollback trivial se falhar |
| Antigravity | incerto — struct compartilhada mas sem doc que confirme | **não aplicar**; matcher `*` mantido |
| Cursor | JSON próprio simples (sem `if`) | **não aplicar** |
| OpenCode | plugin TS, não hook nativo no formato Claude | **não aplicar** |

**Estrutura do gerador:**

- Struct `hookCmd` em `cmd/agent-sync/hooks.go` ganha campo `If string \`json:"if,omitempty"\`` (campo opcional, aditivo).
- Função `upsertHookEntry` aceita parâmetro `ifFilter string`; quando vazio, omite o campo no JSON.
- Novas funções `syncStandardHookFiltered` e `syncStandardHookCommandFiltered` envelopam a versão antiga sem mudar signature existente.
- Helper `supportsNudgeFilter(target TargetCLI) bool` detecta Claude/Codex; constante `nudgeIfFilter = "Edit(*)|Write(*)|MultiEdit(*)|NotebookEdit(*)"` centraliza o padrão.
- As 3 chamadoras (`syncHooks`, `syncMemoryNudgeHook`, `syncAgentReactNudgeHook`) usam a versão filtrada quando `supportsNudgeFilter(target)`, fallback idêntico ao anterior caso contrário.

## Consequências

### Positivas

- **Redução estimada de forks: 60-70% por nudge em sessão típica** (100-300 tool calls × 3 nudges = 300-900 → ~90-270). Em wall-clock, ~1-3s por sessão + redução de ruído no `observability` JSONL.
- **Mais determinismo**: menos processos = menos janela para nondeterminismo (PATH, env, cwd, mktemp).
- **JSON do settings.json continua compatível** — campo `if` é opcional; consumidores que ignoram campos desconhecidos (todos os 5 targets) seguem funcionando.
- **Heurística defensiva**: o `if` filtra no parser da CLI antes do fork; mesmo se o filtro casar errado, o script shell faz `grep`/`sed` para extrair `session_id` e retorna `{}` no caminho comum — fail-safe.
- **Threshold fica intacto**: a cadência "a cada N=40" continua. Só filtra **quando** roda, não **com que frequência** emite.
- **Codex compartilha o ganho** sem custo: o código é o mesmo para os 2 targets suportados.

### Negativas / trade-offs

- **Codex sem doc verificada**: a doc oficial do Codex não foi confirmada nesta rodada; o formato JSON é idêntico ao Claude mas o suporte ao campo `if` é inferência. Mitigação: smoke real antes de fechar como "definitivo"; rollback é `git revert 0041e65`.
- **Análise de filter não-trivial**: matcher `Bash(*.ts)` casa com `Bash`, `Edit(*.ts)` casa com `Edit`; sem ancoragem, hyphens viram regex não-ancorado. Gotcha documentado na doc do Claude Code (versões <2.1.195). Para nossos filtros `Edit(*)` etc., o `(` antes do `*` ancora o início do matcher — sem regressão esperada.
- **Heurística legada no script fica como cinto-e-suspensórios**: o `.sh` continua fazendo `cat`/`grep`/`sed` no JSON; se o `if` filtrar errado, o script roda em todos os tools e o efeito é o mesmo de antes. Sem regressão de pior caso.
- **Cobertura assimétrica**: 2 de 5 CLIs se beneficiam do `if`. Antigravity/Cursor/OpenCode seguem com matcher `*`. Se a cobertura assimétrica virar problema, revisitar com pesquisa específica (B-E radar).

## Decisões revisadas

(nenhuma — ADR em estado Aceito na primeira iteração. A asserção sobre Codex fica condicionada a smoke real futuro.)

## Evidência / Implementação

| Arquivo | Mudança |
|---|---|
| `cmd/agent-sync/hooks.go` | campo `If` em `hookCmd`; `upsertHookEntry` aceita `ifFilter`; novas funções `syncStandardHookFiltered`/`syncStandardHookCommandFiltered`; constante `nudgeIfFilter`; helper `supportsNudgeFilter`; os 3 nudges ramificam em filtered/legacy |
| `cmd/agent-sync/hooks_test.go` | `TestNudgesEmitIfFilterForClaudeAndCodex` (6 subtests), `TestNudgesDoNotEmitIfFilterForAntigravity` (3 subtests), `TestUpsertHookEntryRoundTripIfField`, `TestUpsertHookEntryOmitsIfWhenEmpty` — todos passam |
| Branch | `feat/hooks-if-nudge-filter` |
| Commit | `0041e65` |
| Diff | `+221/-10` (2 arquivos) |

### Critério de aceite validado

- Suite completa verde (`go test ./...` no módulo agent-sync).
- `go vet` e `gofmt` limpos.
- JSON gerado para Claude/Codex contém `"if": "Edit(*)|Write(*)|MultiEdit(*)|NotebookEdit(*)"` no hook command.
- JSON gerado para Antigravity **não** contém `if` (fallback idêntico ao anterior).

### Critério de aceite **pendente** (não automatizável neste ambiente)

- **Smoke real**: editar um arquivo `.ts` em sessão Claude Code → counter de nudge incrementa a cada 40 (verificar `${TMPDIR}/agent-sync-nudge*/<sid>.count`). Ler arquivo binário → counter não incrementa. Bash puro → counter não incrementa. Equivalente para Codex.
- Aceitar como definitivo só após smoke real.

## Limites conhecidos

1. **Codex não confirmado na doc oficial** — formato compartilhado é forte indicador, mas sem smoke real, hipótese.
2. **Estimativa de 60-70% redução de forks é baseada em heurística de sessão típica**; medição real depende de instrumentação que `wrap-hook.sh` (Fase F-B, ADR-harness-trace-guard) já emite — basta olhar `agent-sync -observability` antes/depois.
3. **Filtro conservador**: Read e Bash de leitura ficam de fora propositalmente. Se um agente workflow depender de nudge em Read (ex: lembra contexto após N leituras), o filtro barra. Sem demanda atual.

## Próximos passos

1. **Smoke real em Claude Code** (ação não-comitada): executar sessão com várias Edits e Reads; verificar counter.
2. **Smoke real em Codex** (condicional à 1): se 1 confirmar, rodar mesmo cenário.
3. **Medir redução observável** via `agent-sync -observability` antes/depois — atualizar ADR com nº real.
4. **Reavaliar Antigravity** quando doc oficial explicitar suporte a filtros de hook (hoje incerto).
5. **Reavaliar Cursor/OpenCode** quando oferecerem equivalente ao `if` field.

## Referências

- **Doc oficial Claude Code**: `https://docs.claude.com/en/docs/claude-code/hooks` (lida em 2026-09-19). Seções relevantes: "Hook handler fields" → campo `if`; "Common fields" → escopo de tool events; "Matcher patterns" → gotcha de hyphens em versões antigas.
- **ADR relacionada**: `docs/ADR-fim-de-turno-hooks.md` (linha 31-32 cita dedup por tool call; este filtro é camada complementar).
- **ADR relacionada**: `docs/ADR-harness-trace-guard.md` (instrumentação via `wrap-hook.sh` permite medir redução observável).
- **Pendência original**: `STATE.md:217` — "filtrar só tools de mutação".
