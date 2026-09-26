package hooks

// apply_cline.go: wiramento dos hooks agent-sync em Cline (A-80.1').
//
// Evidência empírica (docs/investigations/cline-hooks-contract.md): o Cline CLI
// v3 NÃO executa hooks por arquivo em ~/.cline/hooks — o loader existe no SDK,
// mas só é criado quando há config-extension com capability "hooks", e a lista
// de config-extensions do CLI é fixa em ["rules","skills","plugins"]. Logo
// hooks de arquivo (e o shim do A-79 / A-80.1-v1) são inertes no CLI.
//
// A rota suportada é um **Cline Plugin** (AgentPlugin) instalado em
// ~/.cline/plugins/_installed/<kind>/<nome>-<hash>/package/ e registrado pelo
// package.json agregador ({"cline":{"plugins":[{"paths":["./package/index.js"]}]}}).
// O plugin (cline-plugin/index.js no repo) é um adapter fino que chama o
// binário Go (`agent-sync hook cline`), onde vive a tradução dos contratos.
//
// O wiramento é idempotente (escreve só quando o conteúdo muda) e remove
// artefatos inertes de iterações anteriores (shims de evento do A-80.1-v1 e o
// core copiado pelo A-79).

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const clinePluginName = "agent-sync-hooks"

// clineLegacyHookArtifacts são arquivos de wiramentos anteriores que ficaram
// inertes (o CLI não lê ~/.cline/hooks) e são removidos para evitar drift:
// shims de evento do A-80.1-v1 + cópias do A-79.
var clineLegacyHookArtifacts = []string{
	"PreToolUse", "PostToolUse", "TaskStart", "TaskComplete",
	"bash-rm-guardian.sh",
}

// clinePluginFiles são os arquivos do plugin versionados no repo (fonte).
var clinePluginFiles = []string{"index.js", "package.json", "plugin.json"}

// syncClineHooks instala (ou atualiza) o Cline Plugin. Para o Cline,
// target.HooksSettingsPath aponta para ~/.cline/hooks (config dir do CLI).
func syncClineHooks(baseDir string, target TargetCLI) error {
	if target.HooksFormat != "cline" || target.HooksSettingsPath == "" {
		return nil
	}
	srcDir := filepath.Join(baseDir, "cline-plugin")
	installDir := clinePluginInstallDir(target.HooksSettingsPath, baseDir)
	if err := os.MkdirAll(filepath.Join(installDir, "package"), 0o755); err != nil {
		return err
	}

	for _, name := range clinePluginFiles {
		data, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			return fmt.Errorf("plugin Cline: ler %s: %w", name, err)
		}
		if err := writeFileIfChanged(filepath.Join(installDir, "package", name), data, 0o644); err != nil {
			return err
		}
	}

	configJSON, err := json.Marshal(map[string]string{"baseDir": baseDir, "bin": clinePluginBinPath()})
	if err != nil {
		return err
	}
	if err := writeFileIfChanged(filepath.Join(installDir, "package", "agent-sync-config.json"), configJSON, 0o644); err != nil {
		return err
	}

	aggregator, err := json.MarshalIndent(map[string]any{
		"name":    clinePluginName,
		"private": true,
		"cline": map[string]any{
			"plugins": []map[string]any{{"paths": []string{"./package/index.js"}}},
		},
	}, "", "  ")
	if err != nil {
		return err
	}
	aggregator = append(aggregator, '\n')
	if err := writeFileIfChanged(filepath.Join(installDir, "package.json"), aggregator, 0o644); err != nil {
		return err
	}

	for _, legacy := range clineLegacyHookArtifacts {
		if err := os.Remove(filepath.Join(target.HooksSettingsPath, legacy)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// clinePluginBinPath resolve o caminho do binário agent-sync para o plugin
// chamar. O PATH do processo do Cline pode não incluir ~/.local/bin, então
// preferimos o caminho absoluto do executável que está rodando o apply
// (fallback: "agent-sync", resolvido via PATH).
func clinePluginBinPath() string {
	if exe, err := os.Executable(); err == nil && exe != "" {
		return exe
	}
	return "agent-sync"
}

// clinePluginInstallDir espelha o layout do `cline plugin install`:
// <configDir>/plugins/_installed/local/<nome>-<hash12>/package.
// O hash (sha256 do baseDir, 12 hex) é determinístico para o apply ser idempotente.
func clinePluginInstallDir(hooksDir, baseDir string) string {
	configDir := filepath.Dir(hooksDir)
	sum := sha256.Sum256([]byte(baseDir))
	suffix := hex.EncodeToString(sum[:])[:12]
	return filepath.Join(configDir, "plugins", "_installed", "local", clinePluginName+"-"+suffix)
}

// clineHooksDetail é a linha de sucesso do apply (o path completo depende do
// baseDir, que chega em runtime).
func clineHooksDetail(t TargetCLI) string {
	if t.HooksSettingsPath == "" {
		return ""
	}
	return "plugin em " + filepath.Join(filepath.Dir(t.HooksSettingsPath), "plugins", "_installed", "local", clinePluginName+"-<hash>") + "/package"
}

// writeFileIfChanged grava o arquivo com o modo dado apenas quando o conteúdo
// difere (o apply roda com frequência; preservar mtime evita churn).
func writeFileIfChanged(path string, content []byte, mode os.FileMode) error {
	if existing, err := os.ReadFile(path); err == nil && string(existing) == string(content) {
		if info, statErr := os.Stat(path); statErr == nil && info.Mode().Perm() == mode.Perm() {
			return nil
		}
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}
