package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/matheusdutra/token-tools/internal/docscache"
)

func TestInitialize(t *testing.T) {
	resp, ok := handle(rpcRequest{ID: json.RawMessage(`1`), Method: "initialize"}, t.TempDir())
	if !ok {
		t.Fatal("initialize deveria responder")
	}
	result, _ := resp.Result.(map[string]any)
	if result["protocolVersion"] != protocolVersion {
		t.Fatalf("protocolVersion inesperado: %v", result["protocolVersion"])
	}
}

func TestNotificationNoResponse(t *testing.T) {
	if _, ok := handle(rpcRequest{Method: "notifications/initialized"}, t.TempDir()); ok {
		t.Fatal("notificação não deveria gerar resposta")
	}
}

func TestToolsList(t *testing.T) {
	resp, _ := handle(rpcRequest{ID: json.RawMessage(`2`), Method: "tools/list"}, t.TempDir())
	result, _ := resp.Result.(map[string]any)
	tools, _ := result["tools"].([]map[string]any)
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool["name"].(string)] = true
	}
	if !names["search_docs"] || !names["list_docs"] {
		t.Fatalf("ferramentas esperadas ausentes: %v", names)
	}
}

func TestCallSearchDocs(t *testing.T) {
	dir := t.TempDir()
	if err := docscache.Save(dir, "https://symfony.com/doc/doctrine.html", "text/html", "<x/>", "O Doctrine usa lazy loading\nautowire no container"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	params, _ := json.Marshal(map[string]any{"name": "search_docs", "arguments": map[string]any{"query": "lazy"}})
	resp, _ := handle(rpcRequest{ID: json.RawMessage(`3`), Method: "tools/call", Params: params}, dir)

	result, _ := resp.Result.(map[string]any)
	content, _ := result["content"].([]map[string]any)
	if len(content) == 0 || !strings.Contains(content[0]["text"].(string), "lazy loading") {
		t.Fatalf("resultado inesperado: %+v", resp.Result)
	}
}

func TestCallListDocsEmpty(t *testing.T) {
	params, _ := json.Marshal(map[string]any{"name": "list_docs", "arguments": map[string]any{}})
	resp, _ := handle(rpcRequest{ID: json.RawMessage(`4`), Method: "tools/call", Params: params}, t.TempDir())
	result, _ := resp.Result.(map[string]any)
	content, _ := result["content"].([]map[string]any)
	if !strings.Contains(content[0]["text"].(string), "Cache vazio") {
		t.Fatalf("esperava aviso de cache vazio: %+v", content)
	}
}

func TestUnknownMethod(t *testing.T) {
	resp, ok := handle(rpcRequest{ID: json.RawMessage(`5`), Method: "nope"}, t.TempDir())
	if !ok || resp.Error == nil {
		t.Fatalf("esperava erro para método desconhecido: %+v", resp)
	}
}
