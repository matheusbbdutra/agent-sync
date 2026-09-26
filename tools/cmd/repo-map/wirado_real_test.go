package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestWiradoRealClineHook é o smoke de wiramento real do A-80.1: executa o
// shim wirado em ~/.cline/hooks/PreToolUse (que delega para
// `agent-sync hook cline`) com um payload Cline real e valida que o contexto
// do bash-rm-guardian chega no contrato do Cline.
//
// Depende de estado do host (hook wirado + binario agent-sync instalado +
// repo-map): pula quando qualquer peça falta, para não quebrar CI/outros PCs.
func TestWiradoRealClineHook(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("HOME indefinido")
	}
	shim := filepath.Join(home, ".cline", "hooks", "PreToolUse")
	if _, err := os.Stat(shim); err != nil {
		t.Skipf("hook Cline não wirado em %s (rode 'make apply')", shim)
	}

	bin := os.Getenv("AGENT_SYNC_BIN")
	if bin == "" {
		bin = filepath.Join(home, ".local", "bin", "agent-sync")
	}
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("binário agent-sync ausente em %s", bin)
	}

	repoMap := os.Getenv("AGENT_SYNC_REPO_MAP")
	if repoMap == "" {
		repoMap = "/tmp/repo-map-mcp"
	}
	if _, err := os.Stat(repoMap); err != nil {
		t.Skipf("repo-map com --audit-removal ausente em %s", repoMap)
	}

	root := os.Getenv("AGENT_SYNC_ROOT")
	if root == "" {
		root = "/home/matheus_dutra/Projects/agent-sync"
	}
	if _, err := os.Stat(root); err != nil {
		t.Skipf("repo root ausente em %s", root)
	}

	payload := `{"taskId":"ses-wirado-real","iteration":1,"workspaceRoots":["` + root + `"],
		"tool_call":{"id":"c1","name":"execute_command","input":{"command":"rm -rf tools/cmd/memory-mcp"}},
		"preToolUse":{"toolName":"execute_command","parameters":"{\"command\":\"rm -rf tools/cmd/memory-mcp\"}"}}`

	cmd := exec.Command("bash", shim)
	cmd.Dir = root
	cmd.Env = []string{
		"PATH=" + filepath.Dir(bin) + ":/usr/local/bin:/usr/bin:/bin",
		"AGENT_SYNC_BIN=" + bin,
		"AGENT_SYNC_HOME=" + root,
		"AGENT_SYNC_REPO_MAP=" + repoMap,
		"HOME=" + home,
	}
	cmd.Stdin = strings.NewReader(payload)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shim exit %v\noutput: %s", err, out)
	}

	// O stdout deve ser exatamente um JSON (o Cline faz JSON.parse do stdout).
	trimmed := strings.TrimSpace(string(out))
	var resp struct {
		Cancel  bool   `json:"cancel"`
		Context string `json:"context"`
	}
	if err := json.Unmarshal([]byte(trimmed), &resp); err != nil {
		t.Fatalf("stdout do shim não é JSON: %v\noutput: %s", err, out)
	}
	t.Logf("wirado real output:\n%s", trimmed)
	if resp.Cancel {
		t.Errorf("bash-rm-guardian não deve cancelar: %s", trimmed)
	}
	for _, want := range []string{"bash-rm-guardian", "tools/cmd/memory-mcp"} {
		if !strings.Contains(resp.Context, want) {
			t.Errorf("contexto sem %q:\n%s", want, trimmed)
		}
	}
}
