package hooks

import (
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// hooks_secret_guard.go: secret-guard cross-CLI (A-63).
//
// Migrado em 2026-09-25 de hooks orfaos (scripts existiam mas nao wirados
// em nenhuma CLI). Wiramento target multi-CLI:
//
//   - Claude Code + Codex: formato padrao (PreToolUse + PostToolUse com
//     matcher "*") via syncHookCommandAtEvent (hooks_apply.go:79).
//   - Antigravity: mesmo formato nested de Claude/Codex (PreToolUse +
//     PostToolUse com matcher "*") via syncAntigravityHookCommand
//     (formato flat nested de hooks_antigravity_apply.go:54).
//   - Cursor: postToolUse via syncCursorCommandAtEvent (merge gerenciado
//     em hooks_cursor_apply.go:15) + beforeShellExecution (so Bash; Read/
//     Edit/Write/etc. nao tem equivalente gerenciado pelo agent-sync).
//   - OpenCode v2: gap 🟡 aceito (A-64+ via permission.hook("evaluate")
//     com effect mutation — investigacao 2026-09-25 confirmou que
//     tool.hook("execute.before") e so observacional, mas permission.
//     hook tem effect mutavel: schema @opencode/schema/dist/permission.
//     d.ts Permission.Effect = "allow"|"deny"|"ask").
//
// Camadas (ver §1 do ADR-secret-guard-cross-cli.md):
//   Camada 1 (deny-list de path): bloqueia Read/Edit/Write/MultiEdit/Grep/
//     Glob/Bash quando o path casa qualquer pattern que costuma conter
//     secret. Defesa primaria — objetivo e nao catalogar todos os formatos
//     de segredo, e sim garantir que o agente nunca leia arquivo que possa
//     conter segredo.
//   Camada 2 (regex de literal nos args): JWT/AWS access key/GitHub PAT
//     inline em comandos. Cinto + suspensorio para secrets passados sem
//     path (`export X=eyJ...`, `curl -H 'Bearer ...'`).
//
// PostToolUse faz redacao total (<REDACTED:FILE_IN_DENYLIST>) quando o path
// do arquivo lido casa a deny-list; redacao por regex (JWT/AWS/GitHub) para
// outputs de arquivos fora da deny-list.
//
// NOTA: cada wiramento usa um hookName distinto (pre/post) porque o helper
// syncHookCommandAtEvent remove wiramentos orfaos do mesmo hookName em
// outros eventos (linhas 38-75 de hooks_apply.go) — sem isso, o segundo
// wiramento apagaria o primeiro.

const (
	secretGuardPreToolUseScript  = "secret-guard.pretooluse.sh"
	secretGuardPostToolUseScript = "secret-guard.posttooluse.sh"
)

// syncSecretGuardPreToolUseHook: wirar PreToolUse em Claude Code + Codex +
// Antigravity (formato padrao / nested).
func syncSecretGuardPreToolUseHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	switch target.AgentKind {
	case "claude", "codex":
		scriptPath, err := pathutil.HookScriptPath(baseDir, secretGuardPreToolUseScript)
		if err != nil {
			return err
		}
		return syncHookCommandAtEvent(baseDir, target, secretGuardPreToolUseHookName,
			scriptPath, "*", "PreToolUse", nil)
	case "antigravity":
		// PreToolUse em Antigravity: syncAntigravityHookCommand grava no
		// formato nested (raiz=hookName, sub-grupo por evento). Mesmo
		// matcher "*" das outras CLIs. Verificado em bash-guardian
		// (~/.gemini/config/hooks.json:65 ja wirado com sucesso).
		scriptPath, err := pathutil.HookScriptPath(baseDir, secretGuardPreToolUseScript)
		if err != nil {
			return err
		}
		targetCopy := target
		targetCopy.HooksEvent = "PreToolUse"
		return syncAntigravityHookCommand(targetCopy, secretGuardPreToolUseHookName,
			scriptPath, "*")
	default:
		return nil
	}
}

// syncSecretGuardPostToolUseHook: wirar PostToolUse em Claude Code + Codex +
// Antigravity + Cursor (postToolUse).
func syncSecretGuardPostToolUseHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	switch target.AgentKind {
	case "claude", "codex":
		scriptPath, err := pathutil.HookScriptPath(baseDir, secretGuardPostToolUseScript)
		if err != nil {
			return err
		}
		return syncHookCommandAtEvent(baseDir, target, secretGuardPostToolUseHookName,
			scriptPath, "*", "PostToolUse", nil)
	case "antigravity":
		scriptPath, err := pathutil.HookScriptPath(baseDir, secretGuardPostToolUseScript)
		if err != nil {
			return err
		}
		targetCopy := target
		targetCopy.HooksEvent = "PostToolUse"
		return syncAntigravityHookCommand(targetCopy, secretGuardPostToolUseHookName,
			scriptPath, "*")
	case "cursor":
		// Cursor: postToolUse ja e wirado por outros hooks via
		// syncCursorCommandAtEvent (merge gerenciado por agent-sync em
		// hooks_cursor_apply.go:36). A entrada em cursorManagedHooks()
		// (adicionada separadamente) ativa o wiramento real.
		// Aqui apenas sinaliza que a CLI esta no escopo; syncCursorAll
		// fara o trabalho.
		return nil
	default:
		return nil
	}
}
