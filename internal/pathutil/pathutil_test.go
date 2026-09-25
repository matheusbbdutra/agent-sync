package pathutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindBaseDir(t *testing.T) {
	tempDir := t.TempDir()
	rulesDir := filepath.Join(tempDir, "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "global-rules.md"), []byte("# Rules"), 0o644); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(tempDir, "a", "b", "c")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	base, ok := FindBaseDir([]string{subDir})
	if !ok {
		t.Fatalf("FindBaseDir falhou ao encontrar baseDir a partir de %s", subDir)
	}
	if base != tempDir {
		t.Errorf("baseDir esperado %q, obteve %q", tempDir, base)
	}

	_, ok = FindBaseDir([]string{"/nonexistent/directory/xyz"})
	if ok {
		t.Errorf("FindBaseDir deveria falhar para diretorio inexistente")
	}
}

func TestResolveBaseDir(t *testing.T) {
	tempDir := t.TempDir()
	rulesDir := filepath.Join(tempDir, "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "global-rules.md"), []byte("# Rules"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := ResolveBaseDir("", "", tempDir)
	if err != nil {
		t.Fatalf("esperava sucesso, obteve erro: %v", err)
	}
	if res != tempDir {
		t.Errorf("esperado %s, obteve %s", tempDir, res)
	}

	if _, err := ResolveBaseDir("", "/tmp", ""); err == nil {
		t.Errorf("esperava erro fail-fast para /tmp sem rules/global-rules.md, obteve nil")
	}
}

// TestResolveBaseDir_FailsFastWhenNotInRepo trava a regressao do bug
// 2026-09-23: antes deste fix, ResolveBaseDir retornava cwd silenciosamente
// quando rules/global-rules.md nao era encontrado, permitindo que
// `agent-sync -apply` rodado da $HOME gravasse paths /home/<user>/hooks/...
// em ~/.claude/settings.json (ver docs/postmortems/2026-09-23-skills-truncation-and-hook-paths.md).
func TestResolveBaseDir_FailsFastWhenNotInRepo(t *testing.T) {
	t.Setenv("AGENT_SYNC_HOME", "")
	_, err := ResolveBaseDir("", t.TempDir(), "")
	if err == nil {
		t.Fatal("esperava erro fail-fast quando nenhum start contem rules/global-rules.md")
	}
	if !strings.Contains(err.Error(), "rules/global-rules.md") {
		t.Errorf("mensagem de erro deveria mencionar rules/global-rules.md, obteve: %v", err)
	}
}

func TestIsProtectedSkillsDir(t *testing.T) {
	if !IsProtectedSkillsDir("/home/user/.cursor/skills-cursor") {
		t.Errorf("deveria ser protegido: skills-cursor")
	}
	if !IsProtectedSkillsDir("/home/user/.config/plugins/some-plugin/skills") {
		t.Errorf("deveria ser protegido: config/plugins")
	}
	if IsProtectedSkillsDir("/home/user/.cursor/skills") {
		t.Errorf("nao deveria ser protegido: ~/.cursor/skills")
	}
}

func TestHookScriptPath_Found(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(hooksDir, "x.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := HookScriptPath(dir, "x.sh")
	if err != nil {
		t.Fatalf("esperava sucesso, obteve erro: %v", err)
	}
	if got != script {
		t.Errorf("path esperado %q, obteve %q", script, got)
	}
}

// TestHookScriptPath_Missing trava a regressao do bug 2026-09-23:
// quando baseDir apontava para /home/matheus_dutra (cwd-fallback do
// ResolveBaseDir), scripts como wrap-hook.sh ficavam silenciosamente
// ausentes e o apply gravava paths fantasma em ~/.claude/settings.json.
func TestHookScriptPath_Missing(t *testing.T) {
	t.Setenv("AGENT_SYNC_QUIET", "1")
	if _, err := HookScriptPath(t.TempDir(), "wrap-hook.sh"); err == nil {
		t.Fatal("esperava erro para hook script inexistente")
	}
}
