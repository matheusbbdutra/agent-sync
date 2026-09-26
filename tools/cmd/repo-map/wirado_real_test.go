package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestWiradoRealClineHook(t *testing.T) {
	cmd := exec.Command("bash", "/home/matheus_dutra/.cline/hooks/PreToolUse")
	cmd.Dir = "/home/matheus_dutra/Projects/agent-sync"
	cmd.Env = []string{
		"PATH=/tmp:/usr/local/bin:/usr/bin:/bin",
		"AGENT_SYNC_ROOT=/home/matheus_dutra/Projects/agent-sync",
		"AGENT_SYNC_REPO_MAP=/tmp/repo-map-mcp",
		"HOME=/tmp",
	}
	cmd.Stdin = strings.NewReader(`{"tool":"Bash","input":{"command":"rm -rf tools/cmd/memory-mcp"}}`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script exit %v\noutput: %s", err, out)
	}
	t.Logf("wirado real output:\n%s", string(out))
	if !strings.Contains(string(out), "bash-rm-guardian") {
		t.Errorf("wiramento real falhou - context sem mensagem:\n%s", out)
	}
}
