// hooks/memory-prune-session-start.ts — auto-prune scratch no agent_start (A-90).
// Migração 1:1 de hooks/memory-prune-session-start.sh.

import * as fs from "node:fs";
import * as path from "node:path";
import * as os from "node:os";
import type { HookContext, HookResult } from "../../types.js";
import { callCore } from "../bridge.js";

const STATE_DIR = path.join(os.tmpdir(), "agent-sync-auto-prune");
const STALE_HOURS = 24;

function stateFile(sid: string): string {
  return path.join(STATE_DIR, `${sid}.ts`);
}

function isFresh(sid: string): boolean {
  try {
    const ts = Number(fs.readFileSync(stateFile(sid), "utf8").trim());
    return Date.now() - ts < STALE_HOURS * 3600 * 1000;
  } catch {
    return false;
  }
}

export async function runBeforeRun(ctx: HookContext): Promise<HookResult | undefined> {
  if (process.env["AGENT_SYNC_AUTO_PRUNE"] === "0") return undefined;
  const sid = ctx.snapshot?.conversationId ?? ctx.snapshot?.runId ?? "unknown";
  if (isFresh(sid)) return undefined;

  fs.mkdirSync(STATE_DIR, { recursive: true });

  // Lê stats de scratch para alerta.
  const stats = await callCore("memory-mcp", {
    args: ["stats", "--json"],
    timeoutMs: 5_000,
  });
  let pct = 0;
  if (stats.exitCode === 0) {
    try {
      const parsed = JSON.parse(stats.stdout) as { scratch?: number; total?: number };
      if (parsed.total && parsed.total > 0) {
        pct = Math.round(((parsed.scratch ?? 0) / parsed.total) * 100);
      }
    } catch { /* ignore */ }
  }

  // Dispara prune em background.
  void callCore("memory-mcp", {
    args: ["prune", "--older-than", "7d"],
    timeoutMs: 10_000,
  }).catch(() => { /* fail-open */ });

  fs.writeFileSync(stateFile(sid), String(Date.now()), "utf8");

  if (pct >= 80) {
    return { appendContext: `[memory-prune] WARNING: scratch=${pct}% >= threshold=80%. Prune disparado em background.` };
  }
  return undefined;
}