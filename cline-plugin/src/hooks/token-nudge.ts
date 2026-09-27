// hooks/token-nudge.ts — chama `agent-sync budget nudge` (A-90).

import type { HookContext, HookResult } from "../../types.js";
import { callCore, tryCallCore } from "../bridge.js";

const MIRROR_THRESHOLD = 10;
const mirrorThreshold = Number(process.env["AGENT_SYNC_TOKEN_MIRROR_THRESHOLD"] ?? MIRROR_THRESHOLD);

const counters = new Map<string, number>();

export async function runPostToolUse(ctx: HookContext): Promise<HookResult | undefined> {
  if (process.env["AGENT_SYNC_TOKEN_NUDGE"] === "0") return undefined;
  const sid = ctx.snapshot?.conversationId ?? ctx.snapshot?.runId ?? "unknown";
  const n = (counters.get(sid) ?? 0) + 1;
  counters.set(sid, n);

  const result = await tryCallCore("agent-sync", {
    args: ["budget", "nudge", "--actor", "cline"],
    timeoutMs: 3_000,
  });
  if (!result || result.exitCode !== 0) return undefined;

  let parsed: { should_nudge?: boolean; reason?: string } = {};
  try { parsed = JSON.parse(result.stdout); } catch { /* ignore */ }

  if (!parsed.should_nudge) return undefined;
  if (n % mirrorThreshold !== 0) return undefined;

  return {
    appendContext: `[token-nudge] ${parsed.reason ?? "utilization alta"}. Sugere ` +
      `agent-sync budget status --actor cline para detalhes.`,
  };
}