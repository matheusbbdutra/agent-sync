package main

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestMCPAuditRemoval lista as tools MCP expostas e valida que audit_removal
// está presente com schema correto. Test end-to-end (tools/call) está fora
// de escopo deste test porque depende de cache priming e tem flake —
// o smoke da CLI (`--audit-removal`) cobre o caminho funcional.
func TestMCPAuditRemoval(t *testing.T) {
	binPath := os.Getenv("AGENT_SYNC_REPO_MAP")
	if binPath == "" {
		binPath = "/tmp/repo-map-mcp"
	}
	if _, err := os.Stat(binPath); err != nil {
		t.Skipf("binário %s não encontrado", binPath)
	}

	root := t.TempDir()
	cacheDir := root + "/.agent-sync/cache"
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "--root", root, "--cache-dir", cacheDir, "--mcp")
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()

	w := bufio.NewWriter(stdin)
	r := bufio.NewReader(stdout)

	// initialize + tools/list
	writeRPC(t, w, 1, "initialize", nil)
	writeRPC(t, w, 2, "tools/list", nil)
	w.Flush()

	// Lê 2 respostas
	deadline := time.Now().Add(3 * time.Second)
	var initResp, listResp map[string]any
	for time.Now().Before(deadline) && (initResp == nil || listResp == nil) {
		resp := readRPC(t, r)
		if len(resp) == 0 {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		idF, _ := resp["id"].(float64)
		switch int(idF) {
		case 1:
			initResp = resp
		case 2:
			listResp = resp
		}
	}
	if initResp == nil || listResp == nil {
		t.Fatalf("initialize ou tools/list sem resposta: init=%v list=%v", initResp, listResp)
	}

	result, _ := listResp["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	if len(tools) == 0 {
		t.Fatalf("tools/list vazio")
	}

	var found map[string]any
	for _, tool := range tools {
		if t, ok := tool.(map[string]any); ok {
			if name, _ := t["name"].(string); name == "audit_removal" {
				found = t
				break
			}
		}
	}
	if found == nil {
		t.Fatalf("audit_removal não está em tools/list (tools=%d)", len(tools))
	}
	schema, _ := found["inputSchema"].(map[string]any)
	required, _ := schema["required"].([]any)
	if len(required) == 0 {
		t.Errorf("audit_removal schema sem required")
	}
	hasTarget := false
	for _, r := range required {
		if s, _ := r.(string); s == "target" {
			hasTarget = true
		}
	}
	if !hasTarget {
		t.Errorf("audit_removal schema sem 'target' em required")
	}
}

func writeRPC(t *testing.T, w *bufio.Writer, id int, method string, params any) {
	t.Helper()
	type rpc struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params,omitempty"`
	}
	data, _ := json.Marshal(rpc{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	data = append(data, '\n')
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
}

func readRPC(t *testing.T, r io.Reader) map[string]any {
	t.Helper()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	if !scanner.Scan() {
		return nil
	}
	var resp map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Logf("parse error: %v", err)
		return nil
	}
	return resp
}

func mustWriteRM(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
