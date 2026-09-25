package hooks

// hooks_all.go: hook de feature com wirar cross-CLI principal.

// Migrado de hooks.go em 2026-09-21 (Fase 2.3 do refator por feature).
// Sem mudanca de comportamento: mesma logica, mesmo package.

// syncHooks: hook de feature com wirar cross-CLI principal
// syncHooks injeta (ou atualiza) o hook de lembrete do context-guard no
// arquivo de configuracao da CLI alvo, preservando o que ja existir.
// So age quando o target define HooksSettingsPath e HooksEvent.
//
// Migrado para PreToolUse em codex/claude (ver hooks_agent_react_nudge.go
// para o rationale completo). Antigravity e cursor mantem PostToolUse.
func syncHooks(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" {
		return syncAntigravityHook(baseDir, target, contextGuardHookName, "context-guard-nudge.antigravity.sh", "*")
	}
	if target.HooksFormat == "cursor" {
		if supportsNudgeFilter(target) {
			return syncStandardHookFiltered(baseDir, target, contextGuardHookName, "context-guard-nudge.sh", "*", nudgeIfFilters)
		}
		return syncStandardHook(baseDir, target, contextGuardHookName, "context-guard-nudge.sh", "*")
	}
	// Codex + Claude: PreToolUse + .pretooluse.sh
	return syncStandardHookAtEvent(baseDir, target, contextGuardHookName,
		"context-guard-nudge.pretooluse.sh", "*", "PreToolUse", nudgeIfFilters)
}
