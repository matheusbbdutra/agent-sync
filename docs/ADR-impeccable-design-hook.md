# ADR — Impeccable design hook no OpenCode e Antigravity (A-100)

- **Status**: **Superado** (removido em 2026-11-16, D-141). A integração dependia da skill de
  terceiros `impeccable` (não gerida neste repo, ausente de `~/.agents/skills`), com risco de
  comportamento indefinido quando instalada em versão divergente. O hook fail-open escondia a
  falha em vez de expor. Removidos: `hooks/impeccable-design.{opencode.v2.ts,antigravity.sh}`,
  `internal/hooks/{impeccable_runtime,apply_impeccable_design}.go` + testes, as 2 entradas em
  `internal/hooks/apply_table.go` e o subcomando `agent-sync hook impeccable`.
- **Data**: 2026-11-10
- **Decisor**: usuário + agente (ses_atual)
- **Fonte**: `~/.config/opencode/skills/impeccable/reference/hooks.md`,
  `~/.gemini/antigravity-cli/builtin/skills/agy-customizations/docs/hooks.md`,
  `crates/hook/src/hook_lib.rs` do Impeccable 4.8.2 (contrato do subcomando
  interno `impeccable hook`)
- **Tags**: impeccable, design-hook, opencode, antigravity, detector

## Contexto

O Impeccable instala hook per-edit nativo apenas em Claude Code, Codex, Cursor,
GitHub Copilot, Grok Build e Hermes Agent. OpenCode e Antigravity ficam sem
checagem automática: o detector só roda se o agente lembrar de executar
`impeccable detect`/`audit` ao final.

O engine já está instalado junto da skill nas duas CLIs. O subcomando interno
`impeccable hook` lê um payload estilo Claude, faz o scan por arquivo, aplica o
tier per-edit, dedupe por sessão, supressão após N edições e respeita
`.impeccable/config.json` (`hook.enabled`, `hook.quiet`). Medição local: ~14 ms
por edição.

## Decisão

Criar um **bridge Go** (`agent-sync hook impeccable --host=opencode|antigravity`)
que normaliza o evento de cada host e delega ao binário oficial da skill. Nenhuma
regra do detector é reimplementada.

1. **Gate de ativação**: skill instalada no path do host **e** projeto com
   `.impeccable/`, `PRODUCT.md` ou `DESIGN.md` (opt-in real). Sem skill/projeto
   inicializado, é no-op.
2. **OpenCode**: plugin TS `impeccable-design.opencode.v2.ts` observa
   `ctx.tool.hook("execute.after")`, chama o bridge e injeta
   `{"context": "..."}` via `ctx.session.hook("context")` no próximo model call.
3. **Antigravity**: um script fino é wirado em dois eventos:
   - `PostToolUse` (`matcher: "*"`): o bridge escaneia o arquivo, grava os
     findings em `~/.cache/agent-sync/impeccable/<conversationId>.txt` e
     responde `{}` (o contrato do evento só aceita `{}`).
   - `PreInvocation`: o bridge drena o pending e responde
     `{"injectSteps":[{"systemMessage":{"systemMessage":"..."}}]}`.
4. **Fail-open**: skill ausente, projeto sem opt-in, timeout (5 s), JSON
   inválido ou erro do detector retornam `{}` e nunca bloqueiam o turno.
5. **Higiene**: acks de "nenhum problema" são descartados; findings reais e
   re-nudges de pendência são preservados. Pending expira em 1 h.

## Consequências

**Positivas**
- OpenCode e Antigravity passam a receber o mesmo sinal determinístico do
  Impeccable, sem custo de API e sem duplicar as 62 regras.
- Atualizações do engine via `npx impeccable update` continuam funcionando; o
  bridge usa o launcher instalado, não uma cópia.

**Riscos / limites**
- `impeccable hook` é subcomando interno (não listado no `--help`). Se o
  contrato mudar, o bridge falha aberto; os testes cobrem o shape atual.
- No Antigravity, `PostToolUse` não injeta contexto; a entrega depende do
  `PreInvocation` seguinte. O shape `systemMessage` está documentado, mas não
  foi smokeado em turno real de modelo nesta entrega.
- O hook roda o detector no path do arquivo editado; projetos com
  `.impeccable` apenas em subdiretórios não são cobertos pelo gate no root.

## Validação

- `go test ./...` (raiz + tools) e `go vet ./...` verdes.
- Testes unitários do bridge: OpenCode/antigravity, gate, clean ack, pending
  com TTL, sanitização de session, extração de paths e idempotência do wiring.
- Harness isolado do plugin OpenCode com o binário real: finding
  `gradient-text` injetado e consumido uma única vez.
- Script wirado do Antigravity executado com payload simulado: `{}` no
  PostToolUse, pending criado, `injectSteps` no PreInvocation e pending drenado.
- `agy -p "/hooks"` lista `agent-sync-impeccable-design` em `PreInvocation` e
  `PostToolUse`.
