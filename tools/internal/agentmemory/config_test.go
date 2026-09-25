package agentmemory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadRemoteConfigTokenEnvVar cobre o suporte a AGENT_SYNC_TURSO_TOKEN
// (ses_f2a16545bffeeCcxIFwZ04VVDa, 2026-09-25): env var sobrescreve campo `token`
// do JSON. URL permanece no JSON (URL nao e segredo). User prefere tokens via
// env var no zshrc em vez de campo JSON (chmod 600 fragil).
func TestLoadRemoteConfigTokenEnvVar(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, err := EnsureConfig()
	if err != nil {
		t.Fatal(err)
	}

	// Caso 1: JSON com url+token, env var setada -> token da env sobrescreve JSON.
	if err := os.WriteFile(path, []byte(`{"turso":{"url":"libsql://json-url.test","token":"json-token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_SYNC_TURSO_TOKEN", "env-token")
	address, token, err := LoadRemoteConfig()
	if err != nil {
		t.Fatalf("caso 1 falhou: %v", err)
	}
	if address != "libsql://json-url.test" {
		t.Errorf("url deveria vir do JSON: %q", address)
	}
	if token != "env-token" {
		t.Errorf("token deveria vir da env var: %q", token)
	}

	// Caso 2: JSON com url, sem token, env var setada -> funciona.
	if err := os.WriteFile(path, []byte(`{"turso":{"url":"libsql://only-url.test","token":""}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	address, token, err = LoadRemoteConfig()
	if err != nil {
		t.Fatalf("caso 2 falhou: %v", err)
	}
	if address != "libsql://only-url.test" || token != "env-token" {
		t.Errorf("esperava url=libsql://only-url.test, token=env-token, obtive %q, %q", address, token)
	}

	// Caso 3: JSON com url+token, env var UNSETED -> comportamento atual (token do JSON).
	if err := os.WriteFile(path, []byte(`{"turso":{"url":"libsql://json-url.test","token":"json-token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Unsetenv("AGENT_SYNC_TURSO_TOKEN"); err != nil {
		t.Fatal(err)
	}
	address, token, err = LoadRemoteConfig()
	if err != nil {
		t.Fatalf("caso 3 falhou: %v", err)
	}
	if token != "json-token" {
		t.Errorf("sem env var, token deveria vir do JSON: %q", token)
	}

	// Caso 4: mensagem de erro menciona env var quando nada esta preenchido.
	if err := os.WriteFile(path, []byte(`{"turso":{"url":"","token":""}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err = LoadRemoteConfig()
	if err == nil || !strings.Contains(err.Error(), "AGENT_SYNC_TURSO_TOKEN") {
		t.Errorf("erro deveria mencionar env var AGENT_SYNC_TURSO_TOKEN: %v", err)
	}
}

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
	if err != nil {
		t.Fatalf("configuração existente ilegível: %v", err)
	}
	// Verifica que os valores originais foram preservados (merge não apaga campos).
	if !strings.Contains(string(after), "libsql://example.test") || !strings.Contains(string(after), "secret-token") {
		t.Fatalf("configuração existente sobrescrita: %s", after)
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
