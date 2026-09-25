package hooks

import (
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// hooks_agent_react_nudge.go: agent-react nudge cross-CLI.

// Migrado de hooks.go em 2026-09-21 (Fase 2.3 do refator por feature).
// Sem mudanca de comportamento: mesma logica, mesmo package.

// syncAgentReactNudgeHook: agent-react nudge cross-CLI
// syncAgentReactNudgeHook instala o lembrete de validacao de hipoteses
// (skill agent-react): hipotese != fato; validar ou pedir passo ao usuario.
//
// Estrategia de wirar por CLI (2026-09-23):
//   - Codex + Claude Code: PreToolUse com .pretooluse.sh (aceita
//     additionalContext sem bloquear tool call; validado por principles-inject).
//   - Antigravity: PreInvocation (formato proprio, nao suporta PreToolUse).
//   - Cursor: postToolUse (formato proprio, schema diferente).
//
// Antes desta mudanca, Codex e Claude wiravam PostToolUse mas o schema
// PostToolUse bloqueia tool call quando tenta injetar additionalContext
// (verificado em runtime 2026-09-22). Resultado: hooks mudos, nudge so'
// chegava ao memory-mcp. Migrar para PreToolUse reativa o nudge visivel.
func syncAgentReactNudgeHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" {
		return syncAntigravityHook(baseDir, target, agentReactNudgeHookName, "agent-react-nudge.antigravity.sh", "*")
	}
	if target.HooksFormat == "cursor" {
		if supportsNudgeFilter(target) {
			return syncStandardHookFiltered(baseDir, target, agentReactNudgeHookName, "agent-react-nudge.sh", "*", nudgeIfFilters)
		}
		return syncStandardHook(baseDir, target, agentReactNudgeHookName, "agent-react-nudge.sh", "*")
	}
	// Codex + Claude: PreToolUse + .pretooluse.sh
	return syncStandardHookAtEvent(baseDir, target, agentReactNudgeHookName,
		"agent-react-nudge.pretooluse.sh", "*", "PreToolUse", nudgeIfFilters)
}

// syncStandardHookAtEvent wirar em evento arbitrario (PreToolUse,
// PostToolUse, etc.) com filtro `if` opcional. Usado pelos nudges que
// migraram para PreToolUse para codex/claude.
func syncStandardHookAtEvent(baseDir string, target TargetCLI, hookName, scriptName, matcher, event string, ifFilters []string) error {
	scriptPath, err := pathutil.HookScriptPath(baseDir, scriptName)
	if err != nil {
		return err
	}
	return syncHookCommandAtEvent(baseDir, target, hookName, scriptPath, matcher, event, ifFilters)
}
