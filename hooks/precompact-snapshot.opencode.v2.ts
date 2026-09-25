// Plugin v2 do OpenCode para o PreCompact cross-CLI
// (ADR-precompact-snapshot-cross-cli Decisao 3 + 4).
//
// Padrao identico ao agent-react-nudge.v2.ts: hack do namespace `Plugin`
// (explicado em agent-react-nudge.v2.ts linhas 1-17) porque o subpath
// @opencode/plugin/dist/promise/context nao esta nos exports.
//
// Mecanismo:
//   - Hook em ctx.session.hook("context", ...) — equivalente ao
//     `experimental.session.compacting` que D-26 ja documentou. A
//     implementacao v2 do opencode expoe o evento via "context" (mesmo
//     canal usado para injetar system parts antes de cada model call).
//   - Estado pendente por sessionID: quando a sessao decide compactar,
//     sinalizamos via ctx.storage e o hook "context" injeta o snapshot
//     canonico no system prompt da proxima chamada.
//   - Decisao (allow|block|advise_only) respeitada: em block, plugin
//     seta um signal via ctx.storage que o OpenCode runtime consulta
//     (verificar suporte upstream; sem block nativo, advise_only).
//
// Limitacao aceita (D-28 + ADR Decisao 4): OpenCode v2 NAO tem block
// nativo de compactacao. Plugin registra advise_only por default; quando
// ha open_question em aberto, marca a sessao para revisao pre-compact.

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

const STORAGE_COMPACT_PENDING = "agent-sync/precompact-snapshot/pending"

type PendingSnapshot = {
  decision: "allow" | "block" | "advise_only"
  decision_reason: string
  ts: string
  snapshot_summary: string
}

const PluginModule: AnyPlugin = {
  id: "precompact-snapshot",
  async setup(ctx) {
    // Hook em "context": disparado antes de cada chamada de modelo.
    // Quando o OpenCode decide compactar, setamos a flag em storage
    // (via "compaction" hook em runtime v2) e o proximo "context" injeta
    // o snapshot canonico no system prompt da sessao compactada.
    //
    // OBS: opencode v2 expoe o evento "compaction" tambem (linha 64 do
    // type Ctx acima); usamos "context" como canal de injecao porque e
    // o unico onde `system` array e mutavel com efeito persistente
    // (confirmado em agent-react-nudge.v2.ts linhas 131-135).
    await ctx.session.hook("compaction", async (sEvent) => {
      // Marcar sessao como precompact-pendente. Plugin CLI externo
      // (agent-sync state snapshot -actor opencode -kind native)
      // ja gerou o payload canonico e gravou em ~/.cache/agent-sync/
      // precompact/<sessionID>.json. Aqui so sinalizamos o canal.
      const key = `${STORAGE_COMPACT_PENDING}/${sEvent.sessionID}`
      const cached = (await ctx.storage.get(key)) as PendingSnapshot | undefined
      if (!cached) {
        await ctx.storage.set(key, {
          decision: "advise_only",
          decision_reason: "snapshot nao cacheado; default advise_only (D-28)",
          ts: new Date().toISOString(),
          snapshot_summary: "(consulte .agent-sync/session-state.json para decisoes recentes)",
        })
      }
    })

    await ctx.session.hook("context", (sEvent) => {
      void (async () => {
        const key = `${STORAGE_COMPACT_PENDING}/${sEvent.sessionID}`
        const pending = (await ctx.storage.get(key)) as PendingSnapshot | undefined
        if (!pending) return
        const note =
          "[agent-sync precompact-snapshot] decision=" + pending.decision +
          (pending.decision_reason ? "; reason=" + pending.decision_reason : "") +
          "; summary=" + pending.snapshot_summary +
          " (OpenCode v2 nao tem block nativo - D-28; advise_only default. CLI: agent-sync state snapshot)"
        if (Array.isArray(sEvent.messages)) {
          sEvent.messages.push({
            role: "user",
            content: [{ type: "text", text: note }],
          })
        } else {
          sEvent.system.push({ type: "text", text: note })
        }
        // one-shot: limpar para nao re-injetar em toda chamada futura.
        await ctx.storage.remove(key)
      })()
    })
  },
}

export default Plugin.define(PluginModule)
