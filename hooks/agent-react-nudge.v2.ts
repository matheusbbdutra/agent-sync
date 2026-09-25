// NOTA sobre o import `Plugin`: a doc oficial usa
//   `import { Plugin } from "@opencode/plugin"` e o runtime do opencode
//   (Bun bundler) aceita porque ele exporta o namespace `Plugin` que
//   contem `define(...)`. TypeScript estrito (CI) enxerga `Plugin` como
//   *namespace object* (devido a `export * as Plugin from "./plugin.js"`
//   no index.d.ts), o que quebra `const x: Plugin = { id, setup }`.
//   Resolucao que funciona nos dois mundos:
//
//     - Em runtime: Plugin.define({ id, setup }) vem do namespace.
//     - Em typecheck: usamos um cast via Helper local para dar forma
//       `{ id: string; setup: (ctx) => ... }` sem precisar importar o
//       type do subpath (que nao esta nos exports do package.json).
//
// Se um dia o @opencode/plugin adicionar `export type { Plugin }` ao
// `dist/promise/index.d.ts`, este hack pode sumir e voltar para o
// import simples.
import { Plugin as _PluginNS } from "@opencode/plugin"

// Aliases locais baseados nos .d.ts reais do @opencode/plugin@2.0.11.
// Mantemos local para evitar depender de subpaths nao-exportados.
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

// Lembrete pos-ferramenta: a cada N chamadas, cobra validacao de hipoteses
// (agent-react) — hipotese != fato.
//
// Migrado para opencode v2 (linha 2.x, runtime 2.0.11):
// - Hook equivalente: ctx.tool.hook("execute.after", ...) (executado apos cada
//   tool call; evento tem status="completed"|"error" e campo .result mutavel).
// - Equivalente ao antigo "mutar output.output": usamos ctx.session.hook("context", ...)
//   para injetar uma SystemPart no momento anterior a cada chamada de modelo.
//   Isso funciona CONFIAVELMENTE (substitui a limitacao upstream #13574 do v1).
// - Contador persistido em ctx.storage (chave por sessionID) — sobrevive a
//   recargas do plugin e a sessoes paralelas sem dupla-contagem.

const STORAGE_PREFIX = "agent-sync/react-nudge/count"
const STORAGE_LAST_NOTE_AT = "agent-sync/react-nudge/lastNoteAt"

type NoteState = { count: number; lastNoteAt: number }

const NOTE =
  "[agent-sync] Hipotese ativa sem validacao? Nao conclua/implemente como fato. " +
  "Valide com tool/leitura ou peca o passo concreto ao usuario. Skill: agent-react."

const PluginModule: AnyPlugin = {
  id: "agent-react-nudge",
  async setup(ctx) {
    const threshold = Number(process.env.AGENT_SYNC_REACT_NUDGE_THRESHOLD || 15)
    const pending = new Set<string>()

    await ctx.tool.hook("execute.after", async (event) => {
      if (event.status !== "completed") return
      const key = `${STORAGE_PREFIX}/${event.sessionID}`
      const current = (await ctx.storage.get(key)) as NoteState | undefined
      const nextCount = (current?.count ?? 0) + 1
      await ctx.storage.set(key, { count: nextCount, lastNoteAt: current?.lastNoteAt ?? 0 })
      if (nextCount % threshold !== 0) return
      pending.add(event.sessionID)
      await ctx.storage.set(STORAGE_LAST_NOTE_AT, nextCount)
    })

    // Hook persistente: filtra por sessionID (SessionContext traz
    // sessionID readonly) e consome o pending — one-shot por threshold,
    // sem reprocessar em model calls futuras.
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
