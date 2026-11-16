package apply

import (
	"os"
	"path/filepath"
	"testing"
)

// naoDryRun garante que syncSkills escreve de verdade no teste.
func naoDryRun(t *testing.T) {
	t.Helper()
	prevEnabled, prevEnv := dryRunEnabled, dryRunEnvCache
	t.Cleanup(func() { dryRunEnabled, dryRunEnvCache = prevEnabled, prevEnv })
	t.Setenv("AGENT_SYNC_DRY_RUN", "")
	dryRunEnabled = false
	dryRunEnvCache = false
	if shouldDryRun() {
		t.Fatal("dry-run ativo; o teste precisa escrever em disco")
	}
}

// TestSyncSkillsPreservaEntradasNaoGerenciadas prova que -apply nunca apaga o
// que o repo nao gerencia (skill instalada a mao, plugin de terceiro).
func TestSyncSkillsPreservaEntradasNaoGerenciadas(t *testing.T) {
	naoDryRun(t)
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	if err := os.MkdirAll(filepath.Join(src, "gerenciada"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "gerenciada", "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(dst, "instalada-a-mao")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "SKILL.md"), []byte("meu"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncSkills(src, dst); err != nil {
		t.Fatalf("syncSkills: %v", err)
	}

	if _, err := os.Stat(filepath.Join(local, "SKILL.md")); err != nil {
		t.Fatalf("entrada nao gerenciada foi perdida: %v", err)
	}
	fi, err := os.Lstat(filepath.Join(dst, "gerenciada"))
	if err != nil {
		t.Fatalf("skill gerenciada nao foi criada: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("skill gerenciada deveria ser symlink para o repo")
	}
}

// TestSyncSkillsNaoSubstituiDirLocalComMesmoNome cobre o caso destrutivo real:
// existe conteudo local com o nome de uma skill do repo. Nada pode ser apagado.
func TestSyncSkillsNaoSubstituiDirLocalComMesmoNome(t *testing.T) {
	naoDryRun(t)
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	if err := os.MkdirAll(filepath.Join(src, "colidente"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "colidente", "SKILL.md"), []byte("repo"), 0o644); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(dst, "colidente")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "SKILL.md"), []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncSkills(src, dst); err != nil {
		t.Fatalf("syncSkills: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(local, "SKILL.md"))
	if err != nil {
		t.Fatalf("conteudo local apagado: %v", err)
	}
	if string(got) != "local" {
		t.Fatalf("conteudo local sobrescrito: %q", string(got))
	}
	if fi, err := os.Lstat(local); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Fatal("diretorio local nao deveria ter virado symlink")
	}
}

// TestSyncSkillsAtualizaSymlinkAntigo garante que o symlink gerenciado continua
// sendo reapontado para o repo (comportamento que ja existia).
func TestSyncSkillsAtualizaSymlinkAntigo(t *testing.T) {
	naoDryRun(t)
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	if err := os.MkdirAll(filepath.Join(src, "gerenciada"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "gerenciada", "SKILL.md"), []byte("novo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "caminho-velho"), filepath.Join(dst, "gerenciada")); err != nil {
		t.Fatal(err)
	}

	if err := syncSkills(src, dst); err != nil {
		t.Fatalf("syncSkills: %v", err)
	}

	target, err := os.Readlink(filepath.Join(dst, "gerenciada"))
	if err != nil {
		t.Fatalf("symlink nao existe: %v", err)
	}
	if target != filepath.Join(src, "gerenciada") {
		t.Fatalf("symlink nao foi reapontado: %s", target)
	}
}
