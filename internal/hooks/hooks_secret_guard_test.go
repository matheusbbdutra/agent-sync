package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSyncSecretGuardHooksWirePreAndPost valida que wirar secret-guard em
// Claude Code, Codex e Antigravity grava entradas em PreToolUse E PostToolUse
// (defesa em profundidade: Camada 1 path-deny + Camada 2 regex literal).
// Cada evento usa um hookName distinto (pre vs post) por causa da
// migracao automatica do syncHookCommandAtEvent (limpa wiramentos orfaos
// do mesmo hookName em outros eventos).
func TestSyncSecretGuardHooksWirePreAndPost(t *testing.T) {
	for _, kind := range []string{"claude", "codex", "antigravity"} {
		t.Run(kind, func(t *testing.T) {
			tempBase := t.TempDir()
			hooksDir := filepath.Join(tempBase, "hooks")
			if err := os.MkdirAll(hooksDir, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, script := range []string{secretGuardPreToolUseScript, secretGuardPostToolUseScript} {
				if err := os.WriteFile(filepath.Join(hooksDir, script),
					[]byte("#!/usr/bin/env bash\necho '{}'\n"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			tempHooksPath := filepath.Join(tempBase, "settings.json")
			target := TargetCLI{
				Name:              kind,
				AgentKind:         kind,
				HooksSettingsPath: tempHooksPath,
				HooksFormat:       kind,
			}

			if err := syncSecretGuardPreToolUseHook(tempBase, target); err != nil {
				t.Fatalf("falha ao wirar secret-guard PreToolUse para %s: %v", kind, err)
			}
			if err := syncSecretGuardPostToolUseHook(tempBase, target); err != nil {
				t.Fatalf("falha ao wirar secret-guard PostToolUse para %s: %v", kind, err)
			}

			data, err := os.ReadFile(tempHooksPath)
			if err != nil {
				t.Fatalf("settings.json nao foi criado para %s: %v", kind, err)
			}
			var root map[string]interface{}
			if err := json.Unmarshal(data, &root); err != nil {
				t.Fatalf("json invalido para %s: %v", kind, string(data))
			}
			hooks := root
			if h, _ := root["hooks"].(map[string]interface{}); h != nil {
				hooks = h
			}

			// Claude/Codex usam root["hooks"]["PreToolUse"]/[\"PostToolUse\"].
			// Antigravity usa formato nested: root[hookName][evento].
			var preList, postList []interface{}
			if v, ok := hooks["PreToolUse"].([]interface{}); ok {
				preList = v
			} else if group, ok := hooks[secretGuardPreToolUseHookName].(map[string]interface{}); ok {
				if v, ok := group["PreToolUse"].([]interface{}); ok {
					preList = v
				}
			}
			if v, ok := hooks["PostToolUse"].([]interface{}); ok {
				postList = v
			} else if group, ok := hooks[secretGuardPostToolUseHookName].(map[string]interface{}); ok {
				if v, ok := group["PostToolUse"].([]interface{}); ok {
					postList = v
				}
			}
			if len(preList) == 0 {
				t.Fatalf("PreToolUse hook nao encontrado para %s: %s", kind, string(data))
			}
			if len(postList) == 0 {
				t.Fatalf("PostToolUse hook nao encontrado para %s: %s", kind, string(data))
			}

			jsonStr := string(data)
			if !strings.Contains(jsonStr, secretGuardPreToolUseHookName) {
				t.Fatalf("hookName pre (%s) ausente do settings.json para %s: %s",
					secretGuardPreToolUseHookName, kind, jsonStr)
			}
			if !strings.Contains(jsonStr, secretGuardPostToolUseHookName) {
				t.Fatalf("hookName post (%s) ausente do settings.json para %s: %s",
					secretGuardPostToolUseHookName, kind, jsonStr)
			}
		})
	}
}

// TestSyncSecretGuardCursorPostToolUseInCursorManagedHooks valida que a
// entrada de secret-guard.posttooluse.sh esta em cursorManagedHooks()
// (merge gerenciado por syncCursorAll).
func TestSyncSecretGuardCursorPostToolUseInCursorManagedHooks(t *testing.T) {
	found := false
	for _, h := range cursorManagedHooks() {
		if h.Event == "postToolUse" && h.Script == secretGuardPostToolUseScript {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("cursorManagedHooks() deveria conter entrada postToolUse para %s",
			secretGuardPostToolUseScript)
	}
}

// TestSyncSecretGuardHooksSkipNonSupported valida que wiramento e no-op
// para CLIs fora do escopo (matriz 5xN do ADR §2: 1 gap 🟡 aceito fica
// para A-64+ via permission.hook).
func TestSyncSecretGuardHooksSkipNonSupported(t *testing.T) {
	for _, kind := range []string{"opencode"} {
		t.Run(kind, func(t *testing.T) {
			tempBase := t.TempDir()
			tempHooksPath := filepath.Join(t.TempDir(), "settings.json")
			target := TargetCLI{
				Name:              kind,
				AgentKind:         kind,
				HooksSettingsPath: tempHooksPath,
				HooksFormat:       kind,
			}
			if err := syncSecretGuardPreToolUseHook(tempBase, target); err != nil {
				t.Fatalf("PreToolUse em %s deveria ser no-op sem erro: %v", kind, err)
			}
			if err := syncSecretGuardPostToolUseHook(tempBase, target); err != nil {
				t.Fatalf("PostToolUse em %s deveria ser no-op sem erro: %v", kind, err)
			}
			if _, err := os.Stat(tempHooksPath); err == nil {
				t.Fatalf("settings.json NAO deveria ter sido criado para %s (gap aceito no ADR)", kind)
			}
		})
	}
}
