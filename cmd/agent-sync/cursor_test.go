package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncCursorRulesWrapsFrontmatter(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "global-rules.md")
	dst := filepath.Join(dir, "rules", "agent-sync-global.mdc")
	if err := os.WriteFile(src, []byte("# Hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := syncCursorRules(src, dst); err != nil {
		t.Fatalf("syncCursorRules: %v", err)
	}
	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(raw)
	for _, want := range []string{"alwaysApply: true", "# Hello"} {
		if !strings.Contains(content, want) {
			t.Errorf("faltando %q em:\n%s", want, content)
		}
	}
}

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
	if len(post) != 4 {
		t.Fatalf("esperava 4 postToolUse gerenciados, got %d: %+v", len(post), post)
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
