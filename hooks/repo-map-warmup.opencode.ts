import type { Plugin } from "@opencode-ai/plugin"
import { execFile } from "node:child_process"
import path from "node:path"

// Warm-up lazy do cache do repo-map no OpenCode.
//
// O OpenCode não expõe um hook de "session.start" confiável na API pública
// de plugins, então fazemos um warm-up preguiçoso: na primeira chamada de
// ferramenta de uma sessão, disparamos `repo-map --update --quiet` em
// background. Depois disso, o cache fica quente para quando o agente
// (ou usuário) quiser usar `--focus` ou `--summary`.
//
// IMPORTANTE: este plugin é best-effort. Falhas do binário (ausente,
// sem permissão, etc.) nunca devem bloquear tool calls. O retorno do hook
// é sempre {} para não poluir o prompt.

const WARMUP_FLAG = path.join(import.meta.dirname, ".repo-map-warmed")

function warmup(repoMapBin: string) {
  if (!repoMapBin) return
  const args = ["--update", "--quiet"]
  try {
    const child = execFile(repoMapBin, args, { timeout: 30_000 }, () => {})
    child.on("error", () => {
      // best-effort
    })
  } catch {
    // best-effort
  }
}

function resolveBin(): string {
  const env = process.env.AGENT_SYNC_REPO_MAP_BIN
  if (env) return env
  const inRepo = path.join(import.meta.dirname, "..", "bin", "repo-map")
  if (existsFile(inRepo)) return inRepo
  return "repo-map"
}

function existsFile(p: string): boolean {
  try {
    // sync para simplificar; chamada única na inicialização do plugin
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const fs = require("node:fs") as typeof import("node:fs")
    fs.accessSync(p, 1 /* X_OK */)
    return true
  } catch {
    return false
  }
}

export const RepoMapWarmup: Plugin = async () => {
  let armed = false
  const bin = resolveBin()
  return {
    "tool.execute.after": async () => {
      if (armed) return
      armed = true
      warmup(bin)
    },
  }
}