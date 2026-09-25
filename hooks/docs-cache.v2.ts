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

// Cacheia passivamente docs ja consultadas via webfetch ou context7
// (mcp__context7__query-docs), sem refazer requisicao de rede - so persiste
// o que a ferramenta ja trouxe, chamando o binario docs-cache-write.
//
// Migrado para opencode v2 (linha 2.x, runtime 2.0.11):
// - Hook equivalente para tool.execute.after: ctx.tool.hook("execute.after", ...).
// - O evento em v2 expoe `tool`, `sessionID`, `input` (args) e `result` para
//   status="completed". Pegamos input diretamente (sem fallback input.args/etc
//   do v1 - o shape agora e unico).
// - import.meta.dirname resolve para o diretorio do arquivo, igual v1.

import { execFile } from "node:child_process"
import path from "node:path"

const WRITER_PATH = path.join(import.meta.dirname, "..", "bin", "docs-cache-write")

function writeCache(url: string, text: string) {
  if (!url || !text) return
  const payload = JSON.stringify({ url, contentType: "text/plain", text })
  try {
    const child = execFile(WRITER_PATH, [], { timeout: 10000 }, () => {})
    child.stdin?.end(payload)
  } catch {
    // best-effort: nunca falha o hook por causa do cache
  }
}

const PluginModule: AnyPlugin = {
  id: "docs-cache",
  async setup(ctx) {
    await ctx.tool.hook("execute.after", async (event) => {
      if (event.status !== "completed") return
      const toolName = event.tool
      const args = (event.input ?? {}) as {
        url?: unknown
        URL?: unknown
        libraryId?: unknown
        context7CompatibleLibraryID?: unknown
        query?: unknown
        topic?: unknown
      }
      // Em v2, Tool.Result = { output?, content?, metadata? }. Pegamos
      // `output` se string, ou primeiro `content` do tipo "text" (read/file).
      let resultText = ""
      const r = event.result
      if (r) {
        if (typeof r.output === "string") {
          resultText = r.output
        } else if (Array.isArray(r.content)) {
          const firstText = r.content.find((c) => c.type === "text")
          if (firstText && "text" in firstText) resultText = firstText.text
        }
      }

      if (toolName === "webfetch") {
        const url = typeof args.url === "string" ? args.url : typeof args.URL === "string" ? args.URL : ""
        writeCache(url, resultText)
        return
      }

      if (toolName.endsWith("query-docs") || (toolName.includes("context7") && toolName.includes("query"))) {
        const lib =
          typeof args.libraryId === "string"
            ? args.libraryId
            : typeof args.context7CompatibleLibraryID === "string"
              ? args.context7CompatibleLibraryID
              : "unknown-library"
        const query =
          typeof args.query === "string" ? args.query : typeof args.topic === "string" ? args.topic : "index"
        writeCache(`context7:/${lib}/${query}`, resultText)
      }
    })
  },
}

export default Plugin.define(PluginModule)
