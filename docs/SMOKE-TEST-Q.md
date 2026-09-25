# Smoke Test Q — ADR-003 smoke em sessão planejada (critério atualizado por D-23)

> 2026-09-19 — protocolo imprimível para fechar ADR-003 ("Implementado" → "Aceito").
> **Atualização 2026-09-19 (D-23)**: critério mudou de "≥2 sessões reais ≥30min × ≥20 eventos" para
> **sessão planejada com ≥2 cenários distintos, ≥20 eventos, mix de kinds, simulação de compactação**.
> Razão: cobertura sistemática é mais rigorosa que uso real improvisado; tempo humano
> cai de ≥60min para ~15-20min. Ver rationale completo em `STATE.md` decisão D-23.

## TL;DR

- **Wiração 100% pronta** (verificada em 2026-09-19, ver `SMOKE-TEST-P.md`):
  - `tools/jsonschema/` (ADR-001) ✅
  - `cmd/agent-sync/session_event.go` (Append/Read/Stats + rotação 10MB) ✅
  - `cmd/agent-sync/session_state.go:188` (`appendStateDiffEvents` no rename OK) ✅
  - `cmd/agent-sync/event_cli.go` (subcommands `read`/`tail`/`stats`) ✅
  - Binário reinstalado via `make install` — `agent-sync event` funcional ✅
- **Critério revisado (D-23)**: ≥2 cenários planejados, ≥20 eventos, mix de kinds,
  simulação explícita de compactação, recuperação via `event read`.
- **Pacote de cenários prontos:** `docs/smoke-evidence/PACOTE-CENARIOS.md`.

## Verificação rápida do wirado (rodar antes do smoke)

```bash
# 1. Binário disponível e wirado
which agent-sync && agent-sync event -help 2>&1 | head -10

# 2. Schema carrega
go test ./tools/jsonschema/... -count=1 -run TestLoadSessionEvent -v

# 3. Smoke white-box automatizado (39 testes, deve passar)
go test ./cmd/agent-sync/... -count=1 2>&1 | tail -3

# 4. Hook wirado no caminho de WriteSessionState
grep -n "appendStateDiffEvents" cmd/agent-sync/session_state.go
# esperado: cmd/agent-sync/session_state.go:188: appendStateDiffEvents(...)
```

Se qualquer um falhar, **NÃO inicie o smoke** — reporte o gap antes.

## Setup do projeto temporário (~2min)

```bash
mkdir /tmp/smoke-adr-003 && cd /tmp/smoke-adr-003
git init -q
mkdir -p .agent-sync
# Payload mínimo (decisão+ação) para inicializar o STATE.
cat > /tmp/state-init.json <<'EOF'
{
  "schema_version": "1.0",
  "project": {"name": "smoke-adr-003", "root": "/tmp/smoke-adr-003"},
  "git": {"branch": "main", "head": "0000000000000000000000000000000000000000"},
  "session": {"id": "sess-smoke-001", "started_at": "2026-09-19T20:00:00Z", "updated_at": "2026-09-19T20:00:00Z"},
  "decisions": [{"id": "D-1", "title": "smoke iniciado", "rationale": "ADR-003 protocolo", "made_at": "2026-09-19T20:00:00Z"}],
  "next_actions": [{"id": "A-1", "title": "rodar cenario 1", "status": "in_progress"}],
  "blockers": [],
  "open_questions": []
}
EOF
agent-sync state write -from /tmp/state-init.json -root .
```

## Protocolo da sessão planejada (~15min)

### Escolha ≥2 cenários do pacote (`PACOTE-CENARIOS.md`)

Cada cenário tem **passos numerados** e **resultado esperado**. Marque qual(is) você rodou na evidência.

### Durante cada cenário

1. **Persistir cada decisão/ação** via `agent-sync state write` (use o payload mínimo como template; atualize o array relevante).
2. **A cada 5min**, inspecionar o log:
   ```bash
   agent-sync event stats -root .
   agent-sync event read -last 10 -root .
   ```
3. **Após o último passo do cenário**, simular compactação:
   ```bash
   # Simula perda de contexto: limpar STATE em memória, manter apenas o log
   rm .agent-sync/session-state.json
   # Recriar via o log:
   agent-sync event read -last 50 -root .   # ainda acessível
   agent-sync state render -root .          # falha (sem state) — confirma dependencia
   # Reconstruir payload minimo a partir do log (teste mental: o agente faz isso via LLM)
   ```

### Critérios de aceite

| # | Critério | Threshold | Como verificar |
|---|----------|-----------|----------------|
| C1 | Cenários completados | ≥2 | Lista no YAML de evidência |
| C2 | Eventos totais | ≥20 | `agent-sync event stats -root .` → `total` |
| C3 | Mix de kinds | ≥3 (decision/action/blocker/open_question/state_render) | `agent-sync event stats -root .` → `by_kind` ≥3 chaves |
| C4 | Log sobrevive à remoção do STATE | mesmo conteúdo | `event read` retorna mesmas linhas |
| C5 | `state_render` presente | ≥3 | `event read -kind state_render -root .` ≥3 |
| C6 | Re-orientação via log | subjetivo | Anotar no `notes:` se você conseguiu re-orientar mentalmente o que fazer a partir do log |

**Se qualquer critério falhar**: registrar gap em `notes:` e tentar novamente com mais cenários ou ajustes.

## Coleta de evidência

```bash
# Ao fim da sessão planejada
scripts/collect-smoke-evidence.sh \
  -session 1 \
  -duration 15 \
  -root /tmp/smoke-adr-003 \
  -compact yes \
  -out docs/smoke-evidence/adr-003-sess-1.md

# Edite manualmente:
# - duration_min (real)
# - by_kind/by_actor (preenchidos automaticamente)
# - criteria.C1-C6 (pass/fail manual)
# - compact_test.triggered + trigger_time_min + recovered_via
# - notes: (observações livres + cenários que você rodou)
```

## Após smoke passar

1. Atualizar `docs/ADR-session-event-jsonl-append-only.md:3`: `Implementado` → **Aceito**
2. Editar `.agent-sync/session-state.json`: A-11 → done, B-4 removido
3. Re-render STATE.md: `go run ./cmd/agent-sync state render -root . > STATE.md`
4. Commit:
   ```bash
   git add docs/ADR-session-event-jsonl-append-only.md STATE.md .agent-sync/session-state.json docs/smoke-evidence/
   git commit -m "docs(adr-003): aceita após smoke planejado (≥2 cenários, ≥20 eventos)"
   ```

## Pendência atual

- [ ] Rodar ≥2 cenários de `PACOTE-CENARIOS.md`
- [ ] Coletar evidência via `collect-smoke-evidence.sh`
- [ ] Mover ADR → Aceito
- [ ] Fechar A-11 e B-4 no STATE.md

## Riscos / limites aceitos (não bloqueiam aceitação)

1. Não substitui transcript do LLM (ADR Limite 1).
2. Rotação descarta `.1` anterior (ADR Limite 2).
3. `actor` enum fechado (ADR Limite 3).
4. Sem proteção contra corrupção parcial — `Read` pula linhas inválidas (ADR Limite 4).
5. `details: additionalProperties: true` (ADR Limite 5, exceção ADR-001).

## Histórico

- **2026-09-19 (v1)**: ≥2 sessões reais ≥30min × ≥20 eventos (critério original ADR-003).
- **2026-09-19 (v2, D-23)**: sessão planejada com ≥2 cenários distintos (este doc).
