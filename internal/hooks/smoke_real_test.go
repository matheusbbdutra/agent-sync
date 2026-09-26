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

// TestSmokeRealAntigravityEndToEnd orquestra o cenário real:
//  1. Cria repo fake com refs a "tools/cmd/memory-mcp"
//  2. Wira o hook em ~/.gemini/config/hooks.json (formato antigravity)
//  3. Simula payload PreToolUse:Bash para o script wirado com `rm -rf`
//  4. Verifica stdout do script: deve ter decision=allow + injectSteps com blockers
func TestSmokeRealAntigravityEndToEnd(t *testing.T) {
	repoMapPath := os.Getenv("AGENT_SYNC_REPO_MAP")
	if repoMapPath == "" {
		repoMapPath = "/tmp/repo-map-mcp"
	}
	if _, err := os.Stat(repoMapPath); err != nil {
		t.Skipf("repo-map não encontrado em %s; defina AGENT_SYNC_REPO_MAP", repoMapPath)
	}

	root := t.TempDir()

	// 1. Repo fake com refs
	targetDir := filepath.Join(root, "tools", "cmd", "memory-mcp")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "store.go"), []byte("package m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# tools/cmd/memory-mcp\nreferencias aqui\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Wiramento
	baseDir := filepath.Join(root, "base")
	hooksDir := filepath.Join(baseDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	srcScript := filepath.Join(wd, "..", "..", "hooks", "bash-rm-guardian.antigravity.sh")
	if data, err := os.ReadFile(srcScript); err == nil {
		_ = os.WriteFile(filepath.Join(hooksDir, "bash-rm-guardian.antigravity.sh"), data, 0o755)
	}
	srcCore := filepath.Join(wd, "..", "..", "hooks", "bash-rm-guardian.sh")
	coreData, err := os.ReadFile(srcCore)
	if err != nil {
		t.Skipf("core script não encontrado: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "bash-rm-guardian.sh"), coreData, 0o755); err != nil {
		t.Fatal(err)
	}

	hooksJSON := filepath.Join(root, ".gemini", "config", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(hooksJSON), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hooksJSON, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	tgt := target.TargetCLI{
		AgentKind:         "antigravity",
		HooksSettingsPath: hooksJSON,
		HooksEvent:        "PreToolUse",
		HooksFormat:       "antigravity",
	}
	if err := syncBashRmGuardianAntigravity(baseDir, tgt); err != nil {
		t.Fatalf("wiramento: %v", err)
	}

	// 3. Extrai o command wirado
	raw, _ := os.ReadFile(hooksJSON)
	var parsed map[string]any
	_ = json.Unmarshal(raw, &parsed)
	hookEntry, _ := parsed[bashRmGuardianHookName].(map[string]any)
	pre, _ := hookEntry["PreToolUse"].([]any)
	first, _ := pre[0].(map[string]any)
	hooks, _ := first["hooks"].([]any)
	h0, _ := hooks[0].(map[string]any)
	wiradoCmd, _ := h0["command"].(string)
	if wiradoCmd == "" {
		t.Fatal("command wirado vazio")
	}

	// O command wirado tem a forma:
	//   <baseDir>/hooks/wrap-hook.sh PreToolUse <hookName> <baseDir>/hooks/bash-rm-guardian.antigravity.sh
	// Extrai o caminho do script final (último argumento)
	parts := strings.Fields(wiradoCmd)
	scriptFinal := parts[len(parts)-1]
	if _, err := os.Stat(scriptFinal); err != nil {
		t.Fatalf("script wirado não existe: %s", scriptFinal)
	}

	// 4. Simula payload PreToolUse:Bash para o script
	payload := map[string]any{
		"toolCall": map[string]any{
			"name": "run_command",
			"args": map[string]any{
				"CommandLine": "rm -rf tools/cmd/memory-mcp",
			},
		},
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
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("script exit %v\nstderr: %s\nstdout: %s", err, stderr, stdout)
	}

	// 5. Valida output
	outStr := stdout.String()
	if !strings.Contains(outStr, `"decision": "allow"`) && !strings.Contains(outStr, `"decision":"allow"`) {
		t.Errorf("output sem decision=allow: %s", outStr)
	}
	if !strings.Contains(outStr, "injectSteps") {
		t.Errorf("output sem injectSteps: %s", outStr)
	}
	if !strings.Contains(outStr, "bash-rm-guardian") {
		t.Errorf("output sem label bash-rm-guardian: %s", outStr)
	}
	if !strings.Contains(outStr, "tools/cmd/memory-mcp") {
		t.Errorf("output sem nome do target: %s", outStr)
	}

	// Também valida caso sem refs (rm arquivo único)
	payload2 := map[string]any{
		"toolCall": map[string]any{
			"name": "run_command",
			"args": map[string]any{
				"CommandLine": "rm foo.txt",
			},
		},
	}
	payloadBytes2, _ := json.Marshal(payload2)
	cmd2 := exec.Command("bash", scriptFinal)
	cmd2.Dir = root
	cmd2.Env = cmd.Env
	cmd2.Stdin = bytes.NewReader(payloadBytes2)
	stdout2 := &bytes.Buffer{}
	cmd2.Stdout = stdout2
	cmd2.Stderr = &bytes.Buffer{}
	if err := cmd2.Run(); err != nil {
		t.Fatalf("script 2 exit %v", err)
	}
	out2 := stdout2.String()
	if !strings.Contains(out2, `"decision": "allow"`) && !strings.Contains(out2, `"decision":"allow"`) {
		t.Errorf("rm arquivo único sem decision=allow: %s", out2)
	}
	if strings.Contains(out2, "injectSteps") {
		t.Errorf("rm arquivo único NÃO deveria ter injectSteps (sem refs): %s", out2)
	}
}
