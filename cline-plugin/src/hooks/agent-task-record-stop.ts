// hooks/agent-task-record-stop.ts — telemetria de task no agent_end (A-90).
// Migração 1:1 de hooks/agent-task-record.stop.sh.
//
// `agent-sync budget write` espera um AgentTask JSON completo no stdin
// (ver internal/budget/tasks.go:AgentTask schema). Flags como --cli/--model
// são IGNORADAS — passam pelo flag.Parse mas não populam o struct.
// Corrigido em codex-review P2 (PR #3).

import type { HookContext, HookResult } from "../../types.js";
import { callCore } from "../bridge.js";

export function runAfterRun(ctx: HookContext): HookResult | undefined {
  if (process.env["AGENT_SYNC_TASK_RECORD"] === "0") return undefined;
  const sid = ctx.snapshot?.conversationId ?? ctx.snapshot?.runId ?? "unknown";
  const task = {
    schema_version: "1.0",
    task_id: `cline-${sid}`,
    ts: new Date().toISOString(),
    cli: "cline",
    model: "unknown",
    status: "completed",
    session_id: sid,
  };
  void callCore("agent-sync", {
    args: ["budget", "write"],
    stdin: JSON.stringify(task),
    timeoutMs: 5_000,
  }).catch(() => { /* fail-open */ });
  return undefined;
}