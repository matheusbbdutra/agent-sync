package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractClaudeCodexWebFetch(t *testing.T) {
	raw := []byte(`{"tool_name":"WebFetch","tool_input":{"url":"https://example.com/a"},"tool_response":{"result":"hello doc"}}`)
	url, text := extractClaudeCodex(raw)
	if url != "https://example.com/a" || text != "hello doc" {
		t.Fatalf("got (%q,%q)", url, text)
	}
}

func TestExtractClaudeCodexQueryDocs(t *testing.T) {
	raw := []byte(`{"tool_name":"mcp__context7__query-docs","tool_input":{"libraryId":"/a/b","query":"q"},"tool_response":"body text"}`)
	url, text := extractClaudeCodex(raw)
	if url != "context7://a/b/q" || text != "body text" {
		t.Fatalf("got (%q,%q)", url, text)
	}
}

func TestExtractClaudeCodexNonDoc(t *testing.T) {
	raw := []byte(`{"tool_name":"Bash","tool_input":{"command":"ls"},"tool_response":"x"}`)
	if url, text := extractClaudeCodex(raw); url != "" || text != "" {
		t.Fatalf("esperava vazio, got (%q,%q)", url, text)
	}
}

func TestExtractAntigravityReadURL(t *testing.T) {
	dir := t.TempDir()
	tp := filepath.Join(dir, "transcript.jsonl")
	content := "{\"step_index\":1,\"type\":\"GENERIC\",\"content\":\"doc body\"}\n"
	if err := os.WriteFile(tp, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"toolCall":{"name":"read_url_content","args":{"Url":"https://example.com/b"}},"stepIdx":0,"transcriptPath":"` + tp + `"}`)
	url, text := extractAntigravity(raw)
	if url != "https://example.com/b" || text != "doc body" {
		t.Fatalf("got (%q,%q)", url, text)
	}
}

func TestExtractAntigravityQueryDocs(t *testing.T) {
	dir := t.TempDir()
	tp := filepath.Join(dir, "transcript.jsonl")
	content := "{\"step_index\":5,\"type\":\"GENERIC\",\"content\":\"ctx body\"}\n"
	if err := os.WriteFile(tp, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	inner := `{\"libraryId\":\"/x/y\",\"query\":\"qq\"}`
	raw := []byte(`{"toolCall":{"name":"call_mcp_tool","args":{"ServerName":"context7","ToolName":"query-docs","Arguments":"` + inner + `"}},"stepIdx":4,"transcriptPath":"` + tp + `"}`)
	url, text := extractAntigravity(raw)
	if url != "context7://x/y/qq" || text != "ctx body" {
		t.Fatalf("got (%q,%q)", url, text)
	}
}

func TestExtractFromHookAuto(t *testing.T) {
	claude := []byte(`{"tool_name":"WebFetch","tool_input":{"url":"https://example.com/c"},"tool_response":"txt"}`)
	agy := []byte(`{"toolCall":{"name":"x"},"stepIdx":0}`)
	if url, _ := extractFromHook(claude, "auto"); url != "https://example.com/c" {
		t.Fatalf("auto deveria ir para Claude/Codex, got %q", url)
	}
	if _, text := extractFromHook(agy, "auto"); text != "" {
		t.Fatalf("Antigravity sem transcript deveria dar vazio, got %q", text)
	}
}
