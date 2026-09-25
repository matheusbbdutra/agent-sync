package hooks

// hooks_constants.go: constantes compartilhadas por hooks_<feature>.go e
// apply_table.go.
//
// 16 hookName constants (1 por hook wirado) + nudgeIfFilters (compartilhado
// por hooks de nudge: context-guard, memory, agent-react) + supportsNudgeFilter
// helper (detecta CLIs que respeitam campo `if`).
//
// Migrado de hooks.go em 2026-09-21 (Fase 9 do refator por feature). Sem
// mudanca de comportamento: mesmas constantes, mesmas assinaturas.

// nudgeIfFilters restringe os hooks de nudge (context-guard, memory,
// agent-react) a tools de edicao em Claude Code e Codex, cortando ~60-70%
// dos forks em sessoes tipicas. Cada filtro vira um hook handler
// independente - permission rule syntax do Claude Code nao suporta OR
// entre tool names num unico `if`. Antigravity ignora `if` silenciosamente;
// Cursor mantem matcher `*` em outros targets.
var nudgeIfFilters = []string{"Edit(*)", "Write(*)", "MultiEdit(*)", "NotebookEdit(*)"}

// supportsNudgeFilter retorna true quando o AgentKind atual respeita o
// campo `if` em tool events conforme a documentacao do Claude Code
// (Codex compartilha o mesmo formato JSON).
func supportsNudgeFilter(target TargetCLI) bool {
	return target.AgentKind == "claude" || target.AgentKind == "codex"
}

// Constantes hookName (16): 1 por hook wirado. Usadas em standardHooks table
// (apply_table.go), hooks_codex_adapter.go (adaptCodexProtectionHooks),
// plugins OpenCode v2, e debug.
const (
	contextGuardHookName         = "agent-sync-context-guard"
	docsCacheHookName            = "agent-sync-docs-cache"
	memoryNudgeHookName          = "agent-sync-memory-nudge"
	agentReactNudgeHookName      = "agent-sync-agent-react-nudge"
	ctxCompactHookName           = "agent-sync-ctx-compact"
	ctxHandoffHookName           = "agent-sync-ctx-handoff"
	ctxNudgeHookName             = "agent-sync-ctx-nudge"
	shellValidateHookName        = "agent-sync-shell-validate"
	agentStopHookName            = "agent-sync-agent-stop"
	preInvocationReminderHookName = "agent-sync-preinvocation-reminder"
	principlesInjectHookName     = "agent-sync-principles-inject"
	ctxWindowNudgeHookName       = "agent-sync-ctx-window-nudge"
	ctxWindowSummarizeStopHookName = "agent-sync-ctx-window-summarize-stop"
	ctxSnapshotHookName          = "agent-sync-precompact-snapshot"
	agentTaskRecordHookName      = "agent-sync-agent-task-record"
	tokenNudgeHookName           = "agent-sync-token-nudge"
	memoryObserveHookName        = "agent-sync-memory-observe"
	memoryConsolidateHookName    = "agent-sync-memory-consolidate"
)
