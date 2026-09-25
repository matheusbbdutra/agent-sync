# Smoke Test T — C-4 OpenCode v2 plugins ctx-window (A-17)

> 2026-09-20 — protocolo imprimível para validar runtime dos 2 plugins
> v2 entregues em A-17: `ctx-window-nudge.opencode.v2.ts` (PostToolUse)
> e `ctx-window-summarize-at-stop.opencode.v2.ts` (compactation proxy).

## TL;DR

- **Wiração confirmada (verificada na sessão 2026-09-20, commit `ee9b742`):**
  - `cmd/agent-sync/hooks.go:449-462` — funções `syncOpenCodeCtxWindowNudgePlugin`
    e `syncOpenCodeCtxWindowSummarizeAtStopPlugin` registram os 2 plugins
    via `syncOpenCodePluginVersioned`.
  - `cmd/agent-sync/main.go:416-417` — entries `opencode-ctx-window-nudge`
    e `opencode-ctx-window-summarize-at-stop` no registry `standardHooks`.
  - Plugin v2 wirado em `~/.config/opencode/plugins/ctx-window-nudge.ts`
    (e `-summarize-at-stop.ts`).
  - Bug pré-existente corrigido no mesmo commit: sufixo `.opencode.v2.ts`
    reconhecido + dst sem duplicação.

## Verificação rápida do wirado (rodar antes do smoke)

```bash
# 1. Wirar OpenCode v2 plugins (re-aplica se necessário)
agent-sync -target opencode

# 2. Confirmar 2 plugins A-17 no destino
ls -la ~/.config/opencode/plugins/ctx-window-nudge.ts \
       ~/.config/opencode/plugins/ctx-window-summarize-at-stop.ts

# 3. Confirmar versão OpenCode
opencode --version   # esperado: opencode v2.0.11+

# 4. Confirmar contagem total de plugins wirados (sanity)
ls ~/.config/opencode/plugins/*.ts | wc -l   # esperado: 9+ (4 antigos + 4 novos + ctx-window-*)

# 5. Binário agent-sync com subcommand state (necessário para anexar snapshot)
agent-sync state render -root . | head -5
```

## Cenários (mínimo 2, ≥15min total)

### Cenário T1 — Nudge após ≥MIN_TOOL_CALLS (≈8min)

**Objetivo**: confirmar que `ctx-window-nudge.opencode.v2.ts` dispara
quando `tool_calls >= AGENT_SYNC_CTX_WINDOW_NUDGE_MIN_TOOL_CALLS` (80)
OU quando `summary.md` ausente/velho, e injeta nota one-shot no
`ctx.session.hook("context")`.

**Procedimento**:

1. Setup isolado em `/tmp/smoke-ctx-window-c4`:
   ```bash
   rm -rf /tmp/smoke-ctx-window-c4
   mkdir -p /tmp/smoke-ctx-window-c4/.agent-sync
   cd /tmp/smoke-ctx-window-c4
   git init -q
   git commit --allow-empty -q -m "init"
   ```

2. Wirar plugins v2 no OpenCode (caminho real do usuário, já feito por
   `agent-sync -target opencode` — plugins ficam em
   `~/.config/opencode/plugins/`):
   ```bash
   # Confirmar arquivos no destino antes de prosseguir
   ls ~/.config/opencode/plugins/ctx-window-nudge.ts
   ls ~/.config/opencode/plugins/ctx-window-summarize-at-stop.ts
   ```

3. Forçar threshold baixo para o smoke não precisar de 80 tool calls
   reais (encurtar tempo de teste):
   ```bash
   export AGENT_SYNC_CTX_WINDOW_NUDGE_MIN_TOOL_CALLS=3
   export AGENT_SYNC_CTX_WINDOW_NUDGE_MAX_AGE_HOURS=1   # qualquer summary.md com >1h dispara
   ```

4. Disparar OpenCode v2 com prompt que gera tool calls (ex.: tool de
   read/bash/write):
   ```bash
   cd /tmp/smoke-ctx-window-c4
   opencode run --standalone --auto --format json \
     "Liste todos os arquivos com extensão .md neste diretório" 2>&1 \
     | tee /tmp/opencode-c4-t1.log
   ```

5. Verificar:
   - Log contém referência a `agent-sync ctx-window-nudge` no system
     prompt (grep por "agent-sync ctx-window-nudge").
   - `~/.config/opencode/storage*` ou equivalente teve flag de nudge
     setada (CTX do plugin — verificar via log `-print-logs` se
     storage for SQLite).
   - **Critério passa**: system part com texto `[agent-sync
     ctx-window-nudge]` aparece após ≥3 tool calls.

**Evidência** (template YAML, salvar em
`docs/smoke-evidence/c4-ctx-window-nudge.md`):

```yaml
t1_ctx_window_nudge:
  date: 2026-09-20
  opencode_version: "2.0.11"
  threshold_overrides:
    min_tool_calls: 3
    max_age_hours: 1
  tool_calls_observed: <N>
  system_part_injected: pass | fail
  one_shot: pass | fail   # nao re-injetar em chamadas seguintes
  notes: |
    <livro de notas: timestamps, logs, inspecao>
```

### Cenário T2 — Summarize no compaction proxy (≈7min)

**Objetivo**: confirmar que `ctx-window-summarize-at-stop.opencode.v2.ts`
reage ao `ctx.session.hook("compaction")` (proxy OpenCode v2 para Stop
— SDK 2.0.11 não expoe `stop` nativo como hook) e injeta nota
one-shot via `ctx.session.hook("context")`.

**Procedimento**:

1. Reusar fixture `/tmp/smoke-ctx-window-c4` do Cenário T1.

2. Acionar compact manualmente via `opencode run` com `/compact`:
   ```bash
   cd /tmp/smoke-ctx-window-c4
   opencode run --standalone --auto --format json \
     "/compact" 2>&1 | tee /tmp/opencode-c4-t2.log
   ```

3. Verificar:
   - Hook `compaction` setou flag em storage (`ctx.storage.set`).
   - Próxima chamada de modelo (via segundo `opencode run`) recebe
     nota `[agent-sync ctx-window-summarize]` no `system` array.

4. Confirmar one-shot (segunda chamada NÃO recebe a mesma nota):
   ```bash
   opencode run --standalone --auto --format json "olá" 2>&1 \
     | tee /tmp/opencode-c4-t2b.log
   # Esperado: system NAO contém "agent-sync ctx-window-summarize"
   ```

**Evidência** (template YAML):

```yaml
t2_ctx_window_summarize_at_stop:
  date: 2026-09-20
  opencode_version: "2.0.11"
  compact_triggered: pass | fail
  storage_flag_set: pass | fail
  next_call_system_part: <texto contendo "ctx-window-summarize" ou vazio>
  one_shot: pass | fail
  notes: |
```

## Critérios de aceitação (todos devem passar)

| Critério | Descrição | Cenário |
| -------- | --------- | ------- |
| C1 | Plugin `ctx-window-nudge` wirado em `~/.config/opencode/plugins/` | setup |
| C2 | Plugin `ctx-window-summarize-at-stop` wirado no mesmo destino | setup |
| C3 | Nudge injeta system part após ≥MIN_TOOL_CALLS tool calls | T1 |
| C4 | Nudge é one-shot (sem re-injeção em chamadas seguintes na mesma sessão) | T1 |
| C5 | Compact proxy dispara summarize-at-stop | T2 |
| C6 | Summarize-at-stop é one-shot | T2 |

## Anti-reincidência

- **Storage do OpenCode v2**: ABI pode mudar entre versões; se `ctx.storage`
  não funcionar como esperado, registrar como gap e cair para D-32 plano
  de fallback (`false-success-guard` no bash).
- **Threshold override**: sempre passar `AGENT_SYNC_CTX_WINDOW_NUDGE_MIN_TOOL_CALLS`
  baixo (ex.: 3) no smoke para não depender de 80 tool calls reais.
- **One-shot via `storage.remove`**: confirmar que a chave foi realmente
  removida entre chamadas (storage persistente pode ter quirks).
- **OpenCode v2 `compaction` hook**: pode mudar de nome em versões
  futuras; verificar log `-print-logs` se hook não disparar.

## Pendências separadas (não escopo deste smoke)

- **Spawn do binário `ctx-window summarize`**: não acontece no plugin v2
  (rede de segurança no bash hook wirado em Claude/Codex/Antigravity/Cursor).
  Plugin v2 é observability + advisor injetado no system prompt, não executor.
- **Smoke runtime equivalente para os 4 CLIs bash-wirados**: separado
  (já temos `SMOKE-TEST-R.md` parcial para Cursor).
- **Hardening contra `ctx.session.hook("compaction")` ausente** em
  builds customizadas do OpenCode: registrada como gap, não bloqueia
  aceitação.
