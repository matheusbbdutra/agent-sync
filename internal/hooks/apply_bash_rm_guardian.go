package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// apply_bash_rm_guardian.go: wiramento cross-CLI do hook bash-rm-guardian
// (A-74). Cobre antigravity (A-75), claude/codex/cursor (A-76), opencode
// (A-77). Cline (A-79) migrou para o bridge de hooks por arquivo (A-80.1) —
// ver internal/hooks/apply_cline.go.

const bashRmGuardianHookName = "agent-sync-bash-rm-guardian"

// syncBashRmGuardian é o entry-point registrado em hooks_all.go.
// Despacha por AgentKind (não HooksFormat, porque claude/codex compartilham
// formato mas se identificam por AgentKind). Cada CLI wirar em PreToolUse
// para consistency — formato do output adapta-se ao contrato de cada CLI.
//
// Cline fica fora deste switch: ele não usa JSON de settings, e sim o bridge
// de hooks por arquivo (internal/hooks/apply_cline.go, A-80.1), que já inclui
// bash-rm-guardian na lista de PreToolUse.
func syncBashRmGuardian(baseDir string, target TargetCLI) error {
	switch target.AgentKind {
	case "antigravity":
		return syncBashRmGuardianAntigravity(baseDir, target)
	case "claude":
		return syncBashRmGuardianStandard(baseDir, target)
	case "codex":
		return syncBashRmGuardianStandard(baseDir, target)
	case "opencode":
		return syncBashRmGuardianOpenCode(baseDir, target)
	default:
		return nil
	}
}

// syncBashRmGuardianStandard wirar PreToolUse para Claude Code e Codex.
// Usa o script `bash-rm-guardian.pretooluse.sh` que emite no formato
// `hookSpecificOutput.additionalContext` (Claude/Codex nativos) em vez de
// injectSteps (Antigravity-only).
func syncBashRmGuardianStandard(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	// Codex fica como Claude aqui: o additionalContext entre a tool call e o
	// output derruba o turno com 400 (2013), mas este e o unico hook que
	// AVISA antes de um rm destrutivo. Silencia-lo no codex trocaria um
	// evento raro por perda permanente de aviso. O scenario e raro de
	// proposito: so dispara quando o audit detecta refs ativas.
	return syncStandardHookAtEvent(baseDir, target, bashRmGuardianHookName,
		"bash-rm-guardian.pretooluse.sh", "*", "PreToolUse", nil)
}

// bashRmGuardianOpenCodePatterns lista os patterns bash que disparam o
// guardian em OpenCode. Como OpenCode permission.bash só tem allow/deny/ask
// binário (sem injectSteps), usamos "ask" para forçar confirmação humana
// antes de executar comandos destrutivos — equivalente comportamental ao
// "warn" das outras CLIs (decisão A-74: não bloquear, mas tornar visível).
var bashRmGuardianOpenCodePatterns = []string{
	"rm -rf*",
	"rm -fr*",
	"rm -r *",
	"rmdir *",
	"mv *",
}

// syncBashRmGuardianOpenCode wirar permission.bash no opencode.json com
// patterns de risco marcados como "ask". Idempotente: re-run remove
// entradas anteriores do bash-rm-guardian antes de reinserir.
//
// Limitação OpenCode: permission.bash é binário (allow|deny|ask). Não
// suporta mensagem customizada tipo injectSteps/additionalContext.
// "ask" é o equivalente OpenCode ao "warn" — modelo é forçado a confirmar.
func syncBashRmGuardianOpenCode(baseDir string, target TargetCLI) error {
	if target.OpenCodePluginDir == "" {
		return nil
	}

	configPath := filepath.Join(filepath.Dir(target.OpenCodePluginDir), openCodeConfigFile)

	raw, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		raw = []byte(`{"$schema":"https://opencode.ai/config.json"}`)
	} else if err != nil {
		return err
	}

	var root map[string]json.RawMessage
	if len(raw) == 0 {
		root = map[string]json.RawMessage{}
	} else if err := json.Unmarshal(raw, &root); err != nil {
		return fmt.Errorf("%s: JSON inválido: %w", configPath, err)
	}

	permission := map[string]json.RawMessage{}
	if raw, ok := root["permission"]; ok {
		if err := json.Unmarshal(raw, &permission); err != nil {
			return fmt.Errorf("%s: campo permission inválido: %w", configPath, err)
		}
	}

	var bashPairs []kvPair
	if raw, ok := permission["bash"]; ok {
		bashPairs, err = decodeOrderedStringMap(raw)
		if err != nil {
			return fmt.Errorf("%s: permission.bash inválido: %w", configPath, err)
		}
	}

	// Remove entradas anteriores do bash-rm-guardian (marcadas com prefixo
	// sintético nos patterns), depois reinsere.
	bashPairs = upsertOpenCodePairs(bashPairs, bashRmGuardianOpenCodePatterns, "ask")

	bashRaw, err := encodeOrderedStringMap(bashPairs)
	if err != nil {
		return err
	}
	permission["bash"] = bashRaw

	permissionRaw, err := json.Marshal(permission)
	if err != nil {
		return err
	}
	root["permission"] = permissionRaw

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(configPath, out, 0o644)
}

// upsertOpenCodePairs remove entradas com keys que começam com os patterns
// do guardian e reinsere com o effect desejado. Idempotente.
func upsertOpenCodePairs(pairs []kvPair, patterns []string, effect string) []kvPair {
	kept := pairs[:0:0]
	for _, p := range pairs {
		isGuardian := false
		for _, pat := range patterns {
			if p.Key == pat {
				isGuardian = true
				break
			}
		}
		if !isGuardian {
			kept = append(kept, p)
		}
	}
	for _, pat := range patterns {
		kept = append(kept, kvPair{Key: pat, Value: effect})
	}
	return kept
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
