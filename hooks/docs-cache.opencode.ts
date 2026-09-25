import type { Plugin } from "@opencode-ai/plugin"

// Cacheia passivamente docs ja consultadas via webfetch ou context7
// (mcp__context7__query-docs), sem refazer requisicao de rede - so persiste
// o que a ferramenta ja trouxe, chamando o binario docs-cache-write.
//
// LIMITACAO CONHECIDA: o shape exato de `input` no hook tool.execute.after
// nao esta totalmente documentado publicamente (so confirmamos input.tool e
// output.output em exemplos parciais da doc). Este plugin tenta alguns
// campos comuns para os argumentos da tool (args/arguments/parameters) e
// nao falha caso nenhum bata - e best-effort, igual ao context-guard-nudge.

import { execFile } from "node:child_process"
import path from "node:path"

const WRITER_PATH = path.join(import.meta.dirname, "..", "bin", "docs-cache-write")

function firstString(...candidates: unknown[]): string {
  for (const c of candidates) {
    if (typeof c === "string" && c.length > 0) return c
  }
  return ""
}

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

export const DocsCache: Plugin = async () => {
  return {
    "tool.execute.after": async (input: any, output: any) => {
      const toolName = firstString(input?.tool, input?.name)
      const args = input?.args ?? input?.arguments ?? input?.parameters ?? {}
      const resultText = firstString(output?.output, output?.result, output?.text)

      if (toolName === "webfetch") {
        const url = firstString(args?.url, args?.URL)
        writeCache(url, resultText)
        return
      }

      if (toolName.endsWith("query-docs") || toolName.includes("context7") && toolName.includes("query")) {
        const lib = firstString(args?.libraryId, args?.context7CompatibleLibraryID, "unknown-library")
        const query = firstString(args?.query, args?.topic, "index")
        writeCache(`context7:/${lib}/${query}`, resultText)
      }
    },
  }
}
