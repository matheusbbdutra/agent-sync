# ADR: Hooks de fim de turno (Cursor `stop` + OpenCode `session.idle`)

**Status**: Re-Proposto (Trilha A wirada+testada; Trilha B ainda hipótese; revisão: 2026-10-23; D-64 em 2026-09-23)
**Data**: 2026-09-17
**Decisor**: Matheus Dutra
**Tags**: ai-agent, hooks, observability, agent-react

## Contexto

O agent-sync instala nudges (`agent-react-nudge`, `context-guard-nudge`, `memory-nudge`, `docs-cache`, `false-success-guard`) e o pipeline de compactação (`ctx-window`) em até 5 CLIs (Claude Code, Codex, OpenCode, Cursor, Antigravity). O contrato `agente-react` exige **validação de hipótese ativa antes de concluir** — atualmente reforçado por um lembrete periódico em `postToolUse` (Cursor/Codex/Claude/Antigravity) ou `tool.execute.after` (OpenCode).

Três limitações conhecidas motivam esta ADR:

1. **Cursor não tem cobertura de fim de turno**. Hook `stop` existe oficialmente (`cursor.com/docs/hooks`), mas não está sendo usado pelo agent-sync — só `postToolUse`, `afterMCPExecution` e `beforeShellExecution`. A pendência está registrada em `STATE.md:27` ("Garantia semântica plena exige camada extra (ex. `stop`/`afterAgentResponse` no Cursor) — não implementada ainda").

2. **OpenCode está preso na issue #13574** (anomalyco/opencode). Mutações de `output.output` no hook `tool.execute.after` nem sempre chegam ao modelo — os 4 plugins OpenCode do agent-sync (context-guard, memory, agent-react, ctx-compact) operam exatamente nesse hook e portanto são **best-effort** por design. O OpenCode oferece oficialmente outros eventos (`session.idle`, `session.compacted`, `message.updated`, `tool.execute.before`, `permission.asked`) — não explorados.

3. **Cobertura assimétrica entre CLIs**. Cursor e Claude têm `Stop`-like nativo; Codex tem `agent-turn-end`; OpenCode só tem `tool.execute.after` confiável. Sem padronizar, o `agent-react-nudge` lembra a cada N tool calls numa CLI e fica mudo no fim do turno de outra — a "garantia semântica plena" citada no STATE.md:27 nunca é atingida.

## Decisão

Adotar a estratégia de **hook de fim de turno por CLI**, com **fallback de hook pré-ferramenta** quando o evento nativo de fim de turno não existe ou não pode injetar mensagem de volta no contexto do modelo.

### Trilha A — Cursor `stop` (implementar agora)

Criar `hooks/agent-react-nudge.stop.cursor.sh` registrado no evento `stop` de `~/.cursor/hooks.json` (via `cursorManagedHooks()` em `cmd/agent-sync/cursor.go`):

- **Matcher**: `"Stop"` (regex contra o valor `Stop`, conforme doc oficial).
- **Ação**: emitir `{"followup_message": "[agent-sync] Hipótese ativa sem validação? ..."}` no stdout (campo documentado em `cursor.com/docs/hooks`).
- **`loop_limit`**: 5 (default; evita loop se o próprio followup virar nova parada — Cursor respeita o limite e para de chamar o hook).
- **Idempotência**: deduplicar contra `~/.cache/agent-sync/react-nudge/<conversation_id>.last` — só emite se a sessão recebeu ≥ 1 tool call desde o último nudge (evita poluir turnos triviais).
- **Coexistência**: o `agent-react-nudge.cursor.sh` existente em `postToolUse` (lembrete a cada N tool calls) **permanece** — `stop` é complemento, não substituto. `stop` é a rede de segurança final; `postToolUse` é a cadência periódica. No Claude Code e Codex, o `postToolUse` foi refinado em 2026-09-19 com filtro `if` restrito a tools de edição (`Edit(*)|Write(*)|MultiEdit(*)|NotebookEdit(*)`) — ver `docs/ADR-hooks-if-field-filter.md`. Cursor não tem equivalente ao campo `if`; o filtro continua sendo puramente no script `.sh`.

Migrar `false-success-guard` (já ativo em Claude `Stop`) para Cursor `stop` — espelha a cobertura. Schema do payload já é conhecido (campo `transcript_path` na base; campo `hook_event_name`).

### Trilha B — OpenCode `session.idle` (validar antes de implementar)

Substituir (ou somar a) o `tool.execute.after` no plugin `agent-react-nudge.opencode.ts` por escuta no evento `session.idle` (documentado em `opencode.ai/docs/plugins/`):

```ts
"session.idle": async (input, output) => {
  // emitir lembrete — mas como?
}
```

**Risco não verificado**: a documentação do OpenCode lista o evento mas **não explicita se o callback pode injetar texto no contexto do modelo**. O `tool.execute.after` permitia mutação de `output.output` (com a limitação do #13574); `session.idle` pode ser puramente observacional. Antes de implementar, **rodar um teste real** (não assumi):

1. Plugin TS mínimo que loga entrada/saída de `session.idle`.
2. Verificar se algum campo da resposta vira mensagem do usuário ou do sistema na próxima request.
3. Se sim: portar o nudge. Se não: tratar `session.idle` apenas como gatilho para **observabilidade** (contador em `~/.cache/agent-sync/observability/opencode-sessions.jsonl`) e manter o nudge em `tool.execute.after` como hoje.

### Trilha C — OpenCode `session.compacted` (validar; útil para ctx-window)

Quando o OpenCode decide compactar autonomamente, o `ctx-window` em Go **não é notificado** — perde a chance de gerar sumário semântico. Hook em `session.compacted` fecha o gap. Mesma dependência de validação da Trilha B (precisa confirmar o que o callback pode fazer nesse momento).

### Trilha D — Cursor `postToolUseFailure` e `afterAgentResponse` (oportunidades)

Sem dependência técnica forte, ficam em **próximo passo opcional**:

- `postToolUseFailure`: sinal de falha de ferramenta (espelha o tool-status marker que o `ctx-window` Go já emite para Claude/Codex via paper 2609.14758).
- `afterAgentResponse` / `afterAgentThought`: observabilidade fina — quando `false-success-guard` flagga uma afirmação de sucesso, ter acesso à resposta completa do agente ajuda diagnóstico.

Não bloqueiam o objetivo principal (validação de hipótese no fim do turno) — podem ser puxadas em ADR futura se a observação empírica justificar.

## Consequências

### Positivas

- **Cobertura fim-de-turno nas 5 CLIs** (Trilhas A + B, se B validar): Claude já tem `Stop` (false-success-guard ativo), Cursor ganha `stop`, Codex tem `agent-turn-end` (a validar quando virar prioridade), Antigravity tem `afterAgentResponse` (a validar), OpenCode tenta `session.idle`.
- **Reduz dependência do `tool.execute.after` no OpenCode** (Trilha B): se `session.idle` aceita injeção, saímos do best-effort imposto pelo #13574 para o `agent-react-nudge`.
- **Auditoria**: nudge no fim do turno vira ponto único para cross-CLI, em vez de cadência por N tool calls (que polui turnos triviais).
- **Coerência com paper 2609.10209v1**: o próprio paper cita que agentes melhoram quando recebem âncoras explícitas no boundary de turno — `stop`/`session.idle` são exatamente esses boundaries.

### Negativas / trade-offs

- **Risco Trilha B**: se `session.idle` não permitir injeção de mensagem, gastamos esforço de investigação sem ganho funcional. Mitigação: investigação via teste real em plugin mínimo antes de portar — não implementar com base em hipótese.
- **`followup_message` no Cursor vira nova mensagem do usuário na próxima request** (confirmado na doc): pode alterar o tom do turno seguinte. Aceitável — o lembrete é semanticamente equivalente a uma pergunta do usuário.
- **`loop_limit` precisa ser bem afinado**: default 5 (doc oficial) é defensável; loop só acontece se o nudge gerar nova parada, o que ele não gera diretamente — `followup_message` é processado como user, não como nova parada.
- **Mais um hook por CLI**: aumenta a matriz de manutenção. Mitigação: factor o payload parsing de Cursor/OpenCode/Codex no `ctx-window` Go (já temos o padrão `ctx-window hook <cli>` para Claude/Codex/Cursor/Antigravity) e usar `--cli cursor` ou `--cli opencode` para reusar.

## Decisões revisadas

(nenhuma ainda — ADR em estado Proposto)

## Evidência consultada

| Fonte | URL | Confirmado |
| --- | --- | --- |
| Cursor docs — Hooks | https://cursor.com/docs/hooks | eventos disponíveis, schema base, `followup_message`, `loop_limit`, matcher por evento |
| OpenCode docs — Plugins | https://opencode.ai/docs/plugins/ | eventos disponíveis (lista completa de session/tool/message/etc.) |
| Issue upstream | https://github.com/anomalyco/opencode/issues/13574 | limitação de `tool.execute.after` (já referenciada em `ADR-context-window-strategy.md`) |
| STATE.md | linhas 27 e 54 | pendência explícita do `stop` no Cursor e do best-effort do OpenCode |

**Não verificado** (rótulo explícito):

- Se `session.idle` do OpenCode permite injeção de mensagem de volta no contexto. A doc não diz; precisa de teste real. Decisão de implementar depende do resultado.
- Se `session.compacted` permite o mesmo. Idem.
- Como `agy` (Antigravity) trata o conceito de "fim de turno" — não consultado nesta rodada. Pendência separada.

## Implementação proposta (esqueleto)

### Trilha A — Cursor `stop`

| Arquivo | Mudança |
| --- | --- |
| `hooks/agent-react-nudge.stop.cursor.sh` | novo: emite `followup_message` no fim do turno, dedup por `conversation_id` |
| `cmd/agent-sync/cursor.go` | `cursorManagedHooks()` adiciona `{Event: "stop", Script: "agent-react-nudge.stop.cursor.sh", Matcher: "Stop", LoopLimit: 5}` — atenção: o struct atual não tem `LoopLimit`; adicionar campo |
| `hooks/false-success-guard` (Cursor) | espelhar o hook Stop já ativo no Claude — avaliar se é o mesmo binário Go ou se precisa de wrapper Cursor |
| `docs/ADR-fim-de-turno-hooks.md` | esta ADR |
| `STATE.md` | referenciar ADR e registrar status |

### Trilha B — OpenCode `session.idle`

| Arquivo | Mudança |
| --- | --- |
| `hooks/agent-react-nudge.opencode.ts` | somar handler `"session.idle"` ao plugin existente (não substituir `tool.execute.after` ainda) |
| teste real (manual) | plugin mínimo que loga entrada/saída de `session.idle` antes de portar lógica |

### Trilha C — `session.compacted`

Depende do resultado da Trilha B.

## Limites conhecidos

1. **Hook `stop` no Cursor não foi testado de verdade** nesta sessão — só a documentação foi consultada. Antes de fechar a ADR como Aceita, rodar smoke real (mesmo padrão da Fase 5 do `ctx-window` em `STATE.md`).
2. **Risco de `loop_limit`**: o Cursor permite `null` para sem limite; vamos usar o default 5 e ajustar empiricamente se o nudge virar loop.
3. **OpenCode `session.idle`**: hipótese de injeção não validada — Trilha B pode resultar em "manter `tool.execute.after` mesmo" sem ganho.

## Status Re-Proposto (2026-09-23, auditoria D-64)

**Trilha A (Cursor `stop`) — ACEITA localmente:** hook `hooks/agent-react-nudge.stop.cursor.sh` wirado em `~/.cursor/hooks.json` (campo `stop` com `loop_limit: 5`) e validado por `internal/hooks/hooks_cursor_apply_test.go:100-105` (TestCursorStopHookWirado). Comando segue schema oficial `{"followup_message": "..."}`. Wirar também inclui `agent-stop.cursor.sh` (false-success-guard espelhado).

**Trilha B (OpenCode `session.idle`) — HIPÓTESE NÃO VALIDADA:** ADR declara em `## Evidência consultada / Não verificado` (linha 96) que "a doc não diz se callback pode injetar mensagem". `grep -rn "session.idle" hooks/ internal/ --include='*.go' --include='*.ts' --include='*.sh'` retorna 0 hits em código real (só citação em `internal/state/render_cross_test.go:78` como string esperada em teste cross-CLI). OpenCode ainda opera via `tool.execute.after` com a limitação issue #13574 registrada.

**Trilha C/D — pendentes** (Trilha C depende da B).

**Próximo passo concreto:** rodar plugin TS mínimo de teste (conforme plano na linha 47 do ADR) com log de `session.idle`, antes de implementar — não assumir capacidade de injeção.

Revisão: 2026-10-23 (1 mês) — se Trilha B não for validada + implementada, manter `session.idle` como observabilidade e fechar ADR como Aceito com ressalva (ou Descartar parcial).

## Próximos passos

1. **Implementar Trilha A** (baixo risco, alta utilidade) — smoke test real antes de fechar a ADR.
2. **Investigar Trilha B** com plugin mínimo — se validar, portar; se não, registrar achado em memória e considerar `message.updated` ou `tool.execute.before` como complemento ao `tool.execute.after` atual.
3. **Avaliar Trilha C** como bônus, na esteira da B.
4. **Fechar a ADR como Aceita/Descartada** com smoke real de cada trilha implementada — não marcar Aceita só pela decisão de arquitetura.

## Referências

- **Cursor Hooks**: https://cursor.com/docs/hooks (puxado em 2026-09-17; lista completa de eventos, schema, `followup_message`, `loop_limit`).
- **OpenCode Plugins**: https://opencode.ai/docs/plugins/ (puxado em 2026-09-17; lista de eventos `session.*`, `tool.*`, `message.*`, `permission.*`, etc.).
- **ADR relacionada**: `docs/ADR-context-window-strategy.md` (cita o limite do OpenCode #13574).
- **Issue upstream**: https://github.com/anomalyco/opencode/issues/13574.
- **Papers adotados** (citados pela decisão):
  - arXiv [2609.14758](https://arxiv.org/abs/2609.14758) — *Fabrication After Tool Failure* (tool-status marker).
  - arXiv [2606.09863v1](https://arxiv.org/abs/2606.09863v1) — false-success detector (já implementado em `tools/cmd/false-success-guard/`).
- **Pendências correlatas no STATE.md**: linhas 27 (camada extra fim de turno) e 54 (OpenCode best-effort).
