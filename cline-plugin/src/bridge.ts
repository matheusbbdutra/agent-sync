// bridge.ts — wrapper para chamar binários core Go via subprocess (A-90).
//
// Substitui a parte "executar script externo" do `cline_bridge.go`.
// Os hooks TS chamam esta função quando precisam de:
//   - audit-removal (repo-map)
//   - ctx-window summarize/handoff
//   - agent-sync budget nudge/write
//   - memory-mcp record_event/prune/consolidate
//   - docs-cache-write
//
// Tudo via `execFile` (sem shell injection). Env var AGENT_SYNC_AGENT_KIND=cline
// é injetada para que os binários Go saibam que o request veio do Cline.

import { execFile } from "node:child_process";
import * as os$ from "node:os";
import * as path from "node:path";

/** Resultado de chamada a binário core. */
export interface CoreResult {
  stdout: string;
  stderr: string;
  exitCode: number;
}

/** Opções de `callCore`. */
export interface CallCoreOptions {
  /** Argumentos posicionais (cada um escapado via execFile — sem shell). */
  args?: string[];
  /** Dados a enviar via stdin (JSON.stringify no caller). */
  stdin?: string;
  /** Env vars adicionais (mescladas com process.env). */
  env?: Record<string, string>;
  /** Timeout em ms (default: 10_000). */
  timeoutMs?: number;
  /** Working directory (default: process.cwd()). */
  cwd?: string;
}

/**
 * Resolve path absoluto do binário `<name>` do agent-sync.
 * Ordem de precedência:
 *   1. env `AGENT_SYNC_BIN_DIR` (se setado, usa `<dir>/<name>`)
 *   2. `~/.local/bin/<name>` (default cross-CLI do agent-sync)
 */
export function resolveCoreBin(name: string): string {
  const dir = process.env["AGENT_SYNC_BIN_DIR"];
  if (dir && dir.length > 0) return path.join(dir, name);
  return path.join(os$.homedir(), ".local", "bin", name);
}

/**
 * Wrapper Promise discriminated-union em torno de `execFile`.
 * `execFile` aceita `input` em options; o callback nativo propaga stdout/stderr.
 */
type ExecOutcome =
  | { ok: true; stdout: string; stderr: string }
  | { ok: false; stdout: string; stderr: string; code: string | number; message: string };

function execFileP(
  bin: string,
  args: string[],
  opts: {
    input?: string;
    env: NodeJS.ProcessEnv;
    timeout: number;
    cwd: string;
    maxBuffer: number;
  },
): Promise<ExecOutcome> {
  return new Promise((resolve) => {
    let capturedStdout = "";
    let capturedStderr = "";
    execFile(bin, args, opts, (err, stdout, stderr) => {
      capturedStdout = String(stdout);
      capturedStderr = String(stderr);
      if (err) {
        const e = err as NodeJS.ErrnoException & { code?: string | number };
        resolve({
          ok: false,
          stdout: capturedStdout,
          stderr: capturedStderr,
          code: e.code ?? "UNKNOWN",
          message: e.message ?? "",
        });
        return;
      }
      resolve({ ok: true, stdout: capturedStdout, stderr: capturedStderr });
    });
  });
}

/**
 * Chama binário core do agent-sync.
 * Captura stderr, propaga exit code, timeout e stdin.
 * Falhas ENOENT (binário ausente) viram `BridgeError`; demais viram `CoreResult` com `exitCode != 0`.
 */
export async function callCore(
  name: string,
  opts: CallCoreOptions = {},
): Promise<CoreResult> {
  const bin = resolveCoreBin(name);
  const args = opts.args ?? [];
  const timeoutMs = opts.timeoutMs ?? 10_000;

  const env: NodeJS.ProcessEnv = {
    ...process.env,
    AGENT_SYNC_AGENT_KIND: "cline",
    AGENT_SYNC_CLI: "cline",
    ...opts.env,
  };

  const outcome = await execFileP(bin, args, {
    input: opts.stdin,
    env,
    timeout: timeoutMs,
    cwd: opts.cwd ?? process.cwd(),
    maxBuffer: 16 << 20, // 16 MiB
  });

  if (outcome.ok) {
    return { stdout: outcome.stdout, stderr: outcome.stderr, exitCode: 0 };
  }

  if (outcome.code === "ENOENT") {
    throw new BridgeError(
      `core binary not found: ${bin}`,
      "ENOENT",
    );
  }

  return {
    stdout: outcome.stdout,
    stderr: outcome.stderr || outcome.message,
    exitCode: typeof outcome.code === "number" ? outcome.code : 1,
  };
}

/** Erro estruturado para falhas irrecuperáveis (binário ausente, etc.). */
export class BridgeError extends Error {
  constructor(
    message: string,
    public code: "ENOENT" | "ETIMEDOUT" | "UNKNOWN",
    public cause?: unknown,
  ) {
    super(message);
    this.name = "BridgeError";
  }
}

/**
 * Versão fail-open: nunca throw, retorna `null` em falha irrecuperável.
 * Use em hooks que devem ser no-op quando o binário está ausente.
 */
export async function tryCallCore(
  name: string,
  opts: CallCoreOptions = {},
): Promise<CoreResult | null> {
  try {
    return await callCore(name, opts);
  } catch (err) {
    if (err instanceof BridgeError && err.code === "ENOENT") return null;
    throw err;
  }
}