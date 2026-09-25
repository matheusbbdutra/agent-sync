package hooks

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

const codexProtectMCPPluginFixture = `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "npx protect-mcp@0.7.4 evaluate --policy policy.cedar"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "npx protect-mcp@0.7.4 sign --tool \"$TOOL_NAME\""
          }
        ]
      }
    ]
  }
}`

const codexPluginMixedFixture = `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "npx protect-mcp@0.7.4 evaluate --policy policy.cedar"
          },
          {
            "type": "command",
            "command": "echo do-not-touch"
          },
          {
            "type": "command",
            "command": "npx protect-mcp@0.7.4 sign --tool \"$TOOL_NAME\""
          }
        ]
      }
    ]
  }
}`

const codexPluginAlreadyAdaptedFixture = `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "__ADAPTER__ evaluate --policy policy.cedar"
          }
        ]
      }
    ]
  }
}`

func writeCodexTestFixture(t *testing.T, home, pluginName, body string) {
	t.Helper()
	hooksDir := filepath.Join(home, ".codex", "plugins", pluginName, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("criar dir %s: %v", hooksDir, err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "hooks.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("escrever fixture %s: %v", pluginName, err)
	}
}

// createCodexAdapterFixture materializa hooks/codex-protect-mcp-adapter.sh em
// baseDir (necessario apos HookScriptPath validar a existencia do script).
func createCodexAdapterFixture(t *testing.T, baseDir string) {
	t.Helper()
	hooksDir := filepath.Join(baseDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("criar dir %s: %v", hooksDir, err)
	}
	adapter := filepath.Join(hooksDir, "codex-protect-mcp-adapter.sh")
	if err := os.WriteFile(adapter, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("escrever adapter fixture %s: %v", adapter, err)
	}
}

func redirectHome(t *testing.T) string {
	t.Helper()
	if _, err := os.UserHomeDir(); err != nil {
		t.Fatalf("ler HOME atual: %v", err)
	}
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	return tempHome
}

func readCodexPluginHooksJSON(t *testing.T, home, pluginName string) map[string]interface{} {
	t.Helper()
	path := filepath.Join(home, ".codex", "plugins", pluginName, "hooks", "hooks.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler %s: %v", path, err)
	}
	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("JSON inválido em %s: %v", path, err)
	}
	return root
}

func codexPluginCommandList(t *testing.T, root map[string]interface{}, event string) []string {
	t.Helper()
	hooks, _ := root["hooks"].(map[string]interface{})
	if hooks == nil {
		t.Fatalf("chave hooks ausente em root: %v", root)
	}
	raw, ok := hooks[event]
	if !ok {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		t.Fatalf("evento %s não é array: %v", event, raw)
	}
	cmds := []string{}
	for _, e := range arr {
		entry, _ := e.(map[string]interface{})
		innerHooks, _ := entry["hooks"].([]interface{})
		for _, h := range innerHooks {
			cmdMap, _ := h.(map[string]interface{})
			if c, ok := cmdMap["command"].(string); ok {
				cmds = append(cmds, c)
			}
		}
	}
	return cmds
}

func TestAdaptCodexPluginsHappyPath(t *testing.T) {
	tempHome := redirectHome(t)
	tempBase := t.TempDir()
	createCodexAdapterFixture(t, tempBase)
	adapterPath := filepath.Join(tempBase, "hooks", "codex-protect-mcp-adapter.sh")
	writeCodexTestFixture(t, tempHome, "protect-mcp", codexProtectMCPPluginFixture)
	writeCodexTestFixture(t, tempHome, "already-adapted", strings.Replace(codexPluginAlreadyAdaptedFixture, "__ADAPTER__", adapterPath, 1))

	if err := adaptCodexPlugins(tempBase); err != nil {
		t.Fatalf("adaptCodexPlugins falhou: %v", err)
	}

	adaptedRoot := readCodexPluginHooksJSON(t, tempHome, "protect-mcp")
	preCmds := codexPluginCommandList(t, adaptedRoot, "PreToolUse")
	postCmds := codexPluginCommandList(t, adaptedRoot, "PostToolUse")
	if len(preCmds) != 1 || !strings.Contains(preCmds[0], adapterPath+" evaluate") {
		t.Errorf("PreToolUse não adaptado em protect-mcp: %v", preCmds)
	}
	if !strings.Contains(preCmds[0], "policy.cedar") {
		t.Errorf("policy.cedar perdida após adaptação: %q", preCmds[0])
	}
	if len(postCmds) != 1 || !strings.Contains(postCmds[0], adapterPath+" sign") {
		t.Errorf("PostToolUse não adaptado em protect-mcp: %v", postCmds)
	}

	bakPath := filepath.Join(tempHome, ".codex", "plugins", "protect-mcp", "hooks", "hooks.json.bak")
	if _, err := os.Stat(bakPath); err != nil {
		t.Errorf("backup .bak não criado em protect-mcp: %v", err)
	}

	alreadyRoot := readCodexPluginHooksJSON(t, tempHome, "already-adapted")
	alreadyCmds := codexPluginCommandList(t, alreadyRoot, "PreToolUse")
	if len(alreadyCmds) != 1 || !strings.Contains(alreadyCmds[0], adapterPath+" evaluate") {
		t.Errorf("already-adapted foi modificado: %v", alreadyCmds)
	}
	alreadyBak := filepath.Join(tempHome, ".codex", "plugins", "already-adapted", "hooks", "hooks.json.bak")
	if _, err := os.Stat(alreadyBak); !os.IsNotExist(err) {
		t.Errorf("already-adapted não deveria ter backup .bak: err=%v", err)
	}
}

func TestAdaptCodexPluginsIdempotent(t *testing.T) {
	tempHome := redirectHome(t)
	tempBase := t.TempDir()
	createCodexAdapterFixture(t, tempBase)
	writeCodexTestFixture(t, tempHome, "protect-mcp", codexProtectMCPPluginFixture)

	if err := adaptCodexPlugins(tempBase); err != nil {
		t.Fatalf("primeira adaptação falhou: %v", err)
	}
	firstRoot := readCodexPluginHooksJSON(t, tempHome, "protect-mcp")
	firstData, err := os.ReadFile(filepath.Join(tempHome, ".codex", "plugins", "protect-mcp", "hooks", "hooks.json"))
	if err != nil {
		t.Fatalf("ler hooks.json após primeira run: %v", err)
	}

	if err := adaptCodexPlugins(tempBase); err != nil {
		t.Fatalf("segunda adaptação falhou: %v", err)
	}
	secondRoot := readCodexPluginHooksJSON(t, tempHome, "protect-mcp")
	secondData, err := os.ReadFile(filepath.Join(tempHome, ".codex", "plugins", "protect-mcp", "hooks", "hooks.json"))
	if err != nil {
		t.Fatalf("ler hooks.json após segunda run: %v", err)
	}

	if string(firstData) != string(secondData) {
		t.Errorf("conteúdo divergiu entre runs:\nprimeira=%s\nsegunda=%s", firstData, secondData)
	}
	firstJSON, _ := json.Marshal(firstRoot)
	secondJSON, _ := json.Marshal(secondRoot)
	if string(firstJSON) != string(secondJSON) {
		t.Errorf("estrutura divergiu entre runs: %s vs %s", firstJSON, secondJSON)
	}

	matches, err := filepath.Glob(filepath.Join(tempHome, ".codex", "plugins", "protect-mcp", "hooks", "hooks.json*.bak"))
	if err != nil {
		t.Fatalf("glob backup: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("esperado exatamente 1 backup .bak, obteve %d: %v", len(matches), matches)
	}
}

func TestAdaptCodexPluginsNoPluginsDir(t *testing.T) {
	tempHome := redirectHome(t)
	tempBase := t.TempDir()

	if err := adaptCodexPlugins(tempBase); err != nil {
		t.Fatalf("esperado nil sem diretório ~/.codex/plugins, obteve: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempHome, ".codex")); !os.IsNotExist(err) {
		t.Errorf("nenhum diretório ~/.codex deveria ter sido criado: err=%v", err)
	}
}

func TestAdaptCodexPluginsPreservesOtherCommands(t *testing.T) {
	tempHome := redirectHome(t)
	tempBase := t.TempDir()
	createCodexAdapterFixture(t, tempBase)
	writeCodexTestFixture(t, tempHome, "mixed", codexPluginMixedFixture)
	adapterPath := filepath.Join(tempBase, "hooks", "codex-protect-mcp-adapter.sh")

	if err := adaptCodexPlugins(tempBase); err != nil {
		t.Fatalf("adaptCodexPlugins falhou: %v", err)
	}

	root := readCodexPluginHooksJSON(t, tempHome, "mixed")
	cmds := codexPluginCommandList(t, root, "PreToolUse")
	if len(cmds) != 3 {
		t.Fatalf("esperado 3 commands preservados, obteve %d: %v", len(cmds), cmds)
	}
	if !strings.Contains(cmds[0], adapterPath+" evaluate") {
		t.Errorf("commands[0] (evaluate) não adaptado: %q", cmds[0])
	}
	if !strings.Contains(cmds[0], "policy.cedar") {
		t.Errorf("commands[0] perdeu policy.cedar: %q", cmds[0])
	}
	if cmds[1] != "echo do-not-touch" {
		t.Errorf("commands[1] (echo) foi modificado indevidamente: %q", cmds[1])
	}
	if !strings.Contains(cmds[2], adapterPath+" sign") {
		t.Errorf("commands[2] (sign) não adaptado: %q", cmds[2])
	}
}

func TestAdaptCodexPluginsNoPluginsDirButConfigTomlMissing(t *testing.T) {
	tempHome := redirectHome(t)
	tempBase := t.TempDir()
	createCodexAdapterFixture(t, tempBase)

	if err := os.MkdirAll(filepath.Join(tempHome, ".codex", "plugins", "protect-mcp", "hooks"), 0o755); err != nil {
		t.Fatalf("setup dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempHome, ".codex", "plugins", "protect-mcp", "hooks", "hooks.json"), []byte(codexProtectMCPPluginFixture), 0o644); err != nil {
		t.Fatalf("escrever fixture: %v", err)
	}

	if err := adaptCodexPlugins(tempBase); err != nil {
		t.Fatalf("adaptCodexPlugins falhou sem config.toml: %v", err)
	}
}

func TestAdaptCodexProtectionHooks(t *testing.T) {
	entries := []hookEntry{{Hooks: []hookCmd{
		{Command: "npx protect-mcp@0.7.4 evaluate --policy policy.cedar"},
		{Command: "if [ -f flag ]; then exit 0; fi; npx protect-mcp@0.7.4 sign --tool \"$TOOL_NAME\""},
	}}}

	adapted := adaptCodexProtectionHooks(entries, "/repo/hooks/codex-protect-mcp-adapter.sh")
	if adapted[0].Hooks[0].Command != "/repo/hooks/codex-protect-mcp-adapter.sh evaluate --policy policy.cedar" {
		t.Fatalf("evaluate não adaptado: %q", adapted[0].Hooks[0].Command)
	}
	if adapted[0].Hooks[1].Command != "if [ -f flag ]; then exit 0; fi; /repo/hooks/codex-protect-mcp-adapter.sh sign --tool \"$TOOL_NAME\"" {
		t.Fatalf("sign não adaptado: %q", adapted[0].Hooks[1].Command)
	}
}

func TestSyncPrinciplesInjectHookWiresPreToolUse(t *testing.T) {
	for _, kind := range []string{"claude", "codex"} {
		t.Run(kind, func(t *testing.T) {
			tempBase := t.TempDir()
			hooksDir := filepath.Join(tempBase, "hooks")
			if err := os.MkdirAll(hooksDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(hooksDir, "wrap-hook.sh"), []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			scriptPath := filepath.Join(hooksDir, "principles-inject.pretooluse.sh")
			if err := os.WriteFile(scriptPath, []byte("#!/usr/bin/env bash\necho '{}'\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			tempHooksPath := filepath.Join(tempBase, "settings.json")
			target := TargetCLI{
				Name:              kind,
				AgentKind:         kind,
				HooksSettingsPath: tempHooksPath,
				HooksFormat:       "",
			}
			if err := syncPrinciplesInjectHook(tempBase, target); err != nil {
				t.Fatalf("falha ao sincronizar principles-inject hook para %s: %v", kind, err)
			}
			data, err := os.ReadFile(tempHooksPath)
			if err != nil {
				t.Fatalf("arquivo settings não foi criado para %s", kind)
			}
			var root map[string]interface{}
			if err := json.Unmarshal(data, &root); err != nil {
				t.Fatalf("json inválido: %v", err)
			}
			hooks, _ := root["hooks"].(map[string]interface{})
			preToolUse, ok := hooks["PreToolUse"].([]interface{})
			if !ok || len(preToolUse) == 0 {
				t.Fatalf("PreToolUse hook não encontrado no json para %s: %s", kind, string(data))
			}
		})
	}
}

func TestSyncPrinciplesInjectHookSkipsNonSupported(t *testing.T) {
	for _, kind := range []string{"antigravity", "cursor", "opencode"} {
		t.Run(kind, func(t *testing.T) {
			tempBase := t.TempDir()
			tempHooksPath := filepath.Join(t.TempDir(), "settings.json")
			target := TargetCLI{
				Name:              kind,
				AgentKind:         kind,
				HooksSettingsPath: tempHooksPath,
				HooksFormat:       kind,
			}
			if err := syncPrinciplesInjectHook(tempBase, target); err != nil {
				t.Fatalf("hooks nao suportados devem ser no-op (sem erro): %v", err)
			}
			if _, err := os.Stat(tempHooksPath); err == nil {
				t.Fatalf("settings.json nao deveria ter sido criado para %s", kind)
			}
		})
	}
}

func TestSyncAntigravityStopHook(t *testing.T) {
	tempBase := t.TempDir()
	hooksDir := filepath.Join(tempBase, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(hooksDir, "agent-stop.antigravity.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	tempHooksPath := filepath.Join(t.TempDir(), "hooks.json")
	target := TargetCLI{
		Name:              "antigravity",
		AgentKind:         "antigravity",
		HooksSettingsPath: tempHooksPath,
		HooksFormat:       "antigravity",
		HooksEvent:        "PreInvocation",
	}

	// Idempotência: sincronizar mais de uma vez não deve duplicar
	for i := 0; i < 2; i++ {
		if err := syncStopHook(tempBase, target); err != nil {
			t.Fatalf("syncStopHook falhou: %v", err)
		}
	}

	root, err := readJSONObject(tempHooksPath)
	if err != nil {
		t.Fatalf("falha ao ler JSON: %v", err)
	}

	group, ok := root[agentStopHookName].(map[string]interface{})
	if !ok {
		t.Fatalf("grupo %q não encontrado em root: %v", agentStopHookName, root)
	}

	stopList, ok := group["Stop"].([]interface{})
	if !ok || len(stopList) != 1 {
		t.Fatalf("Stop deve ser um flat array com 1 elemento, obteve: %v", group["Stop"])
	}

	entry, ok := stopList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("elemento inválido em Stop: %v", stopList[0])
	}

	wrappedCmd := wrapHookCommand(tempBase, "Stop", "agent-stop.antigravity", scriptPath)
	if entry["command"] != wrappedCmd {
		t.Errorf("command incorreto: obteve %v, esperado %v", entry["command"], wrappedCmd)
	}
	if entry["type"] != "command" {
		t.Errorf("type incorreto: obteve %v, esperado 'command'", entry["type"])
	}
	if entry["timeout"] != float64(10) {
		t.Errorf("timeout incorreto: obteve %v, esperado 10", entry["timeout"])
	}
	if entry["name"] != agentStopHookName {
		t.Errorf("name incorreto: obteve %v, esperado %s", entry["name"], agentStopHookName)
	}
	if _, hasMatcher := entry["matcher"]; hasMatcher {
		t.Errorf("flat handler não deve conter matcher: %v", entry)
	}
}

func TestSyncAntigravityPreInvocationReminderHook(t *testing.T) {
	tempBase := t.TempDir()
	hooksDir := filepath.Join(tempBase, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(hooksDir, "agent-preinvocation.antigravity.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	tempHooksPath := filepath.Join(t.TempDir(), "hooks.json")
	target := TargetCLI{
		Name:              "antigravity",
		AgentKind:         "antigravity",
		HooksSettingsPath: tempHooksPath,
		HooksFormat:       "antigravity",
		HooksEvent:        "PreInvocation",
	}

	// Idempotência
	for i := 0; i < 2; i++ {
		if err := syncPreInvocationReminderHook(tempBase, target); err != nil {
			t.Fatalf("syncPreInvocationReminderHook falhou: %v", err)
		}
	}

	root, err := readJSONObject(tempHooksPath)
	if err != nil {
		t.Fatalf("falha ao ler JSON: %v", err)
	}

	group, ok := root[preInvocationReminderHookName].(map[string]interface{})
	if !ok {
		t.Fatalf("grupo %q não encontrado: %v", preInvocationReminderHookName, root)
	}

	preList, ok := group["PreInvocation"].([]interface{})
	if !ok || len(preList) != 1 {
		t.Fatalf("PreInvocation deve ser um flat array com 1 elemento: %v", group["PreInvocation"])
	}

	entry, ok := preList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("elemento inválido em PreInvocation: %v", preList[0])
	}

	wrappedCmd := wrapHookCommand(tempBase, "PreInvocation", "agent-preinvocation.antigravity", scriptPath)
	if entry["command"] != wrappedCmd {
		t.Errorf("command incorreto: obteve %v, esperado %v", entry["command"], wrappedCmd)
	}
	if entry["type"] != "command" {
		t.Errorf("type incorreto: obteve %v, esperado 'command'", entry["type"])
	}
	if entry["timeout"] != float64(10) {
		t.Errorf("timeout incorreto: obteve %v, esperado 10", entry["timeout"])
	}
	if entry["name"] != preInvocationReminderHookName {
		t.Errorf("name incorreto: obteve %v, esperado %s", entry["name"], preInvocationReminderHookName)
	}
	if _, hasMatcher := entry["matcher"]; hasMatcher {
		t.Errorf("flat handler não deve conter matcher: %v", entry)
	}
}

func TestSyncStopHookNonSupportedIsNoOp(t *testing.T) {
	tempHooksPath := filepath.Join(t.TempDir(), "settings.json")
	target := TargetCLI{
		Name:              "opencode",
		AgentKind:         "opencode",
		HooksSettingsPath: tempHooksPath,
		HooksFormat:       "",
		HooksEvent:        "PostToolUse",
	}
	if err := syncStopHook(t.TempDir(), target); err != nil {
		t.Fatalf("esperado nil para target não suportado, obteve: %v", err)
	}
	if _, err := os.Stat(tempHooksPath); !os.IsNotExist(err) {
		t.Fatalf("arquivo não deveria ter sido criado para target não suportado")
	}
}

func TestSyncStopHookClaudeAndCodex(t *testing.T) {
	for _, kind := range []string{"claude", "codex"} {
		t.Run(kind, func(t *testing.T) {
			tempHooksPath := filepath.Join(t.TempDir(), "settings.json")
			target := TargetCLI{
				Name:              kind,
				AgentKind:         kind,
				HooksSettingsPath: tempHooksPath,
				HooksFormat:       "",
				HooksEvent:        "PostToolUse",
			}
			if err := syncStopHook(t.TempDir(), target); err != nil {
				t.Fatalf("falha ao sincronizar stop hook para %s: %v", kind, err)
			}
			data, err := os.ReadFile(tempHooksPath)
			if err != nil {
				t.Fatalf("arquivo settings não foi criado para %s", kind)
			}
			var root map[string]interface{}
			if err := json.Unmarshal(data, &root); err != nil {
				t.Fatalf("json inválido: %v", err)
			}
			hooks, _ := root["hooks"].(map[string]interface{})
			if hooks == nil || hooks["Stop"] == nil {
				t.Fatalf("Stop hook não encontrado no json para %s: %s", kind, string(data))
			}
		})
	}
}

func findHookIfFilters(t *testing.T, settingsPath, hookName string) []string {
	return findHookIfFiltersAtEvent(t, settingsPath, hookName, "PostToolUse")
}

// findHookIfFiltersAtEvent: codex/claude wirar nudges em PreToolUse (2026-09-23);
// antigravity/cursor continuam em PostToolUse.
func findHookIfFiltersAtEvent(t *testing.T, settingsPath, hookName, event string) []string {
	t.Helper()
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ler %s: %v", settingsPath, err)
	}
	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("json inválido em %s: %v", settingsPath, err)
	}
	hooks, _ := root["hooks"].(map[string]interface{})
	if hooks == nil {
		return nil
	}
	entries, ok := hooks[event].([]interface{})
	if !ok {
		return nil
	}
	out := []string{}
	for _, e := range entries {
		entry, _ := e.(map[string]interface{})
		matcher, _ := entry["matcher"].(string)
		if matcher != "*" {
			continue
		}
		inner, _ := entry["hooks"].([]interface{})
		for _, h := range inner {
			cmd, _ := h.(map[string]interface{})
			if cmd["name"] != hookName {
				continue
			}
			if v, ok := cmd["if"].(string); ok {
				out = append(out, v)
			}
		}
	}
	return out
}

func TestNudgesEmitAllIfFiltersForClaudeAndCodex(t *testing.T) {
	tempBase := t.TempDir()
	hooksDir := filepath.Join(tempBase, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// codex/claude wirar em PreToolUse com .pretooluse.sh (2026-09-23)
	for _, script := range []string{
		"context-guard-nudge.pretooluse.sh",
		"memory-nudge.pretooluse.sh",
		"agent-react-nudge.pretooluse.sh",
	} {
		if err := os.WriteFile(filepath.Join(hooksDir, script), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("criar stub %s: %v", script, err)
		}
	}

	cases := []struct {
		agentKind string
		hookName  string
		syncFn    func(string, TargetCLI) error
	}{
		{"claude", contextGuardHookName, syncHooks},
		{"claude", memoryNudgeHookName, syncMemoryNudgeHook},
		{"claude", agentReactNudgeHookName, syncAgentReactNudgeHook},
		{"codex", contextGuardHookName, syncHooks},
		{"codex", memoryNudgeHookName, syncMemoryNudgeHook},
		{"codex", agentReactNudgeHookName, syncAgentReactNudgeHook},
	}
	for _, tc := range cases {
		t.Run(tc.agentKind+"/"+tc.hookName, func(t *testing.T) {
			tempHooksPath := filepath.Join(t.TempDir(), "settings.json")
			target := TargetCLI{
				Name:              tc.agentKind,
				AgentKind:         tc.agentKind,
				HooksSettingsPath: tempHooksPath,
				HooksFormat:       "",
				HooksEvent:        "PostToolUse",
			}
			if err := tc.syncFn(tempBase, target); err != nil {
				t.Fatalf("sync falhou: %v", err)
			}
			got := findHookIfFiltersAtEvent(t, tempHooksPath, tc.hookName, "PreToolUse")
			if len(got) != len(nudgeIfFilters) {
				t.Fatalf("esperava %d entradas com `if`, obteve %d (%v)", len(nudgeIfFilters), len(got), got)
			}
			for i, want := range nudgeIfFilters {
				if got[i] != want {
					t.Errorf("entrada %d: if filter divergente: obtido %q, esperado %q", i, got[i], want)
				}
			}
		})
	}
}

func TestNudgesEmitOneEntryPerFilterAfterReapply(t *testing.T) {
	tempBase := t.TempDir()
	hooksDir := filepath.Join(tempBase, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// codex/claude wirar em PreToolUse com .pretooluse.sh (2026-09-23)
	if err := os.WriteFile(filepath.Join(hooksDir, "context-guard-nudge.pretooluse.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	tempHooksPath := filepath.Join(t.TempDir(), "settings.json")
	target := TargetCLI{
		Name:              "claude",
		AgentKind:         "claude",
		HooksSettingsPath: tempHooksPath,
		HooksEvent:        "PostToolUse",
	}
	for i := 0; i < 3; i++ {
		if err := syncHooks(tempBase, target); err != nil {
			t.Fatalf("syncHooks rodada %d falhou: %v", i, err)
		}
	}
	got := findHookIfFiltersAtEvent(t, tempHooksPath, contextGuardHookName, "PreToolUse")
	if len(got) != len(nudgeIfFilters) {
		t.Fatalf("idempotência falhou: esperado %d entries, obteve %d (%v)", len(nudgeIfFilters), len(got), got)
	}
}

func TestNudgesDoNotEmitIfFilterForAntigravity(t *testing.T) {
	tempBase := t.TempDir()
	hooksDir := filepath.Join(tempBase, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, script := range []string{
		"context-guard-nudge.antigravity.sh",
		"memory-nudge.antigravity.sh",
		"agent-react-nudge.antigravity.sh",
	} {
		if err := os.WriteFile(filepath.Join(hooksDir, script), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("criar stub %s: %v", script, err)
		}
	}

	cases := []struct {
		hookName string
		syncFn   func(string, TargetCLI) error
	}{
		{contextGuardHookName, syncHooks},
		{memoryNudgeHookName, syncMemoryNudgeHook},
		{agentReactNudgeHookName, syncAgentReactNudgeHook},
	}
	for _, tc := range cases {
		t.Run(tc.hookName, func(t *testing.T) {
			tempHooksPath := filepath.Join(t.TempDir(), "hooks.json")
			target := TargetCLI{
				Name:              "antigravity",
				AgentKind:         "antigravity",
				HooksSettingsPath: tempHooksPath,
				HooksFormat:       "antigravity",
				HooksEvent:        "PostToolUse",
			}
			if err := tc.syncFn(tempBase, target); err != nil {
				t.Fatalf("sync falhou: %v", err)
			}
			if got := findHookIfFiltersAtEvent(t, tempHooksPath, tc.hookName, "PreToolUse"); len(got) > 0 {
				t.Errorf("Antigravity não deveria ter campo `if` no hook %s, obteve %v", tc.hookName, got)
			}
		})
	}
}

func TestUpsertHookEntryRoundTripIfField(t *testing.T) {
	entries := upsertHookEntry(nil, "/path/to/script.sh", "test-hook", "*", []string{"Edit(*)"})
	if len(entries) != 1 {
		t.Fatalf("esperado 1 entrada, obteve %d", len(entries))
	}
	if len(entries[0].Hooks) != 1 {
		t.Fatalf("esperado 1 hookCmd, obteve %d", len(entries[0].Hooks))
	}
	if entries[0].Hooks[0].If != "Edit(*)" {
		t.Errorf("If field não propagado: %q", entries[0].Hooks[0].If)
	}

	raw, err := json.Marshal(entries[0].Hooks[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"if":"Edit(*)"`) {
		t.Errorf("JSON não contém if field: %s", raw)
	}
}

func TestUpsertHookEntryOmitsIfWhenEmpty(t *testing.T) {
	entries := upsertHookEntry(nil, "/path/to/script.sh", "test-hook", "*", nil)
	raw, err := json.Marshal(entries[0].Hooks[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), `"if"`) {
		t.Errorf("JSON contém if field quando deveria ser omitido: %s", raw)
	}
}

func TestUpsertHookEntryEmitsOneEntryPerFilter(t *testing.T) {
	filters := []string{"Edit(*)", "Write(*)", "MultiEdit(*)", "NotebookEdit(*)"}
	entries := upsertHookEntry(nil, "/path/to/script.sh", "test-hook", "*", filters)
	if len(entries) != len(filters) {
		t.Fatalf("esperado %d entradas (1 por filtro), obteve %d", len(filters), len(entries))
	}
	for i, want := range filters {
		if got := entries[i].Hooks[0].If; got != want {
			t.Errorf("entrada %d: If field divergente: obtido %q, esperado %q", i, got, want)
		}
		if entries[i].Matcher != "*" {
			t.Errorf("entrada %d: matcher divergente: %q", i, entries[i].Matcher)
		}
		if entries[i].Hooks[0].Name != "test-hook" {
			t.Errorf("entrada %d: name divergente: %q", i, entries[i].Hooks[0].Name)
		}
	}
}

func TestUpsertHookEntryReplacesPreviousEntriesWithSameName(t *testing.T) {
	existing := []hookEntry{
		{Matcher: "*", Hooks: []hookCmd{{Name: "test-hook", Command: "old.sh"}}},
		{Matcher: "*", Hooks: []hookCmd{{Name: "keep-me", Command: "other.sh"}}},
	}
	after := upsertHookEntry(existing, "/path/to/script.sh", "test-hook", "*", []string{"Edit(*)", "Write(*)"})
	if len(after) != 3 {
		t.Fatalf("esperado 3 entradas (1 preservada + 2 novas), obteve %d", len(after))
	}
	for _, e := range after {
		for _, h := range e.Hooks {
			if h.Name == "test-hook" && h.Command == "old.sh" {
				t.Errorf("entrada antiga do test-hook não foi removida: %+v", e)
			}
		}
	}
}

func TestAgentStopScriptContract(t *testing.T) {
	baseDir, ok := pathutil.FindBaseDir([]string{"."})
	if !ok {
		t.Fatal("baseDir não encontrado a partir do diretório de teste")
	}
	scriptPath := filepath.Join(baseDir, "hooks", "agent-stop.antigravity.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("script %s não encontrado", scriptPath)
	}

	// Caso 1: fullyIdle = false (tarefas em background ativas) -> continua
	cmd := exec.Command(scriptPath)
	cmd.Stdin = strings.NewReader(`{"fullyIdle": false, "executionNum": 1}`)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("execução do script falhou: %v", err)
	}
	var res map[string]interface{}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("JSON inválido retornado: %s", string(out))
	}
	if res["decision"] != "continue" {
		t.Errorf("esperado decision 'continue' para fullyIdle=false, obteve: %v", res["decision"])
	}
	if res["reason"] == "" || res["reason"] == nil {
		t.Errorf("reason obrigatório quando decision='continue'")
	}

	// Caso 2: fullyIdle = true -> permite parada ({})
	cmdNormal := exec.Command(scriptPath)
	cmdNormal.Stdin = strings.NewReader(`{"fullyIdle": true, "executionNum": 1}`)
	outNormal, err := cmdNormal.Output()
	if err != nil {
		t.Fatalf("execução normal falhou: %v", err)
	}
	var resNormal map[string]interface{}
	if err := json.Unmarshal(outNormal, &resNormal); err != nil {
		t.Fatalf("JSON inválido: %s", string(outNormal))
	}
	if resNormal["decision"] == "continue" {
		t.Errorf("parada normal não deve retornar continue: %v", resNormal)
	}

	// Caso 3: limite de loop (executionNum >= 3) -> permite parada mesmo se fullyIdle=false
	cmdLimit := exec.Command(scriptPath)
	cmdLimit.Stdin = strings.NewReader(`{"fullyIdle": false, "executionNum": 3}`)
	outLimit, err := cmdLimit.Output()
	if err != nil {
		t.Fatalf("execução limit falhou: %v", err)
	}
	var resLimit map[string]interface{}
	if err := json.Unmarshal(outLimit, &resLimit); err != nil {
		t.Fatalf("JSON inválido: %s", string(outLimit))
	}
	if resLimit["decision"] == "continue" {
		t.Errorf("loop guard não permitiu parada: %v", resLimit)
	}
}

func TestAgentPreInvocationScriptContract(t *testing.T) {
	baseDir, ok := pathutil.FindBaseDir([]string{"."})
	if !ok {
		t.Fatal("baseDir não encontrado a partir do diretório de teste")
	}
	scriptPath := filepath.Join(baseDir, "hooks", "agent-preinvocation.antigravity.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("script %s não encontrado", scriptPath)
	}

	cmd := exec.Command(scriptPath)
	cmd.Stdin = strings.NewReader(`{"invocationNum": 1}`)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("execução do script falhou: %v", err)
	}
	// Contrato Antigravity PreInvocation (runtime 2026-09-22, commit
	// 52587fd): o script emite {} (silent). Schema antigo (injectSteps +
	// ephemeralMessage) faz o Gemini CLI negar a tool seguinte com
	// `tool call denied by pre-tool hook`. A diretriz PT-BR vive em
	// GEMINI.md (system prompt), não precisa de nudge por hook.
	var res map[string]interface{}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("JSON inválido retornado: %s", string(out))
	}
	if len(res) != 0 {
		t.Fatalf("PreInvocation deve emitir {} vazio, retornou: %s", string(out))
	}
}

// TestSyncTokenNudgeHookAntigravityPostToolUse valida que token-nudge no
// Antigravity wirar em PostToolUse (formato {matcher, hooks: [...]}), NAO em
// PreInvocation (formato flat). Sem isso, o Gemini CLI rejeita o hook com
// 'command hook must specify command' (verificado em runtime 2026-09-22).
func TestSyncTokenNudgeHookAntigravityPostToolUse(t *testing.T) {
	tempBase := t.TempDir()
	hooksDir := filepath.Join(tempBase, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(hooksDir, "token-nudge.check.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	tempHooksPath := filepath.Join(t.TempDir(), "hooks.json")
	target := TargetCLI{
		Name:              "antigravity",
		AgentKind:         "antigravity",
		HooksSettingsPath: tempHooksPath,
		HooksFormat:       "antigravity",
		HooksEvent:        "PreInvocation", // default de target.go:69
	}

	if err := syncTokenNudgeHook(tempBase, target); err != nil {
		t.Fatalf("syncTokenNudgeHook falhou: %v", err)
	}

	root, err := readJSONObject(tempHooksPath)
	if err != nil {
		t.Fatalf("falha ao ler JSON: %v", err)
	}

	group, ok := root[tokenNudgeHookName].(map[string]interface{})
	if !ok {
		t.Fatalf("grupo %q nao encontrado: %v", tokenNudgeHookName, root)
	}

	// Hook deve estar em PostToolUse (formato nested {matcher, hooks: [...]})
	postEntries, ok := group["PostToolUse"].([]interface{})
	if !ok || len(postEntries) != 1 {
		t.Fatalf("PostToolUse deve ter 1 entrada nested, obteve: %v", group["PostToolUse"])
	}

	entry := postEntries[0].(map[string]interface{})
	if entry["matcher"] != "*" {
		t.Errorf("matcher esperado '*', obteve: %v", entry["matcher"])
	}
	hooksRaw, ok := entry["hooks"].([]interface{})
	if !ok || len(hooksRaw) != 1 {
		t.Fatalf("hooks deve ser array com 1 elemento, obteve: %v", entry["hooks"])
	}
	hookCmd := hooksRaw[0].(map[string]interface{})
	if hookCmd["type"] != "command" {
		t.Errorf("type esperado 'command', obteve: %v", hookCmd["type"])
	}
	if hookCmd["name"] != tokenNudgeHookName {
		t.Errorf("name esperado %q, obteve: %v", tokenNudgeHookName, hookCmd["name"])
	}

	// Hook NAO deve estar em PreInvocation (formato flat nao suporta matcher)
	if _, present := group["PreInvocation"]; present {
		t.Errorf("token-nudge NAO deve ser wirado em PreInvocation (formato flat rejeitado pelo Gemini CLI): %v", group["PreInvocation"])
	}
}

// TestSyncShellValidateHookAntigravityPreToolUse valida que shell-validate
// wirar em PreToolUse (formato {matcher, hooks: [...]}), NAO em
// PreInvocation (formato flat). Mesmo padrao do token-nudge.
func TestSyncShellValidateHookAntigravityPreToolUse(t *testing.T) {
	tempBase := t.TempDir()
	tempHooksPath := filepath.Join(t.TempDir(), "hooks.json")
	target := TargetCLI{
		Name:              "antigravity",
		AgentKind:         "antigravity",
		HooksSettingsPath: tempHooksPath,
		HooksFormat:       "antigravity",
		HooksEvent:        "PreInvocation", // default de target.go:69
	}

	// Pre-popula com entrada orfa em PreInvocation (simula wirar antigo
	// antes do fix). O wirar novo deve REMOVE-la.
	if err := os.WriteFile(tempHooksPath, []byte(`{
		"agent-sync-shell-validate": {
			"PreInvocation": [
				{"hooks":[{"command":"shell-validate hook","name":"agent-sync-shell-validate","timeout":10,"type":"command"}],"matcher":"*"}
			]
		}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncShellValidateHook(tempBase, target); err != nil {
		t.Fatalf("syncShellValidateHook falhou: %v", err)
	}

	root, err := readJSONObject(tempHooksPath)
	if err != nil {
		t.Fatalf("falha ao ler JSON: %v", err)
	}

	group, ok := root[shellValidateHookName].(map[string]interface{})
	if !ok {
		t.Fatalf("grupo %q nao encontrado: %v", shellValidateHookName, root)
	}

	// Hook deve estar em PreToolUse (formato nested)
	preEntries, ok := group["PreToolUse"].([]interface{})
	if !ok || len(preEntries) != 1 {
		t.Fatalf("PreToolUse deve ter 1 entrada nested, obteve: %v", group["PreToolUse"])
	}

	entry := preEntries[0].(map[string]interface{})
	if entry["matcher"] != "*" {
		t.Errorf("matcher esperado '*', obteve: %v", entry["matcher"])
	}

	// Hook NAO deve estar em PreInvocation (entrada orfa deve ter sido limpa)
	if _, present := group["PreInvocation"]; present {
		t.Errorf("shell-validate NAO deve ter entrada orfa em PreInvocation: %v", group["PreInvocation"])
	}
}

// TestSyncTokenNudgeHookAntigravityClearsOrphanPreInvocation valida que o
// wirar novo de token-nudge (em PostToolUse) remove entrada orfa em
// PreInvocation que o wirar antigo deixou para tras.
func TestSyncTokenNudgeHookAntigravityClearsOrphanPreInvocation(t *testing.T) {
	tempBase := t.TempDir()
	hooksDir := filepath.Join(tempBase, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(hooksDir, "token-nudge.check.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	tempHooksPath := filepath.Join(t.TempDir(), "hooks.json")
	target := TargetCLI{
		Name:              "antigravity",
		AgentKind:         "antigravity",
		HooksSettingsPath: tempHooksPath,
		HooksFormat:       "antigravity",
		HooksEvent:        "PreInvocation",
	}

	// Pre-popula com entrada orfa em PreInvocation (wirar antigo).
	if err := os.WriteFile(tempHooksPath, []byte(`{
		"agent-sync-token-nudge": {
			"PreInvocation": [
				{"hooks":[{"command":"x","name":"agent-sync-token-nudge","timeout":10,"type":"command"}],"matcher":"*"}
			]
		}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncTokenNudgeHook(tempBase, target); err != nil {
		t.Fatalf("syncTokenNudgeHook falhou: %v", err)
	}

	root, err := readJSONObject(tempHooksPath)
	if err != nil {
		t.Fatalf("falha ao ler JSON: %v", err)
	}

	group, ok := root[tokenNudgeHookName].(map[string]interface{})
	if !ok {
		t.Fatalf("grupo %q nao encontrado: %v", tokenNudgeHookName, root)
	}

	// Hook deve estar em PostToolUse (formato nested)
	if _, ok := group["PostToolUse"].([]interface{}); !ok {
		t.Errorf("PostToolUse deve existir como array nested, obteve: %v", group["PostToolUse"])
	}

	// Entrada orfa em PreInvocation deve ter sido limpa
	if _, present := group["PreInvocation"]; present {
		t.Errorf("token-nudge NAO deve ter entrada orfa em PreInvocation: %v", group["PreInvocation"])
	}
}

func TestObserveErrorCLIDetection(t *testing.T) {
	baseDir, ok := pathutil.FindBaseDir([]string{"."})
	if !ok {
		t.Fatal("baseDir nao encontrado")
	}

	observeScript := filepath.Join(baseDir, "hooks", "observe-error.sh")
	wrapScript := filepath.Join(baseDir, "hooks", "wrap-hook.sh")

	// 1. Explicit AGENT_SYNC_CLI
	t.Run("explicit_env", func(t *testing.T) {
		logFile := filepath.Join(t.TempDir(), "errors.jsonl")
		cmd := exec.Command("bash", observeScript, "PreToolUse", "test_exit_1", "msg")
		cmd.Env = append(os.Environ(), "AGENT_SYNC_HOOK_LOG="+logFile, "AGENT_SYNC_CLI=claude")
		if err := cmd.Run(); err != nil {
			t.Fatalf("cmd.Run: %v", err)
		}
		data, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatal(err)
		}
		var entry map[string]interface{}
		if err := json.Unmarshal(data, &entry); err != nil {
			t.Fatal(err)
		}
		if entry["cli"] != "claude" {
			t.Errorf("want claude, got %v", entry["cli"])
		}
	})

	// 2. Detection via hook name suffix
	t.Run("hook_name_detection", func(t *testing.T) {
		logFile := filepath.Join(t.TempDir(), "errors.jsonl")
		cmd := exec.Command("bash", observeScript, "Stop", "agent-stop.antigravity_exit_1", "msg")
		cmd.Env = append(os.Environ(), "AGENT_SYNC_HOOK_LOG="+logFile)
		// garante que AGENT_SYNC_CLI nao esta setada
		for i, env := range cmd.Env {
			if strings.HasPrefix(env, "AGENT_SYNC_CLI=") {
				cmd.Env[i] = "AGENT_SYNC_CLI="
			}
		}
		if err := cmd.Run(); err != nil {
			t.Fatalf("cmd.Run: %v", err)
		}
		data, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatal(err)
		}
		var entry map[string]interface{}
		if err := json.Unmarshal(data, &entry); err != nil {
			t.Fatal(err)
		}
		if entry["cli"] != "antigravity" {
			t.Errorf("want antigravity, got %v", entry["cli"])
		}
	})

	// 3. wrap-hook.sh with --cli flag
	t.Run("wrap_hook_cli_flag", func(t *testing.T) {
		logFile := filepath.Join(t.TempDir(), "errors.jsonl")
		failScript := filepath.Join(t.TempDir(), "fail.sh")
		if err := os.WriteFile(failScript, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", wrapScript, "--cli=cursor", "PostToolUse", "my-hook", failScript)
		cmd.Env = append(os.Environ(), "AGENT_SYNC_HOOK_LOG="+logFile)
		_ = cmd.Run() // esperado falhar com exit code 3

		data, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatal(err)
		}
		var entry map[string]interface{}
		if err := json.Unmarshal(data, &entry); err != nil {
			t.Fatal(err)
		}
		if entry["cli"] != "cursor" {
			t.Errorf("want cursor, got %v", entry["cli"])
		}
	})
}
