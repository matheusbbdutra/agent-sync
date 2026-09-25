package hooks

import (
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// hooks_memory_pipeline.go: observação contínua e consolidação automática de memória cross-CLI (A-39).

// syncMemoryObserveHook instala o hook PostToolUse não-bloqueante que bufferiza observações efêmeras.
func syncMemoryObserveHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	const scriptName = "memory-observe.posttooluse.sh"
	scriptPath, err := pathutil.HookScriptPath(baseDir, scriptName)
	if err != nil {
		return err
	}
	command := "AGENT_SYNC_AGENT_KIND=" + target.AgentKind + " "
	if target.HooksFormat == "cursor" {
		return syncCursorCommandAtEvent(target.HooksSettingsPath, "postToolUse",
			command+"./hooks/"+scriptName+" "+scriptName)
	}
	if target.HooksFormat == "antigravity" {
		targetCopy := target
		targetCopy.HooksEvent = "PostToolUse"
		return syncAntigravityHookCommand(targetCopy, memoryObserveHookName,
			command+wrapHookCommand(baseDir, "PostToolUse", memoryObserveHookName,
				scriptPath), "*")
	}
	// Claude Code + Codex: PostToolUse
	return syncHookCommandAtEvent(baseDir, target, memoryObserveHookName,
		command+wrapHookCommand(baseDir, "PostToolUse", memoryObserveHookName,
			scriptPath),
		"*", "PostToolUse", nil)
}

// syncMemoryConsolidateHook instala o hook Stop que consolida o buffer da sessão no memory.db (FTS5).
func syncMemoryConsolidateHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	const scriptName = "memory-consolidate.stop.sh"
	scriptPath, err := pathutil.HookScriptPath(baseDir, scriptName)
	if err != nil {
		return err
	}
	command := "AGENT_SYNC_AGENT_KIND=" + target.AgentKind + " "
	if target.HooksFormat == "cursor" {
		return syncCursorCommandAtEvent(target.HooksSettingsPath, "stop", "./hooks/"+scriptName+" "+scriptName)
	}
	if target.HooksFormat == "antigravity" {
		return syncAntigravityFlatHook(target, memoryConsolidateHookName, "Stop",
			command+wrapHookCommand(baseDir, "Stop", memoryConsolidateHookName, scriptPath))
	}
	// Claude Code + Codex: Stop
	return syncHookCommandAtEvent(baseDir, target, memoryConsolidateHookName,
		command+wrapHookCommand(baseDir, "Stop", memoryConsolidateHookName, scriptPath),
		"*", "Stop", nil)
}
