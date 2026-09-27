// hooks/memory-observe.ts — observação contínua em background (A-90).
// Fail-open fire-and-forget: nunca bloqueia o turno.

import type { HookContext, HookResult } from "../../types.js";
import { callCore } from "../bridge.js";

const HIGH_SIGNAL = new Set(["Edit","Write","MultiEdit","NotebookEdit","replace_in_file","write_to_file","Bash"]);

const SCOPE = process.env["AGENT_SYNC_MEMORY_OBSERVE_SCOPE"] ?? "high-signal";

function shouldRecord(toolName: string, isError: boolean): boolean {
  if (SCOPE === "off") return false;
  if (SCOPE === "all") return true;
  // high-signal: edits + bash, ou qualquer status!=ok
  if (isError) return true;
  return HIGH_SIGNAL.has(toolName);
}

export function runPostToolUse(ctx: HookContext): HookResult | undefined {
  if (process.env["AGENT_SYNC_MEMORY_OBSERVE_DISABLE"] === "1") return undefined;
  const toolName = ctx.toolCall?.toolName ?? "unknown";
  const result = (ctx.result ?? {}) as Record<string, unknown>;
  const isError = Boolean(result["isError"]);
  if (!shouldRecord(toolName, isError)) return undefined;

  void callCore("memory-mcp", {
    args: [
      "record_event",
      "--kind", "observation",
      "--tool", toolName,
      "--error", String(isError),
    ],
    timeoutMs: 3_000,
  }).catch(() => { /* fail-open */ });

  return undefined;
}