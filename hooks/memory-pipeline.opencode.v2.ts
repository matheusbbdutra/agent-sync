// Plugin v2 do OpenCode para pipeline de observação e consolidação contínua de memória
// (ADR-automated-memory-observation-pipeline).
//
// Integra com o memory-mcp em Go:
// 1. execute.after: bufferiza observações não-bloqueantes no buffer efêmero.
// 2. compaction: aciona consolidação de aprendizados para o memory.db (FTS5).

import { Plugin as _PluginNS } from "@opencode/plugin"
import { spawn } from "node:child_process"
import { existsSync } from "node:fs"
import { join } from "node:path"
import { homedir } from "node:os"

type Ctx = {
  readonly app: { version: string; channel: string }
  readonly location: { directory: string; project: { id: string }; workspaceID?: string }
  readonly storage: {
    get(key: string): Promise<unknown>
    set(key: string, value: unknown): Promise<void>
    remove(key: string): Promise<void>
  }
  readonly tool: {
    hook(
      name: "execute.before" | "execute.after",
      cb: (event: {
        tool: string
        sessionID: string
        agent: string
        input: unknown
        result?: unknown
        status?: string
        error?: unknown
      }) => Promise<void> | void,
    ): Promise<{ dispose: () => Promise<void> }>
  }
  readonly session: {
    hook(
      name: "context" | "compaction" | "title" | "generate",
      cb: (event: {
        sessionID: string
        agent: string
        system: Array<{ type: string; text?: string }>
        messages?: unknown[]
      }) => Promise<void> | void,
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

function resolveMemoryBin(): string | null {
  const local = join(homedir(), ".local", "bin", "memory-mcp")
  if (existsSync(local)) return local
  return "memory-mcp"
}

const PluginModule: AnyPlugin = {
  id: "memory-pipeline",
  async setup(ctx: Ctx) {
    const bin = resolveMemoryBin()
    if (!bin) return

    // 1. Buffer efêmero após execução de ferramentas
    await ctx.tool.hook("execute.after", async (event) => {
      try {
        const tool = event.tool || "unknown"
        const status = event.status || (event.error ? "error" : "ok")
        let note = ""
        if (event.error) {
          note = typeof event.error === "string" ? event.error : JSON.stringify(event.error)
        } else {
          note = `executou ${tool}`
        }

        const proc = spawn(
          bin,
          [
            "buffer-record",
            "-session",
            event.sessionID || "default",
            "-tool",
            tool,
            "-status",
            status,
            "-note",
            note.slice(0, 300),
            "-path",
            ctx.location.directory,
          ],
          { stdio: "ignore", detached: true },
        )
        proc.unref()
      } catch {
        // Best-effort: não interrompe o fluxo de execução da ferramenta
      }
    })

    // 2. Consolidação automática no fim de turno / compactação
    await ctx.session.hook("compaction", async (event) => {
      try {
        const proc = spawn(
          bin,
          [
            "consolidate",
            "-session",
            event.sessionID || "default",
            "-project",
            ctx.location.directory,
          ],
          { stdio: "ignore", detached: true },
        )
        proc.unref()
      } catch {
        // Best-effort
      }
    })
  },
}

export default Plugin.define(PluginModule)
