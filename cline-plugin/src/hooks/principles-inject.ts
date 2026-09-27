// hooks/principles-inject.ts — injeta 2 premissas críticas UMA vez por sessão (A-90).
//
// Migração 1:1 de `hooks/principles-inject.pretooluse.sh`. Puro TS, sem binário
// core — texto estático + dedup por `session_id` via Set em memória.
//
// Comportamento:
//   - Antes da 1ª tool call por sessão, injeta as 2 premissas como `appendContext`.
//   - Em tool calls subsequentes da mesma sessão: no-op silencioso.
//   - Opt-out via `AGENT_SYNC_PRINCIPLES_INJECT=0`.

import type { HookContext, HookResult } from "../../types.js";

const PRINCIPLES = `[agent-sync principles — injetado 1× por sessão]
1. Verdade absoluta: registre apenas afirmações verificáveis empiricamente
   (git, doc oficial, runtime, file:line). Sem evidência = sem afirmação.
2. Anti-overengineering: prefira editar existente a criar novo
   (skill, hook, agente, arquivo). Crie só quando edit >50 linhas
   ou for estruturalmente inviável.`;

/** Sessões que já receberam o inject (Set em memória, reset on restart). */
const seenSessions = new Set<string>();

/** Hook PreToolUse puro TS. */
export function runPreToolUse(ctx: HookContext): HookResult | undefined {
  if (process.env["AGENT_SYNC_PRINCIPLES_INJECT"] === "0") return undefined;

  const sid =
    ctx.snapshot?.conversationId ??
    ctx.snapshot?.runId ??
    "unknown";

  if (seenSessions.has(sid)) return undefined;
  seenSessions.add(sid);

  return { appendContext: PRINCIPLES };
}