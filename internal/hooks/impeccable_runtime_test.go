package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newImpeccableTestHome(t *testing.T) (home, cache string) {
	t.Helper()
	home = t.TempDir()
	cache = filepath.Join(home, ".cache")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("AGENT_SYNC_IMPECCABLE_SKILL", "")
	return home, cache
}

func writeImpeccableFakeSkill(t *testing.T, path, additionalContext string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "PostToolUse",
			"additionalContext": additionalContext,
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/usr/bin/env bash\ncat >/dev/null\nprintf '%s' '" + string(encoded) + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeImpeccableProject(t *testing.T) (root, file string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".impeccable"), 0o755); err != nil {
		t.Fatal(err)
	}
	file = filepath.Join(root, "Bad.tsx")
	if err := os.WriteFile(file, []byte("export const Bad = () => null\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, file
}

func TestImpeccableOpenCodeInjectsFinding(t *testing.T) {
	home, _ := newImpeccableTestHome(t)
	root, file := writeImpeccableProject(t)
	writeImpeccableFakeSkill(t, filepath.Join(home, ".config", "opencode", "skills", "impeccable", "scripts", "impeccable"), "[impeccable] gradient-text in Bad.tsx")

	input, _ := json.Marshal(map[string]any{
		"session_id": "s1",
		"cwd":        root,
		"tool_name":  "edit",
		"tool_input": map[string]any{"filePath": file},
	})

	out := handleImpeccableEvent("opencode", input)
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output invalido: %s", out)
	}
	contextText, _ := parsed["context"].(string)
	if !strings.Contains(contextText, "gradient-text") {
		t.Fatalf("context sem finding: %s", out)
	}
}

func TestImpeccableOpenCodeDropsCleanAck(t *testing.T) {
	home, _ := newImpeccableTestHome(t)
	root, file := writeImpeccableProject(t)
	writeImpeccableFakeSkill(t, filepath.Join(home, ".config", "opencode", "skills", "impeccable", "scripts", "impeccable"),
		"[impeccable@1] Design hook scanned Bad.tsx. No deterministic design-quality issues found.")

	input, _ := json.Marshal(map[string]any{
		"session_id": "s1",
		"cwd":        root,
		"tool_name":  "edit",
		"tool_input": map[string]any{"filePath": file},
	})

	if out := handleImpeccableEvent("opencode", input); string(out) != "{}" {
		t.Fatalf("ack limpo deveria virar {}: %s", out)
	}
}

func TestImpeccableOpenCodeRequiresProjectOptIn(t *testing.T) {
	home, _ := newImpeccableTestHome(t)
	root := t.TempDir()
	file := filepath.Join(root, "Bad.tsx")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(home, ".config", "opencode", "skills", "impeccable", "scripts", "impeccable")
	writeImpeccableFakeSkill(t, skill, "[impeccable] finding")

	input, _ := json.Marshal(map[string]any{
		"session_id": "s1",
		"cwd":        root,
		"tool_name":  "edit",
		"tool_input": map[string]any{"filePath": file},
	})

	if out := handleImpeccableEvent("opencode", input); string(out) != "{}" {
		t.Fatalf("projeto sem opt-in deveria virar {}: %s", out)
	}
}

func TestImpeccableOpenCodeSkipsNonEditTool(t *testing.T) {
	home, _ := newImpeccableTestHome(t)
	root, file := writeImpeccableProject(t)
	writeImpeccableFakeSkill(t, filepath.Join(home, ".config", "opencode", "skills", "impeccable", "scripts", "impeccable"), "[impeccable] finding")

	input, _ := json.Marshal(map[string]any{
		"session_id": "s1",
		"cwd":        root,
		"tool_name":  "read",
		"tool_input": map[string]any{"filePath": file},
	})

	if out := handleImpeccableEvent("opencode", input); string(out) != "{}" {
		t.Fatalf("read nao deveria escanear: %s", out)
	}
}

func TestImpeccableAntigravityPostThenPreInvocation(t *testing.T) {
	home, _ := newImpeccableTestHome(t)
	root, file := writeImpeccableProject(t)
	writeImpeccableFakeSkill(t, filepath.Join(home, ".gemini", "config", "skills", "impeccable", "scripts", "impeccable"), "[impeccable] side-tab in Bad.tsx")

	post, _ := json.Marshal(map[string]any{
		"conversationId": "conv1",
		"cwd":            root,
		"toolCall": map[string]any{
			"name": "write_file",
			"args": map[string]any{"path": file},
		},
	})
	if out := handleImpeccableEvent("antigravity", post); string(out) != "{}" {
		t.Fatalf("PostToolUse deveria responder {}: %s", out)
	}

	pre, _ := json.Marshal(map[string]any{
		"conversationId": "conv1",
		"cwd":            root,
		"invocationNum":  2,
	})
	out := handleImpeccableEvent("antigravity", pre)
	var parsed struct {
		InjectSteps []struct {
			SystemMessage struct {
				SystemMessage string `json:"systemMessage"`
			} `json:"systemMessage"`
		} `json:"injectSteps"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output invalido: %s", out)
	}
	if len(parsed.InjectSteps) != 1 || !strings.Contains(parsed.InjectSteps[0].SystemMessage.SystemMessage, "side-tab") {
		t.Fatalf("injectSteps sem finding: %s", out)
	}

	if got := readImpeccablePending("conv1"); got != "" {
		t.Fatalf("pending deveria ter sido drenado, ainda tem: %q", got)
	}
}

func TestImpeccableAntigravityPendingExpires(t *testing.T) {
	_, _ = newImpeccableTestHome(t)
	dir := impeccablePendingDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(file, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(file, old, old); err != nil {
		t.Fatal(err)
	}
	if got := readImpeccablePending("old"); got != "" {
		t.Fatalf("pending expirado deveria ser ignorado: %q", got)
	}
}

func TestImpeccableMissingSkillIsNoop(t *testing.T) {
	_, _ = newImpeccableTestHome(t)
	root, file := writeImpeccableProject(t)

	input, _ := json.Marshal(map[string]any{
		"session_id": "s1",
		"cwd":        root,
		"tool_name":  "edit",
		"tool_input": map[string]any{"filePath": file},
	})
	if out := handleImpeccableEvent("opencode", input); string(out) != "{}" {
		t.Fatalf("skill ausente deveria virar {}: %s", out)
	}
}

func TestImpeccableMalformedInputIsNoop(t *testing.T) {
	if out := handleImpeccableEvent("opencode", []byte("{")); string(out) != "{}" {
		t.Fatalf("json invalido deveria virar {}: %s", out)
	}
}

func TestExtractImpeccablePaths(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "A.tsx")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		tool  string
		input map[string]any
		want  bool
	}{
		{"camelCase", "edit", map[string]any{"filePath": file}, true},
		{"antigravity", "write_file", map[string]any{"TargetFile": file}, true},
		{"relative", "edit", map[string]any{"path": "A.tsx"}, true},
		{"read tool", "read_file", map[string]any{"path": file}, false},
		{"no path", "edit", map[string]any{}, false},
		{"directory", "edit", map[string]any{"path": root}, false},
		{"apply patch", "apply_patch", map[string]any{"patchText": "*** Update File: A.tsx\n"}, true},
		{"patch alias", "patch", map[string]any{"patchText": "*** Update File: A.tsx\n"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractImpeccablePaths(tc.tool, tc.input, root)
			if (len(got) > 0) != tc.want {
				t.Fatalf("paths=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestSanitizeImpeccableSession(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"../../etc/passwd", "_.._etc_passwd"},
		{"", "default"},
		{"...", "default"},
		{"conv-123_ABC.4", "conv-123_ABC.4"},
	} {
		if got := sanitizeImpeccableSession(tc.in); got != tc.want {
			t.Errorf("sanitize(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
