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
//
// A-84 (Pendência 4): como o nome do diretório deriva do baseDir, applies de
// baseDirs diferentes criavam plugins paralelos (o CLI carrega todos, dobrando
// hooks). Agora só o repo canônico é adotado e os órfãos são podados — ver
// canonical_repo.go.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
// A partir do A-87, o plugin é TypeScript: a fonte é index.ts (+ types.ts +
// tsconfig.json + package.json + plugin.json) e o wiramento consome
// `dist/index.js` produzido por `npx tsc -p cline-plugin/`.
var clinePluginFiles = []string{
	"index.ts",
	"types.ts",
	"tsconfig.json",
	"package.json",
	"plugin.json",
	"dist/index.js",
	"dist/types.js",
}

// syncClineHooks instala (ou atualiza) o Cline Plugin. Para o Cline,
// target.HooksSettingsPath aponta para ~/.cline/hooks (config dir do CLI).
//
// Dois cuidados que evitam hooks duplicados (ver canonical_repo.go):
//   - o plugin só é instalado quando o baseDir é o repo canônico (ou quando o
//     escape hatch AGENT_SYNC_ALLOW_BASEDIR=1 está ligado);
//   - plugins agent-sync-hooks-* de outros baseDirs são podados, porque o CLI
//     carrega todos eles.
func syncClineHooks(baseDir string, target TargetCLI) error {
	if target.HooksFormat != "cline" || target.HooksSettingsPath == "" {
		return nil
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("plugin Cline: resolver baseDir %q: %w", baseDir, err)
	}

	adoption, err := adoptCanonicalBaseDir(absBase)
	if err != nil {
		return err
	}
	if !adoption.Adopted {
		return fmt.Errorf("plugin Cline não instalado nem podado (%s)", adoption.Note)
	}
	if adoption.Note != "" {
		prefix := "ℹ️ "
		if adoption.Caveat {
			prefix = "⚠️ "
		}
		fmt.Fprintf(os.Stderr, "%s [cline/cline-bridge] %s\n", prefix, adoption.Note)
	}

	srcDir := filepath.Join(absBase, "cline-plugin")
	installDir := clinePluginInstallDir(target.HooksSettingsPath, absBase)
	if err := os.MkdirAll(filepath.Join(installDir, "package"), 0o755); err != nil {
		return err
	}
	// dist/ é artefato de build do TS (A-87) — criamos o diretório antes do
	// loop para que o writeFileIfChanged de `dist/index.js`/`dist/types.js`
	// não falhe em "no such file or directory".
	if err := os.MkdirAll(filepath.Join(installDir, "package", "dist"), 0o755); err != nil {
		return err
	}

	for _, name := range clinePluginFiles {
		data, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			if os.IsNotExist(err) && strings.HasPrefix(name, "dist/") {
				// dist/ é artefato de build (A-87). Em checkout fresh sem
				// `npx tsc` ainda não existe — wiramos o resto do plugin e
				// avisamos para o usuário rodar o build.
				fmt.Fprintf(os.Stderr, "⚠️  [cline/cline-bridge] %s ausente — rode `npx tsc -p cline-plugin/` antes de usar o plugin\n", name)
				continue
			}
			return fmt.Errorf("plugin Cline: ler %s: %w", name, err)
		}
		if err := writeFileIfChanged(filepath.Join(installDir, "package", name), data, 0o644); err != nil {
			return err
		}
	}

	configJSON, err := json.Marshal(map[string]string{"baseDir": absBase, "bin": clinePluginBinPath()})
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
			"plugins": []map[string]any{{"paths": []string{"./package/dist/index.js"}}},
		},
	}, "", "  ")
	if err != nil {
		return err
	}
	aggregator = append(aggregator, '\n')
	if err := writeFileIfChanged(filepath.Join(installDir, "package.json"), aggregator, 0o644); err != nil {
		return err
	}

	// Poda só no caminho canônico: no escape hatch o usuário pediu para
	// coexistir (ex.: testar um worktree), então não removemos o canônico.
	if !allowNonCanonicalBaseDir() {
		removed, err := pruneOrphanClinePluginDirs(target.HooksSettingsPath, installDir)
		if err != nil {
			return err
		}
		if len(removed) > 0 {
			fmt.Fprintf(os.Stderr, "ℹ️  [cline/cline-bridge] plugins órfãos removidos (evita hooks duplicados): %s\n",
				strings.Join(removed, ", "))
		}
	}

	for _, legacy := range clineLegacyHookArtifacts {
		if err := os.Remove(filepath.Join(target.HooksSettingsPath, legacy)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// pruneOrphanClinePluginDirs remove diretórios de plugin do agent-sync em
// _installed/local que não sejam keep, devolvendo os nomes removidos. O CLI do
// Cline carrega todos os diretórios de _installed/local (sem dedup por nome),
// então um plugin remanescente de outro baseDir faria cada hook rodar 2×.
// Só remove nomes que são inequivocamente nossos (nome exato ou nome-<hash12>).
func pruneOrphanClinePluginDirs(hooksDir, keep string) ([]string, error) {
	base := filepath.Join(filepath.Dir(hooksDir), "plugins", "_installed", "local")
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	keepClean := filepath.Clean(keep)
	var removed []string
	for _, entry := range entries {
		if !entry.IsDir() || !isOwnClinePluginDirName(entry.Name()) {
			continue
		}
		full := filepath.Join(base, entry.Name())
		if filepath.Clean(full) == keepClean {
			continue
		}
		if err := os.RemoveAll(full); err != nil {
			return removed, fmt.Errorf("plugin Cline: remover órfão %s: %w", entry.Name(), err)
		}
		removed = append(removed, entry.Name())
	}
	return removed, nil
}

// isOwnClinePluginDirName reconhece o diretório deste plugin: exatamente
// "agent-sync-hooks" ou "agent-sync-hooks-<12 hex>" (formato de clinePluginInstallDir).
func isOwnClinePluginDirName(name string) bool {
	if name == clinePluginName {
		return true
	}
	suffix, ok := strings.CutPrefix(name, clinePluginName+"-")
	if !ok || len(suffix) != 12 {
		return false
	}
	for _, r := range suffix {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
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
