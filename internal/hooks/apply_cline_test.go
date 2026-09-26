package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeClineTarget(hooksDir string) TargetCLI {
	return TargetCLI{
		Name:              "cline",
		AgentKind:         "cline",
		HooksFormat:       "cline",
		HooksSettingsPath: hooksDir,
		HooksEvent:        "PreToolUse",
	}
}

// repoBaseDir devolve a raiz do repo (o go test roda em internal/hooks).
func repoBaseDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSyncClineHooksInstalaPlugin(t *testing.T) {
	baseDir := repoBaseDir(t)
	configDir := t.TempDir()
	hooksDir := filepath.Join(configDir, "hooks")

	if err := syncClineHooks(baseDir, fakeClineTarget(hooksDir)); err != nil {
		t.Fatalf("syncClineHooks: %v", err)
	}

	installDir := clinePluginInstallDir(hooksDir, baseDir)
	if !strings.HasPrefix(installDir, configDir) {
		t.Fatalf("plugin instalado fora do config dir: %s", installDir)
	}
	for _, name := range []string{"index.js", "package.json", "plugin.json", "agent-sync-config.json"} {
		if _, err := os.Stat(filepath.Join(installDir, "package", name)); err != nil {
			t.Errorf("arquivo do plugin ausente (%s): %v", name, err)
		}
	}

	// package.json agregador: é o que o CLI usa para descobrir os entrypoints.
	data, err := os.ReadFile(filepath.Join(installDir, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var aggregator struct {
		Cline struct {
			Plugins []struct {
				Paths []string `json:"paths"`
			} `json:"plugins"`
		} `json:"cline"`
	}
	if err := json.Unmarshal(data, &aggregator); err != nil {
		t.Fatalf("package.json agregador invalido: %v", err)
	}
	if len(aggregator.Cline.Plugins) != 1 || len(aggregator.Cline.Plugins[0].Paths) != 1 {
		t.Fatalf("agregador sem paths: %s", data)
	}
	if got := aggregator.Cline.Plugins[0].Paths[0]; got != "./package/index.js" {
		t.Errorf("path do plugin = %q", got)
	}

	// agent-sync-config.json carrega o baseDir absoluto do repo.
	cfgRaw, err := os.ReadFile(filepath.Join(installDir, "package", "agent-sync-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		BaseDir string `json:"baseDir"`
		Bin     string `json:"bin"`
	}
	if err := json.Unmarshal(cfgRaw, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.BaseDir != baseDir {
		t.Errorf("baseDir=%q, esperado %q", cfg.BaseDir, baseDir)
	}
	// O binário é resolvido em runtime (caminho absoluto do executável que
	// rodou o apply); "agent-sync" é apenas fallback quando não há executável.
	if cfg.Bin == "" {
		t.Errorf("bin vazio no agent-sync-config.json")
	}

	// O plugin precisa declarar a capability "hooks" (senão o Cline não o usa).
	js, err := os.ReadFile(filepath.Join(installDir, "package", "index.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), `capabilities: ["hooks"]`) {
		t.Errorf("index.js sem capability hooks")
	}
}

func TestSyncClineHooksIdempotente(t *testing.T) {
	baseDir := repoBaseDir(t)
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	target := fakeClineTarget(hooksDir)

	if err := syncClineHooks(baseDir, target); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(clinePluginInstallDir(hooksDir, baseDir), "package", "index.js")
	first, err := os.Stat(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncClineHooks(baseDir, target); err != nil {
		t.Fatal(err)
	}
	second, err := os.Stat(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !first.ModTime().Equal(second.ModTime()) {
		t.Errorf("mtime mudou em apply idempotente (rewrite desnecessario)")
	}
}

func TestSyncClineHooksRemoveArtefatosLegados(t *testing.T) {
	baseDir := repoBaseDir(t)
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := []string{"PreToolUse", "PostToolUse", "TaskStart", "TaskComplete", "bash-rm-guardian.sh"}
	for _, name := range legacy {
		if err := os.WriteFile(filepath.Join(hooksDir, name), []byte("# legado\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := syncClineHooks(baseDir, fakeClineTarget(hooksDir)); err != nil {
		t.Fatalf("syncClineHooks: %v", err)
	}
	for _, name := range legacy {
		if _, err := os.Stat(filepath.Join(hooksDir, name)); !os.IsNotExist(err) {
			t.Errorf("artefato legado %s nao foi removido", name)
		}
	}
}

func TestSyncClineHooksIgnoraOutrosAlvos(t *testing.T) {
	configDir := t.TempDir()
	target := TargetCLI{
		AgentKind:         "codex",
		HooksFormat:       "",
		HooksSettingsPath: filepath.Join(configDir, "hooks.json"),
	}
	if err := syncClineHooks(repoBaseDir(t), target); err != nil {
		t.Fatalf("syncClineHooks: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "plugins")); err == nil {
		t.Errorf("syncClineHooks nao deveria tocar em alvo nao-cline")
	}
}

func TestSyncHookCommandAtEventIgnoraCline(t *testing.T) {
	// Cline não usa JSON de settings para hooks: os hooks default precisam sair
	// sem erro (era a causa dos warnings "HooksSettingsPath is a directory").
	hooksDir := filepath.Join(t.TempDir(), ".cline", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	err := syncHookCommandAtEvent(t.TempDir(), fakeClineTarget(hooksDir),
		"agent-sync-context-guard", "/bin/true", "*", "PreToolUse", nil)
	if err != nil {
		t.Fatalf("esperava no-op para cline, obteve erro: %v", err)
	}
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("nada deveria ser gravado no diretorio de hooks do Cline: %v", entries)
	}
}
