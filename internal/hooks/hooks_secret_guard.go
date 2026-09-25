package hooks

import (
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// hooks_secret_guard.go: secret-guard cross-CLI (A-63).
//
// Migrado em 2026-09-25 de hooks orfaos (scripts existiam mas nao wirados
// em nenhuma CLI). Wiramento target: Claude Code + Codex no formato padrao
// (PreToolUse + PostToolUse com matcher "*"). Antigravity, Cursor e
// OpenCode NAO wirados nesta entrega (matriz 5xN do ADR-secret-guard-
// cross-cli.md §2):
//   - Antigravity: gap 🟡 — depende de PreInvocation como proxy (A-64+)
//   - Cursor: gap ⛔ aceito — preToolUse nao e evento gerenciado
//   - OpenCode v2: gap 🟡 — depende de plugin TS especifico (A-64+)
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

// syncSecretGuardPreToolUseHook: wirar PreToolUse em Claude Code + Codex.
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
	default:
		return nil
	}
}

// syncSecretGuardPostToolUseHook: wirar PostToolUse em Claude Code + Codex.
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
	default:
		return nil
	}
}
