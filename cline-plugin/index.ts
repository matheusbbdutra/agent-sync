// index.ts: Cline Plugin (AgentPlugin) — ponte de hooks agent-sync para o Cline
// (A-87 — migração 1:1 do index.js original para TypeScript).
// (A-87 — migração 1:1 do index.js original para TypeScript).
//
// Cline CLI v3 NÃO executa hooks por arquivo em ~/.cline/hooks (o loader só é
// habilitado para config-extension com capability "hooks", nunca presente no
// CLI — ver docs/investigations/cline-hooks-contract.md). A rota suportada é
// um Cline Plugin (AgentPlugin) com `manifest.capabilities=["hooks"]`, que é
// exatamente o que este arquivo exporta.
//
// Este adapter é deliberadamente fino: toda a tradução de payload/contrato e
// a orquestração dos scripts agent-sync vivem no binário Go
// (`agent-sync hook cline --event=...`). Aqui só mapeamos os contextos do
// plugin para o payload Cline e o resultado de volta para o contrato interno
// do runtime ({appendContext} injeta contexto, {skip, reason} bloqueia a tool).
//
// Contrato interno observado no runtime do Cline:
//   beforeTool -> {input?, policy?, appendContext?, stop?, skip?, reason?}
//   afterTool  -> {appendContext?, result?}
//   beforeRun  -> {appendContext?, stop?}
//
// Origem: A-80.1'. Ver docs/ADR-cline-hooks-mcp-wiramento.md.
// Migração TS: A-87. Ver docs/ADR-cline-plugin-ts-first.md.

import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import type {
  AgentPlugin,
  AgentSyncConfig,
  BridgeResponse,
  ClineEvent,
  HookContext,
  HookResult,
} from "./types.js";

const BRIDGE_TIMEOUT_MS = Number(process.env.AGENT_SYNC_CLINE_TIMEOUT_MS || 15000);

/** Carrega agent-sync-config.json (opcional). */
function loadConfig(): AgentSyncConfig {
  // Tenta (1) o próprio diretório do bundle e (2) o diretório pai.
  // Após `npx tsc`, o plugin roda de `package/dist/` mas o wirer copia o
  // config para `package/` (um nível acima) — precisamos olhar nos dois.
  const candidates = [
    join(__dirname, "agent-sync-config.json"),
    join(__dirname, "..", "agent-sync-config.json"),
  ];
  for (const configPath of candidates) {
    try {
      if (!existsSync(configPath)) continue;
      const parsed = JSON.parse(readFileSync(configPath, "utf8")) as unknown;
      if (parsed && typeof parsed === "object") {
        return parsed as AgentSyncConfig;
      }
    } catch {
      // tenta próximo candidato
    }
  }
  return {};
}

const CONFIG = loadConfig();
const BIN: string = process.env.AGENT_SYNC_BIN || CONFIG.bin || "agent-sync";
const BASE_DIR: string = process.env.AGENT_SYNC_HOME || CONFIG.baseDir || "";
const DEBUG: boolean = process.env.AGENT_SYNC_CLINE_DEBUG === "1";

function debug(message: string): void {
  if (!DEBUG) return;
  try {
    process.stderr.write(`[agent-sync cline plugin] ${message}\n`);
  } catch {
    /* ignore */
  }
}

/**
 * callBridge executa o binário Go e devolve a resposta parseada.
 * Qualquer falha vira no-op: hook de nudge/guard nunca deve derrubar o run.
 */
function callBridge(event: ClineEvent, payload: Record<string, unknown>): BridgeResponse {
  if (!BASE_DIR) {
    debug("baseDir ausente (agent-sync-config.json) — bridge desativado");
    return {};
  }
  try {
    const out = execFileSync(
      BIN,
      ["hook", "cline", `--event=${event}`, `--base-dir=${BASE_DIR}`],
      {
        input: JSON.stringify(payload),
        encoding: "utf8",
        timeout: BRIDGE_TIMEOUT_MS,
        env: process.env,
      },
    );
    const trimmed = (out || "").trim();
    if (!trimmed) return {};
    return (JSON.parse(trimmed) as BridgeResponse) || {};
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    debug(`bridge falhou (${event}): ${msg}`);
    return {};
  }
}

/**
 * basePayload monta o payload no formato do Cline (mesmo shape que o loader de
 * hooks por arquivo entregaria), a partir do contexto do plugin.
 */
function basePayload(context: HookContext, hookName: string): Record<string, unknown> {
  const snapshot = (context && context.snapshot) || {};
  const toolCall = (context && context.toolCall) || {};
  const input = context && context.input;
  const payload: Record<string, unknown> = {
    taskId:
      snapshot.conversationId || snapshot.runId || snapshot.agentId || "default",
    hookName: hookName,
    iteration:
      typeof snapshot.iteration === "number" ? snapshot.iteration : undefined,
    sessionContext: {
      rootSessionId: snapshot.conversationId || snapshot.runId || "",
    },
    workspaceRoots: BASE_DIR ? [BASE_DIR] : [],
    agent_id: snapshot.agentId || "",
    parent_agent_id: snapshot.parentAgentId || null,
  };
  if (Object.prototype.hasOwnProperty.call(toolCall, "toolName")) {
    payload.tool_call = {
      id: toolCall.toolCallId || "",
      name: toolCall.toolName || "",
      input: input === undefined ? {} : input,
    };
  }
  return payload;
}

/**
 * toHookResult traduz a resposta do bridge para o contrato interno do Cline.
 * `cancel` do bridge = decisão de bloqueio (deny/permissionDecision), que no
 * plugin vira `skip` (bloqueia só a tool em vez de abortar o run inteiro).
 */
function toHookResult(response: BridgeResponse): HookResult | undefined {
  if (!response || typeof response !== "object") return undefined;
  if (response.cancel === true) {
    return {
      skip: true,
      reason: response.cancelReason || "agent-sync: bloqueado por hook",
    };
  }
  const context =
    typeof response.context === "string" ? response.context.trim() : "";
  if (!context) return undefined;
  return { appendContext: context };
}

const plugin: AgentPlugin = {
  name: "agent-sync-hooks",
  manifest: { capabilities: ["hooks"] },
  hooks: {
    beforeTool(context: HookContext): HookResult | undefined {
      return toHookResult(callBridge("tool_call", basePayload(context, "tool_call")));
    },
    afterTool(context: HookContext): HookResult | undefined {
      const payload = basePayload(context, "tool_result");
      const result = context && context.result;
      if (result !== undefined) {
        payload.tool_result = {
          id: (context.toolCall && context.toolCall.toolCallId) || "",
          name: (context.toolCall && context.toolCall.toolName) || "",
          input: context.input,
          output:
            result && typeof result === "object"
              ? (result as Record<string, unknown>).output
              : result,
          isError: Boolean(
            result && typeof result === "object" && (result as Record<string, unknown>).isError,
          ),
        };
        payload.postToolUse = {
          toolName: (context.toolCall && context.toolCall.toolName) || "",
          parameters: JSON.stringify(context.input === undefined ? {} : context.input),
          success: !(
            result &&
            typeof result === "object" &&
            (result as Record<string, unknown>).isError
          ),
        };
      }
      return toHookResult(callBridge("tool_result", payload));
    },
    beforeRun(context: HookContext): HookResult | undefined {
      const payload = basePayload(context, "agent_start");
      payload.taskStart = { taskMetadata: {} };
      return toHookResult(callBridge("agent_start", payload));
    },
    afterRun(context: HookContext): HookResult | undefined {
      const payload = basePayload(context, "agent_end");
      payload.taskComplete = { taskMetadata: {} };
      callBridge("agent_end", payload);
      return undefined;
    },
  },
};

// CommonJS export (Cline CLI carrega via `require()`).
// `export = plugin` é o padrão TS para módulos CommonJS.
export = plugin;