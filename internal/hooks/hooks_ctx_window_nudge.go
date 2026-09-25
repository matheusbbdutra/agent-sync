package hooks

// hooks_ctx_window_nudge.go: ctx-window nudge cross-CLI.

// Migrado de hooks.go em 2026-09-21 (Fase 2.3 do refator por feature).
// Sem mudanca de comportamento: mesma logica, mesmo package.

// syncCtxWindowNudgeHook: ctx-window nudge cross-CLI
// syncCtxWindowNudgeHook wirar o nudge combinado de ctx-window summarize:
// dispara quando (a) >= MIN_TOOL_CALLS tool calls ou (b) summary.md velho.
// Suportado em Claude Code, Codex, Antigravity e Cursor (postToolUse).
// OpenCode fica para um próximo turno (precisaria plugin TS paralelo).
func syncCtxWindowNudgeHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	const scriptName = "ctx-window-nudge.sh"
	if target.HooksFormat == "cursor" {
		return syncCursorCommandAtEvent(target.HooksSettingsPath, "postToolUse", "./hooks/"+scriptName+" "+scriptName)
	}
	if target.HooksFormat == "antigravity" {
		return syncAntigravityHook(baseDir, target, ctxWindowNudgeHookName, scriptName, "*")
	}
	return syncStandardHook(baseDir, target, ctxWindowNudgeHookName, scriptName, "*")
}
