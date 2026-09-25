import type { Plugin } from "@opencode-ai/plugin"

// Lembrete pos-ferramenta: a cada N chamadas nesta sessao do plugin, cobra a
// checagem/gravacao de memoria via memory-mcp (store_memory).
//
// LIMITACAO CONHECIDA: mutacoes em output.output no hook tool.execute.after
// nem sempre sao refletidas de forma confiavel na UI/contexto do modelo
// (ver https://github.com/anomalyco/opencode/issues/13574). Este plugin e
// best-effort ate essa issue ser resolvida ou o comportamento ser validado
// manualmente em uma instancia real do OpenCode.

const THRESHOLD = Number(process.env.AGENT_SYNC_MEMORY_NUDGE_THRESHOLD || 25)

export const MemoryNudge: Plugin = async () => {
  let count = 0

  return {
    "tool.execute.after": async (_input, output) => {
      count += 1
      if (count % THRESHOLD !== 0) {
        return
      }
      const note = `[agent-sync] Aconteceu algo nesta sessao que deveria virar memoria (correcao do usuario, decisao de projeto, preferencia confirmada)? Se sim, grave um resumo com o porque via store_memory (memory-mcp).`
      if (output && typeof output.output === "string") {
        output.output += `\n\n${note}`
      }
    },
  }
}
