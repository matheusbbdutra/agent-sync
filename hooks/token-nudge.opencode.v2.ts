// Plugin v2 do OpenCode para token nudge (ADR-token-nudge-contract
// Decisao 4).
//
// Padrao identico ao agent-react-nudge.v2.ts: hack do namespace `Plugin`
// (explicado em agent-react-nudge.v2.ts linhas 1-17).
//
// Mecanismo:
//   - Hook em ctx.tool.hook("execute.after") captura fim de cada tool call.
//   - Tenta obter tokens via `ctx.client.session.tokens({sessionID})` (SDK
//     OpenCode v2.0.11; pode mudar ABI entre versoes).
//   - Cache em ctx.storage por sessionID para reuso entre chamadas.
//   - Hook em ctx.session.hook("context") injeta nota one-shot no system
//     prompt quando threshold atingido + sobe flag one-shot.
//
// Fallback gracioso: se `client.session.tokens` nao existir, plugin fica
// silent no-op (heuristica legada do shell hook nao aplica ao OpenCode v2 -
// contexto continua ate o limite do modelo). Documentado como D-32.
//
// Threshold: AGENT_SYNC_TOKEN_NUDGE_THRESHOLD (default 80, same do shell hook).

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
      name: "execute.before" | "execute.after",
      cb: (event: { tool: string; sessionID: string; agent: string; messageID: string; id: string; input: unknown; result?: unknown; status?: string; error?: unknown }) => Promise<void> | void,
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

const STORAGE_TOKEN_NUDGE = "agent-sync/token-nudge/should-nudge"
const STORAGE_TOKEN_STATUS = "agent-sync/token-nudge/status"

type TokenStatus = {
  input: number
  output: number
  total: number
  contextWindow: number
  utilizationPct: number
  thresholdPct: number
  shouldNudge: boolean
  ts: string
}

type SessionTokens = {
  input?: number
  output?: number
  total?: number
  contextWindow?: number
}

const PluginModule: AnyPlugin = {
  id: "token-nudge",
  async setup(ctx) {
    const threshold = Number(process.env.AGENT_SYNC_TOKEN_NUDGE_THRESHOLD || 80)

    await ctx.tool.hook("execute.after", async (event) => {
      if (event.status !== "completed") return
      try {
        // SDK OpenCode v2.0.11 expoe client.session.tokens({sessionID}).
        // Type unknown aqui - validamos em runtime.
        const client = ctx.client as { session?: { tokens?: (args: { sessionID: string }) => Promise<SessionTokens> } }
        if (!client?.session?.tokens) return
        const t = await client.session.tokens({ sessionID: event.sessionID })
        if (!t || typeof t.total !== "number") return
        const input = typeof t.input === "number" ? t.input : 0
        const output = typeof t.output === "number" ? t.output : Math.max(0, t.total - input)
        const contextWindow = typeof t.contextWindow === "number" ? t.contextWindow : 200000
        const utilPct = contextWindow > 0 ? Math.floor((t.total / contextWindow) * 100) : 0
        const should = utilPct >= threshold
        const status: TokenStatus = {
          input,
          output,
          total: t.total,
          contextWindow,
          utilizationPct: utilPct,
          thresholdPct: threshold,
          shouldNudge: should,
          ts: new Date().toISOString(),
        }
        await ctx.storage.set(`${STORAGE_TOKEN_STATUS}/${event.sessionID}`, status)
        if (should) {
          await ctx.storage.set(`${STORAGE_TOKEN_NUDGE}/${event.sessionID}`, status)
        }
      } catch {
        // Silent - SDK method pode nao existir nesta versao.
      }
    })

    await ctx.session.hook("context", (sEvent) => {
      void (async () => {
        const cached = (await ctx.storage.get(`${STORAGE_TOKEN_NUDGE}/${sEvent.sessionID}`)) as TokenStatus | undefined
        if (!cached) return
        const note =
          "[agent-sync token-nudge] utilization=" + cached.utilizationPct + "%" +
          " (total=" + cached.total + " / window=" + cached.contextWindow + ")" +
          " >= threshold=" + cached.thresholdPct + "%" +
          ". Considere /compact ou encerrar a sessao."
        if (Array.isArray(sEvent.messages)) {
          sEvent.messages.push({
            role: "user",
            content: [{ type: "text", text: note }],
          })
        } else {
          sEvent.system.push({ type: "text", text: note })
        }
        // one-shot: consumir para nao re-injetar.
        await ctx.storage.remove(`${STORAGE_TOKEN_NUDGE}/${sEvent.sessionID}`)
      })()
    })
  },
}

export default Plugin.define(PluginModule)
