package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseClaudeCodexPayload(t *testing.T) {
	raw := []byte(`{
		"session_id": "sess-1",
		"tool_name": "Bash",
		"tool_input": {"command": "go test ./..."},
		"tool_response": {"output": "panic: division by zero at calculator.go:58"}
	}`)
	sessionID, toolName, content := parseClaudeCodexPayload(raw)
	if sessionID != "sess-1" {
		t.Errorf("sessionID = %q, want sess-1", sessionID)
	}
	if toolName != "Bash" {
		t.Errorf("toolName = %q, want Bash", toolName)
	}
	if !strings.Contains(content, "go test") || !strings.Contains(content, "division by zero") {
		t.Errorf("content missing expected substrings: %q", content)
	}
}

func TestParseClaudeCodexPayloadInvalidJSON(t *testing.T) {
	sessionID, toolName, content := parseClaudeCodexPayload([]byte("not json"))
	if sessionID != "" || toolName != "" || content != "" {
		t.Errorf("expected all empty on invalid JSON, got (%q,%q,%q)", sessionID, toolName, content)
	}
}

func TestParseCursorPayloadFallsBackToConversationID(t *testing.T) {
	raw := []byte(`{
		"conversation_id": "conv-1",
		"tool_name": "Edit",
		"tool_input": "{\"path\":\"calculator.go\"}",
		"tool_output": {"stdout": "ok"}
	}`)
	sessionID, toolName, content := parseCursorPayload(raw)
	if sessionID != "conv-1" {
		t.Errorf("sessionID = %q, want conv-1", sessionID)
	}
	if toolName != "Edit" {
		t.Errorf("toolName = %q, want Edit", toolName)
	}
	if !strings.Contains(content, "calculator.go") || !strings.Contains(content, "ok") {
		t.Errorf("content missing expected substrings: %q", content)
	}
}

func TestParseAntigravityPayloadReadsTranscript(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "transcript.jsonl")
	transcript := `{"step_index":0,"type":"GENERIC","content":"irrelevant"}
{"step_index":1,"type":"GENERIC","content":"panic: division by zero at calculator.go:58"}
`
	if err := os.WriteFile(transcriptPath, []byte(transcript), 0o600); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{
		"sessionId": "agy-sess",
		"toolCall": {"name": "Bash", "args": {"command": "go test ./..."}},
		"stepIdx": 0,
		"transcriptPath": "` + transcriptPath + `"
	}`)
	sessionID, toolName, content := parseAntigravityPayload(raw)
	if sessionID != "agy-sess" {
		t.Errorf("sessionID = %q, want agy-sess", sessionID)
	}
	if toolName != "Bash" {
		t.Errorf("toolName = %q, want Bash", toolName)
	}
	if !strings.Contains(content, "division by zero") {
		t.Errorf("content missing transcript result: %q", content)
	}
}

func TestParseAntigravityPayloadMissingTranscriptIsSafe(t *testing.T) {
	raw := []byte(`{"sessionId":"agy-sess","toolCall":{"name":"Bash"},"stepIdx":0,"transcriptPath":"/does/not/exist.jsonl"}`)
	sessionID, toolName, content := parseAntigravityPayload(raw)
	if sessionID != "agy-sess" || toolName != "Bash" {
		t.Fatalf("unexpected sessionID/toolName: %q/%q", sessionID, toolName)
	}
	if content != "" {
		t.Errorf("expected empty content when transcript is missing, got %q", content)
	}
}

func TestRunHookUnknownCLI(t *testing.T) {
	var stdout, stderr strings.Builder
	err := runHook([]string{"gemini-cli"}, strings.NewReader("{}"), &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for unknown cli")
	}
}

func TestRunHookRequiresCLIArg(t *testing.T) {
	var stdout, stderr strings.Builder
	err := runHook(nil, strings.NewReader("{}"), &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when no cli arg given")
	}
}

func TestRunHookEndToEndRecordsTurn(t *testing.T) {
	dir := t.TempDir()
	oldRoot := cacheRoot
	cacheRoot = dir
	defer func() { cacheRoot = oldRoot }()

	oldThreshold := os.Getenv("AGENT_SYNC_CTX_COMPACT_AT")
	os.Setenv("AGENT_SYNC_CTX_COMPACT_AT", "999999")
	defer os.Setenv("AGENT_SYNC_CTX_COMPACT_AT", oldThreshold)

	payload := `{"session_id":"hook-e2e","tool_name":"Bash","tool_input":{"command":"go test"},"tool_response":{"output":"ok"}}`
	var stdout, stderr strings.Builder
	if err := runHook([]string{"claude"}, strings.NewReader(payload), &stdout, &stderr); err != nil {
		t.Fatalf("runHook: %v (stderr: %s)", err, stderr.String())
	}
	s, err := Load("hook-e2e")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.Turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(s.Turns))
	}
	if s.CLIName != "claude" {
		t.Errorf("CLIName = %q, want claude", s.CLIName)
	}
	if !strings.Contains(s.Turns[0].Content, "go test") {
		t.Errorf("turn content missing tool input: %q", s.Turns[0].Content)
	}
}
