package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withHome aponta os.UserHomeDir para um diretório temporário durante o teste.
// Persiste zshrc/bashrc pré-configurados conforme opts.
func withHome(t *testing.T, zshrc, bashrc string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if zshrc != "" {
		if err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte(zshrc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if bashrc != "" {
		if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte(bashrc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPersistShellEnvCriaBlocoEmBashrcQuandoNenhumRcExiste(t *testing.T) {
	home := withHome(t, "", "")
	// existe nem zshrc nem bashrc — fallback deve criar ~/.bashrc
	if err := persistShellEnv(); err != nil {
		t.Fatalf("first persist: %v", err)
	}
	got := readFile(t, filepath.Join(home, ".bashrc"))
	if !strings.Contains(got, shellEnvMarker) {
		t.Errorf("missing marker; got:\n%s", got)
	}
	if !strings.Contains(got, "export AGENT_SYNC_PRETOOLUSE_VALIDATE=1") {
		t.Errorf("missing export line; got:\n%s", got)
	}
}

func TestPersistShellEnvPrefereZshrcQuandoExiste(t *testing.T) {
	home := withHome(t, "alias ls='ls --color=auto'\n", "")
	if err := persistShellEnv(); err != nil {
		t.Fatalf("persist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".bashrc")); err == nil {
		t.Errorf("~/.bashrc should not be created when ~/.zshrc exists")
	}
	got := readFile(t, filepath.Join(home, ".zshrc"))
	if !strings.Contains(got, shellEnvMarker) {
		t.Errorf("zshrc missing marker; got:\n%s", got)
	}
	if !strings.HasPrefix(got, "alias ls='ls --color=auto'") {
		t.Errorf("original content mangled; got:\n%s", got)
	}
}

func TestPersistShellEnvIdempotenteEmChamadasRepetidas(t *testing.T) {
	home := withHome(t, "", "")
	for i := 0; i < 3; i++ {
		if err := persistShellEnv(); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	got := readFile(t, filepath.Join(home, ".bashrc"))
	count := strings.Count(got, shellEnvMarker)
	if count != 1 {
		t.Errorf("marker duplicated %d times after 3 calls (want 1):\n%s", count, got)
	}
	count = strings.Count(got, "export AGENT_SYNC_PRETOOLUSE_VALIDATE=1")
	if count != 1 {
		t.Errorf("export duplicated %d times (want 1):\n%s", count, got)
	}
}

func TestPersistShellEnvSubstituiBlocoExistente(t *testing.T) {
	// Simula um rc onde o bloco anterior foi escrito por uma versão antiga do
	// agent-sync com valor diferente da env (apenas cenário defensivo — não
	// há versão antiga hoje, mas a idempotência precisa tolerar).
	old := "# cabeçalho\n" + shellEnvMarker + "\nexport AGENT_SYNC_PRETOOLUSE_VALIDATE=0\n\n# rodapé\n"
	home := withHome(t, "", old)
	if err := persistShellEnv(); err != nil {
		t.Fatalf("persist: %v", err)
	}
	got := readFile(t, filepath.Join(home, ".bashrc"))
	if !strings.Contains(got, "export AGENT_SYNC_PRETOOLUSE_VALIDATE=1") {
		t.Errorf("expected new export line; got:\n%s", got)
	}
	if strings.Contains(got, "AGENT_SYNC_PRETOOLUSE_VALIDATE=0") {
		t.Errorf("stale value should have been replaced; got:\n%s", got)
	}
	if !strings.Contains(got, "# cabeçalho") || !strings.Contains(got, "# rodapé") {
		t.Errorf("surrounding content lost; got:\n%s", got)
	}
	if strings.Count(got, shellEnvMarker) != 1 {
		t.Errorf("marker should appear exactly once; got:\n%s", got)
	}
}

func TestReplaceBlockIsolado(t *testing.T) {
	data := []byte("# topo\n" + shellEnvMarker + "\nlinha antiga 1\nlinha antiga 2\n\n# fim\n")
	novo := shellEnvMarker + "\nexport FOO=bar\n"
	out := string(replaceBlock(data, shellEnvMarker, novo))
	if !strings.Contains(out, "# topo") || !strings.Contains(out, "# fim") {
		t.Errorf("surrounding content lost: %s", out)
	}
	if strings.Contains(out, "linha antiga") {
		t.Errorf("old block content not removed: %s", out)
	}
	if !strings.Contains(out, "export FOO=bar") {
		t.Errorf("new block content missing: %s", out)
	}
}
