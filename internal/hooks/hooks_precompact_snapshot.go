package hooks

import (
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// hooks_precompact_snapshot.go: precompact-snapshot cross-CLI.

// Migrado de hooks.go em 2026-09-21 (Fase 2.3 do refator por feature).
// Sem mudanca de comportamento: mesma logica, mesmo package.

// syncContextSnapshotHook: precompact-snapshot cross-CLI
// syncContextSnapshotHook wirar o hook PreCompact cross-CLI
// (ADR-precompact-snapshot-cross-cli Decisao 3):
//   - Claude Code + Codex: PreCompact direto, matcher "*".
//   - Antigravity: PreInvocation como proxy (D-28).
//   - Cursor: NAO wirar novo (observacional ja registrado em
//     syncCtxCompactHook; gap aceito registrado na ADR Decisao 4).
//
// Anti-alucinacao (D-24): wirar antigo chamava `ctx-window snapshot`
// que NAO EXISTE (subcomando valido: hook/handoff/summarize). Hook
// falhava sempre com exit 1 -> Gemini CLI bloqueava tool call
// ('hook de seguranca do terminal'). Fix 2026-09-22: trocar para
// `agent-sync state snapshot` (subcommand canonico) invocado via
// wrapper bash (hooks/agent-sync-precompact-snapshot.sh) que resolve
// o project root subindo diretorios ate encontrar go.mod do agent-sync.
// Sem wrapper, agent-sync cai em resolveBaseDir do CWD que em
// PreInvocation e' ~/.gemini/config (estado vazio - falha).
func syncContextSnapshotHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	switch target.HooksFormat {
	case "claude", "codex":
		command := "agent-sync state snapshot"
		return syncHookCommandAtEvent(baseDir, target, ctxSnapshotHookName,
			command, "*", "PreCompact", nil)
	case "antigravity":
		// PreInvocation = proxy do PreCompact (D-28). Wrapper bash
		// resolve o project root via walk-up no filesystem (mesma
		// estrategia do agent-react-nudge.stop.cursor.sh).
		scriptPath, err := pathutil.HookScriptPath(baseDir, "agent-sync-precompact-snapshot.sh")
		if err != nil {
			return err
		}
		wrapped := wrapHookCommand(baseDir, "PreInvocation", ctxSnapshotHookName, scriptPath)
		return syncAntigravityPreInvocation(target, ctxSnapshotHookName, wrapped)
	case "cursor":
		// Observacional ja wirado via syncCtxCompactHook; nao duplicar.
		return nil
	default:
		return nil
	}
}
