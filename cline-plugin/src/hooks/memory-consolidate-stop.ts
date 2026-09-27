// hooks/memory-consolidate-stop.ts — consolidação automática no agent_end (A-90).
// Fire-and-forget: lê buffer, sintetiza fatos via memory-mcp.

import type { HookContext, HookResult } from "../../types.js";
import { callCore } from "../bridge.js";

export function runAfterRun(_ctx: HookContext): HookResult | undefined {
  if (process.env["AGENT_SYNC_MEMORY_CONSOLIDATE"] === "0") return undefined;
  void callCore("memory-mcp", {
    args: ["consolidate", "--since-last-stop"],
    timeoutMs: 10_000,
  }).catch(() => { /* fail-open */ });
  return undefined;
}