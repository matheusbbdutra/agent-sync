import type { Plugin } from "@opencode-ai/plugin"
import { execFile } from "node:child_process"
import path from "node:path"

// Memory prune + alerta staleness proxy para OpenCode (A-69).
//
// OpenCode v2.0.11 NAO expoe hook SessionStart nativo (@opencode/plugin
// Ctx.session.hook so aceita "context"|"compaction"|"title"|"generate").
// Proxy: na 1a chamada de ferramenta da sessao, dispara o bash hook
// memory-prune-session-start.sh em background (padrao identico ao
// repo-map-warmup.opencode.ts). O bash ja tem idempotencia 24h, alerta
// staleness, opt-out via AGENT_SYNC_AUTO_PRUNE=0, e config via env vars
// (AGENT_SYNC_AUTO_PRUNE_DAYS, AGENT_SYNC_AUTO_PRUNE_STALE_PCT).
//
// IMPORTANTE: best-effort. Falhas do bash (ausente, sem permissao, memory-mcp
// ausente) nunca bloqueiam tool calls. Hook retorna {} sempre.

const PRUNE_FLAG = path.join(import.meta.dirname, ".memory-prune-armed")

function resolveScript(): string {
  const env = process.env.AGENT_SYNC_MEMORY_PRUNE_SH
  if (env) return env
  // assume symlink/wrapper instalado em ~/.config/opencode/plugins/
  return path.join(import.meta.dirname, "memory-prune-session-start.sh")
}

function runPrune(script: string) {
  // execFile em background; erros nunca propagam
  try {
    const child = execFile("/bin/bash", [script], { timeout: 30_000 }, () => {})
    child.on("error", () => {
      // best-effort: ignora
    })
  } catch {
    // best-effort
  }
}

export const MemoryPruneSessionStart: Plugin = async () => {
  let armed = false
  const script = resolveScript()
  return {
    "tool.execute.after": async () => {
      if (armed) return
      armed = true
      runPrune(script)
    },
  }
}