// hooks/ctx-window-summarize-at-stop.ts — auto-summarize em agent_end (A-90).
//
// Migração 1:1 de `hooks/ctx-window-summarize-at-stop.sh`. Roda `ctx-window
// summarize` em background no TaskComplete (afterRun), sem bloquear o turno.
//
// Opt-out via `AGENT_SYNC_AUTO_SUMMARIZE=0`.

import type { HookContext, HookResult } from "../../types.js";
import { callCore } from "../bridge.js";

/** Hook afterRun: dispara summarize em background (fire-and-forget). */
export async function runAfterRun(_ctx: HookContext): Promise<HookResult | undefined> {
  if (process.env["AGENT_SYNC_AUTO_SUMMARIZE"] === "0") return undefined;

  // Fire-and-forget — não bloqueia o turno.
  void callCore("ctx-window", {
    args: ["summarize", "-cli", "cline"],
    timeoutMs: 5_000,
  }).catch(() => {
    /* fail-open */
  });

  // afterRun nunca devolve contexto (é evento terminal).
  return undefined;
}