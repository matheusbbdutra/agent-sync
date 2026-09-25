package hooks

// hooks_shell_validate.go: shell validate opt-in.

// Migrado de hooks.go em 2026-09-21 (Fase 2.3 do refator por feature).
// Sem mudanca de comportamento: mesma logica, mesmo package.

// syncShellValidateHook: shell validate opt-in
// syncShellValidateHook instala o hook PreToolUse que sinaliza comandos
// shell provavelmente inválidos (binário sem argumento posicional). É
// opt-in: o binário só emite aviso se AGENT_SYNC_PRETOOLUSE_VALIDATE=1
// estiver setado no ambiente. Por isso o hook é seguro de registrar —
// sem essa env, ele é no-op silencioso.
//
// Importante: este hook tem que ir em PreToolUse (verificar ANTES da
// execução), não no HooksEvent padrão da CLI (que é PostToolUse).
//
// Mapping de eventos por CLI (verificado no settings.json/schema oficial):
//   - Claude Code / Codex:  "PreToolUse" / matcher "Bash" (schema padrão)
//   - Cursor:               shell-validate não se aplica — Cursor já tem
//     `beforeShellExecution` com bash-guardian que faz
//     papel equivalente (instalado por syncBash- ou
//     outro helper específico do Cursor). Não escreve.
//   - Antigravity:          usa syncAntigravityHookCommand direto, formato
//     top-level já gravado em HooksEvent="PreInvocation".
func syncShellValidateHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	if target.HooksFormat == "cursor" {
		// bash-guardian já cobre a checagem pré-execução em
		// beforeShellExecution; shell-validate seria redundante aqui.
		return nil
	}
	command := "shell-validate hook"
	if target.HooksFormat == "antigravity" {
		// shell-validate roda em PreToolUse (valida comando ANTES da execucao).
		// Antigravity: PreToolUse aceita {matcher, hooks: [...]} (igual Claude/Codex).
		// target.HooksEvent vem como "PreInvocation" do default de target.go:69
		// (que e' formato flat), entao sobrescrevemos para "PreToolUse" antes
		// de chamar syncAntigravityHookCommand (formato nested). Tambem limpamos
		// entrada orfa em PreInvocation. Verificado em runtime 2026-09-22.
		targetCopy := target
		targetCopy.HooksEvent = "PreToolUse"
		if err := syncAntigravityHookCommand(targetCopy, shellValidateHookName, command, "*"); err != nil {
			return err
		}
		// Limpa entrada orfa em PreInvocation.
		root, err := readJSONObject(target.HooksSettingsPath)
		if err == nil {
			if g, ok := root[shellValidateHookName].(map[string]interface{}); ok {
				delete(g, "PreInvocation")
				if err := writeJSONObject(target.HooksSettingsPath, root); err != nil {
					return err
				}
			}
		}
		return nil
	}
	// Claude, Codex (e qualquer outro que use schema padrão)
	return syncHookCommandAtEvent(baseDir, target, shellValidateHookName, command, "Bash", "PreToolUse", nil)
}
