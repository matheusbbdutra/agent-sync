package hooks

import (
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// hooks_principles_inject.go: principles-inject cross-CLI.

// Migrado de hooks.go em 2026-09-21 (Fase 2.3 do refator por feature).
// Sem mudanca de comportamento: mesma logica, mesmo package.

// syncPrinciplesInjectHook: principles-inject cross-CLI
// syncPrinciplesInjectHook wirar one-shot de regras criticas em PreToolUse
// (verdade absoluta + anti-overengineering). Texto derivado verbatim de
// rules/global-rules.md (verdade absoluta mantida pelo hook -> doc canonico).
//   - Claude Code + Codex: PreToolUse direto, matcher "*".
//   - Antigravity + Cursor: NAO wirar nesta entrega (matriz 5xN).
//   - OpenCode: plugin TS paralelo em entrega subsequente (A-N+).
// Dedup por sessionID via $TMPDIR/agent-sync-principles-injected/<sid>.flag.
func syncPrinciplesInjectHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	switch target.AgentKind {
	case "claude", "codex":
		scriptPath, err := pathutil.HookScriptPath(baseDir, "principles-inject.pretooluse.sh")
		if err != nil {
			return err
		}
		return syncHookCommandAtEvent(baseDir, target, principlesInjectHookName,
			scriptPath, "*", "PreToolUse", nil)
	default:
		return nil
	}
}
