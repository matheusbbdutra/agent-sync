package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const contextGuardHookName = "agent-sync-context-guard"
const docsCacheHookName = "agent-sync-docs-cache"
const memoryNudgeHookName = "agent-sync-memory-nudge"
const agentReactNudgeHookName = "agent-sync-agent-react-nudge"
const ctxCompactHookName = "agent-sync-ctx-compact"

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
	scriptName := "context-guard-nudge.sh"
	if target.HooksFormat == "antigravity" {
		scriptName = "context-guard-nudge.antigravity.sh"
		return syncAntigravityHook(baseDir, target, contextGuardHookName, scriptName, "*")
	}
	return syncStandardHook(baseDir, target, contextGuardHookName, scriptName, "*")
}

// syncMemoryNudgeHook instala o hook que lembra periodicamente de checar/
// gravar memoria via memory-mcp (store_memory), ja que hoje isso depende
// so da disciplina do modelo seguindo o CLAUDE.md. Suportado onde já existe
// HooksSettingsPath/HooksEvent (Claude Code, Codex e Antigravity).
func syncMemoryNudgeHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" {
		return syncAntigravityHook(baseDir, target, memoryNudgeHookName, "memory-nudge.antigravity.sh", "*")
	}
	return syncStandardHook(baseDir, target, memoryNudgeHookName, "memory-nudge.sh", "*")
}

// syncAgentReactNudgeHook instala o lembrete de validacao de hipoteses
// (skill agent-react): hipotese != fato; validar ou pedir passo ao usuario.
func syncAgentReactNudgeHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" {
		return syncAntigravityHook(baseDir, target, agentReactNudgeHookName, "agent-react-nudge.antigravity.sh", "*")
	}
	return syncStandardHook(baseDir, target, agentReactNudgeHookName, "agent-react-nudge.sh", "*")
}

// syncCtxCompactHook instala o hook que registra tool calls no working memory
// do ctx-window e dispara auto-compactacao quando o budget estimado e atingido.
// Cobre Claude, Codex, Antigravity e Cursor (formatos suportados pelo agent-sync).
// OpenCode fica best-effort via plugin TS (syncOpenCodeCtxCompactPlugin).
func syncCtxCompactHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" {
		return syncAntigravityHook(baseDir, target, ctxCompactHookName, "ctx-compact.antigravity.sh", "*")
	}
	return syncStandardHook(baseDir, target, ctxCompactHookName, "ctx-compact.sh", "*")
}

// syncOpenCodeCtxCompactPlugin instala o plugin TS best-effort para OpenCode
// (mesma limitacao documentada em syncOpenCodePlugin — output do hook nem
// sempre chega ao modelo ate a issue upstream #13574 fechar).
func syncOpenCodeCtxCompactPlugin(baseDir string, target TargetCLI) error {
	if target.OpenCodePluginDir == "" {
		return nil
	}
	src := filepath.Join(baseDir, "hooks", "ctx-compact.opencode.ts")
	if _, err := os.Stat(src); err != nil {
		// plugin opcional — silencioso se nao existir
		return nil
	}
	return copyFile(src, filepath.Join(target.OpenCodePluginDir, "ctx-compact.ts"))
}

// syncDocsCacheHook instala o hook que cacheia passivamente docs consultadas
// via WebFetch/read_url_content e context7 (query-docs). Claude Code e Codex
// compartilham o mesmo script (schema de PostToolUse equivalente); Antigravity
// usa o seu próprio (lê o resultado do transcriptPath).
func syncDocsCacheHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" {
		return syncAntigravityHook(baseDir, target, docsCacheHookName, "docs-cache.antigravity.sh", "read_url_content|call_mcp_tool")
	}
	return syncStandardHook(baseDir, target, docsCacheHookName, "docs-cache.sh", "WebFetch|mcp__context7__.*")
}

// syncStandardHook cobre o formato compartilhado por Claude Code e Codex:
// {"hooks": {"<Evento>": [{"matcher", "hooks": [...]}]}}.
func syncStandardHook(baseDir string, target TargetCLI, hookName, scriptName, matcher string) error {
	scriptPath := filepath.Join(baseDir, "hooks", scriptName)
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
	if target.AgentKind == "codex" {
		adapterPath := filepath.Join(baseDir, "hooks", "codex-protect-mcp-adapter.sh")
		entries = adaptCodexProtectionHooks(entries, adapterPath)
		if target.HooksEvent != "PreToolUse" {
			hooksRoot["PreToolUse"] = adaptCodexProtectionHooks(decodeHookEntries(hooksRoot["PreToolUse"]), adapterPath)
		}
	}
	hooksRoot[target.HooksEvent] = upsertHookEntry(entries, scriptPath, hookName, matcher)
	settings["hooks"] = hooksRoot

	return writeJSONObject(target.HooksSettingsPath, settings)
}

func adaptCodexProtectionHooks(entries []hookEntry, adapterPath string) []hookEntry {
	for i := range entries {
		for j := range entries[i].Hooks {
			command := entries[i].Hooks[j].Command
			if strings.Contains(command, "npx protect-mcp@0.7.4 evaluate") {
				entries[i].Hooks[j].Command = strings.Replace(command, "npx protect-mcp@0.7.4 evaluate", adapterPath+" evaluate", 1)
			}
			if strings.Contains(command, "npx protect-mcp@0.7.4 sign") {
				entries[i].Hooks[j].Command = strings.Replace(command, "npx protect-mcp@0.7.4 sign", adapterPath+" sign", 1)
			}
		}
	}
	return entries
}

// syncAntigravityHook cobre o formato próprio do Antigravity CLI, sem chave
// "hooks" de topo: {"<nome-do-hook>": {"<Evento>": [{"matcher", "hooks": [...]}]}}.
func syncAntigravityHook(baseDir string, target TargetCLI, hookName, scriptName, matcher string) error {
	scriptPath := filepath.Join(baseDir, "hooks", scriptName)
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("script do hook não encontrado: %s", scriptPath)
	}

	root, err := readJSONObject(target.HooksSettingsPath)
	if err != nil {
		return err
	}

	hookGroup, _ := root[hookName].(map[string]interface{})
	if hookGroup == nil {
		hookGroup = map[string]interface{}{}
	}

	entries := decodeHookEntries(hookGroup[target.HooksEvent])
	hookGroup[target.HooksEvent] = upsertHookEntry(entries, scriptPath, hookName, matcher)
	root[hookName] = hookGroup

	return writeJSONObject(target.HooksSettingsPath, root)
}

// upsertHookEntry remove uma entrada anterior do hook nomeado (se existir) e
// adiciona a versão atual, mantendo entradas de outras origens intactas.
func upsertHookEntry(entries []hookEntry, scriptPath, hookName, matcher string) []hookEntry {
	filtered := entries[:0:0]
	for _, e := range entries {
		if !hasNamedHook(e, hookName) {
			filtered = append(filtered, e)
		}
	}
	filtered = append(filtered, hookEntry{
		Matcher: matcher,
		Hooks: []hookCmd{
			{
				Type:    "command",
				Command: scriptPath,
				Name:    hookName,
				Timeout: 10,
			},
		},
	})
	return filtered
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

// syncOpenCodeMemoryNudgePlugin instala o plugin best-effort de lembrete de
// memory-mcp (store_memory) para o OpenCode.
func syncOpenCodeMemoryNudgePlugin(baseDir string, target TargetCLI) error {
	if target.OpenCodePluginDir == "" {
		return nil
	}
	src := filepath.Join(baseDir, "hooks", "memory-nudge.opencode.ts")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("plugin do hook não encontrado: %s", src)
	}
	return copyFile(src, filepath.Join(target.OpenCodePluginDir, "memory-nudge.ts"))
}

// syncOpenCodeAgentReactNudgePlugin instala o plugin best-effort de lembrete
// de validacao de hipoteses (agent-react) para o OpenCode.
func syncOpenCodeAgentReactNudgePlugin(baseDir string, target TargetCLI) error {
	if target.OpenCodePluginDir == "" {
		return nil
	}
	src := filepath.Join(baseDir, "hooks", "agent-react-nudge.opencode.ts")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("plugin do hook não encontrado: %s", src)
	}
	return copyFile(src, filepath.Join(target.OpenCodePluginDir, "agent-react-nudge.ts"))
}

// syncOpenCodeDocsCachePlugin instala o plugin best-effort que cacheia
// passivamente docs consultadas via webfetch/context7 no OpenCode.
func syncOpenCodeDocsCachePlugin(baseDir string, target TargetCLI) error {
	if target.OpenCodePluginDir == "" {
		return nil
	}
	src := filepath.Join(baseDir, "hooks", "docs-cache.opencode.ts")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("plugin do docs-cache não encontrado: %s", src)
	}
	return copyFile(src, filepath.Join(target.OpenCodePluginDir, "docs-cache.ts"))
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
