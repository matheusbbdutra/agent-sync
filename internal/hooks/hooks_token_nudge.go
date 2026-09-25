package hooks

import (
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// hooks_token_nudge.go: token nudge cross-CLI.

// Migrado de hooks.go em 2026-09-21 (Fase 2.3 do refator por feature).
// Sem mudanca de comportamento: mesma logica, mesmo package.

// syncTokenNudgeHook: token nudge cross-CLI
// syncTokenNudgeHook wirar o hook postToolUse cross-CLI (A-16) que
// consulta o status de tokens via `agent-sync budget nudge` e emite
// additionalContext quando should_nudge=true (utilization_pct >= threshold
// OU heuristica legada). Wirado em Claude Code, Codex, Antigravity e
// Cursor. OpenCode fica para plugin TS separado.
func syncTokenNudgeHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	const scriptName = "token-nudge.check.sh"
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
		// Antigravity: PreInvocation/Stop/PostInvocation exigem formato flat
		// (lista direta de handlers, sem matcher). PreToolUse/PostToolUse
		// aceitam {matcher, hooks: [...]}. Token nudge roda em PostToolUse
		// (apos execucao da tool), entao usamos syncAntigravityHookCommand
		// com targetCopy.HooksEvent="PostToolUse" para nao cair no flat.
		// Tambem limpamos a entrada orfa em PreInvocation que vinha de wirar
		// antigo (antes do fix): sem isso, o Gemini CLI rejeita o hook com
		// 'command hook must specify command'. Verificado em runtime 2026-09-22.
		targetCopy := target
		targetCopy.HooksEvent = "PostToolUse"
		if err := syncAntigravityHookCommand(targetCopy, tokenNudgeHookName,
			command+wrapHookCommand(baseDir, "PostToolUse", tokenNudgeHookName,
				scriptPath), "*"); err != nil {
			return err
		}
		// Limpa entrada orfa em PreInvocation (formato flat rejeitado pelo parser).
		root, err := readJSONObject(target.HooksSettingsPath)
		if err == nil {
			if g, ok := root[tokenNudgeHookName].(map[string]interface{}); ok {
				delete(g, "PreInvocation")
				if err := writeJSONObject(target.HooksSettingsPath, root); err != nil {
					return err
				}
			}
		}
		return nil
	}
	// Claude Code + Codex: PostToolUse nativo.
	return syncHookCommandAtEvent(baseDir, target, tokenNudgeHookName,
		command+wrapHookCommand(baseDir, "PostToolUse", tokenNudgeHookName,
			scriptPath),
		"*", "PostToolUse", nil)
}
