// Plugin v2 do OpenCode para ctx-window nudge (PostToolUse cross-CLI).
//
// Espelha o hook bash hooks/ctx-window-nudge.sh no formato do OpenCode v2.
// Padrão identico a token-nudge.opencode.v2.ts: hack do namespace `Plugin`
// (explicado em precompact-snapshot.opencode.v2.ts linhas 24-31).
//
// Mecanismo:
//   - Hook em ctx.tool.hook("execute.after") conta tool calls por sessionID
//     (contador persistido em ctx.storage, sobrevive entre chamadas).
//   - Regra 1: tool calls >= AGENT_SYNC_CTX_WINDOW_NUDGE_MIN_TOOL_CALLS (80)
//     -> seta flag one-shot em storage.
//   - Regra 2: summary.md (.agent-sync/summary.md) ausente OU com mais de
//     AGENT_SYNC_CTX_WINDOW_NUDGE_MAX_AGE_HOURS (4h) -> seta flag.
//   - Hook em ctx.session.hook("context") injeta nota no system prompt
//     quando flag existe + consome a flag (one-shot para nao repetir).
//
// Observacao: o plugin v2 NAO faz spawn do binario ctx-window (rede de
// seguranca fica no bash hook wirado em Claude/Codex/Antigravity/Cursor).
// Aqui apenas lembra o agente via system context que summarize deve ser
// rodado manualmente ou pelo bash hook no Stop.
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

const STORAGE_COUNT = "agent-sync/ctx-window-nudge/count"
const STORAGE_PENDING = "agent-sync/ctx-window-nudge/should-nudge"
// Chave global auxiliar (lastNoteAt estilo agent-react-nudge): o
// `ctx.storage` do OpenCode v2.0.11 demonstrou persistir apenas valores
// primitivos (number/string), nao objects por sessao. Gravamos tambem
// o count e a flag em chave global para que o hook `context` possa
// saber se ha nudge pendente sem precisar ler objeto por sessao.
const STORAGE_LAST_NOTE_AT = "agent-sync/ctx-window-nudge/lastNoteAt"

type Pending = {
  count: number
  reason: string
  ts: string
}

const PluginModule: AnyPlugin = {
  id: "ctx-window-nudge",
  async setup(ctx) {
    const minCalls = Number(process.env.AGENT_SYNC_CTX_WINDOW_NUDGE_MIN_TOOL_CALLS || 80)
    const maxAgeHours = Number(process.env.AGENT_SYNC_CTX_WINDOW_NUDGE_MAX_AGE_HOURS || 4)

    // Hook PostToolUse (v2): conta e dispara nudge.
    await ctx.tool.hook("execute.after", async (event) => {
      if (event.status !== "completed") return
      const countKey = `${STORAGE_COUNT}/${event.sessionID}`
      try {
        const prev = (await ctx.storage.get(countKey)) as number | undefined
        const count = (typeof prev === "number" ? prev : 0) + 1
        await ctx.storage.set(countKey, count)

        let reason = ""
        // Regra 1: tool calls >= min.
        if (count >= minCalls) {
          reason = "tool_calls>=" + String(minCalls)
        }
        // Regra 2: summary.md velho ou ausente.
        try {
          const dir = ctx.location?.directory || ""
          if (dir) {
            const fs = await import("node:fs/promises")
            const summaryPath = dir + "/.agent-sync/summary.md"
            let mtime = 0
            let exists = true
            try {
              const st = await fs.stat(summaryPath)
              mtime = Math.floor(st.mtimeMs / 1000)
            } catch {
              exists = false
            }
            const now = Math.floor(Date.now() / 1000)
            const ageHours = exists ? Math.floor((now - mtime) / 3600) : Number.POSITIVE_INFINITY
            if (!exists) {
              reason = reason ? reason + "; summary_missing" : "summary_missing"
            } else if (ageHours >= maxAgeHours) {
              reason = reason
                ? reason + "; summary_age=" + String(ageHours) + "h>=" + String(maxAgeHours) + "h"
                : "summary_age=" + String(ageHours) + "h>=" + String(maxAgeHours) + "h"
            }
          }
        } catch {
          // Ignorar: filesystem inacessivel no sandbox do plugin.
        }

        if (reason) {
          // Sinal de nudge pendente: gravamos em chave global primitiva
          // (number). O OpenCode v2.0.11 demonstrou persistir valores
          // primitivos confiavelmente; objects por sessao nao persistem
          // (observado durante smoke T1 2026-09-20). Chave global
          // funciona como em agent-react-nudge: leitura no hook
          // `context` checa o numero != lastNoteAt sessao anterior.
          const pending: Pending = {
            count,
            reason,
            ts: new Date().toISOString(),
          }
          // Mantemos a chave por sessao para o caso de upgrades futuros
          // do SDK resolverem a persistencia de object. Sem efeito colateral
          // se o set for silenciosamente ignorado.
          await ctx.storage.set(`${STORAGE_PENDING}/${event.sessionID}`, pending.count)
          await ctx.storage.set(STORAGE_LAST_NOTE_AT, pending.count)
        }
      } catch {
        // Silent: storage indisponivel nesta sessao.
      }
    })

    // Injecao one-shot no system prompt quando flag esta ligada.
    // Estrategia: gravamos `lastNoteAt` (number) global apos nudge.
    // Ao receber `context`, checamos se o count desta sessao ja
    // consumiu o lastNoteAt (count > lastNoteAt consumido). Se sim,
    // injeta nota e zera lastNoteAt (one-shot).
    await ctx.session.hook("context", (sEvent) => {
      void (async () => {
        const countKey = `${STORAGE_COUNT}/${sEvent.sessionID}`
        try {
          const count = (await ctx.storage.get(countKey)) as number | undefined
          const lastNoteAt = (await ctx.storage.get(STORAGE_LAST_NOTE_AT)) as number | undefined
          if (typeof count !== "number" || typeof lastNoteAt !== "number") return
          if (count <= lastNoteAt) return
          // count > lastNoteAt -> nudge pendente. Injeta + consome.
          const note =
            "[agent-sync ctx-window-nudge] tool_calls=" + String(count) +
            " (lastNoteAt=" + String(lastNoteAt) + ")" +
            ". Considere rodar ctx-window summarize para manter summary.md atualizado."
          if (Array.isArray(sEvent.messages)) {
            sEvent.messages.push({
              role: "user",
              content: [{ type: "text", text: note }],
            })
          } else {
            sEvent.system.push({ type: "text", text: note })
          }
          await ctx.storage.set(STORAGE_LAST_NOTE_AT, count)
          // cleanup da chave por sessao (nao persistente em v2.0.11
          // mas mantido para upgrade-resilience).
          await ctx.storage.remove(`${STORAGE_PENDING}/${sEvent.sessionID}`)
        } catch {
          // Silent: storage indisponivel.
        }
      })()
    })
  },
}

export default Plugin.define(PluginModule)
