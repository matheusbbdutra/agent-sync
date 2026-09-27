// hooks/context-guard-nudge.ts — lembrete pos-N-edits para atualizar STATE.md (A-90).

import type { HookContext, HookResult } from "../../types.js";

const DEFAULT_THRESHOLD = 5;
const threshold = Number(process.env["AGENT_SYNC_CTX_NUDGE_THRESHOLD"] ?? DEFAULT_THRESHOLD);
const EDIT_TOOLS = new Set(["Edit","Write","MultiEdit","NotebookEdit","replace_in_file","write_to_file"]);
const counters = new Map<string, number>();

export function runPreToolUse(ctx: HookContext): HookResult | undefined {
  if (process.env["AGENT_SYNC_CTX_NUDGE"] === "0") return undefined;
  const sid = ctx.snapshot?.conversationId ?? ctx.snapshot?.runId ?? "unknown";
  const tool = ctx.toolCall?.toolName ?? "";
  if (!EDIT_TOOLS.has(tool)) return undefined;
  const n = (counters.get(sid) ?? 0) + 1;
  counters.set(sid, n);
  if (n !== threshold) return undefined;
  return { appendContext: `[context-guard] ${n} edições nesta sessão — considere atualizar STATE.md via agent-sync state render.` };
}