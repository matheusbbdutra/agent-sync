package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matheusdutra/token-tools/internal/repomap"
)

func TestMCPToolsListAndCall(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, ".agent-sync", "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}

	srcFile := filepath.Join(dir, "calc.go")
	if err := os.WriteFile(srcFile, []byte("package calc\n\nfunc Add(a, b int) int { return a + b }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	callerFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(callerFile, []byte("package main\n\nfunc Run() { _ = Add(1, 2) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Update cache
	_, _, err := repomap.Update(dir, cacheDir)
	if err != nil {
		t.Fatalf("update falhou: %v", err)
	}

	// 1. Test tools/list
	reqList := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/list",
	}
	resp, ok := handle(reqList, cacheDir, dir)
	if !ok || resp.Error != nil {
		t.Fatalf("tools/list falhou: %+v", resp)
	}
	toolsMap, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("resultado inválido de tools/list: %+v", resp.Result)
	}
	toolsList, ok := toolsMap["tools"].([]map[string]any)
	if !ok || len(toolsList) != 3 {
		t.Fatalf("esperava 3 tools no MCP, obteve %d", len(toolsList))
	}

	// 2. Test get_file_impact
	callArgs, _ := json.Marshal(map[string]any{
		"path": "calc.go",
	})
	reqCall := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "get_file_impact", "arguments": ` + string(callArgs) + `}`),
	}
	respCall, ok := handle(reqCall, cacheDir, dir)
	if !ok || respCall.Error != nil {
		t.Fatalf("get_file_impact call falhou: %+v", respCall)
	}
	resMap := respCall.Result.(map[string]any)
	content := resMap["content"].([]map[string]any)[0]["text"].(string)
	if !strings.Contains(content, `"target_file": "calc.go"`) {
		t.Errorf("resposta inesperada de get_file_impact: %s", content)
	}
	if !strings.Contains(content, `"blast_radius":`) {
		t.Errorf("esperava blast_radius em get_file_impact: %s", content)
	}

	// 3. Test get_symbol_callers
	symArgs, _ := json.Marshal(map[string]any{
		"symbol": "Add",
	})
	reqSym := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "get_symbol_callers", "arguments": ` + string(symArgs) + `}`),
	}
	respSym, ok := handle(reqSym, cacheDir, dir)
	if !ok || respSym.Error != nil {
		t.Fatalf("get_symbol_callers call falhou: %+v", respSym)
	}
	symContent := respSym.Result.(map[string]any)["content"].([]map[string]any)[0]["text"].(string)
	if !strings.Contains(symContent, `"main.go"`) {
		t.Errorf("esperava main.go como caller de Add: %s", symContent)
	}

	// 4. Test repo_summary
	reqSum := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`4`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "repo_summary", "arguments": {}}`),
	}
	respSum, ok := handle(reqSum, cacheDir, dir)
	if !ok || respSum.Error != nil {
		t.Fatalf("repo_summary call falhou: %+v", respSum)
	}
	sumContent := respSum.Result.(map[string]any)["content"].([]map[string]any)[0]["text"].(string)
	if !strings.Contains(sumContent, "Top hubs") {
		t.Errorf("esperava Top hubs em repo_summary: %s", sumContent)
	}
}
