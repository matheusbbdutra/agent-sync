/**
 * agent-sync — adapter fino do Pi (regra §4: Go é a fonte da verdade).
 * Cada evento nativo chama `agent-sync hook pi --event=...`; o binário Go
 * decide e devolve { context?, block?, reason?, output? }.
 * Sem lógica aqui. Falha em qualquer chamada é fail-open.
 */
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { homedir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

type HookResult = { context?: string; block?: boolean; reason?: string; output?: string };
type BridgeConfig = { agent_sync_bin?: string; base_dir?: string };
type CtxLike = {
	cwd?: string;
	model?: { provider?: string; id?: string };
	sessionManager?: { getSessionId?: () => string };
	getContextUsage?: () => { tokens: number | null; contextWindow: number } | undefined;
};

const selfDir = dirname(fileURLToPath(import.meta.url));

function loadConfig(): BridgeConfig {
	try {
		return JSON.parse(readFileSync(join(selfDir, "pi-agent-sync.json"), "utf8")) as BridgeConfig;
	} catch {
		return {};
	}
}

const config = loadConfig();
const localBin = join(homedir(), ".local", "bin", "agent-sync");
// Ordem: env > sidecar do apply > binário instalado > PATH. O PATH pode
// resolver um agent-sync antigo (ex.: GOBIN do mise); ~/.local/bin vence.
const AGENT_SYNC_BIN =
	process.env.AGENT_SYNC_BIN ||
	config.agent_sync_bin ||
	(existsSync(localBin) ? localBin : "agent-sync");

function callBridge(event: string, payload: Record<string, unknown>): HookResult {
	try {
		const env: NodeJS.ProcessEnv = { ...process.env };
		if (config.base_dir) env.AGENT_SYNC_HOME = config.base_dir;
		const stdout = execFileSync(AGENT_SYNC_BIN, ["hook", "pi", `--event=${event}`], {
			input: JSON.stringify(payload),
			encoding: "utf8",
			timeout: 15000,
			env,
		});
		return (JSON.parse(stdout.trim() || "{}") as HookResult) ?? {};
	} catch {
		return {};
	}
}

function basePayload(ctx: CtxLike): Record<string, unknown> {
	const usage = ctx.getContextUsage?.();
	const model = ctx.model ? [ctx.model.provider, ctx.model.id].filter(Boolean).join("/") : "";
	return {
		session_id: ctx.sessionManager?.getSessionId?.() ?? "",
		cwd: ctx.cwd ?? process.cwd(),
		model,
		context_tokens: usage?.tokens ?? 0,
		context_window: usage?.contextWindow ?? 0,
	};
}

function textOf(content: unknown): string {
	if (typeof content === "string") return content;
	if (!Array.isArray(content)) return "";
	return content
		.map((b) => (b && typeof b === "object" && typeof (b as { text?: unknown }).text === "string" ? (b as { text: string }).text : ""))
		.filter(Boolean)
		.join("\n");
}

export default function (pi: ExtensionAPI) {
	// Contexto que não pode ser injetado na hora (session_start/tool_call/
	// agent_settled). Drenado no before_agent_start (início de run) ou no
	// context (antes de cada request, para nudges no meio do run).
	let pending: string[] = [];
	const push = (text?: string) => {
		if (text?.trim()) pending.push(text.trim());
	};
	const drain = (): string | undefined => {
		if (pending.length === 0) return undefined;
		const text = pending.join("\n");
		pending = [];
		return text;
	};

	pi.on("session_start", async (_e, ctx) => {
		push(callBridge("session_start", basePayload(ctx as CtxLike)).context);
	});

	pi.on("tool_call", async (event, ctx) => {
		const res = callBridge("tool_call", {
			...basePayload(ctx as CtxLike),
			tool_name: event.toolName,
			tool_input: event.input,
		});
		if (res.block) return { block: true, reason: res.reason ?? "agent-sync: bloqueado" };
		push(res.context);
	});

	pi.on("tool_result", async (event, ctx) => {
		const res = callBridge("tool_result", {
			...basePayload(ctx as CtxLike),
			tool_name: event.toolName,
			tool_input: event.input,
			tool_output: textOf(event.content),
		});
		// Redação: substitui o conteúdo (só quando há segredo detectado).
		if (res.output) {
			return { content: [{ type: "text" as const, text: res.output }], structuredContent: undefined };
		}
		// Nudge pós-tool não muta o resultado; entra no próximo request.
		push(res.context);
	});

	pi.on("before_agent_start", async () => {
		const text = drain();
		if (!text) return;
		return { message: { customType: "agent-sync", content: text, display: false } };
	});

	pi.on("context", async (event) => {
		const text = drain();
		if (!text) return;
		return { messages: [...event.messages, { role: "user" as const, content: [{ type: "text" as const, text }] }] };
	});

	pi.on("agent_settled", async (_e, ctx) => {
		push(callBridge("agent_settled", basePayload(ctx as CtxLike)).context);
	});

	pi.on("session_before_compact", async (_e, ctx) => {
		callBridge("session_before_compact", basePayload(ctx as CtxLike));
	});
}
