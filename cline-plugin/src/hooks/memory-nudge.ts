// hooks/memory-nudge.ts — nudge de consolidação de memória pós-N-edits (A-90).
//
// Migração 1:1 de `hooks/memory-nudge.pretooluse.sh`. Conta tools de mutação
// (Edit/Write/MultiEdit/NotebookEdit) por sessão e dispara nudge a cada N.
//
// Dependência: chama `memory-mcp record_event` via bridge (sink remoto).

import type { HookContext, HookResult } from "../../types.js";
import { callCore } from "../bridge.js";

const DEFAULT_THRESHOLD = 15;
const threshold = Number(
  process.env["AGENT_SYNC_REACT_NUDGE_THRESHOLD"] ?? DEFAULT_THRESHOLD,
);

const EDIT_TOOLS = new Set([
  "Edit",
  "Write",
  "MultiEdit",
  "NotebookEdit",
  "replace_in_file",
  "write_to_file",
]);

/** Contadores por sessão. */
const sessionCounters = new Map<string, number>();

/** Hook PreToolUse: nudge a cada `threshold` tools de mutação. */
export async function runPreToolUse(ctx: HookContext): Promise<HookResult | undefined> {
  if (process.env["AGENT_SYNC_MEMORY_NUDGE"] === "0") return undefined;

  const sid =
    ctx.snapshot?.conversationId ??
    ctx.snapshot?.runId ??
    "unknown";
  const toolName = ctx.toolCall?.toolName ?? "unknown";

  // Conta SOMENTE tools de mutação.
  if (!EDIT_TOOLS.has(toolName)) return undefined;

  const count = (sessionCounters.get(sid) ?? 0) + 1;
  sessionCounters.set(sid, count);

  // Dispara nudge apenas em múltiplos exatos do threshold.
  if (count % threshold !== 0) return undefined;

  // Notifica memory-mcp (fire-and-forget — não bloqueia o turno).
  void callCore("memory-mcp", {
    args: ["record_event", "--kind", "memory_nudge", "--count", String(count)],
  }).catch(() => {
    /* fail-open: nudge binário ausente não derruba o hook */
  });

  return {
    appendContext:
      `[agent-sync memory-nudge] ${count} tools de mutação nesta sessão ` +
      `(threshold=${threshold}). Considere consolidar aprendizados na memória ` +
      `persistida via memory-mcp store.`,
  };
}