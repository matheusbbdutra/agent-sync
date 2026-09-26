package hooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSmokeClineBridgeEndToEnd valida a cadeia completa do A-80.1 com os
// scripts REAIS do repo: payload Cline -> normalizacao -> bash-rm-guardian
// (com repo-map fake) -> traducao -> contrato Cline.
//
// Usa um binario repo-map fake para nao depender de build/estado externo; o
// smoke com o binario real fica em TestWiradoRealClineHook.
func TestSmokeClineBridgeEndToEnd(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	srcHooks := filepath.Join(repoRoot, "hooks")

	// Repo fake com o alvo do rm -rf.
	fakeRepo := t.TempDir()
	targetDir := filepath.Join(fakeRepo, "tools", "cmd", "memory-mcp")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "store.go"), []byte("package m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeRepo, "README.md"), []byte("# tools/cmd/memory-mcp\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// repo-map fake: devolve ledger com blockers (formato do audit-removal).
	fakeBin := t.TempDir()
	fakeRepoMap := filepath.Join(fakeBin, "repo-map")
	if err := os.WriteFile(fakeRepoMap, []byte(`#!/usr/bin/env bash
printf '%s' '{"verdict":{"status":"needs-review","blockers":["2 active docs-active hit(s) remain","1 active unknown-literal-hit hit(s) remain"]}}'
`), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_SYNC_REPO_MAP", fakeRepoMap)

	// baseDir com apenas os scripts relevantes (evita efeitos colaterais dos
	// demais hooks e mantem o smoke deterministico).
	baseDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bash-rm-guardian.pretooluse.sh", "bash-rm-guardian.sh"} {
		data, err := os.ReadFile(filepath.Join(srcHooks, name))
		if err != nil {
			t.Fatalf("ler %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(baseDir, "hooks", name), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	payload := `{"taskId":"ses-smoke-cline","iteration":1,
		"workspaceRoots":["` + fakeRepo + `"],
		"tool_call":{"id":"c1","name":"execute_command","input":{"command":"rm -rf tools/cmd/memory-mcp"}},
		"preToolUse":{"toolName":"execute_command","parameters":"{\"command\":\"rm -rf tools/cmd/memory-mcp\"}"}}`

	var stdout, stderr bytes.Buffer
	if err := RunClineBridge(
		[]string{"--event=PreToolUse", "--base-dir=" + baseDir},
		strings.NewReader(payload), &stdout, &stderr); err != nil {
		t.Fatalf("RunClineBridge: %v (stderr=%s)", err, stderr.String())
	}

	var resp clineHookResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout nao e JSON: %s (stderr=%s)", stdout.String(), stderr.String())
	}
	if resp.Cancel {
		t.Errorf("bash-rm-guardian nunca deve cancelar (decisao do user): %s", stdout.String())
	}
	for _, want := range []string{"bash-rm-guardian", "tools/cmd/memory-mcp", "docs-active"} {
		if !strings.Contains(resp.Context, want) {
			t.Errorf("contexto sem %q: %s", want, stdout.String())
		}
	}
	t.Logf("smoke context: %s", resp.Context)
}

// TestClineHooksRemovidosNaoReferenciamAdapterLegado guarda contra a
// re-introducao do adapter .cline.sh do A-79 (substituido pelo bridge).
func TestClineHooksRemovidosNaoReferenciamAdapterLegado(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "hooks", "bash-rm-guardian.cline.sh")); err == nil {
		t.Errorf("hooks/bash-rm-guardian.cline.sh foi substituido pelo bridge (A-80.1) e nao deve voltar")
	}
}

// TestSmokeClinePluginAdapter valida o contrato JS<->Go: copia o plugin do
// repo para um tempdir, aponta agent-sync-config.json para o repo e roda o
// hook `beforeTool` do plugin com um payload de tool destrutiva, esperando
// que o adapter converta o context do bridge em `appendContext`.
//
// Pula quando node ou o binário agent-sync instalado não existem.
func TestSmokeClinePluginAdapter(t *testing.T) {
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node ausente")
	}
	home := os.Getenv("HOME")
	agentSync := filepath.Join(home, ".local", "bin", "agent-sync")
	if _, err := os.Stat(agentSync); err != nil {
		t.Skipf("binário agent-sync ausente em %s", agentSync)
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	pluginDir := filepath.Join(t.TempDir(), "cline-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index.js", "package.json", "plugin.json"} {
		data, rerr := os.ReadFile(filepath.Join(repoRoot, "cline-plugin", name))
		if rerr != nil {
			t.Fatalf("ler %s: %v", name, rerr)
		}
		if werr := os.WriteFile(filepath.Join(pluginDir, name), data, 0o644); werr != nil {
			t.Fatal(werr)
		}
	}
	cfg := `{"baseDir":"` + repoRoot + `","bin":"` + agentSync + `"}`
	if err := os.WriteFile(filepath.Join(pluginDir, "agent-sync-config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	payloadJSON := `{"snapshot":{"conversationId":"ses-plugin-smoke","iteration":1,"agentId":"a1"},"toolCall":{"toolCallId":"c1","toolName":"run_commands"},"input":{"commands":["rm -rf tools/cmd/memory-mcp"]}}`
	harness := fmt.Sprintf(`
const plugin = require(%q);
const context = JSON.parse(process.argv[1]);
const result = plugin.hooks.beforeTool(context);
console.log(JSON.stringify(result || null));
`, filepath.Join(pluginDir, "index.js"))

	// repo-map fake: sem ele o bash-rm-guardian não produz contexto e o teste
	// fica dependente de AGENT_SYNC_REPO_MAP no ambiente (ou de /tmp/repo-map-mcp).
	fakeRepoMap := filepath.Join(t.TempDir(), "repo-map")
	if err := os.WriteFile(fakeRepoMap, []byte(`#!/usr/bin/env bash
printf '%s' '{"verdict":{"status":"needs-review","blockers":["2 active docs-active hit(s) remain"]}}'
`), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_SYNC_REPO_MAP", fakeRepoMap)

	cmd := exec.Command(nodeBin, "-e", harness, payloadJSON)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"AGENT_SYNC_BIN="+agentSync,
		"AGENT_SYNC_HOME="+repoRoot,
		"AGENT_SYNC_REPO_MAP="+envOr("AGENT_SYNC_REPO_MAP", fakeRepoMap),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("adapter do plugin falhou: %v\noutput: %s", err, out)
	}
	t.Logf("plugin output: %s", strings.TrimSpace(string(out)))
	var result struct {
		AppendContext string `json:"appendContext"`
		Skip          bool   `json:"skip"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), &result); err != nil {
		t.Fatalf("saida do plugin nao e JSON: %v\n%s", err, out)
	}
	if result.Skip {
		t.Errorf("bash-rm-guardian nao deve bloquear a tool")
	}
	if !strings.Contains(result.AppendContext, "bash-rm-guardian") {
		t.Errorf("appendContext sem o aviso do guardian: %q", result.AppendContext)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
