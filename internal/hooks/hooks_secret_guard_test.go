package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSyncSecretGuardHooksWirePreAndPost valida que wirar secret-guard em
// Claude Code e Codex grava entradas em PreToolUse E PostToolUse (defesa
// em profundidade: Camada 1 path-deny + Camada 2 regex literal).
// Cada evento usa um hookName distinto (pre vs post) por causa da
// migracao automatica do syncHookCommandAtEvent (limpa wiramentos orfaos
// do mesmo hookName em outros eventos).
func TestSyncSecretGuardHooksWirePreAndPost(t *testing.T) {
	for _, kind := range []string{"claude", "codex"} {
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
				HooksFormat:       "",
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
			hooks, _ := root["hooks"].(map[string]interface{})
			if hooks == nil {
				t.Fatalf("chave 'hooks' ausente no json para %s: %s", kind, string(data))
			}

			preList, ok := hooks["PreToolUse"].([]interface{})
			if !ok || len(preList) == 0 {
				t.Fatalf("PreToolUse hook nao encontrado para %s: %s", kind, string(data))
			}
			postList, ok := hooks["PostToolUse"].([]interface{})
			if !ok || len(postList) == 0 {
				t.Fatalf("PostToolUse hook nao encontrado para %s: %s", kind, string(data))
			}

			// Verifica que ambos os hookNames distintos estao presentes no JSON.
			jsonStr := string(data)
			if !strings.Contains(jsonStr, secretGuardPreToolUseHookName) {
				t.Fatalf("hookName pre (%s) ausente do settings.json para %s: %s",
					secretGuardPreToolUseHookName, kind, jsonStr)
			}
			if !strings.Contains(jsonStr, secretGuardPostToolUseHookName) {
				t.Fatalf("hookName post (%s) ausente do settings.json para %s: %s",
					secretGuardPostToolUseHookName, kind, jsonStr)
			}

			// Verifica que os 2 wiramentos estao em eventos DIFERENTES.
			preHasGuard := false
			for _, entry := range preList {
				if entryMap, ok := entry.(map[string]interface{}); ok {
					if hooksArr, ok := entryMap["hooks"].([]interface{}); ok {
						for _, h := range hooksArr {
							if hMap, ok := h.(map[string]interface{}); ok {
								if name, _ := hMap["name"].(string); name == secretGuardPreToolUseHookName {
									preHasGuard = true
								}
							}
						}
					}
				}
			}
			postHasGuard := false
			for _, entry := range postList {
				if entryMap, ok := entry.(map[string]interface{}); ok {
					if hooksArr, ok := entryMap["hooks"].([]interface{}); ok {
						for _, h := range hooksArr {
							if hMap, ok := h.(map[string]interface{}); ok {
								if name, _ := hMap["name"].(string); name == secretGuardPostToolUseHookName {
									postHasGuard = true
								}
							}
						}
					}
				}
			}
			if !preHasGuard {
				t.Fatalf("PreToolUse nao contem hookName pre em %s: %s", kind, jsonStr)
			}
			if !postHasGuard {
				t.Fatalf("PostToolUse nao contem hookName post em %s: %s", kind, jsonStr)
			}
		})
	}
}

// TestSyncSecretGuardHooksSkipNonSupported valida que wiramento e no-op
// para CLIs fora do escopo (matriz 5xN do ADR §2: 2 gaps 🟡 + 1 impossivel
// ⛔ ficam para A-64+).
func TestSyncSecretGuardHooksSkipNonSupported(t *testing.T) {
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
