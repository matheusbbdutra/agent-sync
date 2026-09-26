package hooks

import (
	"fmt"
	"os"
	"path/filepath"
)

// hooks_cursor_apply.go: wirar do Cursor com merge proprio em hooks.json.
//
// Cursor tem formato JSON proprio (merge gerenciado por agent-sync) ao
// inves de upsertHookEntry das outras CLIs. Migrado de cursor.go em
// 2026-09-21 (Fase 7 do refator por feature).

// cursorHookDef descreve uma entrada em ~/.cursor/hooks.json gerenciada pelo agent-sync.
type cursorHookDef struct {
	Event     string
	Script    string // nome do arquivo em hooks/ (repo) e em ~/.cursor/hooks/
	Matcher   string // opcional
	WrapStage string // não-vazio: comando é envelopado via ./hooks/wrap-hook.sh <WrapStage> <Script> <Script>
	LoopLimit int    // opcional: limite de iterações de followup_message no evento Stop; 0 = default Cursor
}

func cursorManagedHooks() []cursorHookDef {
	return []cursorHookDef{
		{Event: "postToolUse", Script: "context-guard-nudge.cursor.sh", WrapStage: "postToolUse"},
		{Event: "postToolUse", Script: "memory-nudge.cursor.sh", WrapStage: "postToolUse"},
		{Event: "postToolUse", Script: "agent-react-nudge.cursor.sh", WrapStage: "postToolUse"},
		{Event: "postToolUse", Script: "ctx-window-nudge.sh", WrapStage: "postToolUse"},
		{Event: "postToolUse", Script: "docs-cache.cursor.sh", Matcher: "WebFetch", WrapStage: "postToolUse"},
		{Event: "afterMCPExecution", Script: "docs-cache-mcp.cursor.sh", Matcher: "query-docs", WrapStage: "afterMCPExecution"},
		{Event: "beforeShellExecution", Script: "bash-guardian.cursor.sh", WrapStage: "beforeShellExecution"},
		// bash-rm-guardian (A-76): detecta rm/rmdir/mv destrutivo, roda
		// audit_removal, emite agent_message warn (Cursor). NÃO bloqueia.
		{Event: "beforeShellExecution", Script: "bash-rm-guardian.cursor.sh", WrapStage: "beforeShellExecution"},
		// Secret guard (A-63 / ADR-secret-guard-cross-cli.md). postToolUse
		// wirado para redacao total (<REDACTED:FILE_IN_DENYLIST>) quando o
		// file_path do input PostToolUse casa deny-list + redacao por regex
		// (JWT/AWS/GitHub PAT) para outputs de arquivos fora da deny-list.
		// PreToolUse em Cursor = so shell (beforeShellExecution para Bash);
		// Read/Edit/Write/etc. nao tem equivalente gerenciado pelo
		// agent-sync -> gap parcial (reconhecido no ADR §2).
		{Event: "postToolUse", Script: "secret-guard.posttooluse.sh", WrapStage: "postToolUse"},
		{Event: "stop", Script: "agent-stop.cursor.sh", WrapStage: "stop"},
		{Event: "stop", Script: "agent-react-nudge.stop.cursor.sh", WrapStage: "stop", LoopLimit: 5},
		{Event: "stop", Script: "ctx-window-summarize-at-stop.sh", WrapStage: "stop"},
	}
}

// syncCursorRules foi removido: Cursor agora wira AGENTS.md via copyFile
// padrao (mesmo path das outras CLIs). O wirar legado como .mdc foi
// descontinuado (ver removeLegacyCursorRules em apply_fs.go).

// syncCursorAll instala scripts em ~/.cursor/hooks/ e faz merge idempotente em hooks.json.
// Também copia o patterns.txt usado pelo bash-guardian e o helper Python do docs-cache.
func syncCursorAll(baseDir string, target TargetCLI) error {
	if target.HooksFormat != "cursor" || target.HooksSettingsPath == "" {
		return nil
	}
	if shouldDryRun() {
		fmt.Printf("[dry-run] cursor hooks/scripts em %s\n", target.HooksSettingsPath)
		return nil
	}

	hooksDir := filepath.Join(filepath.Dir(target.HooksSettingsPath), "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return err
	}

	extras := []string{
		"bash-guardian-patterns.txt",
		"docs-cache.cursor.py",
		"wrap-hook.sh",
	}
	for _, name := range extras {
		src := filepath.Join(baseDir, "hooks", name)
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("arquivo auxiliar Cursor não encontrado: %s", src)
		}
		dst := filepath.Join(hooksDir, name)
		if err := copyFile(src, dst); err != nil {
			return err
		}
		if name == "wrap-hook.sh" {
			if err := os.Chmod(dst, 0o755); err != nil {
				return err
			}
		}
	}

	for _, h := range cursorManagedHooks() {
		src := filepath.Join(baseDir, "hooks", h.Script)
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("script do hook Cursor não encontrado: %s", src)
		}
		dst := filepath.Join(hooksDir, h.Script)
		if err := copyFile(src, dst); err != nil {
			return err
		}
		if err := os.Chmod(dst, 0o755); err != nil {
			return err
		}
	}

	return mergeCursorHooksJSON(target.HooksSettingsPath)
}
