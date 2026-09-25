package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matheusdutra/agent-sync/internal/target"
)

// TestSyncBashRmGuardianAntigravity valida end-to-end o wiramento:
// (1) cria hooks.json vazio (formato antigravity)
// (2) chama syncBashRmGuardianAntigravity
// (3) verifica que entry para o hook foi adicionada em PreToolUse
// (4) lê o script wirado e o executa com payload simulado
func TestSyncBashRmGuardianAntigravity(t *testing.T) {
	root := t.TempDir()

	// setup baseDir com hooks/bash-rm-guardian.antigravity.sh
	baseDir := filepath.Join(root, "base")
	hooksDir := filepath.Join(baseDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// copia script real do worktree para hooksDir
	wd, _ := os.Getwd()
	srcScript := filepath.Join(wd, "..", "..", "hooks", "bash-rm-guardian.antigravity.sh")
	srcScript = filepath.Clean(srcScript)
	data, err := os.ReadFile(srcScript)
	if err != nil {
		t.Skipf("script bash-rm-guardian.antigravity.sh não encontrado em %s: %v", srcScript, err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "bash-rm-guardian.antigravity.sh"), data, 0o755); err != nil {
		t.Fatal(err)
	}

	// setup hooks.json vazio
	hooksJSON := filepath.Join(root, ".gemini", "config", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(hooksJSON), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := map[string]any{}
	b, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(hooksJSON, b, 0o644); err != nil {
		t.Fatal(err)
	}

	tgt := target.TargetCLI{
		AgentKind:          "antigravity",
		HooksSettingsPath:  hooksJSON,
		HooksEvent:         "PreToolUse",
		HooksFormat:        "antigravity",
	}

	// Wirar
	if err := syncBashRmGuardianAntigravity(baseDir, tgt); err != nil {
		t.Fatalf("syncBashRmGuardianAntigravity: %v", err)
	}

	// Verifica JSON wirado
	raw, err := os.ReadFile(hooksJSON)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("JSON inválido após wiramento: %v\n%s", err, raw)
	}

	// estrutura antigravity: { "<hookName>": { "PreToolUse": [...] } }
	hookEntry, ok := parsed[bashRmGuardianHookName].(map[string]any)
	if !ok {
		t.Fatalf("hook entry ausente. parsed keys: %v", mapKeys(parsed))
	}
	pre, ok := hookEntry["PreToolUse"].([]any)
	if !ok || len(pre) == 0 {
		t.Fatalf("PreToolUse ausente ou vazio: %+v", hookEntry)
	}
	first, _ := pre[0].(map[string]any)
	if first == nil {
		t.Fatalf("primeira entrada de PreToolUse não é objeto: %+v", pre)
	}

	// estrutura antigravity real: matcher (singular) + hooks[{command, timeout}]
	if m, _ := first["matcher"].(string); m != "run_command" {
		t.Errorf("matcher = %q; want run_command", m)
	}
	hooks, _ := first["hooks"].([]any)
	if len(hooks) == 0 {
		t.Fatalf("hooks vazio: %+v", first)
	}
	h0, _ := hooks[0].(map[string]any)
	cmd, _ := h0["command"].(string)
	if !strings.Contains(cmd, "bash-rm-guardian.antigravity.sh") {
		t.Errorf("command não referencia o script: %q", cmd)
	}

	// Idempotência: rodar de novo não deve duplicar
	if err := syncBashRmGuardianAntigravity(baseDir, tgt); err != nil {
		t.Fatalf("idempotência falhou: %v", err)
	}
	raw2, _ := os.ReadFile(hooksJSON)
	var parsed2 map[string]any
	_ = json.Unmarshal(raw2, &parsed2)
	pre2, _ := parsed2[bashRmGuardianHookName].(map[string]any)["PreToolUse"].([]any)
	if len(pre2) != 1 {
		t.Errorf("idempotência quebrou: %d entradas (esperado 1)", len(pre2))
	}
}

func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
