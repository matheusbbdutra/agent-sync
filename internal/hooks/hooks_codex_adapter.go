package hooks

// hooks_codex_adapter.go: codex-protect-mcp adapter + opencode plugin suffix.
// Migrado de hooks.go (Fase 2.3 continuacao).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// opencodePluginSuffix centraliza o sufixo por versão para facilitar teste.
// v2 retorna o padrão legado ".v2.ts" para retrocompatibilidade; o padrão
// novo ".opencode.v2.ts" é tentado dentro de syncOpenCodePluginVersioned.
func opencodePluginSuffix(major int) string {
	if major >= 2 {
		return ".v2.ts"
	}
	return ".opencode.ts"
}

// adaptCodexPlugins percorre ~/.codex/plugins/*/hooks/hooks.json e adapta
// cada plugin que referenciar npx protect-mcp@0.7.4 evaluate/sign para
// passar pelo codex-protect-mcp-adapter.sh. Idempotente: plugins adaptados
// são pulados silenciosamente e o backup .bak só é criado na primeira vez.
//
// Se ~/.codex/plugins não existir (Codex não instalado), retorna nil.
//
// Se ~/.codex/config.toml contiver trusted_hash para algum plugin
// modificado, emite aviso em stderr mas prossegue com a adaptação
// (interpretação conservadora — Codex pode recusar o hook até o hash
// ser regenerado, mas isso é responsabilidade do usuário).
var adaptCodexPlugins = AdaptCodexPlugins

func AdaptCodexPlugins(baseDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("encontrar HOME para plugins Codex: %w", err)
	}
	pluginsDir := filepath.Join(home, ".codex", "plugins")

	dirEntries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("listar %s: %w", pluginsDir, err)
	}

	adapterPath, err := pathutil.HookScriptPath(baseDir, "codex-protect-mcp-adapter.sh")
	if err != nil {
		return err
	}
	configTOML := filepath.Join(home, ".codex", "config.toml")
	trustedHashes := loadCodexTrustedHashes(configTOML)

	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			continue
		}
		hooksPath := filepath.Join(pluginsDir, dirEntry.Name(), "hooks", "hooks.json")
		if _, statErr := os.Stat(hooksPath); statErr != nil {
			continue
		}

		data, readErr := os.ReadFile(hooksPath)
		if readErr != nil {
			return fmt.Errorf("ler %s: %w", hooksPath, readErr)
		}

		originalSnapshot := snapshotCodexPluginCommands(data)
		if originalSnapshot == nil {
			continue
		}
		if codexPluginAlreadyAdapted(originalSnapshot, adapterPath) {
			continue
		}
		if !codexPluginNeedsAdaptation(originalSnapshot) {
			continue
		}

		var root map[string]interface{}
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("JSON inválido em %s: %w", hooksPath, err)
		}

		hooksRoot, _ := root["hooks"].(map[string]interface{})
		if hooksRoot == nil {
			continue
		}

		for event, raw := range hooksRoot {
			eventEntries := decodeHookEntries(raw)
			adaptCodexProtectionHooks(eventEntries, adapterPath)
			hooksRoot[event] = eventEntries
		}
		root["hooks"] = hooksRoot

		encoded, err := json.MarshalIndent(root, "", "  ")
		if err != nil {
			return fmt.Errorf("serializar %s: %w", hooksPath, err)
		}
		encoded = append(encoded, '\n')

		bakPath := hooksPath + ".bak"
		if _, bakErr := os.Stat(bakPath); os.IsNotExist(bakErr) {
			if err := os.WriteFile(bakPath, data, 0o644); err != nil {
				return fmt.Errorf("criar backup %s: %w", bakPath, err)
			}
		}
		if err := os.WriteFile(hooksPath, encoded, 0o644); err != nil {
			return fmt.Errorf("escrever %s: %w", hooksPath, err)
		}

		if trustedHashes[dirEntry.Name()] {
			fmt.Fprintf(os.Stderr, "⚠️  Plugin %s: trusted_hash presente em config.toml; Codex pode recusar o hook até regenerar hash\n", dirEntry.Name())
		}
	}
	return nil
}

// snapshotCodexPluginCommands devolve cópias (por valor) das entradas de
// cada evento, ou nil se o JSON for inválido / não tiver chave "hooks".
// Usado para detectar "plugin já adaptado" sem mutar o original.
func snapshotCodexPluginCommands(data []byte) map[string][]hookEntry {
	var root struct {
		Hooks map[string][]hookEntry `json:"hooks"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	if root.Hooks == nil {
		return nil
	}
	out := make(map[string][]hookEntry, len(root.Hooks))
	for event, entries := range root.Hooks {
		copied := make([]hookEntry, len(entries))
		for i, e := range entries {
			copied[i] = e
			copied[i].Hooks = make([]hookCmd, len(e.Hooks))
			copy(copied[i].Hooks, e.Hooks)
		}
		out[event] = copied
	}
	return out
}

func codexPluginAlreadyAdapted(snapshot map[string][]hookEntry, adapterPath string) bool {
	for _, entries := range snapshot {
		for _, e := range entries {
			for _, h := range e.Hooks {
				if strings.Contains(h.Command, adapterPath) {
					return true
				}
			}
		}
	}
	return false
}

func codexPluginNeedsAdaptation(snapshot map[string][]hookEntry) bool {
	for _, entries := range snapshot {
		for _, e := range entries {
			for _, h := range e.Hooks {
				if strings.Contains(h.Command, "npx protect-mcp@0.7.4 evaluate") ||
					strings.Contains(h.Command, "npx protect-mcp@0.7.4 sign") {
					return true
				}
			}
		}
	}
	return false
}

// loadCodexTrustedHashes varre ~/.codex/config.toml procurando entradas
// trusted_hash da forma:
//
//	[hooks.state."<plugin-name>@<marketplace>:hooks/hooks.json:<event>:<i>:<j>"]
//	trusted_hash = "sha256:..."
//
// Retorna um map pluginName -> true quando o config.toml declara trusted_hash
// para aquele plugin. Conservador: parsing por regex porque não há
// dependência TOML no go.mod e o objetivo é apenas detectar presença.
func loadCodexTrustedHashes(path string) map[string]bool {
	out := map[string]bool{}
	raw, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	stateHeader := regexp.MustCompile(`(?m)^\[hooks\.state\."([^"]+)"\]\s*$`)
	hashLine := regexp.MustCompile(`(?m)^trusted_hash\s*=\s*"sha256:`)
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		match := stateHeader.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		key := match[1]
		for j := i + 1; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "[") {
				break
			}
			if hashLine.MatchString(lines[j]) {
				plugin := extractCodexPluginFromStateKey(key)
				if plugin != "" {
					out[plugin] = true
				}
				break
			}
		}
	}
	return out
}

func extractCodexPluginFromStateKey(key string) string {
	at := strings.Index(key, "@")
	if at <= 0 {
		return ""
	}
	return key[:at]
}
