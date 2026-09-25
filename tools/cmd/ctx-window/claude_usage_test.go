package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeNudgeUsesLatestAssistantInputUsageOnce(t *testing.T) {
	withTempCache(t)
	t.Setenv("AGENT_SYNC_CTX_NUDGE_TOKENS", "100")
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	transcript := `{"type":"assistant","message":{"usage":{"input_tokens":20,"cache_creation_input_tokens":10,"cache_read_input_tokens":0}}}` + "\n" +
		`{"type":"user","message":{"content":"next"}}` + "\n" +
		`{"type":"assistant","message":{"usage":{"input_tokens":20,"cache_creation_input_tokens":30,"cache_read_input_tokens":60}}}` + "\n"
	if err := os.WriteFile(path, []byte(transcript), 0o600); err != nil {
		t.Fatal(err)
	}
	tokens, err := latestClaudeInputTokens(path)
	if err != nil || tokens != 110 {
		t.Fatalf("tokens=%d err=%v", tokens, err)
	}
	payload := []byte(`{"transcript_path":"` + path + `"}`)
	var output bytes.Buffer
	if err := writeClaudeNudge(payload, "claude-session", &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "110 tokens") || !strings.Contains(output.String(), "additionalContext") {
		t.Fatalf("nudge missing: %s", output.String())
	}
	output.Reset()
	if err := writeClaudeNudge(payload, "claude-session", &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "{}" {
		t.Fatalf("duplicate nudge: %s", output.String())
	}
}
