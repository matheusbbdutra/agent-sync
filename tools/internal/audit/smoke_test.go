package audit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSmokeBashRmGuardian valida o script core via subprocesso, simulando
// 4 cenários: rm -rf em dir com refs, rm em arquivo único, comando neutro,
// mv de diretório.
func TestSmokeBashRmGuardian(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash não disponível")
	}

	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "README.md"), "# tools/cmd/memory-mcp\nreferencias aqui\n")
	mustWrite(t, filepath.Join(root, "main.go"), "package main\n// refers tools/cmd/memory-mcp\n")
	if err := os.MkdirAll(filepath.Join(root, "tools", "cmd", "memory-mcp"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "tools", "cmd", "memory-mcp", "store.go"), "package m\n")

	corePath := findCoreScript(t)
	if corePath == "" {
		t.Skip("script bash-rm-guardian.sh não encontrado no repo")
	}

	// Localiza binário repo-map: prioriza AGENT_SYNC_REPO_MAP no env, senão
	// procura em paths comuns (./repo-map, $REPO_MAP, /tmp/repo-map).
	repoMapPath := os.Getenv("AGENT_SYNC_REPO_MAP")
	if repoMapPath == "" {
		candidates := []string{"./repo-map", "/tmp/repo-map", "/usr/local/bin/repo-map"}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				repoMapPath = c
				break
			}
		}
	}
	if repoMapPath == "" {
		t.Skip("repo-map binário não encontrado; defina AGENT_SYNC_REPO_MAP=/path/to/repo-map")
	}

	cases := []struct {
		name     string
		cmd      string
		wantSubs []string
	}{
		{
			name:     "rm -rf em dir com refs",
			cmd:      "rm -rf tools/cmd/memory-mcp",
			wantSubs: []string{`"blockers":`, `"audit-found-refs"`},
		},
		{
			name:     "rm arquivo único (sem target destrutivo)",
			cmd:      "rm foo.txt",
			wantSubs: []string{`"blockers":[]`, `"no-destructive-target"`},
		},
		{
			name:     "comando neutro (ls)",
			cmd:      "ls -la",
			wantSubs: []string{`"blockers":[]`, `"no-destructive-target"`},
		},
		{
			name:     "mv de diretório com refs",
			cmd:      "mv tools/cmd/memory-mcp /tmp/x",
			wantSubs: []string{`"blockers":`, `"audit-found-refs"`},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := runCore(corePath, repoMapPath, root, c.cmd)
			if err != nil {
				t.Fatalf("runCore: %v\nstdout: %s", err, out)
			}
			for _, sub := range c.wantSubs {
				if !strings.Contains(out, sub) {
					t.Errorf("output missing %q\ngot: %s", sub, out)
				}
			}
		})
	}
}

func runCore(corePath, repoMapPath, root, cmd string) (string, error) {
	c := exec.Command("bash", corePath, "analyze", cmd)
	c.Env = append(os.Environ(),
		"AGENT_SYNC_ROOT="+root,
		"AGENT_SYNC_REPO_MAP="+repoMapPath,
	)
	out, err := c.CombinedOutput()
	return string(out), err
}

// findCoreScript sobe do diretório de testes até encontrar hooks/bash-rm-guardian.sh
func findCoreScript(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	for dir := wd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "hooks", "bash-rm-guardian.sh")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
