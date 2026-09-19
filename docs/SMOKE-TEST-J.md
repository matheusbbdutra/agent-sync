# Smoke Test J — Checklist objetivo por CLI

Validação manual em sessões reais. **Não automatizável** (julgamento humano). Ordem recomendada: **Cursor > Antigravity > Claude > Codex > OpenCode** (prioridade por risco de regressão silenciosa do Gap F — `wrap-hook.sh`).

---

## Cursor (prioridade alta — Gap F)

1. **Wrap-hook.sh ativo**
   - Abra Cursor com este repo como workspace.
   - Dispare uma tool call (ex.: `Bash` simples `echo hi`).
   - Confirme em `~/.cache/agent-sync/hooks/observe-error.jsonl` um evento novo com `stage=cursor`, `status=success`, `duration_ms>0`.

2. **false-success-guard dispara em Stop prematuro**
   - Peça ao Cursor uma tarefa de 4+ passos, mande parar no meio.
   - Esperado: nudge aparece listando passos restantes.
   - Confirme `tools/cmd/false-success-guard/log/errors.jsonl` ficou vazio (sem execução marcada como sucesso parcial).

3. **Handoff Cursor em `sessionStart`**
   - Encerre a conversa, reabra, peça continuação de tarefa recente.
   - Esperado: primeira resposta do Cursor cita resumo anterior (resgatado de `<repo>/.agent-sync/summary.md` ou sessão global).

## Antigravity CLI (`agy`) — prioridade alta (Gap F + L)

1. **Wrap-hook.sh ativo**
   - Abra `agy` no workspace, dispare uma tool call.
   - Mesmo check do Cursor acima (campo `stage=antigravity`).

2. **Handoff em `PreInvocation` (pendência L)**
   - Encerre e reabra `agy`.
   - No turno 1 (`invocationNum==1`), esperado: `ephemeralMessage` injetando resumo anterior.
   - Nos turnos seguintes, esperado: resposta `{}` sem poluir o transcript.

3. **false-success-guard + PreCompact**
   - Tarefa de 4+ passos, mande parar no meio, dispare compactação.
   - Esperado: nudge do false-success-guard antes do resumo automático.

## Claude Code (cobertura indireta — já roda em produção nesta sessão)

1. **Wrap-hook.sh**: dispare uma tool call Bash. Confirme `stage=claude` no JSONL.
2. **false-success-guard**: tarefa 4+ passos, parar no meio → nudge aparece, `errors.jsonl` vazio.
3. **Handoff SessionStart**: reabra sessão, primeira resposta cita resumo.

## Codex (cobertura indireta — já roda em produção)

1. **Wrap-hook.sh**: `codex` exec, dispare tool call. Confirme `stage=codex`.
2. **false-success-guard**: tarefa 4+ passos → nudge, log vazio.
3. **Handoff SessionStart + nudge de tokens Codex (K — já fechado)**: rode sessão longa (>150k tokens) e confirme nudge aparece 1x ao cruzar threshold (não em todo tool call).

## OpenCode (cobertura indireta)

1. **Wrap-hook.sh**: dispare tool call. Confirme `stage=opencode`.
2. **Nudge de tokens OpenCode (K — já fechado)**: sessão longa → nudge via `[AVISO agent-sync]` aparece quando cruza threshold (lê SQLite local `~/.local/share/opencode/opencode.db`).
3. **Plugin TS best-effort** ([upstream #13574](https://github.com/anomalyco/opencode/issues/13574)): confirmado que hook pode ser silencioso — não é regressão, é limitação documentada.

---

## Resultado esperado

- ✓ todas as 5 CLIs produzem pelo menos um evento de wrap com `status=success` no JSONL.
- ✓ nenhuma regressão visível nos nudges do false-success-guard.
- ✓ handoff funciona em pelo menos Claude/Codex/Cursor/Antigravity.
- ✗ OpenCode plugin pode omitir nudge (limitação upstream conhecida).

Anote resultados em `STATE.md` seção "Achados da sessão".
