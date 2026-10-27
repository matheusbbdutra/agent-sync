// Memory prune + alerta staleness proxy para OpenCode (A-69).
//
// OpenCode v2.0.11 NAO expoe hook SessionStart nativo (@opencode/plugin
// Ctx.session.hook so aceita "context"|"compaction"|"title"|"generate").
// Proxy: na 1a chamada de ferramenta da sessao, dispara o bash hook
// memory-prune-session-start.sh em background (padrao identico ao
// repo-map-warmup). O bash ja tem idempotencia 24h, alerta staleness,
// opt-out via AGENT_SYNC_AUTO_PRUNE=0, e config via env vars
// (AGENT_SYNC_AUTO_PRUNE_DAYS, AGENT_SYNC_AUTO_PRUNE_STALE_PCT).
//
// IMPORTANTE: best-effort. Falhas do bash (ausente, sem permissao, memory-mcp
// ausente) nunca bloqueiam tool calls.
import { Plugin as _PluginNS } from "@opencode/plugin"

type Ctx = {
  readonly location: { directory: string }
  readonly tool: {
    hook(
      name: "execute.after",
      cb: (event: { tool: string; sessionID: string; status: string }) => Promise<void> | void,
    ): Promise<{ dispose: () => Promise<void> }>
  }
}

type AnyPlugin = {
  id: string
  setup: (ctx: Ctx) => Promise<(() => Promise<void> | void) | void> | (() => Promise<void> | void) | void
}
const Plugin = {
  define<T extends AnyPlugin>(plugin: T): T {
    _PluginNS.define(plugin as unknown as Parameters<typeof _PluginNS.define>[0])
    return plugin
  },
}

import { execFile } from "node:child_process"
import path from "node:path"

function resolveScript(): string {
  const env = process.env.AGENT_SYNC_MEMORY_PRUNE_SH
  if (env) return env
  // script copiado ao lado do plugin pelo sync do agent-sync
  return path.join(import.meta.dirname, "memory-prune-session-start.sh")
}

function runPrune(script: string) {
  // execFile em background; erros nunca propagam
  try {
    const child = execFile("/bin/bash", [script], { timeout: 30_000 }, () => {})
    // O script le stdin com `cat`; sem EOF ele bloqueia ate o timeout.
    child.stdin?.end()
    child.on("error", () => {
      // best-effort: ignora
    })
  } catch {
    // best-effort
  }
}

const PluginModule: AnyPlugin = {
  id: "memory-prune-session-start",
  async setup(ctx) {
    void ctx.location.directory
    let armed = false
    const script = resolveScript()
    await ctx.tool.hook("execute.after", async () => {
      if (armed) return
      armed = true
      runPrune(script)
    })
  },
}

export default Plugin.define(PluginModule)
