package hooks

// hooks_memory_nudge.go: memory nudge cross-CLI.

// Migrado de hooks.go em 2026-09-21 (Fase 2.3 do refator por feature).
// Sem mudanca de comportamento: mesma logica, mesmo package.

// syncMemoryNudgeHook: memory nudge cross-CLI
// syncMemoryNudgeHook instala o hook que lembra periodicamente de checar/
// gravar memoria via memory-mcp (store_memory), ja que hoje isso depende
// so da disciplina do modelo seguindo o AGENTS.md. Suportado onde ja existe
// HooksSettingsPath/HooksEvent (Claude Code, Codex e Antigravity).
//
// Migrado para PreToolUse em codex/claude (ver hooks_agent_react_nudge.go).
func syncMemoryNudgeHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" {
		return syncAntigravityHook(baseDir, target, memoryNudgeHookName, "memory-nudge.antigravity.sh", "*")
	}
	if target.HooksFormat == "cursor" {
		if supportsNudgeFilter(target) {
			return syncStandardHookFiltered(baseDir, target, memoryNudgeHookName, "memory-nudge.sh", "*", nudgeIfFilters)
		}
		return syncStandardHook(baseDir, target, memoryNudgeHookName, "memory-nudge.sh", "*")
	}
	// Codex + Claude: PreToolUse + .pretooluse.sh
	return syncStandardHookAtEvent(baseDir, target, memoryNudgeHookName,
		"memory-nudge.pretooluse.sh", "*", "PreToolUse", nudgeIfFilters)
}
