package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexNudgeUsesLatestUsageOnce(t *testing.T) {
	withTempCache(t)
	t.Setenv("AGENT_SYNC_CTX_NUDGE_TOKENS", "1000")
	path := filepath.Join(t.TempDir(), "rollout-2026-09-18T10-00-00-codex-sess-123.jsonl")
	content := `{"type":"event_msg","payload":{"type":"item_completed"}}` + "\n" +
		`{"type":"token_usage_record","payload":{"usage":{"input_tokens":300,"cached_input_tokens":200,"total_tokens":550}}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":800,"cached_input_tokens":400}}}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	tokens, err := latestCodexInputTokens(path)
	if err != nil || tokens != 1200 {
		t.Fatalf("tokens=%d err=%v (expected 1200)", tokens, err)
	}

	payload := []byte(`{"transcript_path":"` + path + `"}`)
	var output bytes.Buffer
	if err := writeCodexNudge(payload, "codex-sess-123", &output); err != nil {
		t.Fatal(err)
	}
	outStr := output.String()
	if !strings.Contains(outStr, "1200 tokens") || !strings.Contains(outStr, "additionalContext") || !strings.Contains(outStr, "ctx-window summarize") {
		t.Fatalf("nudge missing or malformed: %s", outStr)
	}

	// Segundo disparo deve ser silencioso ({})
	output.Reset()
	if err := writeCodexNudge(payload, "codex-sess-123", &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "{}" {
		t.Fatalf("duplicate nudge should be {}: got %s", output.String())
	}
}

func TestCodexNudgeBelowThreshold(t *testing.T) {
	withTempCache(t)
	t.Setenv("AGENT_SYNC_CTX_NUDGE_TOKENS", "5000")
	path := filepath.Join(t.TempDir(), "rollout-codex-low.jsonl")
	content := `{"type":"token_usage_record","payload":{"usage":{"input_tokens":100,"cached_input_tokens":50,"total_tokens":150}}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	payload := []byte(`{"transcript_path":"` + path + `"}`)
	var output bytes.Buffer
	if err := writeCodexNudge(payload, "codex-low", &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "{}" {
		t.Fatalf("expected {} for low tokens: got %s", output.String())
	}
}
