// types.ts: tipos locais do plugin Cline (A-87).
//
// Cópia local dos contratos observados empiricamente (não dependemos de
// `@cline/sdk` — subpath exports podem mudar entre versões; mesmo padrão
// dos `*.v2.ts` que copiam `PluginContext` de `@opencode/plugin`, ver
// `docs/ADR-opencode-v2-ts-runtime.md:36`).
//
// Fontes verificadas:
//   - `cline-plugin/index.js:1-160` (adapter atual, usado em produção)
//   - `docs/investigations/cline-hooks-contract.md` (engenharia reversa)
//   - `docs.cline.bot/sdk/plugins` (overview da API AgentPlugin)

// ===== Bridge (chamada ao binário Go `agent-sync hook cline`) =====

/** Eventos Cline aceitos pelo bridge. */
export type ClineEvent =
  | "tool_call"      // PreToolUse
  | "tool_result"    // PostToolUse
  | "agent_start"    // TaskStart
  | "agent_end";     // TaskComplete

/** Payload enviado ao bridge no stdin (JSON-encoded). */
export interface BridgeRequest {
  event: ClineEvent;
  payload: Record<string, unknown>;
}

/** Resposta do bridge (JSON no stdout). */
export interface BridgeResponse {
  cancel?: boolean;
  cancelReason?: string;
  context?: string;
  [k: string]: unknown;
}

// ===== Plugin shape (Cline AgentPlugin) =====

/** Capabilities registradas no manifest do plugin. */
export interface AgentPluginManifest {
  capabilities: ReadonlyArray<string>;
}

/** Contexto entregue ao plugin em cada hook. */
export interface HookContext {
  snapshot?: {
    conversationId?: string;
    runId?: string;
    agentId?: string;
    parentAgentId?: string | null;
    iteration?: number;
  };
  toolCall?: {
    toolCallId?: string;
    toolName?: string;
  };
  input?: unknown;
  result?: unknown;
}

/** Retorno de um hook do plugin (contrato interno do Cline). */
export interface HookResult {
  appendContext?: string;
  skip?: boolean;
  reason?: string;
  result?: unknown;
  input?: unknown;
  policy?: unknown;
  stop?: boolean;
}

/** Contrato AgentPlugin exportado pelo módulo. */
export interface AgentPlugin {
  name: string;
  manifest: AgentPluginManifest;
  hooks: {
    beforeTool?: (ctx: HookContext) => HookResult | undefined | void;
    afterTool?: (ctx: HookContext) => HookResult | undefined | void;
    beforeRun?: (ctx: HookContext) => HookResult | undefined | void;
    afterRun?: (ctx: HookContext) => HookResult | undefined | void;
    beforeModel?: (ctx: HookContext) => HookResult | undefined | void;
    afterModel?: (ctx: HookContext) => HookResult | undefined | void;
    onEvent?: (ctx: HookContext) => HookResult | undefined | void;
  };
}

// ===== Config interno =====

export interface AgentSyncConfig {
  bin?: string;
  baseDir?: string;
}
