# Pacote de Cenários — Smoke ADR-003 (sessão planejada)

> 2026-09-19 — lista de cenários planejados para o smoke da ADR-003. Cada
> cenário exercita um aspecto diferente do event log: variabilidade de
> kinds, simulação de compactação, detecção de mudanças. Rode **≥2** deles
> para satisfazer o critério de aceitação (D-23).

## Convenções

- **`agent-sync state write -from <payload> -root /tmp/smoke-adr-003`** persiste no STATE; cada write emite 1 `state_render` + N per-item events automaticamente.
- **Timestamp**: usar `date -u +"%Y-%m-%dT%H:%M:%SZ"` para `updated_at` no payload (coerência com ADR-002 schema).
- **Ref**: usar IDs `D-N` (decision), `A-N` (action), `B-N` (blocker), `Q-N` (open_question), `S-N` (state_render).
- **`open_questions`**: array de **strings** (não objetos). O event log ainda emite `ref=Q-N` automaticamente.
- **Inspeção**: `agent-sync event stats -root .` e `agent-sync event read -last N -root .`.

---

## Cenário 1 — "Feature com blockers e decisões" (≈6min)

**Objetivo:** gerar mix completo de kinds (decision/action/blocker/open_question/state_render).

**Passos:**

1. **Decisão inicial** (escolha tecnológica):
   ```json
   {
     "schema_version": "1.0",
     "project": {"name": "smoke-adr-003", "root": "/tmp/smoke-adr-003"},
     "git": {"branch": "main", "head": "0000000000000000000000000000000000000000"},
     "session": {"id": "sess-smoke-001", "started_at": "2026-09-19T20:00:00Z", "updated_at": "<AGORA>"},
     "decisions": [
       {"id": "D-1", "title": "escolher lib HTTP", "rationale": "net/http para zero deps", "made_at": "<AGORA>"},
       {"id": "D-2", "title": "schema fechado", "rationale": "additionalProperties false", "made_at": "<AGORA>"}
     ],
     "next_actions": [{"id": "A-1", "title": "implementar endpoint /health", "status": "in_progress"}],
     "blockers": [],
     "open_questions": []
   }
   ```
   → Esperado: 2 events `decision` + 1 `state_render`.

2. **Ação concluída**:
   ```json
   ...
   "next_actions": [
     {"id": "A-1", "title": "implementar endpoint /health", "status": "done"},
     {"id": "A-2", "title": "escrever testes", "status": "pending"}
   ],
   ```
   → Esperado: 1 event com `details.status: "done"`.

3. **Blocker surge**:
   ```json
   "blockers": [
     {"id": "B-1", "title": "porta 8080 ocupada por outro servico", "blocking_action_ids": ["A-2"]}
   ],
   "open_questions": [
     "usar porta alternativa ou parar servico conflitante?"
   ]
   ```
   → Esperado: 1 event `blocker` + 1 event `open_question`.

4. **Inspecionar:**
   ```bash
   agent-sync event stats -root .    # esperado: total ≥6, by_kind ≥4 chaves
   agent-sync event read -kind decision -root .
   agent-sync event read -kind blocker -root .
   ```

**Resultado esperado:** ≥6 eventos totais, 4 kinds diferentes.

---

## Cenário 2 — "Iteração com mudança de status" (≈5min)

**Objetivo:** validar que mudança de status em ação existente emite evento (test `TestWriteSessionStateDetectaMudancaDeStatusEmAction`).

**Passos:**

1. **Estado inicial**: continuar do cenário 1 OU começar fresh com 1 action `pending`.
2. **Mudar status para `done`** via novo `state write` mantendo mesma ação.
   → Esperado: evento com `details.status: "done"`.
3. **Mudar status para `cancelled`** (rollback):
   → Esperado: outro evento com `details.status: "cancelled"`.
4. **Re-escrever sem mudar nada**:
   → Esperado: NÃO emite evento per-item (já tem `state_render` do timestamp, mas items inalterados).

**Resultado esperado:** distinguir eventos "reais" (mudança) vs "no-op" (sem mudança).

---

## Cenário 3 — "Compactação simulada" (≈4min)

**Objetivo:** validar que o log é fonte de verdade para re-orientação após perda de contexto.

**Passos:**

1. **Gerar ≥10 eventos** (mistura livre dos cenários 1/2 OU fresh com 10 state writes).
2. **Simular compactação** (CRÍTICO):
   ```bash
   # Backup do log ANTES da "compactação"
   cp .agent-sync/session-event.jsonl /tmp/event-before-compact.jsonl

   # Simular: deletar STATE, manter apenas log
   rm .agent-sync/session-state.json

   # Confirmar: STATE ausente, log intacto
   ls .agent-sync/
   agent-sync state render -root .   # deve falhar (sem state)
   agent-sync event read -last 50 -root .   # deve funcionar
   ```

3. **Re-orientar mentalmente**: ler `event read -last 50`, identificar decisões/ações/bloqueios recentes, reconstruir mentalmente o que estava acontecendo.

4. **Anotar no `notes:` do YAML**:
   - "Consegui re-orientar sem ler STATE?" (sim/não)
   - "Quais informações foram perdidas?" (se algo)
   - "Qual a 1ª ação que eu retomaria?" (ref D-N ou A-N)

**Resultado esperado:** STATE recriável a partir do log; nenhuma decisão crítica perdida.

---

## Cenário 4 — "Read filtrado" (≈2min)

**Objetivo:** validar filtros `event read` (--kind, --last, --since).

**Passos:**

1. **Gerar ≥15 eventos** de kinds variados (replay rápido dos cenários 1+2).
2. **Testar filtros**:
   ```bash
   # Apenas decisions
   agent-sync event read -kind decision -root .

   # Últimos 5
   agent-sync event read -last 5 -root .

   # Desde timestamp específico
   SINCE=$(date -u -d "5 minutes ago" +"%Y-%m-%dT%H:%M:%SZ")
   agent-sync event read -since "$SINCE" -root .

   # Combinado
   agent-sync event read -kind blocker -last 3 -root .
   ```

**Resultado esperado:** cada filtro retorna subset correto sem linhas de outros kinds/períodos.

---

## Combinações mínimas para satisfazer ≥2 cenários

| Combinação | Tempo | Cobertura |
|---|---|---|
| C1 + C2 | ~10min | Mix kinds + mudança status |
| C1 + C3 | ~10min | Mix kinds + compactação (mais rigoroso) |
| C2 + C3 + C4 | ~12min | Cobertura completa |

**Recomendado:** rodar **C1 + C3** (mix kinds + compactação simulada) — cobre o critério técnico central ("log útil após compactação") em ~10min.

---

## Anti-cenários (o que NÃO fazer)

- ❌ **Não** copiar/colar o payload inteiro a cada step — faça mudanças incrementais (1-2 campos) para que o diff event seja pequeno e útil.
- ❌ **Não** gerar 100+ eventos com o mesmo `kind=state_render` — o critério pede mix.
- ❌ **Não** usar timestamps sintéticos muito no passado/futuro — mantenha coerência temporal.
- ❌ **Não** rodar em worktree sujo do projeto real — use `/tmp/smoke-adr-003` para isolamento.

---

## Validação rápida (rodar após smoke)

```bash
# Total ≥ 20
agent-sync event stats -root /tmp/smoke-adr-003 | jq '.total'

# Mix de kinds ≥ 3
agent-sync event stats -root /tmp/smoke-adr-003 | jq '.by_kind | keys | length'

# Log sobreviveu à compactação
ls /tmp/smoke-adr-003/.agent-sync/session-event.jsonl
wc -l /tmp/smoke-adr-003/.agent-sync/session-event.jsonl
# esperado: wc -l ≈ mesmo número que agent-sync event stats .total
```
