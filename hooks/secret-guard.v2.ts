// Plugin v2 do OpenCode para secret-guard (PreToolUse equivalente).
//
// O shell/TS dos outros hosts só consegue observar `tool.hook("execute.before")`.
// No OpenCode v2 a decisão acontece em `ctx.permission.hook("evaluate")`, cujo
// `effect` é mutável (`allow` | `deny` | `ask`). Este plugin usa esse hook para
// negar a tool call ANTES de o output chegar ao modelo.
//
// Regra §4: a deny-list e a detecção de literais vivem no binário Go
// `secret-scan` (internal/secretscan). O TS não reimplementa regex.
//
// Fail-open: binário ausente, timeout ou JSON inválido = allow. O objetivo é
// não derrubar o turno; a redação do ctx-window continua como segunda camada.
import { Plugin as _PluginNS } from "@opencode/plugin"
import { execFileSync } from "node:child_process"

type PermissionEvaluation = {
  readonly sessionID: string
  readonly agent?: string
  readonly action: string
  readonly resources: ReadonlyArray<string>
  readonly metadata?: Record<string, unknown>
  effect: "allow" | "deny" | "ask"
  message?: string
}

type Ctx = {
  readonly app: { version: string; channel: string }
  readonly location: { directory: string; project: { id: string }; workspaceID?: string }
  readonly options: Record<string, unknown>
  readonly client: unknown
  readonly permission?: {
    hook(
      name: "evaluate",
      cb: (evaluation: PermissionEvaluation) => Promise<void> | void,
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

const SECRET_SCAN_TIMEOUT_MS = 2000

// Ação do OpenCode -> payload aceito pelo `secret-scan check`. Ações de
// arquivo viram Read (file_path); ações de shell viram Bash (command).
// Ações desconhecidas ficam de fora para não gerar falso positivo.
function payloadFor(action: string, resource: string): { tool_name: string; tool_input: Record<string, string> } | null {
  const a = action.toLowerCase()
  if (a === "read" || a === "edit" || a === "write" || a === "patch" || a === "glob" || a === "grep" || a === "list") {
    // Normaliza path relativo para que a deny-list (prefixada por "/") case
    // `.env`, `id_rsa` etc. quando o host manda o caminho sem "./".
    const path = resource.startsWith("/") || resource.startsWith("~") || resource.startsWith("./")
      ? resource
      : "./" + resource
    return { tool_name: "Read", tool_input: { file_path: path } }
  }
  if (a === "bash" || a === "shell" || a === "execute") {
    return { tool_name: "Bash", tool_input: { command: resource } }
  }
  return null
}

function denyReason(action: string, resource: string): string {
  try {
    const payload = payloadFor(action, resource)
    if (!payload) return ""
    const out = execFileSync("secret-scan", ["check", "--format", "decision"], {
      input: JSON.stringify(payload),
      encoding: "utf8",
      timeout: SECRET_SCAN_TIMEOUT_MS,
    })
    const decision = JSON.parse(out) as { decision?: string; reason?: string }
    if (decision.decision === "deny") {
      return decision.reason || "secret-guard: recurso contém segredo ou caminho na deny-list"
    }
  } catch {
    // Fail-open: binário ausente/timeout não bloqueia o turno.
  }
  return ""
}

const PluginModule: AnyPlugin = {
  id: "secret-guard",
  async setup(ctx) {
    if (!ctx.permission || typeof ctx.permission.hook !== "function") return
    await ctx.permission.hook("evaluate", (evaluation) => {
      for (const resource of evaluation.resources || []) {
        if (!resource) continue
        const reason = denyReason(evaluation.action, resource)
        if (reason) {
          // Nunca rebaixa um deny já decidido pelo usuário; só adiciona deny.
          evaluation.effect = "deny"
          evaluation.message = reason
          return
        }
      }
    })
  },
}

export default Plugin.define(PluginModule)
