import type { Plugin } from "@opencode-ai/plugin"
import { execFileSync } from "node:child_process"

// Pos-tool: a cada chamada de ferramenta, registra o tool call no working
// memory do ctx-window, incluindo input e output (truncados) para que a
// heuristica tenha conteudo real para extrair decisoes/artefatos/erros.
// Falhas sao silenciosas (exit != 0 nao bloqueia o tool call).
//
// LIMITACAO: hook tool.execute.after nao expoe o "thinking" do modelo
// (apenas args/output da tool). Para capturar raciocinio, seria preciso
// hook em chat.message ou similar (depende do que OpenCode expoe).
//
// LIMITACAO CONHECIDA (upstream #13574): mutacoes em output.output no hook
// tool.execute.after nem sempre sao refletidas na UI/contexto do modelo.
// Plugin e best-effort.

const MAX_CONTENT_CHARS = 16000

export const CtxCompact: Plugin = async ({ directory, client }: { directory: string; client?: any }) => {
  return {
    "tool.execute.after": async (input, output) => {
      try {
        const sessionID = (input as { sessionID?: string }).sessionID ?? "default"
        const toolName =
          (input as { tool?: string }).tool ?? "unknown"
        const args = (input as { args?: unknown }).args
        const argsStr = args !== undefined ? JSON.stringify(args).slice(0, MAX_CONTENT_CHARS) : ""
        const outputStr =
          output && typeof (output as { output?: unknown }).output === "string"
            ? ((output as { output: string }).output).slice(0, MAX_CONTENT_CHARS)
            : ""
        const parts = [argsStr, outputStr].filter((s) => s.length > 0)
        const commandArgs = ["on-tool-call-llm", sessionID, "--cli", "opencode", "--tool", toolName, "--project", directory]
        if (parts.length > 0) commandArgs.push("--input", parts.join(" | "))
        const stdout = execFileSync("ctx-window", commandArgs, { encoding: "utf8", timeout: 10000 })
        if (stdout && stdout.includes("[AVISO agent-sync]")) {
          const lines = stdout.split("\n")
          const nudge = lines.find((l) => l.includes("[AVISO agent-sync]"))
          if (nudge) {
            if (output && typeof (output as { output?: unknown }).output === "string") {
              (output as { output: string }).output += `\n\n${nudge}`
            }
            if (client?.tui?.showToast) {
              client.tui.showToast({
                title: "agent-sync: Contexto Alto",
                message: nudge,
                type: "warning",
              })
            }
          }
        }
      } catch {
        // best-effort: nunca bloqueia o tool call
      }
    },
    "experimental.session.compacting": async (input, output) => {
      try {
        const sessionID = (input as { sessionID?: string }).sessionID
        if (!sessionID) return
        const snapshot = execFileSync("ctx-window", ["show", sessionID], {
          encoding: "utf8",
          timeout: 10000,
        }).slice(0, MAX_CONTENT_CHARS)
        if (snapshot.trim()) output.context.push(`ctx-window (estado local da sessão):\n${snapshot}`)
      } catch {
        // sem snapshot local, a compactação nativa continua
      }
    },
  }
}
