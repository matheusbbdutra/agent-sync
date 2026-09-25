// Plugin v2 do OpenCode para ctx-window summarize no Stop cross-CLI.
//
// Espelha o hook bash hooks/ctx-window-summarize-at-stop.sh. Padrao
// identico a precompact-snapshot.opencode.v2.ts (hack do namespace
// `Plugin`, ctx.session.hook para injecao one-shot).
//
// Mecanismo:
//   - Hook em ctx.tool.hook("execute.after") conta tool calls por
//     sessionID em ctx.storage (sobrevive entre chamadas).
//   - Hook em ctx.session.hook("compaction") (ou quando o OpenCode sinaliza
//     fim de sessao via "generate" sem mais messages): verifica threshold
//     de tool calls + idade do summary.md -> injeta nota one-shot no system
//     prompt da proxima chamada pedindo summarize manual.
//
// Limitacao aceita (D-26 + decisao desta entrega): o plugin v2 NAO faz
// spawn do binario ctx-window summarize (rede de seguranca fica no bash
// hook wirado em Claude/Codex/Antigravity/Cursor). Aqui apenas sinaliza
// via system context que summarize deve ser rodado manualmente ou pelo
// bash hook no fim de sessao equivalente (Stop event). Plugin observable
// + advisory; nao bloqueia nativo.
//
// Limiar:
//   - tool_calls >= AGENT_SYNC_CTX_SUMMARIZE_MIN_TOOL_CALLS (100)
//   - summary.md ausente OU com idade >= AGENT_SYNC_CTX_SUMMARIZE_MAX_AGE_HOURS (4h)
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

const STORAGE_COUNT = "agent-sync/ctx-window-summarize-stop/count"
const STORAGE_PENDING = "agent-sync/ctx-window-summarize-stop/should-summarize"
// Chave global auxiliar (padrao agent-react-nudge). OpenCode v2.0.11
// demonstrou persistir apenas valores primitivos; objects por sessao
// nao persistem (smoke T1/T2 2026-09-20). Usamos lastNoteAt (number)
// como sinal one-shot consumido pelo hook `context`.
const STORAGE_LAST_NOTE_AT = "agent-sync/ctx-window-summarize-stop/lastNoteAt"

type Pending = {
  count: number
  reason: string
  ts: string
}

async function readSummaryAgeSeconds(dir: string, maxAgeHours: number): Promise<string | null> {
  if (!dir) return null
  try {
    const fs = await import("node:fs/promises")
    const summaryPath = dir + "/.agent-sync/summary.md"
    try {
      const st = await fs.stat(summaryPath)
      const now = Math.floor(Date.now() / 1000)
      const ageHours = Math.floor((now - Math.floor(st.mtimeMs / 1000)) / 3600)
      if (ageHours >= maxAgeHours) {
        return "summary_age=" + String(ageHours) + "h>=" + String(maxAgeHours) + "h"
      }
      return null
    } catch {
      return "summary_missing"
    }
  } catch {
    return null
  }
}

const PluginModule: AnyPlugin = {
  id: "ctx-window-summarize-at-stop",
  async setup(ctx) {
    const minCalls = Number(process.env.AGENT_SYNC_CTX_SUMMARIZE_MIN_TOOL_CALLS || 100)
    const maxAgeHours = Number(process.env.AGENT_SYNC_CTX_SUMMARIZE_MAX_AGE_HOURS || 4)
    const dir = ctx.location?.directory || ""

    // Contador por sessao (sobrevive entre chamadas).
    await ctx.tool.hook("execute.after", async (event) => {
      if (event.status !== "completed") return
      const countKey = `${STORAGE_COUNT}/${event.sessionID}`
      try {
        const prev = (await ctx.storage.get(countKey)) as number | undefined
        await ctx.storage.set(countKey, (typeof prev === "number" ? prev : 0) + 1)
      } catch {
        // Silent.
      }
    })

    // Quando o OpenCode v2 sinaliza fim de sessao / compactacao,
    // avaliamos se summarize deve ser disparado e injetamos nota
    // one-shot no system prompt da proxima chamada.
    await ctx.session.hook("compaction", (sEvent) => {
      void (async () => {
        const countKey = `${STORAGE_COUNT}/${sEvent.sessionID}`
        const pendingKey = `${STORAGE_PENDING}/${sEvent.sessionID}`
        try {
          const prev = (await ctx.storage.get(countKey)) as number | undefined
          const count = typeof prev === "number" ? prev : 0
          let reason = ""
          if (count >= minCalls) {
            reason = "tool_calls=" + String(count) + ">=" + String(minCalls)
          }
          const ageReason = await readSummaryAgeSeconds(dir, maxAgeHours)
          if (ageReason) {
            reason = reason ? reason + "; " + ageReason : ageReason
          }
          if (!reason) return
          const pending: Pending = {
            count,
            reason,
            ts: new Date().toISOString(),
          }
          // Mesmo padrao de ctx-window-nudge: gravamos number global.
          // Object per-session nao persiste em v2.0.11 (smoke 2026-09-20).
          await ctx.storage.set(pendingKey, pending.count)
          await ctx.storage.set(STORAGE_LAST_NOTE_AT, pending.count)
          // Reset contador para o proximo ciclo.
          await ctx.storage.set(countKey, 0)
        } catch {
          // Silent.
        }
      })()
    })

    // Injecao one-shot no system prompt.
    // Padrao identico ao nudge: chave global lastNoteAt (number) ja
    // gravada pelo hook `compaction`. Aqui comparamos count atual
    // (que foi zerado pelo compaction) com lastNoteAt. Se count=0 e
    // lastNoteAt > 0, injeta e zera lastNoteAt (consome one-shot).
    await ctx.session.hook("context", (sEvent) => {
      void (async () => {
        try {
          const countKey = `${STORAGE_COUNT}/${sEvent.sessionID}`
          const count = (await ctx.storage.get(countKey)) as number | undefined
          const lastNoteAt = (await ctx.storage.get(STORAGE_LAST_NOTE_AT)) as number | undefined
          // Caso 1: compaction rodou -> count=0 (reset) e lastNoteAt>0.
          // Caso 2: ainda nao houve compaction -> lastNoteAt=0 ou aus.
          // Em ambos: so injetar se lastNoteAt>0 E count=0.
          if (typeof lastNoteAt !== "number" || lastNoteAt <= 0) return
          if (typeof count === "number" && count > 0) return
          const note =
            "[agent-sync ctx-window-summarize] last_compact_count=" + String(lastNoteAt) +
            ". Considere rodar ctx-window summarize antes de fechar a sessao " +
            "(summary.md vira insumo do SessionStart da proxima sessao)."
          if (Array.isArray(sEvent.messages)) {
            sEvent.messages.push({
              role: "user",
              content: [{ type: "text", text: note }],
            })
          } else {
            sEvent.system.push({ type: "text", text: note })
          }
          await ctx.storage.set(STORAGE_LAST_NOTE_AT, 0)
          await ctx.storage.remove(`${STORAGE_PENDING}/${sEvent.sessionID}`)
        } catch {
          // Silent.
        }
      })()
    })
  },
}

export default Plugin.define(PluginModule)
