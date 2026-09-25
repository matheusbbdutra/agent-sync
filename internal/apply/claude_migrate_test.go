package apply

import (
	"os"
	"path/filepath"
	"testing"
)

// Migração CLAUDE.md -> AGENTS.md: o legado era cópia verbatim do wirar,
// então a remoção não perde conteúdo.
func TestRemoveLegacyClaudeRules(t *testing.T) {
	oldDry := dryRunEnabled
	oldCache := dryRunEnvCache
	t.Cleanup(func() { dryRunEnabled = oldDry; dryRunEnvCache = oldCache })

	newRules := filepath.Join(t.TempDir(), ".claude", "AGENTS.md")
	legacy := filepath.Join(filepath.Dir(newRules), "CLAUDE.md")

	// Sem legado: (false, nil), silencioso.
	dryRunEnabled, dryRunEnvCache = false, false
	if removed, err := removeLegacyClaudeRules(newRules); err != nil || removed {
		t.Fatalf("sem legado: removed=%v err=%v", removed, err)
	}

	// Com legado: remove e retorna true.
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("# legado"), 0o644); err != nil {
		t.Fatal(err)
	}
	if removed, err := removeLegacyClaudeRules(newRules); err != nil || !removed {
		t.Fatalf("com legado: removed=%v err=%v", removed, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legado deveria ter sido removido: %v", err)
	}

	// Dry-run: não remove, mas sinaliza que removeria.
	if err := os.WriteFile(legacy, []byte("# legado"), 0o644); err != nil {
		t.Fatal(err)
	}
	dryRunEnabled = true
	if removed, err := removeLegacyClaudeRules(newRules); err != nil || !removed {
		t.Fatalf("dry-run: removed=%v err=%v", removed, err)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("dry-run não deve remover: %v", err)
	}
}
