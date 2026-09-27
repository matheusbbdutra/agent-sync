// hooks/wiramento-smoke.ts — confirmação visível do wiramento Cline (A-90).
// Migração 1:1 de hooks/cline-wiramento-smoke.sh.

import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import type { HookContext, HookResult } from "../../types.js";

const SMOKE_DIR = "/tmp/agent-sync-cline-wiramento";

export function runBeforeRun(ctx: HookContext): HookResult | undefined {
  if (process.env["AGENT_SYNC_WIRAMENTO_SMOKE"] === "0") return undefined;
  const sid = ctx.snapshot?.conversationId ?? ctx.snapshot?.runId ?? "unknown";
  const seenFile = path.join(SMOKE_DIR, `${sid}.seen`);
  const logFile = path.join(SMOKE_DIR, `${sid}.log`);
  fs.mkdirSync(SMOKE_DIR, { recursive: true });

  const alreadySeen = fs.existsSync(seenFile);
  fs.writeFileSync(seenFile, new Date().toISOString(), "utf8");

  if (!alreadySeen) {
    const lines = [
      `--- agent-sync Cline wiramento @ ${new Date().toISOString()} ---`,
      `session_id: ${sid}`,
      `PreToolUse: principles-inject, memory-nudge, context-guard, agent-react, secret-guard`,
      `PostToolUse: docs-cache, ctx-window-nudge, memory-observe, token-nudge, secret-guard`,
      `TaskStart: memory-prune-session-start, ctx-handoff, wiramento-smoke`,
      `TaskComplete: memory-consolidate, ctx-window-summarize, agent-task-record`,
      `MCP: context7, docs, memory, code-graph`,
    ];
    fs.writeFileSync(logFile, lines.join("\n") + "\n", "utf8");
  }

  return {
    appendContext: `[agent-sync Cline wiramento OK] hooks wirados via plugin em ${os.homedir()}/.cline/plugins/. Veja ${logFile}.`,
  };
}