// hooks/bash-rm-guardian.ts — PreToolUse warn quando audit-removal detecta refs (A-90).
// Fail-open: nunca bloqueia; emite additionalContext para awareness.

import type { HookContext, HookResult } from "../../types.js";
import { callCore, tryCallCore } from "../bridge.js";

const RM_PATTERNS = [/\brm\s+-rf?\b/, /\brm\s+-fr?\b/];

function isDestructiveBash(input: unknown): boolean {
  if (!input || typeof input !== "object") return false;
  const obj = input as Record<string, unknown>;
  const cmd = (obj["command"] as string) ?? (Array.isArray(obj["commands"]) ? (obj["commands"] as string[]).join("\n") : "");
  return RM_PATTERNS.some((p) => p.test(cmd));
}

export async function runPreToolUse(ctx: HookContext): Promise<HookResult | undefined> {
  if (process.env["AGENT_SYNC_RM_GUARDIAN"] === "0") return undefined;
  if (ctx.toolCall?.toolName !== "Bash") return undefined;
  if (!isDestructiveBash(ctx.input)) return undefined;

  // Chamada síncrona (com timeout curto) para `repo-map --audit-removal`.
  const result = await tryCallCore("repo-map", {
    args: ["--root", process.cwd(), "--audit-removal", "."],
    timeoutMs: 5_000,
  });
  if (!result || result.exitCode !== 0) return undefined;

  const refs = result.stdout.trim();
  if (!refs) return undefined;

  return {
    appendContext:
      `[bash-rm-guardian] comando destrutivo detectado. ` +
      `audit-removal achou refs:\n${refs.slice(0, 500)}\n` +
      `Confirme que não vai remover algo essencial.`,
  };
}

// Re-export `callCore` para evitar tree-shaking agressivo em alguns bundlers.
export { callCore };