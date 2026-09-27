// hooks/secret-guard.ts — bloqueia leitura de segredos via deny-list + regex (A-90).
// Wraps both PreToolUse (bloqueia) e PostToolUse (redige output).

import type { HookContext, HookResult } from "../../types.js";

const DENY_PATTERNS = [
  /\.env(\.|$)/,
  /id_rsa/,
  /\.pem$/,
  /\.key$/,
  /credentials/,
  /secrets?\//,
  /aws\/credentials/,
  /\.netrc$/,
];
const INLINE_PATTERNS: Array<[RegExp, string]> = [
  [/eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/g, "<REDACTED:JWT>"],
  [/AKIA[0-9A-Z]{16}/g, "<REDACTED:AWS_ACCESS_KEY>"],
  [/ghp_[A-Za-z0-9]{36}/g, "<REDACTED:GITHUB_PAT>"],
];

function matchesDenyList(text: string): boolean {
  return DENY_PATTERNS.some((p) => p.test(text));
}

function redactInline(text: string): string {
  let out = text;
  for (const [pat, repl] of INLINE_PATTERNS) out = out.replace(pat, repl);
  return out;
}

export function runPreToolUse(ctx: HookContext): HookResult | undefined {
  if (process.env["AGENT_SYNC_SECRET_GUARD"] === "0") return undefined;
  const input = (ctx.input ?? {}) as Record<string, unknown>;
  const blob = typeof input === "object" ? JSON.stringify(input) : String(input);
  if (matchesDenyList(blob)) {
    return { skip: true, reason: "secret-guard: path/input matches deny-list" };
  }
  return undefined;
}

export function runPostToolUse(ctx: HookContext): HookResult | undefined {
  if (process.env["AGENT_SYNC_SECRET_GUARD"] === "0") return undefined;
  const result = ctx.result as Record<string, unknown> | string | undefined;
  if (!result) return undefined;
  const text = typeof result === "string" ? result : JSON.stringify(result);
  if (matchesDenyList(text)) {
    return { result: "<REDACTED:FILE_IN_DENYLIST>" };
  }
  const redacted = redactInline(text);
  if (redacted !== text) return { result: redacted };
  return undefined;
}