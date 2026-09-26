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

// TestSmokeClineEndToEnd wirar bash-rm-guardian em ~/.cline/hooks/PreToolUse
// e valida que o script emite context warn quando rm -rf em dir com refs.
func TestSmokeClineEndToEnd(t *testing.T) {
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

	// BaseDir com scripts
	baseDir := filepath.Join(root, "base")
	hooksDir := filepath.Join(baseDir, "hooks")
	_ = os.MkdirAll(hooksDir, 0o755)
	for _, name := range []string{"bash-rm-guardian.cline.sh", "bash-rm-guardian.sh"} {
		// caminho absoluto (cwd do go test é tools/)
		src := "/home/matheus_dutra/Projects/agent-sync/.claude/worktrees/a73-audit-removal-go/hooks/" + name
		if data, err := os.ReadFile(src); err == nil {
			_ = os.WriteFile(filepath.Join(hooksDir, name), data, 0o755)
		} else {
			t.Logf("falha ao ler %s: %v", src, err)
		}
	}

	// HooksSettingsPath (simula ~/.cline/hooks/)
	clineHooksDir := filepath.Join(root, ".cline", "hooks")
	_ = os.MkdirAll(clineHooksDir, 0o755)

	tgt := target.TargetCLI{
		AgentKind:         "cline",
		HooksSettingsPath: clineHooksDir,
		HooksEvent:        "PreToolUse",
		HooksFormat:       "cline",
	}
	if err := syncBashRmGuardianCline(baseDir, tgt); err != nil {
		t.Fatalf("syncBashRmGuardianCline: %v", err)
	}

	// Valida que PreToolUse foi criado
	wiradoScript := filepath.Join(clineHooksDir, "PreToolUse")
	if _, err := os.Stat(wiradoScript); err != nil {
		t.Fatalf("PreToolUse não wirado: %v", err)
	}
	t.Logf("wiradoScript=%s", wiradoScript)
	if data, err := os.ReadFile(wiradoScript); err == nil {
		t.Logf("script content (first 500 bytes):\n%s", string(data[:min(500, len(data))]))
	}

	// Executa com payload Cline
	payload := map[string]any{
		"tool":  "Bash",
		"input": map[string]any{"command": "rm -rf tools/cmd/memory-mcp"},
	}
	payloadBytes, _ := json.Marshal(payload)

	cmd := exec.Command(wiradoScript)
	cmd.Dir = root
	cmd.Env = []string{
		"PATH=/tmp:/usr/local/bin:/usr/bin:/bin",
		"AGENT_SYNC_ROOT=" + root,
		"AGENT_SYNC_REPO_MAP=" + repoMapPath,
		"HOME=/tmp",
	}
	cmd.Stdin = bytes.NewReader(payloadBytes)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("script exit %v\nstderr: %s\nstdout: %s", err, stderr, stdout)
	}

	out := stdout.String()
	if !strings.Contains(out, `"cancel": false`) && !strings.Contains(out, `"cancel":false`) {
		t.Errorf("output sem cancel:false: %s\nstderr: %s", out, stderr)
	}
	if !strings.Contains(out, "context") {
		t.Errorf("output sem context: %s\nstderr: %s", out, stderr)
	}
	if !strings.Contains(out, "bash-rm-guardian") {
		t.Errorf("output sem mensagem: %s\nstderr: %s", out, stderr)
	}
	if !strings.Contains(out, "tools/cmd/memory-mcp") {
		t.Errorf("output sem target: %s\nstderr: %s", out, stderr)
	}
}
