package hooks

import (
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// apply_bash_rm_guardian.go: wiramento cross-CLI do hook bash-rm-guardian
// (A-74). Inicialmente só Antigravity (A-75) — replicar para cursor/claude/
// opencode/codex em iterações futuras (A-76+) usando syncBashGuardianAntigravity
// como modelo.

const bashRmGuardianHookName = "agent-sync-bash-rm-guardian"

// syncBashRmGuardian é o entry-point registrado em hooks_all.go.
// Despacha por HooksFormat. Por enquanto só antigravity é wirado; outras
// CLIs ficam como no-op silencioso (return nil) até A-76+.
func syncBashRmGuardian(baseDir string, target TargetCLI) error {
	switch target.HooksFormat {
	case "antigravity":
		return syncBashRmGuardianAntigravity(baseDir, target)
	default:
		return nil
	}
}

// syncBashRmGuardianAntigravity instala o hook PreToolUse que detecta
// comandos destrutivos (rm -rf, rmdir, mv sobre diretório) e roda
// `repo-map --audit-removal` antes, emitindo injectSteps warn com
// blockers se encontrados. NÃO bloqueia (não usa deny).
//
// Modelo: `syncBashGuardianAntigravity` em apply_bash_guardian.go.
func syncBashRmGuardianAntigravity(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksFormat != "antigravity" {
		return nil
	}
	scriptPath, err := pathutil.HookScriptPath(baseDir, "bash-rm-guardian.antigravity.sh")
	if err != nil {
		return err
	}
	wrapped := wrapHookCommand(baseDir, "PreToolUse", bashRmGuardianHookName, scriptPath)

	targetCopy := target
	targetCopy.HooksEvent = "PreToolUse"
	return syncAntigravityHookCommand(targetCopy, bashRmGuardianHookName, wrapped, "run_command")
}
