package hooks

// hooks_ctx_compact.go: ctx-compact cross-CLI.

// Migrado de hooks.go em 2026-09-21 (Fase 2.3 do refator por feature).
// Sem mudanca de comportamento: mesma logica, mesmo package.

// syncCtxCompactHook: ctx-compact cross-CLI
// syncCtxCompactHook registra chamadas de ferramenta para um resumo manual e hooks de nudge.
func syncCtxCompactHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	command := "ctx-window hook " + target.AgentKind
	if target.HooksFormat == "cursor" {
		if err := syncCursorCommandAtEvent(target.HooksSettingsPath, "postToolUse", command); err != nil {
			return err
		}
		return syncCursorCommandAtEvent(target.HooksSettingsPath, "preCompact", command+" precompact")
	}
	if target.HooksFormat == "antigravity" {
		targetCopy := target
		targetCopy.HooksEvent = "PostToolUse"
		if err := syncAntigravityHookCommand(targetCopy, ctxCompactHookName, command, "*"); err != nil {
			return err
		}
		root, err := readJSONObject(target.HooksSettingsPath)
		if err == nil {
			if g, ok := root[ctxCompactHookName].(map[string]interface{}); ok {
				delete(g, "PreInvocation")
				_ = writeJSONObject(target.HooksSettingsPath, root)
			}
		}
		return syncAntigravityPreInvocation(target, ctxNudgeHookName, command+" preinvocation")
	}
	return syncStandardHookCommand(baseDir, target, ctxCompactHookName, command, "*")
}
