// hooks/ctx-window-handoff.ts — dispara handoff de contexto no agent_start (A-90 patch).
//
// Migração do `ctx-window handoff cline` command que vivia injetado pelo
// bridge Go (`cline_bridge.go:116`) e foi perdido na migração TS.
// Wirado em `beforeRun` em plugin.ts:43.
//
// Codex-review P1 (PR #3): o log do wiramento-smoke listava `ctx-handoff`
// como wirado mas nenhum hook efetivamente disparava o comando.

import type { HookContext, HookResult } from "../../types.js";
import { callCore } from "../bridge.js";

export async function runBeforeRun(_ctx: HookContext): Promise<HookResult | undefined> {
  if (process.env["AGENT_SYNC_HANDOFF"] === "0") return undefined;
  void callCore("ctx-window", {
    args: ["handoff", "cline"],
    timeoutMs: 5_000,
  }).catch(() => { /* fail-open */ });
  // beforeRun pode devolver appendContext; este hook não injeta nada.
  return undefined;
}