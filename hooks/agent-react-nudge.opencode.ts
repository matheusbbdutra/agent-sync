import type { Plugin } from "@opencode-ai/plugin"

// Lembrete pos-ferramenta: a cada N chamadas, cobra validacao de hipoteses
// (agent-react) — hipotese != fato.
//
// LIMITACAO CONHECIDA: mutacoes em output.output no hook tool.execute.after
// nem sempre sao refletidas de forma confiavel na UI/contexto do modelo
// (ver https://github.com/anomalyco/opencode/issues/13574). Este plugin e
// best-effort ate essa issue ser resolvida ou o comportamento ser validado
// manualmente em uma instancia real do OpenCode.

const THRESHOLD = Number(process.env.AGENT_SYNC_REACT_NUDGE_THRESHOLD || 15)

export const AgentReactNudge: Plugin = async () => {
  let count = 0

  return {
    "tool.execute.after": async (_input, output) => {
      count += 1
      if (count % THRESHOLD !== 0) {
        return
      }
      const note = `[agent-sync] Hipotese ativa sem validacao? Nao conclua/implemente como fato. Valide com tool/leitura ou peca o passo concreto ao usuario. Skill: agent-react.`
      if (output && typeof output.output === "string") {
        output.output += `\n\n${note}`
      }
    },
  }
}
