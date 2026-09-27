// payload.ts — normalização Cline → Claude (A-90).
//
// Migração 1:1 de `internal/hooks/cline_bridge.go:218-419` (que será deletado
// no passo 9 do plano). Sem dependências externas — só Node stdlib.
//
// Constantes alinhadas com a versão Go (cline_bridge.go:55-62):
const MAX_CONTEXT = 50_000;
const MAX_PAYLOAD = 4 << 20; // 4 MiB
const DEFAULT_TIMEOUT_MS = 10_000;

// ===== Eventos =====

/** Nomes canônicos Cline (runtime de plugin). */
export type ClineEvent = "tool_call" | "tool_result" | "agent_start" | "agent_end";

/** Nomes de arquivo aceitos pelo bridge legado (PreToolUse, etc.). */
const CLINE_EVENT_ALIASES: Record<string, ClineEvent> = {
  tool_call: "tool_call",
  tool_result: "tool_result",
  agent_start: "agent_start",
  agent_end: "agent_end",
  PreToolUse: "tool_call",
  PostToolUse: "tool_result",
  TaskStart: "agent_start",
  TaskComplete: "agent_end",
};

/** Canonicaliza nomes Cline (alias ou canônico) para `ClineEvent`. */
export function normalizeClineEvent(event: string): ClineEvent {
  const e = CLINE_EVENT_ALIASES[event];
  if (!e) throw new Error(`unknown cline event: ${event}`);
  return e;
}

/** Mapeia Cline → Claude (hook_event_name usado no payload normalizado). */
export function claudeEventForClineEvent(event: ClineEvent): string {
  switch (event) {
    case "tool_call":
      return "PreToolUse";
    case "tool_result":
      return "PostToolUse";
    case "agent_start":
      return "TaskStart";
    case "agent_end":
      return "TaskComplete";
  }
}

// ===== Payload =====

/** Shape Claude-compat produzido por `normalizedClinePayload` (Go). */
export interface NormalizedPayload {
  session_id: string;
  tool_name?: string;
  tool_input?: Record<string, unknown>;
  tool_response?: string;
  hook_event_name: string;
  cwd?: string;
  agent_kind: "cline";
  [k: string]: unknown;
}

/** Contexto mínimo de sessão extraído do payload Cline. */
export interface ClineSessionCtx {
  sessionId: string;
  conversationId?: string;
  taskId?: string;
  agentId?: string;
  parentAgentId?: string | null;
  cwd?: string;
}

/**
 * Normaliza payload Cline (raw) para shape Claude-compat.
 * Paridade 1:1 com `internal/hooks/cline_bridge.go:261 normalizedClinePayload`.
 */
export function normalizedClinePayload(
  event: string,
  raw: Uint8Array | string,
): { payload: NormalizedPayload; ctx: ClineSessionCtx } {
  const e = normalizeClineEvent(event);
  const claudeEvent = claudeEventForClineEvent(e);

  // Parse seguro — payload inválido vira no-op (NUNCA derruba o run).
  let parsed: Record<string, unknown> = {};
  try {
    const text =
      typeof raw === "string" ? raw : Buffer.from(raw).toString("utf8");
    parsed = text.trim() === "" ? {} : (JSON.parse(text) as Record<string, unknown>);
  } catch {
    parsed = {};
  }

  const ctx: ClineSessionCtx = {
    sessionId:
      (parsed["conversationId"] as string) ??
      (parsed["session_id"] as string) ??
      (parsed["taskId"] as string) ??
      "unknown",
    conversationId: parsed["conversationId"] as string | undefined,
    taskId: parsed["taskId"] as string | undefined,
    agentId: parsed["agentId"] as string | undefined,
    parentAgentId: (parsed["parentAgentId"] as string | null) ?? null,
    cwd: (parsed["cwd"] as string) ?? process.cwd(),
  };

  const payload: NormalizedPayload = {
    session_id: ctx.sessionId,
    hook_event_name: claudeEvent,
    cwd: ctx.cwd,
    agent_kind: "cline",
  };

  // tool_call / tool_result precisam de tool_name + tool_input
  if (e === "tool_call" || e === "tool_result") {
    const toolCall = parsed["toolCall"] as Record<string, unknown> | undefined;
    const toolName =
      (toolCall?.["toolName"] as string) ?? (parsed["toolName"] as string) ?? "unknown";
    payload.tool_name = toolName;

    const rawInput = toolCall?.["input"] ?? parsed["input"];
    const toolInput = normalizeToolInput(rawInput, toolName);
    payload.tool_input = toolInput;

    if (e === "tool_result") {
      const toolResponse =
        (toolCall?.["result"] as string) ?? (parsed["tool_response"] as string);
      if (toolResponse !== undefined) {
        payload.tool_response = String(toolResponse);
      }
    }
  }

  return { payload, ctx };
}

/**
 * Normaliza tool_input Cline → Claude-compat.
 * - Bash: sintetiza `command` a partir de `commands[]` se necessário.
 * - Demais tools: passa input adiante.
 */
function normalizeToolInput(
  rawInput: unknown,
  toolName: string,
): Record<string, unknown> {
  if (rawInput == null) return {};

  // Se já é objeto, usa direto
  if (typeof rawInput === "object" && !Array.isArray(rawInput)) {
    const obj = { ...(rawInput as Record<string, unknown>) };

    // Bash: se input tem `commands[]` mas não tem `command`, sintetiza.
    if (toolName === "Bash" && Array.isArray(obj["commands"]) && !obj["command"]) {
      obj["command"] = (obj["commands"] as unknown[]).join("\n");
    }

    // Fallback para `parameters` string (probe de Cline v3.0.65)
    if (obj["parameters"] != null && typeof obj["parameters"] === "string") {
      try {
        obj["parameters"] = JSON.parse(obj["parameters"] as string);
      } catch {
        // mantém string se parse falhar
      }
    }
    return obj;
  }

  // Se input é string (parameters como string), tenta parsear
  if (typeof rawInput === "string") {
    try {
      const parsed = JSON.parse(rawInput) as Record<string, unknown>;
      return normalizeToolInput(parsed, toolName);
    } catch {
      return { raw: rawInput };
    }
  }

  return { raw: rawInput };
}

/** Trunca context para tamanho máximo (mesma regra do Go: cline_bridge.go:536). */
export function truncateContext(s: string, max = MAX_CONTEXT): string {
  return s.length > max ? s.slice(0, max) : s;
}

/** Limite de payload parseável (4 MiB — alinhado com Go). */
export function maxPayloadBytes(): number {
  return MAX_PAYLOAD;
}

/** Timeout padrão para chamada a binário core (10s — alinhado com Go). */
export function defaultTimeoutMs(): number {
  return DEFAULT_TIMEOUT_MS;
}