# ADR: Permission.task granular para OpenCode via campo `invokes:` no manifesto canônico

**Status**: Aceito
**Data**: 2026-09-19
**Decisor**: Matheus Dutra
**Tags**: ai-agent, opencode, permissions, agents, orchestration

## Contexto

O agent-sync gera arquivos de agent no formato nativo de 5 CLIs. Para OpenCode, o gerador emitia um permission block restritivo quando `readonly: true` no frontmatter do agent canônico:

```yaml
mode: subagent
permission:
  edit: deny
  bash: ask
```

Esse formato é equivalente ao de outros targets (Claude Code: `permission.edit=deny`; Cursor: `readonly: true`; Codex: `sandbox_mode = "read-only"`), mas perdia uma capacidade nativa do OpenCode: **`permission.task`** — controle de quais subagents podem ser invocados via Task tool com glob patterns.

A doc oficial do OpenCode (`opencode.ai/docs/agents/`, lida em 2026-09-19) mostra o formato:

```yaml
agent:
  orchestrator:
    mode: primary
    permission:
      task:
        "*": "deny"
        "orchestrator-*": "allow"
        "code-reviewer": "ask"
```

Regra de precedência: "last matching rule wins" — específicos DEVEM vir depois do `*` para sobrepor.

Hoje, o gerador do agent-sync:
- **Não emite** `permission.task` para read-only agents (deny implícito seria `allow *` se não declarado).
- **Não emite** `permission.task` para writable agents (allow implícito — qualquer subagent pode ser invocado).
- **Não tem mecanismo** para o autor do agent canônico declarar quais subagents espera invocar.

Resultado prático: `mr-reviewer` (read-only) pode, em teoria, invocar `test-engineer` (writable, com permissão de bash destrutivo) — escape de perímetro. Agentes writable podem invocar qualquer outro agent sem registro.

## Decisão

Adotar `permission.task` granular no OpenCode com **3 decisões de shape** registradas:

### Decisão 1 — `invokes:` como lista simples

No frontmatter dos agents canônicos (`agents/*.md`), aceitar campo opcional:

```yaml
invokes: [code-reviewer, mr-reviewer]
```

Formato YAML inline, parser tolerante a espaços/aspas/vírgula trailing. Sem mapa `{read: [...], write: [...]}` — semântica não justificada agora (revisão se virar dor).

### Decisão 2 — Read-only agents emitem `task: { "*": "deny" }` explícito

Quando `readonly: true` **e** granular ON:
- Sem `invokes:` declarado → `task: { "*": "deny" }` (deny global explícito, sem whitelists).
- Com `invokes:` declarado → `task: { "*": "deny", "<each>": "allow" }` (deny global primeiro, allows específicos depois — last wins).

Quando `readonly: false` (writable) **e** `invokes:` declarado: deny global + allowlist (mesma regra). Writable sem invokes continua sem `task` (allow implícito — comportamento atual preservado).

### Decisão 3 — Rollout via env var, default ON

Flag: `AGENT_SYNC_OPENCODE_GRANULAR=0` (ou `false`/`off`/`no`) desliga o bloco `task`. Default ON: granular vira o comportamento padrão.

Justificativa do default ON (vs conservador OFF): o ganho é a **função principal** desta feature, não um experimento. Quem quiser output largo antigo seta a env var. Mesma filosofia de `AGENT_SYNC_DRY_RUN` (default ON na CLI).

**Compatibilidade por CLI:** campo `invokes:` é emitido **apenas no OpenCode** (no frontmatter YAML renderizado). Claude/Cursor/Codex/Antigravity não recebem o campo (não têm semântica nativa para subagent invocation). Demais campos do manifesto (`name`, `description`, `readonly`) continuam propagados para todas as CLIs.

## Consequências

### Positivas

- **Escape de perímetro fechado**: `mr-reviewer` (read-only, bash=ask) não pode mais invocar `test-engineer` (writable, bash destrutivo) sem declaração explícita.
- **Intenção documentada no manifesto**: ler `agents/*.md` mostra o grafo de invocação declarado por cada agent. Antes era implícito/inferido pelo autor do agent.
- **Princípio do menor privilégio aplicado**: agents ganham allowlists explícitas em vez de allow * implícito.
- **Rollout seguro**: nenhum agent canônico (`agents/*.md`) declarava `invokes:` antes desta ADR. Diff observável em `~/.config/opencode/agents/` é zero até o usuário preencher o campo.
- **Reversão trivial**: `AGENT_SYNC_OPENCODE_GRANULAR=0` desliga sem mudar código; `git revert <sha>` reverte a feature.

### Negativas / trade-offs

- **Manutenção de invariantes**: ao adicionar agent novo, o autor precisa declarar `invokes:` se quiser que ele invoque outros. Sem isso, agent novo vira "isolado" (deny *). Mitigação: documentar no template do agent (próximo passo opcional).
- **Schema não-versionado**: campo `invokes:` é novo no manifesto canônico. Consumers do manifesto (futuro, se houver) precisam conhecer. Risco baixo: o campo é puramente opcional.
- **Codex/Claude/Cursor ficam sem equivalente**: não há `permission.task` em outras CLIs. O ganho OpenCode não migra automaticamente. Aceitável: cada CLI evolui no próprio ritmo.
- **Antigravity sem equivalente verificado**: doc do Antigravity menciona matcher, mas não `if` field (ver ADR-hooks-if-field-filter). Escopo OpenCode-only é conservador.
- **Última regra vence, mas erro de ordem é silencioso**: se autor puser allowlist antes do deny global, a allowlist fica "sombra" (deny global sobrescreve). Mitigação: ordem gerada pelo código (`"*": deny` sempre primeiro, allows depois) — autor não controla a ordem via manifesto, só os nomes.

## Decisões revisadas

(nenhuma — ADR em estado Aceito na primeira iteração.)

## Evidência / Implementação

| Arquivo | Mudança |
|---|---|
| `cmd/agent-sync/agents.go` | `agentSource.Invokes`; `parseAgent` lê `invokes: [...]` do frontmatter; `parseYAMLStringList` (tolerante a `[a, b, "c", ]`); `isOpenCodeGranular()` lê `AGENT_SYNC_OPENCODE_GRANULAR` (default ON); `openCodeGranularExtra` injeta `task:` block quando aplicável; `renderAgent` case "opencode" passa invokes como extraLine e injeta granular |
| `cmd/agent-sync/agents_test.go` | 6 testes novos cobrindo: parser YAML (8 subtests), parsing de `invokes:`, render read-only com deny global, render com allowlist + ordem correta, granular=0 mantém legacy, claude ignora campo, isOpenCodeGranular (8 subtests por valor de env) — todos passam |
| `agents/mr-reviewer.md` | `invokes: [code-reviewer, security-auditor, architecture-reviewer]` (preenchimento inicial, baseado em evidência do próprio corpo: linha 12 cita `code-reviewer`) |
| `agents/code-reviewer.md` | `invokes: [mr-reviewer, token-optimizer]` |
| `agents/architecture-reviewer.md` | `invokes: [mr-reviewer, code-reviewer, token-optimizer]` |
| `agents/security-auditor.md` | `invokes: [code-reviewer, token-optimizer]` |
| Branches | `feat/opencode-granular-permissions` (gerador) + `feat/agents-invokes-readonly` (preenchimento) |
| Commits | `0c924fd` + `e54b804` |
| Diff total | `+205/-0` (gerador) + `+4/-0` (4 agents) |

### Critério de aceite validado

- Suite completa verde (`go test ./...` no módulo agent-sync).
- `go vet` e `gofmt` limpos.
- Para cada um dos 4 agents read-only preenchidos, JSON OpenCode gerado contém:
  ```yaml
  permission:
    edit: deny
    bash: ask
    task:
      "*": deny
      "code-reviewer": allow   # ou outra referência
  ```
- Ordem do JSON: deny global antes dos allows (validado por asserção de índice de string).
- `AGENT_SYNC_OPENCODE_GRANULAR=0` desliga (validado em `TestRenderOpenCodeGranularDisabledKeepsLegacyOutput`).
- Claude/Cursor/Codex/Antigravity **não** emitem `invokes:` no frontmatter (validado em `TestRenderClaudeIgnoresInvokesField`).

### Preenchimento inicial — 4 de 11 agents

Baseado em evidência do próprio corpo de cada agent (não inferência):

| Agent | invokes declarado | Evidência |
|---|---|---|
| mr-reviewer | code-reviewer, security-auditor, architecture-reviewer | linha 12: "Use os critérios do `code-reviewer`" + checklist implícito |
| code-reviewer | mr-reviewer, token-optimizer | trabalha sobre diff (coleta) e bases grandes (leitura) |
| architecture-reviewer | mr-reviewer, code-reviewer, token-optimizer | diffs estruturais + padrões + navegação |
| security-auditor | code-reviewer, token-optimizer | sobrepõe padrão + leitura otimizada |

**7 agents ficam sem invokes nesta rodada** (preenchimento incremental conforme uso real sinalizar):

| Agent | Motivo |
|---|---|
| db-guardian | usa ferramenta binária, não delega |
| sentry-debugger | candidato futuro (próxima iteração) |
| spec-planner | candidato futuro (próxima iteração) |
| token-optimizer | é invocado PELOS outros, não chama |
| debugger (writable) | tem ferramentas próprias; delegar seria overhead |
| refactor-specialist (writable) | candidato futuro (test-engineer) |
| test-engineer (writable) | é invocado PELOS outros |

## Limites conhecidos

1. **Writables sem invokes continuam sem deny global**: `debugger` e `test-engineer` mantêm a regra atual (qualquer task permitida). Se quiser deny global para writable também, é outra decisão de shape (próxima iteração).
2. **Falta validação real em OpenCode**: a unidade testada é o JSON gerado, não o OpenCode consumindo-o. Smoke real (carregar agent em OpenCode e tentar invocar subagent proibido) **não foi executado** nesta sessão.
3. **Cobertura não migra automaticamente para outras CLIs**: Codex/Claude/Cursor continuam sem equivalente. Documentar limitações no README seria trabalho futuro.
4. **Preenchimento parcial**: 4 de 11 agents. `db-guardian`, `token-optimizer`, `debugger`, `test-engineer` ficaram sem invokes por falta de evidência de orquestração natural.

## Próximos passos

1. **Smoke real em OpenCode** (não-comitado): carregar agent com invokes declarado; tentar Task tool para subagent fora da allowlist; verificar deny. Smoke só fecha a ADR como "definitivo".
2. **Preenchimento iterativo** dos 7 agents restantes conforme uso real sinalizar orquestração natural. Candidatos óbvios: `sentry-debugger` (invoca `debugger` + `token-optimizer`), `spec-planner` (invoca `token-optimizer`), `refactor-specialist` (invoca `test-engineer`).
3. **Avaliar writable deny global**: se writable agents sem invokes estiverem abusando de Task tool, considerar deny * também para eles (mudança de shape).
4. **Cross-CLI equivalente**: pesquisar (B-E radar) se Codex/Claude introduzem `permission.task` ou equivalente; se sim, replicar a feature.

## Referências

- **Doc oficial OpenCode**: `https://opencode.ai/docs/agents/` (lida em 2026-09-19). Seção "Permissions" → `task` permission com glob patterns e regra "last matching rule wins".
- **ADR relacionada**: `docs/ADR-hooks-if-field-filter.md` (companion — cobre hooks, este cobre agents).
- **Memória de decisão**: `opencode-permissions-shape` em `memory-mcp` (3 decisões de shape + estado de preenchimento).
- **Pendência anterior**: este era gap conhecido não documentado formalmente; agora é feature com rollout controlado.
