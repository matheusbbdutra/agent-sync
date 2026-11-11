// Plugin v2 do OpenCode para o design hook do Impeccable.
//
// O Impeccable nao instala hook nativo no OpenCode. Este plugin observa
// tool.execute.after, delega a normalizacao/scan para
// `agent-sync hook impeccable --host=opencode` (Go) e injeta o texto via
// ctx.session.hook("context") — mesmo padrao do agent-react/memory-nudge.
//
// Regra §4: nenhuma regra do detector e reimplementada aqui. O binario Go
// valida opt-in do projeto, localiza a skill instalada e chama o engine
// oficial `impeccable hook`.
import { Plugin as _PluginNS } from "@opencode/plugin"
import { execFileSync } from "node:child_process"
import fs from "node:fs"
import os from "node:os"
import path from "node:path"

// Aliases locais baseados nos .d.ts reais do @opencode/plugin@2.0.11.
type Ctx = {
  readonly app: { version: string; channel: string }
  readonly location: { directory: string; project: { id: string }; workspaceID?: string }
  readonly options: Record<string, unknown>
  readonly client: unknown
  readonly tool: {
    hook(
      name: "execute.after",
      cb: (event: {
        tool: string
        sessionID: string
        agent: string
        messageID: string
        id: string
        input: unknown
      } & (
        | { status: "completed"; result: { output?: unknown; content?: ReadonlyArray<{ type: string; text?: string }>; metadata?: unknown } }
        | { status: "error"; error: unknown }
      )) => Promise<void> | void,
    ): Promise<{ dispose: () => Promise<void> }>
  }
  readonly session: {
    hook(
      name: "context",
      cb: (event: {
        sessionID: string
        agent: string
        system: Array<{ type: string; text?: string }>
        messages?: unknown[]
        options?: Record<string, unknown>
        tools?: Record<string, { description: string; input: unknown }>
        result?: unknown
        model?: unknown
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

const TIMEOUT_MS = 5000
const PENDING_SEPARATOR = "\n\n---\n\n"
const EDIT_TOOL =
  /(^|[._-])(edit|write|patch|apply_patch|multi_edit|multiedit|create_file|write_file)([._-]|$)/i

function resolveAgentSync(): string {
  const env = process.env.AGENT_SYNC_BIN
  if (env) return env
  const local = path.join(os.homedir(), ".local", "bin", "agent-sync")
  try {
    if (fs.existsSync(local)) return local
  } catch {
    // best-effort: cai no PATH
  }
  return "agent-sync"
}

function isEditTool(tool: string): boolean {
  const name = (tool || "").toLowerCase()
  if (!name) return false
  if (name === "edit" || name === "write" || name === "patch") return true
  return EDIT_TOOL.test(name)
}

function runHook(tool: string, sessionID: string, input: unknown, cwd: string): string {
  try {
    const out = execFileSync(resolveAgentSync(), ["hook", "impeccable", "--host=opencode"], {
      input: JSON.stringify({
        session_id: sessionID,
        cwd,
        tool_name: tool,
        tool_input: input ?? {},
      }),
      encoding: "utf8",
      timeout: TIMEOUT_MS,
      stdio: ["pipe", "pipe", "ignore"],
    })
    const parsed = JSON.parse(out) as { context?: unknown }
    return typeof parsed.context === "string" ? parsed.context : ""
  } catch {
    // Fail-open: binario ausente/timeout/JSON invalido nao derruba o turno.
    return ""
  }
}

const PluginModule: AnyPlugin = {
  id: "impeccable-design",
  async setup(ctx) {
    const pending = new Map<string, string[]>()

    await ctx.tool.hook("execute.after", async (event) => {
      if (event.status !== "completed") return
      if (!isEditTool(event.tool)) return

      const cwd = ctx.location?.directory || process.cwd()
      const contextText = runHook(event.tool, event.sessionID, event.input, cwd)
      if (!contextText) return

      const list = pending.get(event.sessionID) ?? []
      list.push(contextText)
      pending.set(event.sessionID, list)
    })

    await ctx.session.hook("context", (sEvent) => {
      const list = pending.get(sEvent.sessionID)
      if (!list || list.length === 0) return
      pending.delete(sEvent.sessionID)

      const text = list.join(PENDING_SEPARATOR)
      if (Array.isArray(sEvent.messages)) {
        sEvent.messages.push({
          role: "user",
          content: [{ type: "text", text }],
        })
      } else {
        sEvent.system.push({ type: "text", text })
      }
    })
  },
}

export default Plugin.define(PluginModule)
