package hooks

// apply_impeccable_design.go: wiring for the Impeccable design hook.
//
// Impeccable does not install a native hook for OpenCode or Antigravity. This
// file wires two thin adapters that call `agent-sync hook impeccable`:
//
//   - OpenCode: TS plugin (impeccable-design.opencode.v2.ts).
//   - Antigravity: one shell script used by PostToolUse (scan + persist) and
//     PreInvocation (drain pending findings into injectSteps).
//
// The script itself only resolves the agent-sync binary; all contract
// translation lives in impeccable_runtime.go.

import (
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

const (
	impeccableDesignHookName = "agent-sync-impeccable-design"
	impeccableDesignScript   = "impeccable-design.antigravity.sh"
)

// syncOpenCodeImpeccableDesignPlugin instala o plugin v2 do OpenCode.
func syncOpenCodeImpeccableDesignPlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "impeccable-design", "impeccable-design.ts", true)
}

// syncImpeccableDesignHook wira o adapter do Antigravity em dois eventos:
// PostToolUse (scan + pending) e PreInvocation (injectSteps).
func syncImpeccableDesignHook(baseDir string, target TargetCLI) error {
	if target.AgentKind != "antigravity" {
		return nil
	}

	scriptPath, err := pathutil.HookScriptPath(baseDir, impeccableDesignScript)
	if err != nil {
		return err
	}

	postTarget := target
	postTarget.HooksEvent = "PostToolUse"
	postCommand := wrapHookCommand(baseDir, "PostToolUse", impeccableDesignHookName, scriptPath)
	if err := syncAntigravityHookCommand(postTarget, impeccableDesignHookName, postCommand, "*"); err != nil {
		return err
	}

	preCommand := wrapHookCommand(baseDir, "PreInvocation", impeccableDesignHookName, scriptPath)
	return syncAntigravityFlatHook(target, impeccableDesignHookName, "PreInvocation", preCommand)
}
