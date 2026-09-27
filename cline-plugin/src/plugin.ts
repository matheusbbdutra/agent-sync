// plugin.ts — AgentPlugin entry point do Cline (A-90).
//
// Carrega os 15 hooks wirados e exporta o `AgentPlugin` no contrato `@cline/sdk`.
// Dispatchers acumulam `appendContext` de hooks não-bloqueantes e respeitam
// `skip`/`stop` de hooks bloqueantes — corrige codex-review P1 (PR #3).

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
import * as ctxWindowHandoff from "./hooks/ctx-window-handoff.js";

// ===== Hook lists =====

// PreToolUse: ordem importa — principles-inject é o ancorador do contexto.
const preToolUseHooks = [
  principlesInject,
  memoryNudge,
  contextGuard,
  agentReact,
  secretGuard,
  bashRmGuardian,
];

const postToolUseHooks = [
  docsCache,
  ctxWindowNudge,
  memoryObserve,
  tokenNudge,
  secretGuard,
];

const beforeRunHooks = [wiramentoSmoke, memoryPrune, ctxWindowHandoff];
const afterRunHooks = [ctxWindowSummarize, memoryConsolidate, agentTaskRecord];

// ===== Dispatcher =====

const CONTEXT_SEP = "\n\n---\n\n";

/**
 * Acumula `appendContext` de todos os hooks não-bloqueantes; respeita
 * `skip`/`stop` (primeiro bloqueante vence).
 */
function mergeHookResults(
  results: Array<HookResult | undefined | void>,
): HookResult | undefined {
  let block: HookResult | undefined;
  const contexts: string[] = [];
  for (const r of results) {
    if (!r) continue;
    if (r.skip || r.stop) {
      block = r;
      break;
    }
    if (r.appendContext) contexts.push(r.appendContext);
  }
  if (block) return block;
  if (contexts.length === 0) return undefined;
  if (contexts.length === 1) return { appendContext: contexts[0] };
  return { appendContext: contexts.join(CONTEXT_SEP) };
}

async function dispatchBeforeTool(ctx: HookContext): Promise<HookResult | undefined> {
  const results: Array<HookResult | undefined | void> = [];
  for (const hook of preToolUseHooks) {
    results.push(await hook.runPreToolUse(ctx));
  }
  return mergeHookResults(results);
}

async function dispatchAfterTool(ctx: HookContext): Promise<HookResult | undefined> {
  const results: Array<HookResult | undefined | void> = [];
  for (const hook of postToolUseHooks) {
    const fn = hook.runPostToolUse;
    if (!fn) continue;
    results.push(await fn(ctx));
  }
  return mergeHookResults(results);
}

async function dispatchBeforeRun(ctx: HookContext): Promise<HookResult | undefined> {
  const results: Array<HookResult | undefined | void> = [];
  for (const hook of beforeRunHooks) {
    const fn = hook.runBeforeRun;
    if (!fn) continue;
    results.push(await fn(ctx));
  }
  return mergeHookResults(results);
}

async function dispatchAfterRun(ctx: HookContext): Promise<HookResult | undefined> {
  // afterRun nunca devolve contexto (evento terminal). Apenas roda hooks.
  for (const hook of afterRunHooks) {
    const fn = hook.runAfterRun;
    if (!fn) continue;
    await fn(ctx);
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