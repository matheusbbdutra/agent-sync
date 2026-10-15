package hooks

// hooks_cmd_cache.go: cmd-cache PostToolUse (popula o cache de comandos
// read-only consumido pelo shell-validate PreToolUse).
//
// ADR-reuso-bash-sudo-agents §1: o estágio "overwrite + cache TTL" precisa de
// DOIS lados — o Pre (serve o hit) e o Post (grava a saída). Este é o Post.
// Opt-in: sem AGENT_SYNC_CMD_CACHE=1 o adapter é no-op (`{}`).

// syncCmdCacheHook instala o hook PostToolUse que alimenta o cache de
// comandos read-only determinísticos (lscpu, uname, /etc/os-release, git log).
//
// Escopo desta entrega: Claude Code + Codex (formato {tool_input,tool_response}).
// Gaps declarados (sem superfície equivalente):
//   - Antigravity: o output do Bash vem por transcriptPath (stepIdx), não no
//     payload — precisa de parser próprio (mesmo gap do docs-cache);
//   - OpenCode: PostToolUse exige plugin TS (`tool.execute.after`);
//   - Cline: hooks de arquivo são inertes na CLI v3 (só plugin).
//
// Ver docs/ADR-reuso-bash-sudo-agents.md §1.
func syncCmdCacheHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" || target.HooksFormat == "cline" {
		return nil
	}
	return syncStandardHook(baseDir, target, cmdCacheHookName, "cmd-cache.posttooluse.sh", "Bash")
}
