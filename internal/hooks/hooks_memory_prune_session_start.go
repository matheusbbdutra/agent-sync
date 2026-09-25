package hooks

import (
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// hooks_memory_prune_session_start.go: auto-prune + alerta staleness no
// SessionStart cross-CLI (A-68).
//
// Padrao clone de syncCtxHandoffHook (hooks_ctx_handoff.go:9-22) com 4
// branches: Claude/Codex default, Antigravity via PreInvocation proxy
// (porque a chave SessionStart eh deletada em hooks_antigravity_apply.go:25),
// Cursor via sessionStart, e OpenCode fica A-69 (sem hook nativo
// SessionStart em @opencode/plugin v2.0.11).

func syncMemoryPruneSessionStartHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	const scriptName = "memory-prune-session-start.sh"
	scriptPath, err := pathutil.HookScriptPath(baseDir, scriptName)
	if err != nil {
		return err
	}
	switch target.HooksFormat {
	case "cursor":
		return syncCursorCommandAtEvent(target.HooksSettingsPath, "sessionStart",
			"./hooks/"+scriptName+" "+scriptName)
	case "antigravity":
		return syncAntigravityPreInvocation(target, memoryPruneSessionStartHookName,
			wrapHookCommand(baseDir, "SessionStart", memoryPruneSessionStartHookName, scriptPath))
	}
	// Claude Code + Codex
	command := wrapHookCommand(baseDir, "SessionStart", memoryPruneSessionStartHookName, scriptPath)
	return syncHookCommandAtEvent(baseDir, target, memoryPruneSessionStartHookName,
		command, ".*", "SessionStart", nil)
}