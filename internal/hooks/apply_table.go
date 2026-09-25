package hooks

// apply_table.go: tabela standardHooks - define quais wirar em cada CLI.
//
// Tabela ordenada de hooks instalada em targets nao-Cursor. Cursor usa
// runCursorHooks (syncCursorAll + syncCtxCompactHook + syncCtxHandoffHook)
// por ter merge proprio em hooks.json. Migrado de main.go em 2026-09-21
// (Fase 6).


var standardHooks = []hookSpec{
	// Hooks suportados em Claude Code, Codex e Antigravity (os early-returns
	// internos de cada sync*Hook silenciam o que não se aplica).
	{name: "context-guard", fn: syncHooks, detail: settingsPathDetail},
	{name: "memory-nudge", fn: syncMemoryNudgeHook, detail: settingsPathDetail},
	{name: "agent-react", fn: syncAgentReactNudgeHook, detail: settingsPathDetail},
	{name: "ctx-compact", fn: syncCtxCompactHook, detail: settingsPathDetail},
	{
		name: "ctx-handoff",
		fn:   syncCtxHandoffHook,
		// detail nil → não imprime sucesso (preserva comportamento histórico)
	},
	{name: "shell-validate", fn: syncShellValidateHook, detail: settingsPathDetail},
	{name: "docs-cache", fn: syncDocsCacheHook, detail: settingsPathDetail},
	// Nudge combinado de ctx-window summarize (PostToolUse). Wirar tambem no
	// Cursor via cursorManagedHooks; OpenCode fica para plugin TS separado.
	{name: "ctx-window-nudge", fn: syncCtxWindowNudgeHook, detail: settingsPathDetail},
	// Auto-summarize no Stop/agent-stop. Wirar nos 4 CLIs padrao (Claude,
	// Codex, Antigravity, Cursor). OpenCode fica para plugin TS separado.
	{name: "ctx-window-summarize-at-stop", fn: syncCtxWindowSummarizeAtStopHook, detail: settingsPathDetail},
	// PreCompact cross-CLI com snapshot estruturado (ADR-precompact-snapshot).
	// Wirar Claude Code, Codex (PreCompact direto) + Antigravity (PreInvocation
	// proxy). Cursor ja tem observacional via ctx-compact; nao wirar novo.
	{name: "precompact-snapshot", fn: syncContextSnapshotHook, detail: settingsPathDetail},
	// Budget tracking write path cross-CLI (A-15): hook Stop wirado em Claude
	// Code, Codex, Antigravity e Cursor para popular .agent-sync/agent_tasks.jsonl.
	// OpenCode fica para plugin TS separado (mesma decisao de outros wirar v2).
	{name: "agent-task-record", fn: syncAgentTaskRecordHook, detail: settingsPathDetail},
	// Token nudge contract cross-CLI (A-16): hook postToolUse wirado em
	// Claude Code, Codex, Antigravity e Cursor. OpenCode via plugin v2.
	// Decide should_nudge via agent-sync budget nudge; emite additionalContext
	// quando utilization_pct >= threshold.
	{name: "token-nudge", fn: syncTokenNudgeHook, detail: settingsPathDetail},
	// Principles-inject (PreToolUse): injeta verdade absoluta + anti-overengineering
	// uma vez por sessao (dedup via $TMPDIR). Wirar Claude+Codex; Antigravity,
	// Cursor e OpenCode em entrega subsequente. Texto do hook deve permanecer
	// verbatim com rules/global-rules.md (verdade absoluta).
	{name: "principles-inject", fn: syncPrinciplesInjectHook, detail: settingsPathDetail},
	// Hooks de término/Stop suportados em Antigravity CLI, Claude Code e Codex.
	{name: "stop", fn: syncStopHook, agentKinds: []string{"antigravity", "claude", "codex"}, detail: settingsPathDetail},
	{name: "preinvocation", fn: syncPreInvocationReminderHook, formats: []string{"antigravity"}, detail: settingsPathDetail},
	// Plugins TS do OpenCode com chave de versão: v1 mantém .opencode.ts
	// (best-effort, issue #13574); v2 usa .v2.ts (@opencode/plugin).
	// Plugins novos só existem em v2 (silenciosos em v1).
	{name: "opencode-context-guard", fn: syncOpenCodePlugin, agentKinds: []string{"opencode"}, detail: openCodePluginDetailNote},
	{name: "opencode-memory", fn: syncOpenCodeMemoryNudgePlugin, agentKinds: []string{"opencode"}, detail: openCodePluginDetailNote},
	{name: "opencode-agent-react", fn: syncOpenCodeAgentReactNudgePlugin, agentKinds: []string{"opencode"}, detail: openCodePluginDetailNote},
	{name: "opencode-ctx-compact", fn: syncOpenCodeCtxCompactPlugin, agentKinds: []string{"opencode"}, detail: openCodePluginDetail},
	{name: "opencode-precompact-snapshot", fn: syncOpenCodePrecompactSnapshotPlugin, agentKinds: []string{"opencode"}, detail: openCodePluginDetail},
	{name: "opencode-token-nudge", fn: syncOpenCodeTokenNudgePlugin, agentKinds: []string{"opencode"}, detail: openCodePluginDetail},
	{name: "opencode-ctx-window-nudge", fn: syncOpenCodeCtxWindowNudgePlugin, agentKinds: []string{"opencode"}, detail: openCodePluginDetail},
	{name: "opencode-ctx-window-summarize-at-stop", fn: syncOpenCodeCtxWindowSummarizeAtStopPlugin, agentKinds: []string{"opencode"}, detail: openCodePluginDetail},
	{name: "opencode-docs-cache", fn: syncOpenCodeDocsCachePlugin, agentKinds: []string{"opencode"}, detail: openCodePluginDetail},
	{name: "opencode-repo-map-warmup", fn: syncOpenCodeRepoMapWarmupPlugin, agentKinds: []string{"opencode"}, detail: openCodePluginDetail},
	{name: "opencode-memory-pipeline", fn: syncOpenCodeMemoryPipelinePlugin, agentKinds: []string{"opencode"}, detail: openCodePluginDetail},
	// Auto-prune SessionStart proxy (A-69). tool.execute.after + flag armed ate
	// @opencode/plugin expor session.hook("created") nativo.
	{name: "opencode-memory-prune-session-start", fn: syncOpenCodeMemoryPruneSessionStartPlugin, agentKinds: []string{"opencode"}, detail: openCodePluginDetail},
	// Secret guard cross-CLI (A-63 / ADR-secret-guard-cross-cli.md). Defesa
	// em 2 camadas: deny-list de path (primario) + regex de literal nos args
	// (secundario). Wirar PreToolUse + PostToolUse em Claude Code + Codex +
	// Antigravity (mesmo formato nested). Para Cursor, postToolUse e wirado
	// via cursorManagedHooks (hooks_cursor_apply.go); PreToolUse em Cursor
	// so shell via beforeShellExecution. OpenCode v2 fica A-64+ via
	// permission.hook("evaluate") com effect mutavel
	// (Permission.Effect="allow"|"deny"|"ask" — schema
	// @opencode/schema/dist/permission.d.ts).
	{name: "secret-guard-pretooluse", fn: syncSecretGuardPreToolUseHook, agentKinds: []string{"claude", "codex", "antigravity"}, detail: settingsPathDetail},
	{name: "secret-guard-posttooluse", fn: syncSecretGuardPostToolUseHook, agentKinds: []string{"claude", "codex", "antigravity", "cursor"}, detail: settingsPathDetail},
	// Observação e consolidação contínua de memória (A-39 / ADR-automated-memory-observation-pipeline).
	{name: "memory-observe", fn: syncMemoryObserveHook, detail: settingsPathDetail},
	{name: "memory-consolidate", fn: syncMemoryConsolidateHook, detail: settingsPathDetail},
	// Auto-prune scratch + alerta staleness no SessionStart (A-68).
	// Wirar Claude Code + Codex + Antigravity (PreInvocation como proxy) +
	// Cursor. OpenCode fica A-69 (sem hook nativo SessionStart em
	// @opencode/plugin v2.0.11).
	{name: "memory-prune-session-start", fn: syncMemoryPruneSessionStartHook, detail: settingsPathDetail},
	// bash-guardian: 3 entradas porque cada CLI tem implementação própria
	// (permissions.ask no Claude, PreToolUse ask no Antigravity, permission.bash
	// no OpenCode). Codex fica fora — PreToolUse só suporta allow/deny binário.
	{name: "bash-guardian", fn: syncBashGuardianClaude, agentKinds: []string{"claude"}, detail: settingsPathDetail},
	{name: "bash-guardian", fn: syncBashGuardianAntigravity, agentKinds: []string{"antigravity"}, detail: settingsPathDetail},
	// bash-rm-guardian (A-75/A-76): detecta rm/rmdir/mv destrutivo, roda audit_removal,
	// emite additionalContext warn (Claude/Codex) ou injectSteps (Antigravity). NÃO bloqueia.
	// Wirar em claude + codex (mesmo PreToolUse nativo) + antigravity + cursor (beforeShellExecution).
	{name: "bash-rm-guardian", fn: syncBashRmGuardian, agentKinds: []string{"antigravity", "claude", "codex"}, detail: settingsPathDetail},
	{
		name:       "bash-guardian",
		fn:         syncBashGuardianOpenCode,
		agentKinds: []string{"opencode"},
		detail: func(t TargetCLI) string {
			return "instalado em: " + openCodeConfigFileRef(t)
		},
	},
}