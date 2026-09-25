package hooks

import (
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// hooks_ctx_window_summarize.go: ctx-window summarize-at-stop cross-CLI.

// Migrado de hooks.go em 2026-09-21 (Fase 2.3 do refator por feature).
// Sem mudanca de comportamento: mesma logica, mesmo package.

// syncCtxWindowSummarizeAtStopHook: ctx-window summarize-at-stop cross-CLI
// syncCtxWindowSummarizeAtStopHook wirar o summarize automático em Stop/agent-stop.
// Roda em background quando (a) tool calls >= threshold OU (b) summary.md velho.
// Suportado em Claude Code, Codex, Antigravity e Cursor (stop). OpenCode é plugin TS.
func syncCtxWindowSummarizeAtStopHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	const scriptName = "ctx-window-summarize-at-stop.sh"
	scriptPath, err := pathutil.HookScriptPath(baseDir, scriptName)
	if err != nil {
		return err
	}
	if target.HooksFormat == "cursor" {
		return syncCursorCommandAtEvent(target.HooksSettingsPath, "stop", "./hooks/"+scriptName+" "+scriptName)
	}
	if target.HooksFormat == "antigravity" {
		return syncAntigravityFlatHook(target, ctxWindowSummarizeStopHookName, "Stop",
			wrapHookCommand(baseDir, "Stop", ctxWindowSummarizeStopHookName, scriptPath))
	}
	// Claude Code + Codex: wirar como evento "Stop" (não o HooksEvent default).
	command := wrapHookCommand(baseDir, "Stop", ctxWindowSummarizeStopHookName, scriptPath)
	return syncHookCommandAtEvent(baseDir, target, ctxWindowSummarizeStopHookName, command, "*", "Stop", nil)
}
