package hooks

import (
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// hooks_agent_task_record.go: agent-task-record cross-CLI.

// Migrado de hooks.go em 2026-09-21 (Fase 2.3 do refator por feature).
// Sem mudanca de comportamento: mesma logica, mesmo package.

// syncAgentTaskRecordHook: agent-task-record cross-CLI
// syncAgentTaskRecordHook wirar o hook Stop cross-CLI (A-15) que popula
// .agent-sync/agent_tasks.jsonl com telemetria do turno (cli, model,
// session_id, tokens opcionalmente). Wirado em Claude Code, Codex,
// Antigravity e Cursor. OpenCode fica para plugin TS separado.
//
// Parametro AGENT_SYNC_AGENT_KIND e exportado para o script via env var
// (CLI origem). Cada CLI tem formato proprio de Stop payload; o script
// detecta por heuristica (presenca de campo `model` = codex, etc).
func syncAgentTaskRecordHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	const scriptName = "agent-task-record.stop.sh"
	scriptPath, err := pathutil.HookScriptPath(baseDir, scriptName)
	if err != nil {
		return err
	}
	// Variavel exportada para o script identificar a CLI.
	command := "AGENT_SYNC_AGENT_KIND=" + target.AgentKind + " "
	if target.HooksFormat == "cursor" {
		// Cursor: stop event lowercase.
		return syncCursorCommandAtEvent(target.HooksSettingsPath, "stop", "./hooks/"+scriptName+" "+scriptName)
	}
	if target.HooksFormat == "antigravity" {
		return syncAntigravityFlatHook(target, agentTaskRecordHookName, "Stop",
			command+wrapHookCommand(baseDir, "Stop", agentTaskRecordHookName, scriptPath))
	}
	// Claude Code + Codex: PreCompact direto no evento Stop.
	return syncHookCommandAtEvent(baseDir, target, agentTaskRecordHookName,
		command+wrapHookCommand(baseDir, "Stop", agentTaskRecordHookName, scriptPath),
		"*", "Stop", nil)
}
