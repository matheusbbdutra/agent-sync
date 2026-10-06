package hooks

// hooks_codex_session_nudges.go: wiramento dos lembretes em SessionStart
// para o Codex.
//
// CONTEXTO (nao remover sem ler): com features.code_mode_host ligado
// (codex >= 0.160), a unica tool de execucao emite `custom_tool_call`. O Codex
// injeta o additionalContext de um hook Pre/PostToolUse como `message
// role=developer` ENTRE a tool call e o resultado dela, e o endpoint
// /v1/responses da MiniMax rejeita esse payload com 400 (2013)
// "tool call result does not follow tool call".
//
// Solucao: no Codex os 4 lembretes passam a entrar uma vez em SessionStart
// (sem tool call em andamento, posicao aceita pela MiniMax). Demais CLIs
// seguem wired em PreToolUse, inalteradas.
//
// Os 4 hooks sao registrados aqui com os MESMOS hookName dos PreToolUse. O
// cleanup de orfaos em syncHookCommandAtEvent (hooks_apply.go) remove as
// entradas antigas de PreToolUse automaticamente, porque o destino e
// SessionStart (evento nao pulado pelo cleanup).

import "github.com/matheusdutra/agent-sync/internal/pathutil"

// syncCodexSessionNudgesHook wirar os lembretes (principles-inject,
// context-guard, memory-nudge, agent-react-nudge) em SessionStart no Codex.
// Um unico script, 4 hookName: o mesmo script e chamado uma vez por sessao
// e emite os 4 textos juntos.
func syncCodexSessionNudgesHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.AgentKind != "codex" {
		return nil
	}
	scriptPath, err := pathutil.HookScriptPath(baseDir, "codex-session-nudges.sh")
	if err != nil {
		return err
	}
	for _, hookName := range []string{
		principlesInjectHookName,
		contextGuardHookName,
		memoryNudgeHookName,
		agentReactNudgeHookName,
	} {
		// O hookName vai como argumento de propósito: upsertHookEntry
		// deduplica por `command` (hooks_apply.go:315), então as 4
		// chamadas precisam de comandos distintos — senão cada uma apaga
		// a anterior e só o último fica registrado. O script emite os 4
		// textos de uma vez e ignora o argumento.
		if err := syncHookCommandAtEvent(baseDir, target, hookName,
			scriptPath+" "+hookName, ".*", "SessionStart", nil); err != nil {
			return err
		}
	}
	return nil
}

// codexSilentHookCommand devolve o comando que roda <scriptName> com stdout
// descartado, para hooks cujo unico efeito no Codex seria injetar contexto.
// O script real continua rodando (registros/side-effects preservados).
func codexSilentHookCommand(baseDir, scriptName string) (string, error) {
	scriptPath, err := pathutil.HookScriptPath(baseDir, scriptName)
	if err != nil {
		return "", err
	}
	silentPath, err := pathutil.HookScriptPath(baseDir, "codex-silent-hook.sh")
	if err != nil {
		return "", err
	}
	return silentPath + " " + scriptPath, nil
}