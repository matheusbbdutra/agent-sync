import type { Plugin } from "@opencode-ai/plugin"
import { execSync } from "node:child_process"

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

function safeShellSingleQuote(s: string): string {
  // shell single-quote escape: ' -> '\''
  return "'" + s.replace(/'/g, "'\\''") + "'"
}

export const CtxCompact: Plugin = async () => {
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
        const inputArg = parts.length > 0 ? safeShellSingleQuote(parts.join(" | ")) : ""
        const cmd = inputArg
          ? `ctx-window on-tool-call-llm "${sessionID}" --cli opencode --tool "${toolName}" --input ${inputArg}`
          : `ctx-window on-tool-call-llm "${sessionID}" --cli opencode --tool "${toolName}"`
        execSync(cmd, { stdio: "ignore", timeout: 90000 })
      } catch {
        // best-effort: nunca bloqueia o tool call
      }
    },
  }
}