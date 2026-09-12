package main

import (
	"os"
	"path/filepath"
	"testing"
)

func setupRepo(t *testing.T, root string) {
	t.Helper()
	rulesDir := filepath.Join(root, "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "global-rules.md"), []byte("# rules"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestFindBaseDirUpward(t *testing.T) {
	root := t.TempDir()
	setupRepo(t, root)

	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	got, ok := findBaseDir([]string{deep})
	if !ok {
		t.Fatal("esperava encontrar a raiz do repositório")
	}
	if got != root {
		t.Fatalf("got %q, want %q", got, root)
	}
}

func TestFindBaseDirNotFound(t *testing.T) {
	dir := t.TempDir()
	if _, ok := findBaseDir([]string{dir}); ok {
		t.Fatal("não deveria encontrar rules/global-rules.md")
	}
}

func TestResolveBaseDirPrefersEnv(t *testing.T) {
	envRoot := t.TempDir()
	setupRepo(t, envRoot)

	cwdRoot := t.TempDir()
	setupRepo(t, cwdRoot)

	got := resolveBaseDir("", cwdRoot, envRoot)
	if got != envRoot {
		t.Fatalf("got %q, want %q", got, envRoot)
	}
}

func TestResolveBaseDirFromExecutable(t *testing.T) {
	exeRoot := t.TempDir()
	setupRepo(t, exeRoot)
	exePath := filepath.Join(exeRoot, "bin", "agent-sync")

	got := resolveBaseDir(exePath, t.TempDir(), "")
	if got != exeRoot {
		t.Fatalf("got %q, want %q", got, exeRoot)
	}
}

func TestResolveBaseDirFallsBackToCwd(t *testing.T) {
	cwd := t.TempDir()
	got := resolveBaseDir(filepath.Join(cwd, "exe"), cwd, "")
	if got != cwd {
		t.Fatalf("got %q, want %q", got, cwd)
	}
}

func TestIsProtectedSkillsDir(t *testing.T) {
	protected := []string{
		"/home/u/.gemini/config/plugins/antigravity-skills-manager/skills",
		"/home/u/.config/plugins/foo/skills",
	}
	for _, dir := range protected {
		if !isProtectedSkillsDir(dir) {
			t.Errorf("esperava proteger %q", dir)
		}
	}

	safe := []string{
		"/home/u/.gemini/antigravity-cli/skills",
		"/home/u/.config/opencode/skills",
		"/home/u/.claude/skills",
	}
	for _, dir := range safe {
		if isProtectedSkillsDir(dir) {
			t.Errorf("não deveria proteger %q", dir)
		}
	}
}
