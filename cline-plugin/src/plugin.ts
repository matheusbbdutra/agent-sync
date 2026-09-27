// plugin.ts — AgentPlugin entry point do Cline (A-90, passo 5+7).
//
// Carrega os 15 hooks wirados e exporta o `AgentPlugin` no contrato `@cline/sdk`.

import type { AgentPlugin, HookContext, HookResult } from "../types.js";
import * as principlesInject from "./hooks/principles-inject.js";
import * as memoryNudge from "./hooks/memory-nudge.js";
import * as ctxWindowSummarize from "./hooks/ctx-window-summarize-at-stop.js";
import * as contextGuard from "./hooks/context-guard-nudge.js";
import * as agentReact from "./hooks/agent-react-nudge.js";
import * as secretGuard from "./hooks/secret-guard.js";
import * as wiramentoSmoke from "./hooks/wiramento-smoke.js";
import * as bashRmGuardian from "./hooks/bash-rm-guardian.js";
import * as docsCache from "./hooks/docs-cache.js";
import * as ctxWindowNudge from "./hooks/ctx-window-nudge.js";
import * as memoryObserve from "./hooks/memory-observe.js";
import * as tokenNudge from "./hooks/token-nudge.js";
import * as memoryPrune from "./hooks/memory-prune-session-start.js";
import * as memoryConsolidate from "./hooks/memory-consolidate-stop.js";
import * as agentTaskRecord from "./hooks/agent-task-record-stop.js";

// ===== PreToolUse =====
const preToolUseHooks = [
  principlesInject,
  memoryNudge,
  contextGuard,
  agentReact,
  secretGuard,
  bashRmGuardian,
];

// ===== PostToolUse =====
const postToolUseHooks = [
  docsCache,
  ctxWindowNudge,
  memoryObserve,
  tokenNudge,
  secretGuard,
];

// ===== beforeRun / afterRun =====
const beforeRunHooks = [wiramentoSmoke, memoryPrune];
const afterRunHooks = [ctxWindowSummarize, memoryConsolidate, agentTaskRecord];

async function dispatchBeforeTool(ctx: HookContext): Promise<HookResult | undefined> {
  for (const hook of preToolUseHooks) {
    const r = await hook.runPreToolUse(ctx);
    if (r?.skip || r?.stop) return r;
  }
  return undefined;
}

async function dispatchAfterTool(ctx: HookContext): Promise<HookResult | undefined> {
  for (const hook of postToolUseHooks) {
    const r = await hook.runPostToolUse?.(ctx);
    if (r?.skip) return r;
  }
  return undefined;
}

async function dispatchBeforeRun(ctx: HookContext): Promise<HookResult | undefined> {
  for (const hook of beforeRunHooks) {
    const r = await hook.runBeforeRun?.(ctx);
    if (r?.skip || r?.stop) return r;
  }
  return undefined;
}

async function dispatchAfterRun(ctx: HookContext): Promise<HookResult | undefined> {
  for (const hook of afterRunHooks) {
    await hook.runAfterRun?.(ctx);
  }
  return undefined;
}

const plugin: AgentPlugin = {
  name: "agent-sync-hooks",
  manifest: { capabilities: ["hooks"] },
  hooks: {
    beforeTool: dispatchBeforeTool,
    afterTool: dispatchAfterTool,
    beforeRun: dispatchBeforeRun,
    afterRun: dispatchAfterRun,
  },
};

export = plugin;