// Cline Plugin: ponte de hooks agent-sync (A-80).
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

"use strict";

const fs = require("fs");
const path = require("path");
const { execFileSync } = require("child_process");

const BRIDGE_TIMEOUT_MS = Number(process.env.AGENT_SYNC_CLINE_TIMEOUT_MS || 15000);

function loadConfig() {
  const configPath = path.join(__dirname, "agent-sync-config.json");
  try {
    const parsed = JSON.parse(fs.readFileSync(configPath, "utf8"));
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch (err) {
    return {};
  }
}

const CONFIG = loadConfig();
const BIN = process.env.AGENT_SYNC_BIN || CONFIG.bin || "agent-sync";
const BASE_DIR = process.env.AGENT_SYNC_HOME || CONFIG.baseDir || "";
const DEBUG = process.env.AGENT_SYNC_CLINE_DEBUG === "1";

function debug(message) {
  if (!DEBUG) return;
  try {
    process.stderr.write(`[agent-sync cline plugin] ${message}\n`);
  } catch (err) {
    /* ignore */
  }
}

// callBridge executa o binário Go e devolve {cancel, context, cancelReason}.
// Qualquer falha vira no-op: hook de nudge/guard nunca deve derrubar o run.
function callBridge(event, payload) {
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
      }
    );
    const trimmed = (out || "").trim();
    if (!trimmed) return {};
    return JSON.parse(trimmed) || {};
  } catch (err) {
    debug(`bridge falhou (${event}): ${err && err.message}`);
    return {};
  }
}

// basePayload monta o payload no formato do Cline (mesmo shape que o loader de
// hooks por arquivo entregaria), a partir do contexto do plugin.
function basePayload(context, hookName) {
  const snapshot = (context && context.snapshot) || {};
  const toolCall = (context && context.toolCall) || {};
  const input = context && context.input;
  const payload = {
    taskId: snapshot.conversationId || snapshot.runId || snapshot.agentId || "default",
    hookName: hookName,
    iteration: typeof snapshot.iteration === "number" ? snapshot.iteration : undefined,
    sessionContext: { rootSessionId: snapshot.conversationId || snapshot.runId || "" },
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

// toHookResult traduz a resposta do bridge para o contrato interno do Cline.
// `cancel` do bridge = decisão de bloqueio (deny/permissionDecision), que no
// plugin vira `skip` (bloqueia só a tool em vez de abortar o run inteiro).
function toHookResult(response) {
  if (!response || typeof response !== "object") return undefined;
  if (response.cancel === true) {
    return {
      skip: true,
      reason: response.cancelReason || "agent-sync: bloqueado por hook",
    };
  }
  const context = typeof response.context === "string" ? response.context.trim() : "";
  if (!context) return undefined;
  return { appendContext: context };
}

module.exports = {
  name: "agent-sync-hooks",
  manifest: { capabilities: ["hooks"] },
  hooks: {
    beforeTool(context) {
      return toHookResult(callBridge("tool_call", basePayload(context, "tool_call")));
    },
    afterTool(context) {
      const payload = basePayload(context, "tool_result");
      const result = context && context.result;
      if (result !== undefined) {
        payload.tool_result = {
          id: (context.toolCall && context.toolCall.toolCallId) || "",
          name: (context.toolCall && context.toolCall.toolName) || "",
          input: context.input,
          output: result && typeof result === "object" ? result.output : result,
          isError: Boolean(result && result.isError),
        };
        payload.postToolUse = {
          toolName: (context.toolCall && context.toolCall.toolName) || "",
          parameters: JSON.stringify(context.input === undefined ? {} : context.input),
          success: !(result && result.isError),
        };
      }
      return toHookResult(callBridge("tool_result", payload));
    },
    beforeRun(context) {
      const payload = basePayload(context, "agent_start");
      payload.taskStart = { taskMetadata: {} };
      return toHookResult(callBridge("agent_start", payload));
    },
    afterRun(context) {
      const payload = basePayload(context, "agent_end");
      payload.taskComplete = { taskMetadata: {} };
      callBridge("agent_end", payload);
      return undefined;
    },
  },
};

