package hooks

import (
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// hooks_stop.go: stop (false-success-guard) cross-CLI.

// Migrado de hooks.go em 2026-09-21 (Fase 2.3 do refator por feature).
// Sem mudanca de comportamento: mesma logica, mesmo package.

// syncStopHook: stop (false-success-guard) cross-CLI
// syncStopHook instala o hook para o evento oficial Stop no Antigravity, Claude Code e Codex.
func syncStopHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" {
		scriptPath, err := pathutil.HookScriptPath(baseDir, "agent-stop.antigravity.sh")
		if err != nil {
			return err
		}
		wrapped := wrapHookCommand(baseDir, "Stop", "agent-stop.antigravity", scriptPath)
		return syncAntigravityStop(target, agentStopHookName, wrapped)
	}
	if target.AgentKind == "claude" || target.AgentKind == "codex" {
		return syncHookCommandAtEvent(baseDir, target, "agent-sync-false-success-guard", "false-success-guard hook", "*", "Stop", nil)
	}
	return nil
}

// syncPreInvocationReminderHook: pre-invocation reminder cross-CLI
// syncPreInvocationReminderHook registra no evento PreInvocation do Antigravity CLI o lembrete
// efêmero just-in-time ("Diretriz ativa: responda em PT-BR, sem rodeios e finalize com o resumo de 1-2 frases do que mudou e o que falta.").
func syncPreInvocationReminderHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" {
		scriptPath, err := pathutil.HookScriptPath(baseDir, "agent-preinvocation.antigravity.sh")
		if err != nil {
			return err
		}
		wrapped := wrapHookCommand(baseDir, "PreInvocation", "agent-preinvocation.antigravity", scriptPath)
		return syncAntigravityPreInvocation(target, preInvocationReminderHookName, wrapped)
	}
	return nil
}
