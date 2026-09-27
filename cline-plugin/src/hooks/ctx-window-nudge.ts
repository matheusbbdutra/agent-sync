// hooks/ctx-window-nudge.ts — nudge combinado: ≥N tool calls OU summary velho (A-90).

import type { HookContext, HookResult } from "../../types.js";
import { callCore, tryCallCore } from "../bridge.js";

const DEFAULT_THRESHOLD = 80;
const MIRROR_THRESHOLD = 25;
const threshold = Number(process.env["AGENT_SYNC_CTX_NUDGE_THRESHOLD"] ?? DEFAULT_THRESHOLD);
const mirrorThreshold = Number(process.env["AGENT_SYNC_CTX_MIRROR_THRESHOLD"] ?? MIRROR_THRESHOLD);

const counters = new Map<string, number>();

export async function runPostToolUse(ctx: HookContext): Promise<HookResult | undefined> {
  if (process.env["AGENT_SYNC_CTX_NUDGE"] === "0") return undefined;
  const sid = ctx.snapshot?.conversationId ?? ctx.snapshot?.runId ?? "unknown";
  const n = (counters.get(sid) ?? 0) + 1;
  counters.set(sid, n);

  const trigger = n >= threshold;

  if (trigger && n % mirrorThreshold !== 0) return undefined;

  if (trigger) {
    void callCore("memory-mcp", {
      args: ["record_event", "--kind", "ctx_window_nudge", "--count", String(n)],
      timeoutMs: 3_000,
    }).catch(() => { /* fail-open */ });
  }

  const status = await tryCallCore("ctx-window", {
    args: ["status", "-cli", "cline"],
    timeoutMs: 3_000,
  });
  const stale = status?.stdout.includes("summary_missing") ?? false;

  if (!trigger && !stale) return undefined;

  return {
    appendContext:
      `[ctx-window-nudge] ${n} tool calls${stale ? " ou summary.md ausente" : ""}. ` +
      `Sugere rodar: ctx-window summarize -cli cline`,
  };
}