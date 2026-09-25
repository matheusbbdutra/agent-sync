package apply

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveLegacyCursorRules(t *testing.T) {
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(rulesDir, "agent-sync-global.mdc")
	if err := os.WriteFile(legacy, []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	removed, err := removeLegacyCursorRules(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("removeLegacyCursorRules: %v", err)
	}
	if !removed {
		t.Fatal("esperava removed=true")
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy devia ter sido removido: %v", err)
	}
	// Higiene: rules/ deve ter sido removido por estar vazio.
	if _, err := os.Stat(rulesDir); !os.IsNotExist(err) {
		t.Fatalf("rules/ vazia devia ter sido removida: %v", err)
	}
}

func TestRemoveLegacyCursorRulesAbsent(t *testing.T) {
	dir := t.TempDir()
	// Sem rules/ existente.
	removed, err := removeLegacyCursorRules(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("removeLegacyCursorRules: %v", err)
	}
	if removed {
		t.Fatal("esperava removed=false (sem legado)")
	}
}

func TestRemoveLegacyCursorRulesPreservesOtherFiles(t *testing.T) {
	dir := t.TempDir()
	rulesDir := filepath.Join(dir, "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Outra regra do usuario coexiste.
	other := filepath.Join(rulesDir, "my-project.mdc")
	if err := os.WriteFile(other, []byte("user-rule"), 0o644); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(rulesDir, "agent-sync-global.mdc")
	if err := os.WriteFile(legacy, []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := removeLegacyCursorRules(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Fatalf("removeLegacyCursorRules: %v", err)
	}
	// legacy sumiu
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy devia ter sumido: %v", err)
	}
	// outro arquivo preservado
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("outra regra deveria estar preservada: %v", err)
	}
}
