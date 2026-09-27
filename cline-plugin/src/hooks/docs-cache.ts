// hooks/docs-cache.ts — cacheia passivamente docs consultadas (A-90).
// Fire-and-forget: chama `docs-cache-write` com payload JSON no stdin.
//
// `docs-cache-write` aceita flags `--cache-dir` (opcional) e lê
// `{"url","contentType","text"}` do stdin (ver tools/cmd/docs-cache-write/main.go).
// Corrigido em codex-review P2 (PR #3): antes enviava --source/--url (inválidos).

import type { HookContext, HookResult } from "../../types.js";
import { callCore } from "../bridge.js";

const URL_RE = /https?:\/\/[^\s"'<>)]+/g;

function extractUrlAndText(result: unknown): { url: string; text: string } | null {
  if (!result) return null;
  const text = typeof result === "string" ? result : JSON.stringify(result);
  const url = text.match(URL_RE)?.[0];
  if (!url) return null;
  return { url, text };
}

export function runPostToolUse(ctx: HookContext): HookResult | undefined {
  if (process.env["AGENT_SYNC_DOCS_CACHE"] === "0") return undefined;
  const out = extractUrlAndText(ctx.result);
  if (!out) return undefined;
  void callCore("docs-cache-write", {
    args: [],
    stdin: JSON.stringify({
      url: out.url,
      contentType: "text/plain",
      text: out.text.slice(0, 64 * 1024),
    }),
    timeoutMs: 3_000,
  }).catch(() => { /* fail-open */ });
  return undefined;
}