package agentmemory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigCreatedAndLoaded(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, err := EnsureConfig()
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "agent-sync", "config.json") {
		t.Fatalf("caminho inesperado: %s", path)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("permissões do arquivo: %v %v", info, err)
	}
	if _, _, err := LoadRemoteConfig(); err == nil || !strings.Contains(err.Error(), "preencha") {
		t.Fatalf("esperava orientação para preencher configuração: %v", err)
	}
	data := []byte(`{"turso":{"url":"libsql://example.test","token":"secret-token"}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureConfig(); err != nil {
		t.Fatal(err)
	}
	address, token, err := LoadRemoteConfig()
	if err != nil || address != "libsql://example.test" || token != "secret-token" {
		t.Fatalf("configuração não carregada: %q, %q, %v", address, token, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(data) {
		t.Fatalf("configuração existente sobrescrita: %v", err)
	}
}

func TestConfigRejectsUnsafePermissions(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, err := EnsureConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadRemoteConfig(); err == nil || !strings.Contains(err.Error(), "chmod 600") {
		t.Fatalf("permissões inseguras aceitas: %v", err)
	}
}
