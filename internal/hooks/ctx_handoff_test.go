package hooks

import (
	"path/filepath"
	"testing"
)

func TestSyncCtxHandoffUsesEachHookSchema(t *testing.T) {
	for _, test := range []struct {
		name, format, event string
	}{
		{"claude", "", "SessionStart"},
		{"codex", "", "SessionStart"},
		{"cursor", "cursor", "sessionStart"},
		{"antigravity", "antigravity", "PreInvocation"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "hooks.json")
			target := TargetCLI{AgentKind: test.name, HooksSettingsPath: path, HooksFormat: test.format}
			for i := 0; i < 2; i++ {
				if err := syncCtxHandoffHook(t.TempDir(), target); err != nil {
					t.Fatal(err)
				}
			}
			root, err := readJSONObject(path)
			if err != nil {
				t.Fatal(err)
			}
			if test.format == "antigravity" {
				group := root[ctxHandoffHookName].(map[string]interface{})
				if len(group[test.event].([]interface{})) != 1 {
					t.Fatalf("duplicate Antigravity hooks: %v", group)
				}
				return
			}
			hooks := root["hooks"].(map[string]interface{})
			if len(hooks[test.event].([]interface{})) != 1 {
				t.Fatalf("duplicate hooks: %v", hooks)
			}
		})
	}
}

func TestSyncCursorCtxTrackingUsesCursorSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	target := TargetCLI{AgentKind: "cursor", HooksSettingsPath: path, HooksEvent: "postToolUse", HooksFormat: "cursor"}
	if err := syncCtxCompactHook(t.TempDir(), target); err != nil {
		t.Fatal(err)
	}
	root, err := readJSONObject(path)
	if err != nil {
		t.Fatal(err)
	}
	hooks := root["hooks"].(map[string]interface{})
	entries := decodeCursorHookEntries(hooks["postToolUse"])
	if len(entries) != 1 || entries[0].Command != "ctx-window hook cursor" {
		t.Fatalf("unexpected Cursor tracking hooks: %v", entries)
	}
}
