package hooks

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matheusdutra/agent-sync/internal/target"
)

// TestSmokeClaudeCodeEndToEnd wirar bash-rm-guardian em formato Claude Code
// (settings.json + PreToolUse nested) e validar que stdout do script contém
// `hookSpecificOutput.additionalContext` warn quando rm -rf em dir com refs.
func TestSmokeClaudeCodeEndToEnd(t *testing.T) {
	repoMapPath := os.Getenv("AGENT_SYNC_REPO_MAP")
	if repoMapPath == "" {
		repoMapPath = "/tmp/repo-map-mcp"
	}
	if _, err := os.Stat(repoMapPath); err != nil {
		t.Skipf("repo-map não encontrado em %s", repoMapPath)
	}

	root := t.TempDir()

	// Repo fake
	targetDir := filepath.Join(root, "tools", "cmd", "memory-mcp")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(targetDir, "store.go"), []byte("package m\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "README.md"), []byte("# tools/cmd/memory-mcp\n"), 0o644)

	// BaseDir com hooks
	baseDir := filepath.Join(root, "base")
	hooksDir := filepath.Join(baseDir, "hooks")
	_ = os.MkdirAll(hooksDir, 0o755)
	wd, _ := os.Getwd()
	for _, name := range []string{"bash-rm-guardian.pretooluse.sh", "bash-rm-guardian.sh", "wrap-hook.sh"} {
		src := filepath.Join(wd, "..", "..", "hooks", name)
		if data, err := os.ReadFile(src); err == nil {
			_ = os.WriteFile(filepath.Join(hooksDir, name), data, 0o755)
		}
	}

	// settings.json (Claude Code format)
	settingsJSON := filepath.Join(root, ".claude", "settings.json")
	_ = os.MkdirAll(filepath.Dir(settingsJSON), 0o755)
	_ = os.WriteFile(settingsJSON, []byte(`{"hooks":{}}`), 0o644)

	tgt := target.TargetCLI{
		AgentKind:         "claude",
		HooksSettingsPath: settingsJSON,
		HooksEvent:        "PreToolUse",
		HooksFormat:       "",
	}
	if err := syncBashRmGuardianStandard(baseDir, tgt); err != nil {
		t.Fatalf("syncBashRmGuardianStandard: %v", err)
	}

	// Verifica JSON wirado
	raw, _ := os.ReadFile(settingsJSON)
	var parsed map[string]any
	_ = json.Unmarshal(raw, &parsed)
	hooks, _ := parsed["hooks"].(map[string]any)
	pre, _ := hooks["PreToolUse"].([]any)
	if len(pre) == 0 {
		t.Fatalf("PreToolUse não wirado: %s", raw)
	}
	first, _ := pre[0].(map[string]any)
	if hooksList, ok := first["hooks"].([]any); !ok || len(hooksList) == 0 {
		t.Fatalf("hooks[] vazio: %+v", first)
	}

	// Extrai script wirado (caminho absoluto via wrap-hook.sh com baseDir)
	hooksArr, _ := first["hooks"].([]any)
	h0, _ := hooksArr[0].(map[string]any)
	wiradoCmd, _ := h0["command"].(string)
	parts := strings.Fields(wiradoCmd)
	scriptFinal := parts[len(parts)-1]
	if !filepath.IsAbs(scriptFinal) {
		// wrap-hook.sh resolve relativo a $(dirname $0); para o test,
		// caminho absoluto via baseDir/hooks
		scriptFinal = filepath.Join(baseDir, "hooks", scriptFinal)
	}

	// Payload Claude Code PreToolUse
	payload := map[string]any{
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": "rm -rf tools/cmd/memory-mcp",
		},
		"session_id": "test",
	}
	payloadBytes, _ := json.Marshal(payload)

	cmd := exec.Command("bash", scriptFinal)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"AGENT_SYNC_ROOT="+root,
		"AGENT_SYNC_REPO_MAP="+repoMapPath,
	)
	cmd.Stdin = bytes.NewReader(payloadBytes)
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = &bytes.Buffer{}
	if err := cmd.Run(); err != nil {
		t.Fatalf("script exit %v\nstdout: %s", err, stdout)
	}
	out := stdout.String()
	if !strings.Contains(out, "hookSpecificOutput") {
		t.Errorf("output sem hookSpecificOutput: %s", out)
	}
	if !strings.Contains(out, "additionalContext") {
		t.Errorf("output sem additionalContext: %s", out)
	}
	if !strings.Contains(out, "bash-rm-guardian") {
		t.Errorf("output sem mensagem bash-rm-guardian: %s", out)
	}
	if !strings.Contains(out, "tools/cmd/memory-mcp") {
		t.Errorf("output sem target: %s", out)
	}
}

// TestSmokeCursorEndToEnd wirar bash-rm-guardian em formato Cursor
// (hooks.json + beforeShellExecution) e validar output.
func TestSmokeCursorEndToEnd(t *testing.T) {
	repoMapPath := os.Getenv("AGENT_SYNC_REPO_MAP")
	if repoMapPath == "" {
		repoMapPath = "/tmp/repo-map-mcp"
	}
	if _, err := os.Stat(repoMapPath); err != nil {
		t.Skipf("repo-map não encontrado em %s", repoMapPath)
	}

	root := t.TempDir()

	// Repo fake
	targetDir := filepath.Join(root, "tools", "cmd", "memory-mcp")
	_ = os.MkdirAll(targetDir, 0o755)
	_ = os.WriteFile(filepath.Join(targetDir, "store.go"), []byte("package m\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "README.md"), []byte("# tools/cmd/memory-mcp\n"), 0o644)

	// BaseDir — copiar TODOS os hooks/scripts que syncCursorAll exige
	baseDir := filepath.Join(root, "base")
	hooksDir := filepath.Join(baseDir, "hooks")
	_ = os.MkdirAll(hooksDir, 0o755)
	wd, _ := os.Getwd()
	allHooks, _ := os.ReadDir(filepath.Join(wd, "..", "..", "hooks"))
	for _, h := range allHooks {
		src := filepath.Join(wd, "..", "..", "hooks", h.Name())
		if data, err := os.ReadFile(src); err == nil {
			mode := os.FileMode(0o644)
			if strings.HasSuffix(h.Name(), ".sh") {
				mode = 0o755
			}
			_ = os.WriteFile(filepath.Join(hooksDir, h.Name()), data, mode)
		}
	}

	// Cursor hooks.json
	cursorJSON := filepath.Join(root, ".cursor", "hooks.json")
	_ = os.MkdirAll(filepath.Dir(cursorJSON), 0o755)
	_ = os.WriteFile(cursorJSON, []byte(`{"version":1,"hooks":{}}`), 0o644)

	tgt := target.TargetCLI{
		AgentKind:         "cursor",
		HooksSettingsPath: cursorJSON,
		HooksFormat:       "cursor",
	}
	if err := syncCursorAll(baseDir, tgt); err != nil {
		t.Fatalf("syncCursorAll: %v", err)
	}

	// Verifica wiramento
	raw, _ := os.ReadFile(cursorJSON)
	var parsed map[string]any
	_ = json.Unmarshal(raw, &parsed)
	hooks, _ := parsed["hooks"].(map[string]any)
	before, _ := hooks["beforeShellExecution"].([]any)
	if len(before) < 2 {
		t.Fatalf("beforeShellExecution esperado >=2 entries (bash-guardian + bash-rm-guardian); obteve %d", len(before))
	}
	var wiradoCmd string
	for _, e := range before {
		m, _ := e.(map[string]any)
		cmdStr, _ := m["command"].(string)
		if strings.Contains(cmdStr, "bash-rm-guardian") {
			wiradoCmd = cmdStr
			break
		}
	}
	if wiradoCmd == "" {
		t.Fatal("bash-rm-guardian não wirado em beforeShellExecution")
	}
	parts := strings.Fields(wiradoCmd)
	scriptFinal := parts[len(parts)-1]
	if !filepath.IsAbs(scriptFinal) {
		// script está em ~/.cursor/hooks/ quando wirado — para o test,
		// caminho absoluto via baseDir/hooks
		scriptFinal = filepath.Join(baseDir, "hooks", scriptFinal)
	}

	// Payload Cursor beforeShellExecution
	payload := map[string]any{
		"command": "rm -rf tools/cmd/memory-mcp",
	}
	payloadBytes, _ := json.Marshal(payload)

	cmd := exec.Command("bash", scriptFinal)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"AGENT_SYNC_ROOT="+root,
		"AGENT_SYNC_REPO_MAP="+repoMapPath,
	)
	cmd.Stdin = bytes.NewReader(payloadBytes)
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = &bytes.Buffer{}
	if err := cmd.Run(); err != nil {
		t.Fatalf("script exit %v\nstdout: %s", err, stdout)
	}
	out := stdout.String()
	if !strings.Contains(out, `"permission":"allow"`) && !strings.Contains(out, `"permission": "allow"`) {
		t.Errorf("output sem permission=allow: %s", out)
	}
	if !strings.Contains(out, "bash-rm-guardian") {
		t.Errorf("output sem mensagem bash-rm-guardian: %s", out)
	}
}
