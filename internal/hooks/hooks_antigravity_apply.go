package hooks

// hooks_antigravity_apply.go: helpers especificos para wirar no Antigravity CLI,
// que tem formato proprio (chave raiz = nome do hook, valor = grupo de eventos).
// Diferente do formato Claude/Codex/OpenCode que e' {"hooks": {"<evento>": ...}}.
//
// Migrado de hooks.go em 2026-09-21 (Fase 2.2 do refator por feature).

import (
	"path/filepath"
)

// syncAntigravityFlatHook grava uma entrada generica no formato Antigravity:
// root[hookName] = {event: [{command, name, timeout}]}. Usado internamente
// por syncAntigravityPreInvocation e syncAntigravityStop.
func syncAntigravityFlatHook(target TargetCLI, hookName, event, command string) error {
	root, err := readJSONObject(target.HooksSettingsPath)
	if err != nil {
		return err
	}
	group, _ := root[hookName].(map[string]interface{})
	if group == nil {
		group = map[string]interface{}{}
	}
	delete(group, "SessionStart")
	group[event] = []hookCmd{{Type: "command", Command: command, Name: hookName, Timeout: 10}}
	root[hookName] = group
	return writeJSONObject(target.HooksSettingsPath, root)
}

// syncAntigravityPreInvocation grava o hook no evento PreInvocation do target.
func syncAntigravityPreInvocation(target TargetCLI, hookName, command string) error {
	return syncAntigravityFlatHook(target, hookName, "PreInvocation", command)
}

// syncAntigravityStop grava o hook no evento Stop do target Antigravity.
func syncAntigravityStop(target TargetCLI, hookName, command string) error {
	return syncAntigravityFlatHook(target, hookName, "Stop", command)
}

// syncAntigravityHook e' variante legada para wirar antigravity via pre-existing
// estrutura (hooks/<scriptName>) com matcher. Mantida para Nudge contracts
// que precisam ler script do disco; ver syncMemoryNudgeHook / syncAgentReactNudgeHook.
func syncAntigravityHook(baseDir string, target TargetCLI, hookName, scriptName, matcher string) error {
	scriptPath := filepath.Join(baseDir, "hooks", scriptName)
	wrapped := wrapHookCommand(baseDir, "PreInvocation", hookName, scriptPath)
	return syncAntigravityPreInvocation(target, hookName, wrapped)
}

// syncAntigravityHookCommand grava um comando literal (sem arquivo em disco)
// no formato Antigravity: raiz e' o hookName, sub-grupo por evento. Diferente
// de syncAntigravityFlatHook (que usa array direto), aqui mantemos formato
// nested com decodeHookEntries/upsertHookEntry para idempotencia.
func syncAntigravityHookCommand(target TargetCLI, hookName, command, matcher string) error {
	root, err := readJSONObject(target.HooksSettingsPath)
	if err != nil {
		return err
	}

	hookGroup, _ := root[hookName].(map[string]interface{})
	if hookGroup == nil {
		hookGroup = map[string]interface{}{}
	}

	entries := decodeHookEntries(hookGroup[target.HooksEvent])
	hookGroup[target.HooksEvent] = upsertHookEntry(entries, command, hookName, matcher, nil)
	root[hookName] = hookGroup

	return writeJSONObject(target.HooksSettingsPath, root)
}
