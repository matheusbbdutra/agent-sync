package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const contextGuardHookName = "agent-sync-context-guard"

// hookEntry é o formato comum a Claude Code e Gemini CLI para um item de hooks.<Evento>[].
type hookEntry struct {
	Matcher string    `json:"matcher"`
	Hooks   []hookCmd `json:"hooks"`
}

type hookCmd struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Name    string `json:"name,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
}

// syncHooks injeta (ou atualiza) o hook de lembrete do context-guard no
// arquivo de configuração da CLI alvo, preservando o que já existir.
// Só age quando o target define HooksSettingsPath e HooksEvent.
func syncHooks(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" {
		return syncAntigravityHook(baseDir, target)
	}
	return syncStandardHook(baseDir, target)
}

// syncStandardHook cobre o formato compartilhado por Claude Code e Codex:
// {"hooks": {"<Evento>": [{"matcher", "hooks": [...]}]}}.
func syncStandardHook(baseDir string, target TargetCLI) error {
	scriptPath := filepath.Join(baseDir, "hooks", "context-guard-nudge.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("script do hook não encontrado: %s", scriptPath)
	}

	settings, err := readJSONObject(target.HooksSettingsPath)
	if err != nil {
		return err
	}

	hooksRoot, _ := settings["hooks"].(map[string]interface{})
	if hooksRoot == nil {
		hooksRoot = map[string]interface{}{}
	}

	entries := decodeHookEntries(hooksRoot[target.HooksEvent])
	hooksRoot[target.HooksEvent] = upsertContextGuardEntry(entries, scriptPath)
	settings["hooks"] = hooksRoot

	return writeJSONObject(target.HooksSettingsPath, settings)
}

// syncAntigravityHook cobre o formato próprio do Antigravity CLI, sem chave
// "hooks" de topo: {"<nome-do-hook>": {"<Evento>": [{"matcher", "hooks": [...]}]}}.
func syncAntigravityHook(baseDir string, target TargetCLI) error {
	scriptPath := filepath.Join(baseDir, "hooks", "context-guard-nudge.antigravity.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("script do hook não encontrado: %s", scriptPath)
	}

	root, err := readJSONObject(target.HooksSettingsPath)
	if err != nil {
		return err
	}

	hookGroup, _ := root[contextGuardHookName].(map[string]interface{})
	if hookGroup == nil {
		hookGroup = map[string]interface{}{}
	}

	entries := decodeHookEntries(hookGroup[target.HooksEvent])
	hookGroup[target.HooksEvent] = upsertContextGuardEntry(entries, scriptPath)
	root[contextGuardHookName] = hookGroup

	return writeJSONObject(target.HooksSettingsPath, root)
}

// upsertContextGuardEntry remove uma entrada anterior do agent-sync (se existir)
// e adiciona a versão atual, mantendo entradas de outras origens intactas.
func upsertContextGuardEntry(entries []hookEntry, scriptPath string) []hookEntry {
	filtered := entries[:0:0]
	for _, e := range entries {
		if !hasContextGuardHook(e) {
			filtered = append(filtered, e)
		}
	}
	filtered = append(filtered, hookEntry{
		Matcher: "*",
		Hooks: []hookCmd{
			{
				Type:    "command",
				Command: scriptPath,
				Name:    contextGuardHookName,
				Timeout: 10,
			},
		},
	})
	return filtered
}

func hasContextGuardHook(e hookEntry) bool {
	for _, h := range e.Hooks {
		if h.Name == contextGuardHookName {
			return true
		}
	}
	return false
}

func decodeHookEntries(raw interface{}) []hookEntry {
	if raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var entries []hookEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil
	}
	return entries
}

func readJSONObject(path string) (map[string]interface{}, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]interface{}{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]interface{}{}, nil
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("%s: JSON inválido, corrija manualmente antes de sincronizar: %w", path, err)
	}
	return obj, nil
}

// syncOpenCodePlugin instala o plugin best-effort de lembrete do context-guard
// para o OpenCode (não é config declarativa: precisa de um plugin TS real).
func syncOpenCodePlugin(baseDir string, target TargetCLI) error {
	if target.OpenCodePluginDir == "" {
		return nil
	}
	src := filepath.Join(baseDir, "hooks", "context-guard-nudge.opencode.ts")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("plugin do hook não encontrado: %s", src)
	}
	return copyFile(src, filepath.Join(target.OpenCodePluginDir, "context-guard-nudge.ts"))
}

func writeJSONObject(path string, obj map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(path, encoded, 0o644)
}
