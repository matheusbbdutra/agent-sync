# Smoke Test S — PreCompact cross-CLI (C-1, ADR-precompact-snapshot-cross-cli)

> 2026-09-20 — protocolo imprimível para validar o wirar do hook
> PreCompact cross-CLI em Claude Code, Codex, OpenCode v2 e Antigravity
> (Cursor fica observacional, gap aceito na ADR Decisão 4).

## TL;DR

- **Wiração confirmada (verificada na sessão 2026-09-20):**
  - `cmd/agent-sync/hooks.go` — função `syncContextSnapshotHook` registra
    `PreCompact` em Claude Code e Codex; `PreInvocation` em Antigravity;
    **não** wirar novo no Cursor (já tem observacional via
    `syncCtxCompactHook`).
  - `cmd/agent-sync/main.go:392` — entry `precompact-snapshot` no registry
    `standardHooks`.
  - `cmd/agent-sync/precompact_snapshot.go` — subcommand
    `agent-sync state snapshot` com flags `-actor`, `-kind`, `-cli`,
    `-trigger`, `-to`.
  - `hooks/precompact-snapshot.opencode.v2.ts` — plugin v2 (OpenCode v1
    silencioso; OpenCode v2 wirado via `syncOpenCodePluginVersioned`).
  - `tools/jsonschema/schemas/precompact-snapshot.json` — schema v1.0
    embedded no binário (validado em runtime pelo subcommand).
  - Binário reinstalado em `~/.local/bin/agent-sync` (subcommand
    `state snapshot` funcional, verificado em 2026-09-20 contra o
    SessionState real — devolveu `decision: advise_only` por causa dos
    blockers B-1/B-2 no JSON).

## Verificação rápida do wirado (rodar antes do smoke)

```bash
# 1. Subcommand funciona
agent-sync state snapshot -root . -actor claude -kind auto | head -20
# esperado: JSON com schema_version 1.0, decision allow|block|advise_only

# 2. Plugin OpenCode v2 instalado em ~/.config/opencode/plugins/
ls -la ~/.config/opencode/plugins/precompact-snapshot.ts
# esperado: arquivo presente (link/copy de hooks/precompact-snapshot.opencode.v2.ts)

# 3. Hook PreCompact wirado em ~/.claude/settings.json (Claude Code)
grep -A 3 '"PreCompact"' ~/.claude/settings.json | head -10
# esperado: agent-sync-precompact-snapshot registrado

# 4. Hook PreCompact wirado em Codex
grep -A 3 '"PreCompact"' ~/.codex/settings.json | head -10
# esperado: agent-sync-precompact-snapshot registrado

# 5. Schema embedded no binário (validacao end-to-end)
go test ./tools/jsonschema/... -run TestPrecompact -v
# esperado: PASS
```

## Cenários (mínimo 3, ≥15min total)

### Cenário S1 — Claude Code `PreCompact` auto (≈5min)

**Objetivo**: confirmar que o hook PreCompact wirado emite o snapshot
canônico e o harness Claude Code aceita `exit 0` (não bloqueia) +
payload no stdout.

**Procedimento**:

1. Iniciar sessão Claude Code no repo agent-sync.
2. Disparar 5 tool calls (`Edit`, `Bash`, etc.).
3. Forçar auto-compact (`/compact` no Claude Code, modo `auto`).
4. Verificar:
   - `~/.cache/agent-sync/precompact/claude-<sessionID>.json` criado.
   - `cat .agent-sync/session-event.jsonl | jq '.kind'` mostra última
     linha com `kind=decision` ou `action` (o subcommand não grava
     evento automaticamente; smoke de evento é separado).
   - **Critério passa**: snapshot gerado, JSON parseável, schema válido.

**Evidência** (template YAML, salvar em `docs/smoke-evidence/s1-precompact-claude.md`):

```yaml
s1_claude_precompact:
  date: 2026-09-20
  tool_calls_before_compact: 5
  snapshot_path: ~/.cache/agent-sync/precompact/claude-<sessionID>.json
  decision: allow | block | advise_only
  schema_valid: pass | fail
  notes: |
```

### Cenário S2 — Codex `PreCompact` manual (≈5min)

**Objetivo**: confirmar wrapper `{"continue": false, ...}` quando
`decision=block` (Codex exige JSON estrito) e payload puro quando
`allow/advise_only`.

**Procedimento**:

1. Criar SessionState com open_question Q-1 em aberto (forçar decisão
   `advise_only` ou `block` se blocker registrado).
2. Wirar hook no Codex (`agent-sync -apply` com target=codex).
3. Disparar compact manual no Codex (`/compact` no menu).
4. Verificar:
   - stdout do hook retorna JSON válido (não erro de parsing do harness).
   - **Critério passa**: Codex aceita o payload, não trava.

**Evidência** (template YAML):

```yaml
s2_codex_precompact:
  date: 2026-09-20
  open_question_forcado: Q-1
  expected_decision: advise_only
  actual_decision: allow | block | advise_only
  codex_stdout_json: pass | fail
  notes: |
```

### Cenário S3 — OpenCode v2 plugin (≈5min)

**Objetivo**: confirmar que o plugin v2 wirado injeta o snapshot via
`ctx.session.hook("context", ...)` quando o OpenCode decide compactar.

**Procedimento**:

1. Iniciar sessão OpenCode v2 no repo agent-sync.
2. Disparar compact nativo (`/compact` no OpenCode).
3. Verificar:
   - Plugin carregado (`~/.config/opencode/plugins/precompact-snapshot.ts`
     presente).
   - Hook `compaction` seta flag em `ctx.storage` (inspecionar
     `~/.local/share/opencode/storage.db` ou equivalente).
   - Próxima chamada de modelo recebe `system.push` com nota do
     snapshot.
   - **Critério passa**: SystemPart injetada, plugin one-shot
     (sem re-injeção em chamadas futuras).

**Evidência** (template YAML):

```yaml
s3_opencode_v2:
  date: 2026-09-20
  opencode_version: "2.0.11"  # confirmar antes
  plugin_loaded: pass | fail
  storage_flag_set: pass | fail
  system_part_injected: pass | fail
  one_shot: pass | fail
  notes: |
```

## Critérios de aceitação (todos devem passar)

| Critério | Descrição | Cenário |
| -------- | --------- | ------- |
| C1 | `agent-sync state snapshot` gera payload válido contra schema embedded | S1 |
| C2 | Hook Claude Code PreCompact wirado e invocado | S1 |
| C3 | Hook Codex PreCompact wirado e payload JSON aceito pelo harness | S2 |
| C4 | Plugin OpenCode v2 wirado e injeção de system part funciona | S3 |
| C5 | Decisão `advise_only` quando há open_question em aberto | S1+S2 |
| C6 | Block nunca ocorre com blockers sem `UpdatedAt` (conservador) | S1+S2 |

## Anti-reincidência

- **Schema payload**: sempre validar contra
  `tools/jsonschema/schemas/precompact-snapshot.json` antes de incluir
  exemplo no doc (mesma lição de D-24 — não confiar em suposição).
- **Codex JSON estrito**: nunca emitir texto puro em stdout de hook
  PreCompact no Codex; sempre JSON.
- **OpenCode v2 ABI**: se o typecheck do plugin quebrar após upgrade
  do `@opencode/plugin`, ver `agent-react-nudge.v2.ts:1-17` para o
  pattern de namespace `Plugin`.

## Pendências separadas (não escopo deste smoke)

- **Cursor block nativo**: gap aceito (ADR Decisão 4); só reabri quando
  Cursor publicar doc de block ou agent-sync virar gate.
- **Antigravity PreInvocation proxy**: wirado automaticamente via
  `syncAntigravityPreInvocation`; smoke dedicado em separado (B-2
  histórico).
- **Event log kind `precompact_snapshot`**: bump minor do schema
  `session-event.json` pendente (Decisão 5 da ADR); não implementado
  nesta entrega — `false-success-guard` vai ganhar o novo kind em
  entrega subsequente.
