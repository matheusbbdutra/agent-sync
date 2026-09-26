package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/matheusdutra/agent-sync/internal/target"
)

// TestSmokeOpenCodeEndToEnd wirar bash-rm-guardian em opencode.json
// (permission.bash com patterns ask) e valida idempotência.
func TestSmokeOpenCodeEndToEnd(t *testing.T) {
	root := t.TempDir()

	// BaseDir
	baseDir := filepath.Join(root, "base")
	hooksDir := filepath.Join(baseDir, "hooks")
	_ = os.MkdirAll(hooksDir, 0o755)

	// OpenCodePluginDir (estrutura: <root>/.opencode/plugin/)
	pluginDir := filepath.Join(root, ".opencode", "plugin")
	_ = os.MkdirAll(pluginDir, 0o755)
	configPath := filepath.Join(root, ".opencode", "opencode.json")

	// opencode.json inicial vazio
	if err := os.WriteFile(configPath, []byte(`{"$schema":"https://opencode.ai/config.json"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	tgt := target.TargetCLI{
		AgentKind:         "opencode",
		OpenCodePluginDir: pluginDir,
		HooksFormat:       "opencode",
	}

	if err := syncBashRmGuardianOpenCode(baseDir, tgt); err != nil {
		t.Fatalf("syncBashRmGuardianOpenCode: %v", err)
	}

	// Valida opencode.json
	raw, _ := os.ReadFile(configPath)
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, raw)
	}

	perm, _ := parsed["permission"].(map[string]any)
	if perm == nil {
		t.Fatalf("permission ausente: %s", raw)
	}
	bash, _ := perm["bash"].(map[string]any)
	if bash == nil {
		t.Fatalf("permission.bash ausente: %v", perm)
	}

	// Valida que cada pattern está marcado como "ask"
	for _, pat := range bashRmGuardianOpenCodePatterns {
		effect, _ := bash[pat].(string)
		if effect != "ask" {
			t.Errorf("pattern %q esperado ask, obteve %q", pat, effect)
		}
	}

	// Idempotência: re-run não deve duplicar
	if err := syncBashRmGuardianOpenCode(baseDir, tgt); err != nil {
		t.Fatalf("idempotência falhou: %v", err)
	}
	raw2, _ := os.ReadFile(configPath)
	var parsed2 map[string]any
	_ = json.Unmarshal(raw2, &parsed2)
	bash2, _ := parsed2["permission"].(map[string]any)["bash"].(map[string]any)
	for _, pat := range bashRmGuardianOpenCodePatterns {
		if _, exists := bash2[pat]; !exists {
			t.Errorf("idempotência quebrou: pattern %q sumiu", pat)
		}
	}
	// contar entries com mesmo pattern (não pode ter 2x o mesmo)
	for pat := range bash2 {
		count := 0
		for p := range bash2 {
			if p == pat {
				count++
			}
		}
		if count > 1 {
			t.Errorf("pattern %q duplicado (%d entries)", pat, count)
		}
	}

	// Valida que entries anteriores do bash-guardian foram preservadas
	// (skip se patterns.txt ausente — bash-guardian wiramento exige ele)
	wd, _ := os.Getwd()
	patternsSrc := filepath.Join(wd, "..", "..", "hooks", "bash-guardian-patterns.txt")
	if data, err := os.ReadFile(patternsSrc); err == nil {
		_ = os.WriteFile(filepath.Join(hooksDir, "bash-guardian-patterns.txt"), data, 0o644)
		tgt2 := target.TargetCLI{
			AgentKind:         "opencode",
			OpenCodePluginDir: pluginDir,
			HooksFormat:       "opencode",
		}
		if err := syncBashGuardianOpenCode(baseDir, tgt2); err != nil {
			t.Fatalf("syncBashGuardianOpenCode: %v", err)
		}
		raw3, _ := os.ReadFile(configPath)
		var parsed3 map[string]any
		_ = json.Unmarshal(raw3, &parsed3)
		bash3, _ := parsed3["permission"].(map[string]any)["bash"].(map[string]any)
		if _, ok := bash3["rm -rf*"]; !ok {
			t.Errorf("rm -rf* deveria estar presente após wirar bash-guardian")
		}
	}
}
