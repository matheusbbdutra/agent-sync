package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: cria hooks/ com scriptName e wirar para target
func setupNudgeTest(t *testing.T, scriptName string) (string, TargetCLI) {
	t.Helper()
	tempBase := t.TempDir()
	scriptPath := filepath.Join(tempBase, "hooks", scriptName)
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho {}\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return tempBase, TargetCLI{
		Name:              "codex",
		AgentKind:         "codex",
		HooksSettingsPath: filepath.Join(t.TempDir(), "hooks.json"),
		HooksEvent:        "PostToolUse",
	}
}

// TestNudgeForCodexNotWiredInPreToolUse valida que syncAgentReactNudgeHook NAO
// wirar em PreToolUse para o Codex. Com features.code_mode_host ligado
// (codex >= 0.160) a unica tool emite custom_tool_call, e o Codex injeta o
// additionalContext do hook entre a call e o output dela — o /v1/responses da
// MiniMax responde 400 (2013) "tool call result does not follow tool call".
// No Codex os lembretes entram em SessionStart (syncCodexSessionNudgesHook).
func TestNudgeForCodexNotWiredInPreToolUse(t *testing.T) {
	tempBase, target := setupNudgeTest(t, "agent-react-nudge.pretooluse.sh")

	oldScript := filepath.Join(tempBase, "hooks", "agent-react-nudge.sh")
	if err := os.WriteFile(oldScript, []byte("#!/bin/sh\necho {}\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := syncAgentReactNudgeHook(tempBase, target); err != nil {
		t.Fatalf("syncAgentReactNudgeHook falhou: %v", err)
	}

	data, err := os.ReadFile(target.HooksSettingsPath)
	if err == nil {
		t.Fatalf("nada deveria ter sido wirado, arquivo contem: %s", data)
	}
}

// TestCodexSessionNudgesWiredInSessionStart valida que os 4 lembretes entram
// em SessionStart no Codex, com os mesmos hookName usados antes em
// PreToolUse (para que o cleanup de orfaos remova as entradas antigas).
func TestCodexSessionNudgesWiredInSessionStart(t *testing.T) {
	tempBase := t.TempDir()
	hooksDir := filepath.Join(tempBase, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"codex-session-nudges.sh"} {
		if err := os.WriteFile(filepath.Join(hooksDir, n), []byte("#!/bin/sh\necho {}\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Pre-condicao: wirares antigos em PreToolUse, como antes da mudanca.
	settingsPath := filepath.Join(t.TempDir(), "hooks.json")
	pre := `{"hooks":{"PreToolUse":[{"matcher":"*","hooks":[` +
		`{"name":"agent-sync-principles-inject","command":"/old/principles.pretooluse.sh"},` +
		`{"name":"agent-sync-context-guard","command":"/old/ctx.pretooluse.sh"},` +
		`{"name":"agent-sync-memory-nudge","command":"/old/mem.pretooluse.sh"},` +
		`{"name":"agent-sync-agent-react-nudge","command":"/old/react.pretooluse.sh"},` +
		`{"name":"outros-hook","command":"/keep.sh"}]}]}}`
	if err := os.WriteFile(settingsPath, []byte(pre), 0o644); err != nil {
		t.Fatal(err)
	}

	target := TargetCLI{
		Name:              "codex",
		AgentKind:         "codex",
		HooksSettingsPath: settingsPath,
		HooksEvent:        "PostToolUse",
	}

	if err := syncCodexSessionNudgesHook(tempBase, target); err != nil {
		t.Fatalf("syncCodexSessionNudgesHook falhou: %v", err)
	}

	root, err := readJSONObject(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	hooksRoot := root["hooks"].(map[string]interface{})

	// 1. Os 4 nomes em SessionStart, apontando pro script novo.
	sessionEntries, ok := hooksRoot["SessionStart"].([]interface{})
	if !ok || len(sessionEntries) == 0 {
		t.Fatalf("SessionStart deve ter entradas, obteve: %v", hooksRoot["SessionStart"])
	}
	names := map[string]bool{}
	for _, e := range sessionEntries {
		entry := e.(map[string]interface{})
		for _, h := range entry["hooks"].([]interface{}) {
			hk := h.(map[string]interface{})
			names[hk["name"].(string)] = true
			cmd := hk["command"].(string)
			if !strings.Contains(cmd, "codex-session-nudges.sh") {
				t.Errorf("SessionStart deve usar codex-session-nudges.sh, obteve %q", cmd)
			}
		}
	}
	for _, want := range []string{
		principlesInjectHookName, contextGuardHookName,
		memoryNudgeHookName, agentReactNudgeHookName,
	} {
		if !names[want] {
			t.Errorf("hook %s ausente em SessionStart", want)
		}
	}

	// 2. Os wirares antigos sairam de PreToolUse; outros hooks preservados.
	preEntries, _ := hooksRoot["PreToolUse"].([]interface{})
	for _, e := range preEntries {
		entry := e.(map[string]interface{})
		for _, h := range entry["hooks"].([]interface{}) {
			n := h.(map[string]interface{})["name"].(string)
			if n != "outros-hook" {
				t.Errorf("PreToolUse ainda tem %s apos migrar para SessionStart", n)
			}
		}
	}
}

// TestNudgeForClaudeUsesPreToolUse valida que syncAgentReactNudgeHook wirar
// em PreToolUse para Claude Code (precedente: principles-inject ja wirar assim).
func TestNudgeForClaudeUsesPreToolUse(t *testing.T) {
	tempBase := t.TempDir()
	scriptPath := filepath.Join(tempBase, "hooks", "agent-react-nudge.pretooluse.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho {}\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// tambem o .sh antigo (nao deve ser usado)
	if err := os.WriteFile(filepath.Join(tempBase, "hooks", "agent-react-nudge.sh"),
		[]byte("#!/bin/sh\necho {}\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	target := TargetCLI{
		Name:              "claude",
		AgentKind:         "claude",
		HooksSettingsPath: filepath.Join(t.TempDir(), "settings.json"),
		HooksEvent:        "PostToolUse",
	}

	if err := syncAgentReactNudgeHook(tempBase, target); err != nil {
		t.Fatalf("syncAgentReactNudgeHook falhou: %v", err)
	}

	root, err := readJSONObject(target.HooksSettingsPath)
	if err != nil {
		t.Fatal(err)
	}
	hooksRoot := root["hooks"].(map[string]interface{})

	preEntries, ok := hooksRoot["PreToolUse"].([]interface{})
	if !ok || len(preEntries) == 0 {
		t.Fatalf("PreToolUse deve ter entradas, obteve: %v", hooksRoot["PreToolUse"])
	}

	encontrou := false
	for _, e := range preEntries {
		entry := e.(map[string]interface{})
		for _, h := range entry["hooks"].([]interface{}) {
			cmd := h.(map[string]interface{})["command"].(string)
			if filepath.Base(cmd[:len(cmd)-len(filepath.Ext(cmd))]) == "agent-react-nudge.pretooluse" {
				encontrou = true
			}
		}
	}
	if !encontrou {
		t.Fatalf("agent-react-nudge.pretooluse.sh nao encontrado em PreToolUse para Claude: %v", preEntries)
	}
}

// TestNudgeForAntigravityStillUsesPostToolUse valida que Antigravity NAO
// muda: continua wirando PostToolUse (formato Antigravity + schema proprio
// que aceita `injectSteps`). Regressao silenciosa seria grave aqui.
func TestNudgeForAntigravityStillUsesPostToolUse(t *testing.T) {
	tempBase := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tempBase, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"agent-react-nudge.antigravity.sh", "agent-react-nudge.pretooluse.sh"} {
		if err := os.WriteFile(filepath.Join(tempBase, "hooks", n),
			[]byte("#!/bin/sh\necho {}\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	target := TargetCLI{
		Name:              "antigravity",
		AgentKind:         "antigravity",
		HooksSettingsPath: filepath.Join(t.TempDir(), "hooks.json"),
		HooksFormat:       "antigravity",
		HooksEvent:        "PreInvocation",
	}

	if err := syncAgentReactNudgeHook(tempBase, target); err != nil {
		t.Fatalf("syncAgentReactNudgeHook falhou: %v", err)
	}

	root, err := readJSONObject(target.HooksSettingsPath)
	if err != nil {
		t.Fatal(err)
	}

	// formato antigravity: hook eh chave top-level
	if _, ok := root[agentReactNudgeHookName]; !ok {
		t.Fatalf("hook %s nao encontrado no formato antigravity: %v", agentReactNudgeHookName, root)
	}
}

// TestSyncHookCommandAtEventClearsOrphans valida que ao wirar em um evento,
// wirar com o mesmo name em outros eventos eh removido (migracao de evento
// nao deixa wirar duplicado).
func TestSyncHookCommandAtEventClearsOrphans(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.WriteFile(settingsPath, []byte(`{
		"hooks": {
			"PostToolUse": [
				{
					"matcher": "*",
					"hooks": [
						{"name": "agent-sync-agent-react-nudge", "command": "/x.sh"},
						{"name": "outros-hook", "command": "/y.sh"}
					]
				}
			]
		}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	target := TargetCLI{
		Name:              "codex",
		AgentKind:         "codex",
		HooksSettingsPath: settingsPath,
		HooksEvent:        "PostToolUse",
	}

	if err := syncHookCommandAtEvent(t.TempDir(), target, "agent-sync-agent-react-nudge",
		"/x.pretooluse.sh", "*", "PreToolUse", nil); err != nil {
		t.Fatalf("sync falhou: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	root := map[string]interface{}{}
	json.Unmarshal(data, &root)
	hooks := root["hooks"].(map[string]interface{})

	// PostToolUse deve manter apenas outros-hook
	post, _ := hooks["PostToolUse"].([]interface{})
	if len(post) != 1 {
		t.Fatalf("PostToolUse deve ter 1 entry, obteve %d: %v", len(post), post)
	}
	entry := post[0].(map[string]interface{})
	for _, h := range entry["hooks"].([]interface{}) {
		cmd := h.(map[string]interface{})["name"].(string)
		if cmd == "agent-sync-agent-react-nudge" {
			t.Errorf("wirar orfao nao foi removido de PostToolUse")
		}
	}
}
