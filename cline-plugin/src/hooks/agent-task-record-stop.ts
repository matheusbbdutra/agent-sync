// hooks/agent-task-record-stop.ts — telemetria de task no agent_end (A-90).
// Migração 1:1 de hooks/agent-task-record.stop.sh.

import type { HookContext, HookResult } from "../../types.js";
import { callCore } from "../bridge.js";

export function runAfterRun(ctx: HookContext): HookResult | undefined {
  if (process.env["AGENT_SYNC_TASK_RECORD"] === "0") return undefined;
  const sid = ctx.snapshot?.conversationId ?? ctx.snapshot?.runId ?? "unknown";
  void callCore("agent-sync", {
    args: [
      "budget", "write",
      "--cli", "cline",
      "--model", "unknown",
      "--status", "completed",
      "--session-id", sid,
    ],
    timeoutMs: 5_000,
  }).catch(() => { /* fail-open */ });
  return undefined;
}