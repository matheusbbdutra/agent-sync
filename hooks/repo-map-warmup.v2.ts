// Plugin novo: só existe em v2 (regra "novo plugin só v2").
// Warm-up lazy do cache do repo-map no OpenCode v2.
//
// Na primeira chamada de ferramenta da sessão, dispara
// `repo-map --update --quiet` em background. Depois disso o cache fica
// quente para `--focus` / `--summary`. Best-effort: falhas nunca bloqueiam.
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

function resolveBin(): string {
  const env = process.env.AGENT_SYNC_REPO_MAP_BIN
  if (env) return env
  return path.join(import.meta.dirname, "..", "bin", "repo-map")
}

function warmup(repoMapBin: string) {
  if (!repoMapBin) return
  try {
    const child = execFile(repoMapBin, ["--update", "--quiet"], { timeout: 30_000 }, () => {})
    child.on("error", () => {})
  } catch {
    // best-effort
  }
}

const PluginModule: AnyPlugin = {
  id: "repo-map-warmup",
  async setup(ctx) {
    void ctx.location.directory
    let armed = false
    const bin = resolveBin()
    await ctx.tool.hook("execute.after", async () => {
      if (armed) return
      armed = true
      warmup(bin)
    })
  },
}

export default Plugin.define(PluginModule)
