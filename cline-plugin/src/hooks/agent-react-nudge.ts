// hooks/agent-react-nudge.ts — lembrete de validação de hipótese (A-90).

import type { HookContext, HookResult } from "../../types.js";

const DEFAULT_THRESHOLD = 10;
const threshold = Number(process.env["AGENT_SYNC_REACT_NUDGE_THRESHOLD"] ?? DEFAULT_THRESHOLD);
const counters = new Map<string, number>();

export function runPreToolUse(ctx: HookContext): HookResult | undefined {
  if (process.env["AGENT_SYNC_REACT_NUDGE"] === "0") return undefined;
  const sid = ctx.snapshot?.conversationId ?? ctx.snapshot?.runId ?? "unknown";
  const n = (counters.get(sid) ?? 0) + 1;
  counters.set(sid, n);
  if (n === threshold) {
    return { appendContext: `[agent-react] ${n} tool calls — valide hipótese atual antes da próxima (file:line, comando, teste). Sem evidência = sem afirmação.` };
  }
  return undefined;
}