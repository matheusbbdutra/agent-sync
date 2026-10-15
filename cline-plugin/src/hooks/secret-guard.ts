// hooks/secret-guard.ts — delega ao `secret-scan` Go (fonte única da verdade).
//
// Regra §4 (Go/TS/Shell): TS nunca reimplementa deny-list nem regex de
// segredo. Toda decisão vive em tools/cmd/secret-scan (port 1:1 do shell).
// Fail-open: binário ausente ou erro = allow (nunca derruba o turno).

import type { HookContext, HookResult } from "../../types.js";
import { tryCallCore } from "../bridge.js";

// Cline usa nomes snake_case para file tools; o Go espera o gate
// Claude-compat (Read/Edit/Write/...). Mapeia só o nome — os paths passam
// adiante intocados.
const TOOL_ALIAS: Record<string, string> = {
  replace_in_file: "Edit",
  write_to_file: "Write",
};

function toolNameOf(ctx: HookContext): string {
  const raw = ctx.toolCall?.toolName ?? "unknown";
  return TOOL_ALIAS[raw] ?? raw;
}

function toolInputOf(ctx: HookContext): Record<string, unknown> {
  const input = ctx.input;
  const obj: Record<string, unknown> =
    input && typeof input === "object" && !Array.isArray(input)
      ? { ...(input as Record<string, unknown>) }
      : { raw: input };
  // O gate Go lê file_path/notebook_path/path; Cline file tools usam `path`.
  if (obj["file_path"] == null && typeof obj["path"] === "string") {
    obj["file_path"] = obj["path"];
  }
  return obj;
}

type CheckDecision = { decision?: string; reason?: string };
type RedactDecision = { action?: string; text?: string; detail?: string };

export async function runPreToolUse(ctx: HookContext): Promise<HookResult | undefined> {
  if (process.env["AGENT_SYNC_SECRET_GUARD"] === "0") return undefined;
  const result = await tryCallCore("secret-scan", {
    args: ["check", "--format", "decision"],
    stdin: JSON.stringify({ tool_name: toolNameOf(ctx), tool_input: toolInputOf(ctx) }),
    timeoutMs: 3_000,
  });
  if (!result || result.exitCode !== 0) return undefined;
  let parsed: CheckDecision = {};
  try { parsed = JSON.parse(result.stdout) as CheckDecision; } catch { return undefined; }
  if (parsed.decision === "deny") {
    return { skip: true, reason: `secret-guard: ${parsed.reason ?? "match"}` };
  }
  return undefined;
}

export async function runPostToolUse(ctx: HookContext): Promise<HookResult | undefined> {
  if (process.env["AGENT_SYNC_SECRET_GUARD"] === "0") return undefined;
  const result = ctx.result;
  const text = typeof result === "string" ? result : JSON.stringify(result ?? "");
  const res = await tryCallCore("secret-scan", {
    args: ["redact", "--format", "decision"],
    stdin: JSON.stringify({
      tool_name: toolNameOf(ctx),
      tool_input: toolInputOf(ctx),
      tool_response: text,
    }),
    timeoutMs: 3_000,
  });
  if (!res || res.exitCode !== 0) return undefined;
  let parsed: RedactDecision = {};
  try { parsed = JSON.parse(res.stdout) as RedactDecision; } catch { return undefined; }
  if (parsed.action === "redact-total") {
    return { result: `<REDACTED:FILE_IN_DENYLIST${parsed.detail ? ":" + parsed.detail : ""}>` };
  }
  if (parsed.action === "redact-inline" && typeof parsed.text === "string") {
    return { result: parsed.text };
  }
  return undefined;
}
