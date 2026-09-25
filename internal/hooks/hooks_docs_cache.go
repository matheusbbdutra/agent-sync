package hooks

// hooks_docs_cache.go: docs-cache WebFetch hook.

// Migrado de hooks.go em 2026-09-21 (Fase 2.3 do refator por feature).
// Sem mudanca de comportamento: mesma logica, mesmo package.

// syncDocsCacheHook: docs-cache WebFetch hook
// syncDocsCacheHook instala o hook que cacheia passivamente docs consultadas
// via WebFetch/read_url_content e context7 (query-docs). Claude Code e Codex
// compartilham o mesmo script (schema de PostToolUse equivalente); Antigravity
// usa o seu próprio (lê o resultado do transcriptPath).
func syncDocsCacheHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" {
		return syncAntigravityHook(baseDir, target, docsCacheHookName, "docs-cache.antigravity.sh", "read_url_content|call_mcp_tool")
	}
	return syncStandardHook(baseDir, target, docsCacheHookName, "docs-cache.sh", "WebFetch|mcp__context7__.*")
}
