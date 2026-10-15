package hooks

// hooks_ctx_handoff.go: ctx-handoff SessionStart cross-CLI.

// Migrado de hooks.go em 2026-09-21 (Fase 2.3 do refator por feature).
// Sem mudanca de comportamento: mesma logica, mesmo package.

// syncCtxHandoffHook: ctx-handoff SessionStart cross-CLI
func syncCtxHandoffHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	command := "ctx-window handoff " + target.AgentKind
	switch target.HooksFormat {
	case "antigravity":
		return syncAntigravityPreInvocation(target, ctxHandoffHookName, command)
	default:
		return syncHookCommandAtEvent(baseDir, target, ctxHandoffHookName, command, ".*", "SessionStart", nil)
	}
}
