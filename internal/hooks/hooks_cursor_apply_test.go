package hooks

// hooks_cursor_apply_test.go: testes para hooks_cursor_apply.go (Cursor
// wirar). Migrado de cursor_test.go em 2026-09-21 (Fase 7).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)


func TestMergeCursorHooksJSONIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	existing := map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"afterFileEdit": []interface{}{
				map[string]interface{}{"command": "./hooks/format.sh"},
			},
			"postToolUse": []interface{}{
				map[string]interface{}{"command": "./hooks/context-guard-nudge.cursor.sh"},
			},
		},
	}
	encoded, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := mergeCursorHooksJSON(path); err != nil {
		t.Fatalf("merge 1: %v", err)
	}
	if err := mergeCursorHooksJSON(path); err != nil {
		t.Fatalf("merge 2: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var root map[string]interface{}
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	hooks, _ := root["hooks"].(map[string]interface{})
	afterEdit := decodeCursorHookEntries(hooks["afterFileEdit"])
	if len(afterEdit) != 1 || afterEdit[0].Command != "./hooks/format.sh" {
		t.Fatalf("deveria preservar afterFileEdit: %+v", afterEdit)
	}
	post := decodeCursorHookEntries(hooks["postToolUse"])
	if len(post) != 5 {
		t.Fatalf("esperava 5 postToolUse gerenciados, got %d: %+v", len(post), post)
	}
	var hasReact bool
	for _, e := range post {
		if strings.Contains(e.Command, "agent-react-nudge.cursor.sh") {
			hasReact = true
		}
	}
	if !hasReact {
		t.Fatalf("agent-react-nudge ausente em postToolUse: %+v", post)
	}
	before := decodeCursorHookEntries(hooks["beforeShellExecution"])
	if len(before) != 1 || !strings.Contains(before[0].Command, "bash-guardian.cursor.sh") {
		t.Fatalf("bash-guardian ausente: %+v", before)
	}
	afterMCP := decodeCursorHookEntries(hooks["afterMCPExecution"])
	if len(afterMCP) != 1 || afterMCP[0].Matcher != "query-docs" {
		t.Fatalf("docs-cache mcp ausente: %+v", afterMCP)
	}
}

func TestMergeCursorHooksJSONStopIncludesLoopLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	initial := map[string]interface{}{"version": 1, "hooks": map[string]interface{}{}}
	encoded, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := mergeCursorHooksJSON(path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	var root map[string]interface{}
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	hooks := root["hooks"].(map[string]interface{})
	stops := decodeCursorHookEntries(hooks["stop"])
	if len(stops) != 3 {
		t.Fatalf("esperava 3 stops gerenciados (agent-stop + react-stop + ctx-window-summarize-at-stop), got %d: %+v", len(stops), stops)
	}
	var react *cursorHookEntry
	for i, e := range stops {
		if strings.Contains(e.Command, "agent-react-nudge.stop.cursor.sh") {
			react = &stops[i]
		}
	}
	if react == nil {
		t.Fatalf("agent-react-nudge.stop.cursor.sh ausente: %+v", stops)
	}
	if react.LoopLimit != 5 {
		t.Fatalf("loop_limit esperado 5, got %d", react.LoopLimit)
	}
}

func TestEncodeCursorHookEntriesOmitsZeroLoopLimit(t *testing.T) {
	encoded := encodeCursorHookEntries([]cursorHookEntry{
		{Command: "./hooks/x.sh", Matcher: "Edit"},
	})
	if len(encoded) != 1 {
		t.Fatalf("esperava 1 entrada, got %d", len(encoded))
	}
	if _, ok := encoded[0]["loop_limit"]; ok {
		t.Fatalf("loop_limit não deveria aparecer quando 0: %+v", encoded[0])
	}
}

func TestEncodeCursorHookEntriesIncludesLoopLimitWhenPositive(t *testing.T) {
	encoded := encodeCursorHookEntries([]cursorHookEntry{
		{Command: "./hooks/x.sh", LoopLimit: 7},
	})
	if v, ok := encoded[0]["loop_limit"].(int); !ok || v != 7 {
		t.Fatalf("loop_limit esperado 7, got %+v (type %T)", encoded[0]["loop_limit"], encoded[0]["loop_limit"])
	}
}

func TestDecodeCursorHookEntriesAcceptsFloatLoopLimit(t *testing.T) {
	raw := []interface{}{
		map[string]interface{}{
			"command":    "./hooks/x.sh",
			"loop_limit": float64(3),
		},
	}
	entries := decodeCursorHookEntries(raw)
	if len(entries) != 1 || entries[0].LoopLimit != 3 {
		t.Fatalf("decode float64 loop_limit falhou: %+v", entries)
	}
}
