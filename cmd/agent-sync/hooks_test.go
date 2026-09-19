package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

	if entry["command"] != scriptPath {
		t.Errorf("command incorreto: obteve %v, esperado %v", entry["command"], scriptPath)
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

	if entry["command"] != scriptPath {
		t.Errorf("command incorreto: obteve %v, esperado %v", entry["command"], scriptPath)
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

func TestAgentStopScriptContract(t *testing.T) {
	baseDir, ok := findBaseDir([]string{"."})
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
	baseDir, ok := findBaseDir([]string{"."})
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
	var res struct {
		InjectSteps []struct {
			EphemeralMessage string `json:"ephemeralMessage"`
		} `json:"injectSteps"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("JSON inválido retornado: %s", string(out))
	}
	expectedDirective := "Diretriz ativa: responda em PT-BR, sem rodeios e finalize com o resumo de 1-2 frases do que mudou e o que falta."
	if len(res.InjectSteps) != 1 || res.InjectSteps[0].EphemeralMessage != expectedDirective {
		t.Fatalf("mensagem efêmera inesperada: %+v", res.InjectSteps)
	}
}
