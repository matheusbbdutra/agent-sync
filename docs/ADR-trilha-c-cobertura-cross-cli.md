# ADR — Trilha C: cobertura cross-CLI (regra 5xN)

- **Status**: Aceito
- **Data**: 2026-09-21
- **Decisor**: agente + usuário (sessão de retomada D-35/D-36/D-37)
- **Fonte**: D-27 (2026-09-20) formalizou a regra; D-37 (2026-09-21)
  identificou que gaps 🟡/⛔ não viraram ações estruturadas. Esta ADR
  codifica formalmente.

## Contexto

`agent-sync` precisa funcionar em 5 CLIs (Claude Code, OpenCode v2,
Codex, Antigravity, Cursor). Cada feature (PreCompact, budget tracking,
token nudge, etc.) tem cobertura diferente em cada CLI — alguns hooks
expostos, outros não. Sem regra formal, a cobertura fica implícita nas
ADRs e corre risco de gaps 🟡/⛔ nunca virarem ações estruturadas.

## Decisão

### 1. Matriz 5xN obrigatória

Para cada item da Trilha C (identificado como `C-N`), o estado de
cobertura é uma matriz de **5 CLIs** × 1 feature, com **3 estados por CLI**:

| Estado | Significado | Critério de promoção |
| --- | --- | --- |
| ✅ **Coberto** | Wirado + smoke aceito | Hook wirado em runtime real, smoke planejado passou (D-23) |
| 🟡 **Contornável** | Workaround documentado via mecanismo equivalente | Mecanismo alternativo comprovado, com ADR de gap aceito |
| ⛔ **Impossível** | Hook não exposto pelo harness + toda alternativa via harness vizinho descartada | Todas alternativas testadas e registradas; aceito por prazo ≥30d sem mudança upstream |

### 2. Promoção Proposto → Aceito exige todos os estados preenchidos

- **Não-aceito** quando qualquer célula está `❓ não-mapeada`.
- Para C-1..C-5 atuais: D-30..D-36 preencheram as matrizes.
- **Falha estrutural reconhecida (D-37)**: gaps 🟡/⛔ viraram conteúdo
  das ADRs, mas não viraram `next_actions`. A-19 cataloga agora.

### 3. Regra de promoção de gaps

Para cada gap 🟡/⛔ identificado na matriz de um item `C-N`:

- Deve virar uma `A-N+` estruturada **com critério de reabertura**
  explícito (ex.: PLAYBOOK-K para tokens Cursor/Antigravity).
- ⛔ aceito tem **prazo máximo de reavaliação** (default 30 dias) ou
  evento-gatilho (mudança upstream documentada). Sem um dos dois, não
  é mais ⛔ aceito — vira 🟡.

### 4. Smoke é truth, exceção é exception

- Toda promoção `C-N` para Aceito exige **smoke planejado** (D-23),
  não uso real improvisado.
- Toda decisão de manter 🟡/⛔ exige **registro da tentativa** de
  cobrir ✅ em ADR filha ou PLAYBOOK-* específico.
- Heurística e inferência só são aceitáveis como rede-de-segurança
  (D-32 registro `gap-closure-policy` no memory-mcp), nunca como
  mecanismo primário.

## Consequências

**Positivas:**
- Cobertura cross-CLI rastreável e auditável.
- Gaps 🟡/⛔ com prazo de reavaliação evitam "limbo para sempre por inércia".
- Promoção Proposto→Aceito exige smoke planejado, não aceitação por inércia.

**Negativas:**
- Overhead: cada nova feature precisa matriz 5xN + smoke.
- Risco de "tudo é ⛔ aceito": a regra 3 mitiga com prazo de reavaliação.

## Itens atuais (snapshot 2026-09-21)

- **C-1** PreCompact cross-CLI → matriz em `docs/ADR-precompact-snapshot-cross-cli.md` (D-28 + D-30).
- **C-2** Budget tracking write path → wirado em 5 CLIs (D-31); smoke cross-CLI pendente em A-19.
- **C-3** Nudge tokens → matriz em `docs/ADR-token-nudge-contract.md` (D-32).
- **C-4** OpenCode ctx-window → plugins v2 wirados (D-33 + D-34).
- **C-5** OpenCode precompact-snapshot + token-nudge → smoke runtime T3/T4 (D-36, A-18 done).

Gaps 🟡/⛔ known em 2026-09-21: ver `A-19` (catalogo).

## Pendências separadas

- **A-19**: catalogo de gaps. Status pending.
- **A-20**: smoke T5 pre/post b7bc037 — adiado 30d (até 2026-10-21).
- **A-21**: esta ADR (este commit).
- **A-22** (done em 2026-09-21 / D-38): OpenCode v2 EXPOE
  `/compact` programaticamente via `POST /api/session/{id}/compact`
  (HTTP), `client.session.compact(...)` (SDK) e
  `ctx.session.hook("compaction", ...)` (plugin). Smoke planejado
  real via endpoint HTTP → `SMOKE-TEST-U` (entrega separada).
  Ver `docs/smoke-evidence/c4-opencode-ctx-window.md` (seção
  "Pendências separadas") para retificação completa.
