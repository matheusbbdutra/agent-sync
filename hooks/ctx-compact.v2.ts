// NOTA: veja o comentario extenso em agent-react-nudge.v2.ts sobre o
// pattern `Plugin` (namespace + cast via helper). O type alias `Ctx`
// abaixo e uma copia local do PluginContext v2.0.11 (subpath
// `@opencode/plugin/dist/promise/context` nao esta nos exports).
import { Plugin as _PluginNS } from "@opencode/plugin"

type Ctx = {
  readonly app: { version: string; channel: string }
  readonly location: { directory: string; project: { id: string }; workspaceID?: string }
  readonly options: Record<string, unknown>
  readonly client: unknown
  readonly storage: {
    get(key: string): Promise<unknown>
    set(key: string, value: unknown): Promise<void>
    remove(key: string): Promise<void>
    scan(options: { prefix: string; limit?: number; after?: string }): Promise<{
      entries: readonly { key: string; value: unknown }[]
      next?: string
    }>
  }
  readonly tool: {
    hook(
      name: "execute.before",
      cb: (event: {
        tool: string
        sessionID: string
        agent: string
        messageID: string
        id: string
        input: unknown
      }) => Promise<void> | void,
    ): Promise<{ dispose: () => Promise<void> }>
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
      name: "context" | "compaction" | "title" | "generate",
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
import { execFileSync } from "node:child_process"

// Pos-tool: a cada chamada de ferramenta, registra o tool call no working
// memory do ctx-window, incluindo input e output (truncados) para que a
// heuristica tenha conteudo real para extrair decisoes/artefatos/erros.
// Falhas sao silenciosas (exit != 0 nao bloqueia o tool call).
//
// Migrado para opencode v2 (linha 2.x, runtime 2.0.11):
// - Hook equivalente para tool.execute.after: ctx.tool.hook("execute.after", ...).
//   O evento em v2 expoe `tool`, `sessionID`, `agent`, `messageID`, `id`, `input`
//   e, em status="completed", `result` (estrutura em vez do antigo `output`).
// - O hook v1 `experimental.session.compacting` virou
//   `ctx.session.hook("compaction", ...)`. O evento herda SessionContext (com
//   `system`, `tools`, `options`) E expoe `result?: SessionCompactionResult`
//   — se voce setar `result.summary`, o opencode pula a chamada ao modelo.
//   Aqui so injetamos um snapshot local via `event.context` (que vira parte do
//   prompt de compactacao); nao usamos o atalho de substituir o summary.
//
// LIMITACAO preservada: client.tui.showToast do v1 NAO tem equivalente direto
// no PluginContext v2 confirmado em 2.0.11 (a doc mostra ctx.ui, mas o .d.ts
// instalado nao expoe `ui`). O nudge continua aparecendo no log do tool call,
// como no v1.

const MAX_CONTENT_CHARS = 16000

function safeExec(args: string[]): string {
  try {
    return execFileSync("ctx-window", args, { encoding: "utf8", timeout: 10000 }).slice(0, MAX_CONTENT_CHARS)
  } catch {
    return ""
  }
}

const PluginModule: AnyPlugin = {
  id: "ctx-compact",
  async setup(ctx) {
    const projectDir = ctx.location.directory
    const pendingNudge = new Map<string, string>()

    await ctx.tool.hook("execute.after", async (event) => {
      if (event.status !== "completed") return
      const sessionID = event.sessionID
      const toolName = event.tool
      const argsStr = event.input !== undefined ? JSON.stringify(event.input).slice(0, MAX_CONTENT_CHARS) : ""
      // v1 lia `output.output` (string). Em v2, o schema e Tool.Result
      // ({ output?, content?, metadata? }). Pegamos `output` se string,
      // senao tentamos o primeiro item de `content` (read/file tools).
      let outputStr = ""
      const r = event.result
      if (r) {
        if (typeof r.output === "string") {
          outputStr = r.output.slice(0, MAX_CONTENT_CHARS)
        } else if (Array.isArray(r.content)) {
          const firstText = r.content.find((c) => c.type === "text")
          if (firstText && "text" in firstText) {
            outputStr = firstText.text.slice(0, MAX_CONTENT_CHARS)
          }
        }
      }
      const parts = [argsStr, outputStr].filter((s) => s.length > 0)
      const commandArgs = [
        "on-tool-call-llm",
        sessionID,
        "--cli",
        "opencode",
        "--tool",
        toolName,
        "--project",
        projectDir,
      ]
      if (parts.length > 0) commandArgs.push("--input", parts.join(" | "))
      const stdout = safeExec(commandArgs)
      if (!stdout.includes("[AVISO agent-sync]")) return
      const nudge = stdout.split("\n").find((l) => l.includes("[AVISO agent-sync]"))
      if (!nudge) return
      // Marca nudge pendente para a proxima chamada de modelo desta sessao.
      // O hook "context" persistente abaixo consome (one-shot por sessao).
      pendingNudge.set(sessionID, nudge)
    })

    await ctx.session.hook("context", (sEvent) => {
      const nudge = pendingNudge.get(sEvent.sessionID)
      if (!nudge) return
      pendingNudge.delete(sEvent.sessionID)
      if (Array.isArray(sEvent.messages)) {
        sEvent.messages.push({
          role: "user",
          content: [{ type: "text", text: nudge }],
        })
      } else {
        sEvent.system.push({ type: "text", text: nudge })
      }
    })

    await ctx.session.hook("compaction", async (event) => {
      const sessionID = event.sessionID
      const snapshot = safeExec(["show", sessionID])
      if (!snapshot.trim()) return
      // event.context nao existe em SessionCompaction — a doc de migrate-v1
      // diz para usar `event.system` (compaction herda SessionContext).
      event.system.push({
        type: "text",
        text: `ctx-window (estado local da sessao):\n${snapshot}`,
      })
    })
  },
}

export default Plugin.define(PluginModule)
