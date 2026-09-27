package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// apply_cline_prune_test.go: poda de plugins órfãos + recusa de baseDir não
// canônico (A-84, Pendência 4).

// makeOrphanClinePlugin simula um plugin remanescente de outro baseDir (o
// layout que clinePluginInstallDir produz), sem passar pelo apply.
func makeOrphanClinePlugin(t *testing.T, hooksDir, baseDir string) string {
	t.Helper()
	dir := clinePluginInstallDir(hooksDir, baseDir)
	if err := os.MkdirAll(filepath.Join(dir, "package"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `{"baseDir":"` + baseDir + `","bin":"agent-sync"}`
	if err := os.WriteFile(filepath.Join(dir, "package", "agent-sync-config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeFakeClinePluginSource cria cline-plugin/ num repo fake (o apply copia os
// arquivos de lá; o conteúdo é irrelevante para o teste de poda).
func writeFakeClinePluginSource(t *testing.T, repo string) {
	t.Helper()
	dir := filepath.Join(repo, "cline-plugin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A partir de A-87 o plugin inclui `dist/index.js` e `dist/types.js`
	// (artefatos de build do TS). Criamos a pasta dist/ e os arquivos para
	// que `os.WriteFile` abaixo não falhe em "no such file or directory".
	if err := os.MkdirAll(filepath.Join(dir, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range clinePluginFiles {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("// fake\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestSyncClineHooksPodaPluginOrfao cobre o A-84: o CLI carrega TODOS os dirs
// de _installed/local, então um plugin de outro baseDir faria cada hook rodar
// 2×. No caminho canônico ele é removido.
func TestSyncClineHooksPodaPluginOrfao(t *testing.T) {
	isolateConfigHome(t)
	baseDir := repoBaseDir(t)
	hooksDir := filepath.Join(t.TempDir(), "hooks")

	orphan := makeOrphanClinePlugin(t, hooksDir, "/tmp/repo-antigo-removido")
	if err := syncClineHooks(baseDir, fakeClineTarget(hooksDir)); err != nil {
		t.Fatalf("syncClineHooks: %v", err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Errorf("plugin órfão deveria ter sido podado: %s", orphan)
	}
	if _, err := os.Stat(clinePluginInstallDir(hooksDir, baseDir)); err != nil {
		t.Errorf("plugin canônico deveria existir: %v", err)
	}
}

// TestSyncClineHooksRecusaBaseDirNaoCanonico cobre a outra metade do A-84:
// apply de outro baseDir (ex.: worktree) avisa e NÃO instala nem poda.
func TestSyncClineHooksRecusaBaseDirNaoCanonico(t *testing.T) {
	isolateConfigHome(t)
	canonical := repoBaseDir(t)
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	if err := syncClineHooks(canonical, fakeClineTarget(hooksDir)); err != nil {
		t.Fatal(err)
	}
	canonicalDir := clinePluginInstallDir(hooksDir, canonical)

	outro := repoFake(t)
	writeFakeClinePluginSource(t, outro)
	err := syncClineHooks(outro, fakeClineTarget(hooksDir))
	if err == nil {
		t.Fatal("apply de baseDir não canônico deveria retornar aviso (erro)")
	}
	if !strings.Contains(err.Error(), AllowBasedirEnv) {
		t.Errorf("aviso deveria citar o escape hatch %s: %v", AllowBasedirEnv, err)
	}
	if _, statErr := os.Stat(clinePluginInstallDir(hooksDir, outro)); !os.IsNotExist(statErr) {
		t.Error("plugin do baseDir não canônico não deveria ser criado")
	}
	if _, statErr := os.Stat(canonicalDir); statErr != nil {
		t.Errorf("plugin canônico não deveria ser podado: %v", statErr)
	}
}

// TestSyncClineHooksEscapeHatchInstalaSemPodar: o escape hatch permite o
// wiramento explícito de outro baseDir, mas preserva o canônico (o usuário é
// avisado de que hooks podem rodar 2×).
func TestSyncClineHooksEscapeHatchInstalaSemPodar(t *testing.T) {
	isolateConfigHome(t)
	canonical := repoBaseDir(t)
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	if err := syncClineHooks(canonical, fakeClineTarget(hooksDir)); err != nil {
		t.Fatal(err)
	}
	canonicalDir := clinePluginInstallDir(hooksDir, canonical)

	outro := repoFake(t)
	writeFakeClinePluginSource(t, outro)
	t.Setenv(AllowBasedirEnv, "1")

	if err := syncClineHooks(outro, fakeClineTarget(hooksDir)); err != nil {
		t.Fatalf("escape hatch deveria instalar: %v", err)
	}
	if _, err := os.Stat(clinePluginInstallDir(hooksDir, outro)); err != nil {
		t.Errorf("plugin do baseDir forçado deveria existir: %v", err)
	}
	if _, err := os.Stat(canonicalDir); err != nil {
		t.Errorf("escape hatch não deveria podar o canônico: %v", err)
	}
}

func TestIsOwnClinePluginDirName(t *testing.T) {
	cases := map[string]bool{
		"agent-sync-hooks":              true,
		"agent-sync-hooks-7bda9b533174": true,
		"agent-sync-hooks-DF3A8B490DB8": false, // hash maiúsculo não é o nosso formato
		"agent-sync-hooks-7bda9b53317":  false, // 11 hex
		"agent-sync-hooks-curto":        false,
		"outro-plugin-de-terceiro":      false,
		"agent-sync":                    false,
	}
	for name, want := range cases {
		if got := isOwnClinePluginDirName(name); got != want {
			t.Errorf("isOwnClinePluginDirName(%q) = %v, esperado %v", name, got, want)
		}
	}
}
