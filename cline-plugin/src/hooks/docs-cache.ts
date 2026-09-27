// hooks/docs-cache.ts — cacheia passivamente docs consultadas (A-90).
// Fire-and-forget: chama `docs-cache-write` com URL/path detectado.

import type { HookContext, HookResult } from "../../types.js";
import { callCore } from "../bridge.js";

const URL_RE = /https?:\/\/[^\s"'<>)]+/g;

function extractUrl(result: unknown): string | null {
  if (!result) return null;
  const text = typeof result === "string" ? result : JSON.stringify(result);
  const m = text.match(URL_RE);
  return m?.[0] ?? null;
}

export function runPostToolUse(ctx: HookContext): HookResult | undefined {
  if (process.env["AGENT_SYNC_DOCS_CACHE"] === "0") return undefined;
  const url = extractUrl(ctx.result);
  if (!url) return undefined;
  void callCore("docs-cache-write", {
    args: ["--source", "cline", "--url", url],
    timeoutMs: 3_000,
  }).catch(() => { /* fail-open */ });
  return undefined;
}