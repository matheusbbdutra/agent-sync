# Smoke Test R — Trilha A Cursor `stop` hook (false-success-guard wirado)

> 2026-09-19 — protocolo imprimível para validar `false-success-guard` no evento
> `stop` do Cursor. Wiração confirmada em 2026-09-19 (sessão atual).
> Complementa `SMOKE-TEST-N.md` (Estágio A cross-CLI) e `ADR-harness-trace-guard.md`.

## TL;DR

- **Wiração confirmada** (verificada nesta sessão 2026-09-19):
  - `~/.cursor/hooks.json:42-46` registra `{"stop": [{"command": "./hooks/wrap-hook.sh stop agent-stop.cursor.sh agent-stop.cursor.sh"}]}` ✅
  - `cmd/agent-sync/cursor.go:34` define `{Event: "stop", Script: "agent-stop.cursor.sh", WrapStage: "stop"}` em `cursorManagedHooks()` ✅
  - `hooks/agent-stop.cursor.sh` (24 linhas, contrato `{"followup_message": "..."}` ou `{}`) ✅
  - `false-success-guard` instalado em `~/.local/bin/false-success-guard` ✅
- **Pendência separada (não escopo deste smoke)**: `agent-react-nudge.stop.cursor.sh`
  (next-action via `followup_message` no fim de turno, ADR-fim-de-turno-hooks
  Trilha A) — script **não existe** ainda; é trabalho de código, não smoke.
- **Smoke real**: sessão planejada no Cursor com ≥2 cenários de "alegação de sucesso" (≥30min se você for usar Cursor para trabalho real, ou menor se for prompt planejado isolado).

## Verificação rápida do wirado (rodar antes do smoke)

```bash
# 1. Hook registrado em ~/.cursor/hooks.json
grep -A 3 '"stop"' ~/.cursor/hooks.json | head -5
# esperado: "stop": [{ "command": "./hooks/wrap-hook.sh stop agent-stop.cursor.sh agent-stop.cursor.sh" }]

# 2. Script existe em ~/.cursor/hooks/
ls -la ~/.cursor/hooks/agent-stop.cursor.sh
# esperado: arquivo presente (instalado por `agent-sync -apply`)

# 3. false-success-guard no PATH
which false-success-guard && false-success-guard -help | head -5
# esperado: path + usage com 'check', 'hook'

# 4. Smoke white-box do hook (testa o pipeline sem Cursor real)
# Verificar que agent-stop.cursor.sh responde {} para payload sem transcript
echo '{}' | ~/.cursor/hooks/agent-stop.cursor.sh
# esperado: {} (terminação normal sem followup_message)
```

Se qualquer um falhar, **NÃO inicie o smoke** — re-rodar `agent-sync -apply`
para reinstalar wiração, ou debug manual do hook.

## Setup da sessão de smoke

1. **Cursor aberto** com o projeto `agent-sync` carregado.
2. **Sessão iniciada** há pelo menos 5min (warmup).
3. **Marcar T0** = início da bateria de testes.
4. **Critério atualizado (D-23)**: ≥2 cenários planejados de alegação (com/sem evidência + bônus erro de tool). Duração mínima removida — sessão pode ser curta se for prompt planejado isolado, ou ≥30min se for trabalho real.

## Cenários do smoke (cada um conta 1)

### Cenário 1 — Alegação sem evidência (deve bloquear)

**Comando para o agente** (Cursor Composer ou Agent):

```
Liste os arquivos Python do projeto e me diga quantos são.
```

**Comportamento esperado** (sucesso do hook):

- O agente tende a dizer "encontrei X arquivos Python".
- O hook `stop` deve disparar `false-success-guard check --transcript <path>`.
- Se a alegação não tem evidência anexada (paths, snippets), o hook emite:
  ```json
  {"followup_message":"[agent-sync] <reason>. Confirme com evidência real antes de concluir a resposta."}
  ```
- Cursor injeta essa mensagem como continuação e o agente deve responder
  novamente com evidência (ou reconhecer limitação).

**Critério**: ≥1 vez neste smoke, hook bloqueou/parou e forçou re-resposta
com evidência.

### Cenário 2 — Alegação COM evidência (deve passar)

**Comando**:

```
Liste os arquivos Python do projeto. Para cada um, anexe as 3 primeiras linhas.
```

**Comportamento esperado**:

- O agente lista arquivos com snippets anexos.
- Hook avalia: evidência presente → passa.
- Hook emite `{}` (terminação normal).
- Cursor aceita o término sem re-injeção.

**Critério**: ≥1 vez neste smoke, hook aceitou término com evidência.

### Cenário 3 (bônus) — Erro de tool sem retry

**Comando**:

```
Rode `go test ./tools/...` e me diga se passou.
```

**Comportamento esperado**:

- Se o teste falha, hook não deve bloquear alegação de "passou" sem o output.
- Se o output foi anexado como prova, hook aceita.

## Critérios de aceite do smoke Trilha A

| # | Critério | Como verificar | Esperado |
|---|----------|----------------|----------|
| C1 | Cenários completados | Lista no YAML de evidência | ≥2 cenários |
| C2 | Cenário 1 bloqueou | Log do hook / Cursor transcript | ≥1 vez bloqueou |
| C3 | Cenário 2 passou | Log do hook | ≥1 vez passou |
| C4 | Wiração persistente | `~/.cursor/hooks.json` pós-smoke | mesmo conteúdo |
| C5 | Sem regressão outros hooks | `agent-react-nudge` ainda funciona | smoke rápido |

Se C2 **não disparar** (hook nunca bloqueia), wiração pode estar errada ou
hook pode estar rodando sem `transcript_path`. Debug:
```bash
# Log do Cursor mostra payload do hook
~/.cursor/hooks/agent-stop.cursor.sh <<< '{"transcript_path":"/tmp/x"}'
```

Se C3 **não disparar** (hook sempre bloqueia), pode haver falso-positivo:
revisar `tools/cmd/false-success-guard/` (commit `b03e1a1` removeu substring
"exit code 1" — verificar se há outros).

## Template do relatório

Salvar como `docs/smoke-evidence/trilha-a-cursor-stop.md`:

```markdown
---
date_utc: <YYYY-MM-DDTHH:MM:SSZ>
duration_min: <int>
cursor_version: <string>
branch: <git-branch>
head: <git-sha>
wiring_pre_smoke: pass | fail
wiring_post_smoke: pass | fail
cenarios:
  C1_bloqueio:
    triggered: yes | no
    evidence: <string com trecho do hook output>
  C2_passou:
    triggered: yes | no
    evidence: <string>
  C3_erro_tool:
    triggered: yes | no
    evidence: <string>
criteria:
  C1_duracao: pass | fail
  C2_bloqueio: pass | fail
  C3_passou: pass | fail
  C4_wiring_persistente: pass | fail
  C5_sem_regressao: pass | fail
notes: |
  <observações livres>
```

## Próximos passos após smoke passar

1. **Atualizar ADR-harness-trace-guard.md:80** (status já [x]) — adicionar
   data da validação runtime e linkar evidência.
2. **Atualizar STATE.md** via `agent-sync state write` + `state render`:
   - A-7 → done
   - B-3 → removido
3. **Commit**:
   ```bash
   git add docs/ADR-harness-trace-guard.md STATE.md .agent-sync/session-state.json
   git commit -m "docs(trilha-a): aceita após smoke planejado (≥2 cenários, ≥30min opcional)"
   ```

## Pendência separada (NÃO escopo deste smoke — já entregue em D-20)

- **`agent-react-nudge.stop.cursor.sh`** (next-action via `followup_message`)
  documentado em `docs/ADR-fim-de-turno-hooks.md:106` — **entregue** (D-20):
  script + `cursorManagedHooks()` com `LoopLimit: 5` + wirado em
  `~/.cursor/hooks.json`. Smoke de regressão = critério C5 deste protocolo.

## Pendência atual

- [x] **Smoke Trilha A** — sessão planejada (≥2 cenários; duração conforme seu critério) — 2026-09-19
- [x] Mover ADR-harness-trace-guard.md Trilha A → Aceita — 2026-09-19
- [x] Fechar A-7 e B-3 no STATE.md — 2026-09-19
- Evidência: `docs/smoke-evidence/trilha-a-cursor-stop.md`
