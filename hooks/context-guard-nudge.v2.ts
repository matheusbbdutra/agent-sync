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

// Lembrete pos-ferramenta: a cada N chamadas nesta sessao, cobra o
// carregamento da skill context-guard e a atualizacao do STATE.md.
//
// Migrado para opencode v2 (linha 2.x, runtime 2.0.11):
// - Hook equivalente: ctx.tool.hook("execute.after", ...).
// - Injecao confiavel do lembrete via ctx.session.hook("context", ...)
//   persistente antes da proxima chamada de modelo (substitui a limitacao
//   v1 #13574). O hook de sessao filtra por sessionID (SessionContext traz
//   sessionID readonly) e consome um pending setado pelo hook de tool.

const STORAGE_KEY_PREFIX = "agent-sync/context-guard-nudge/count"

const NOTE = "[agent-sync] Carregue context-guard e atualize STATE.md."

const PluginModule: AnyPlugin = {
  id: "context-guard-nudge",
  async setup(ctx) {
    const threshold = Number(process.env.AGENT_SYNC_NUDGE_THRESHOLD || 120)
    const pending = new Set<string>()

    await ctx.tool.hook("execute.after", async (event) => {
      if (event.status !== "completed") return
      const key = `${STORAGE_KEY_PREFIX}/${event.sessionID}`
      const current = (await ctx.storage.get(key)) as { count: number } | undefined
      const nextCount = (current?.count ?? 0) + 1
      await ctx.storage.set(key, { count: nextCount })
      if (nextCount % threshold !== 0) return
      pending.add(event.sessionID)
    })

    await ctx.session.hook("context", (sEvent) => {
      if (!pending.has(sEvent.sessionID)) return
      pending.delete(sEvent.sessionID)
      if (Array.isArray(sEvent.messages)) {
        sEvent.messages.push({
          role: "user",
          content: [{ type: "text", text: NOTE }],
        })
      } else {
        sEvent.system.push({ type: "text", text: NOTE })
      }
    })
  },
}

export default Plugin.define(PluginModule)
