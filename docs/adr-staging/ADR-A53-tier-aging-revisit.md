# ADR: Memory tier aging — revisita de A-53 sob evidência nova

- **Status**: Proposta (template curto — 28 linhas)
- **Data**: 2026-09-24
- **Origem**: A-53 catalogada em D-73 (ses_f2c3cbf5fffeG2uu6hbJJFamFK) como "DEFERIDO S-0.3+". Usuário escolheu A4 (2026-09-24) — escrever ADR curto antes de codar.

## Contexto (verificável)

1. **D-73 dizia**: A-53 = campo `tier` em `SessionEvent` + sweep de decay. Classificado "alto risco (schema bump + migration de eventos legados)".
2. **Evidência empírica 2026-09-24** que invalida parte de D-73:
   - `internal/event/session.go:18-20`: `session-event.jsonl` é **append-only com rotação 10MB** — campo opcional `tier` em linhas novas é backward-compatible (linhas legadas ganham `tier=""` = `working` por convenção). **Zero migration truncating.**
   - `tools/internal/agentmemory/store.go:55,199,305`: já existem `Memory.AccessedAt` + `Touch` + `PruneScratch` — cobrem ~70% da infra de decay/poda.
   - `tools/internal/agentmemory/store.go:64-68`: `Memory.Search` já aplica decay `searchDecayHalfLife = 30d` como boost sobre BM25 — não é aging/podagem, é re-ranking.

## Decisão proposta (ainda não tomada — só opções)

- **A1/A2**: implementar agora em S-0.2. Sweep opt-in: `agent-sync event sweep --tier <w|e|s|p> --older-than 30d`. ~120L + 1 teste. Sem propagação para `Memory.tier`.
- **A3**: manter DEFERIDO S-0.3+ (status quo D-73). Custo zero.
- **A4** (escolhida): este ADR — decisão estruturada fica para S-0.3.

## Decisões em aberto (para S-0.3)

- **B**: `tier` propaga para `Memory.tier` ou fica só no `SessionEvent`?
- **C**: sweep é opt-in (`event sweep ...`) ou hook fim-de-turno?
- **D**: 4 tiers do ai-memory (working|episodic|semantic|procedural) ou subset?

## Gatilhos de reabertura para S-0.3+

1. Usuário reportar JSONL inflado ou briefing (A-51) poluído por eventos >90d.
2. Race condition em `Memory.AccessedAt` por sweep concorrente.
3. ai-memory/akitaonrails publicar meia-vida validada empiricamente que valha replicar.
4. Decisão de produto: encurtar contexto priorizando working tier.

## Consequências

**Pos**: decisão rastreável; risco desmascarado (não era tão alto); 4 gatilhos evitam reabrir sem motivo OU esquecer de reabrir com motivo.
**Neg**: este ADR não é formal (`docs/ADR-XXX-*.md`) — refinamento só se A-53 for promovido.

## Não-escopo

- Replicar decay verbatim do ai-memory (D-72).
- Sweep automático sem opt-in (D-2 lockless + D-6 zero-LLM).
- Esquecer este ADR: se 2026-10-24 (+30d) transcorrer sem gatilho, manter Proposta+DEFERIDO até S-0.3 nascer.
