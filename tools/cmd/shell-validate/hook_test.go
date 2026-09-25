package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestRunHookDisabledByDefault(t *testing.T) {
	os.Unsetenv("AGENT_SYNC_PRETOOLUSE_VALIDATE")
	payload, _ := json.Marshal(preToolUsePayload{ToolName: "Bash"})
	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{}" {
		t.Errorf("expected no-op when env unset, got: %s", out.String())
	}
}

func TestRunHookFlagsInvalidBash(t *testing.T) {
	os.Setenv("AGENT_SYNC_PRETOOLUSE_VALIDATE", "1")
	defer os.Unsetenv("AGENT_SYNC_PRETOOLUSE_VALIDATE")

	payload, _ := json.Marshal(preToolUsePayload{ToolName: "Bash", ToolInput: json.RawMessage(`{"command":"go"}`)})
	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "additionalContext") || !strings.Contains(out.String(), "argumento posicional") {
		t.Errorf("expected additionalContext flagging missing arg, got: %s", out.String())
	}
}

func TestRunHookIgnoresValidBash(t *testing.T) {
	os.Setenv("AGENT_SYNC_PRETOOLUSE_VALIDATE", "1")
	defer os.Unsetenv("AGENT_SYNC_PRETOOLUSE_VALIDATE")

	payload, _ := json.Marshal(preToolUsePayload{ToolName: "Bash", ToolInput: json.RawMessage(`{"command":"go test ./..."}`)})
	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{}" {
		t.Errorf("expected no-op for valid command, got: %s", out.String())
	}
}

func TestRunHookIgnoresNonBashTools(t *testing.T) {
	os.Setenv("AGENT_SYNC_PRETOOLUSE_VALIDATE", "1")
	defer os.Unsetenv("AGENT_SYNC_PRETOOLUSE_VALIDATE")

	payload, _ := json.Marshal(preToolUsePayload{ToolName: "Read", ToolInput: json.RawMessage(`{"file_path":"/etc/passwd"}`)})
	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{}" {
		t.Errorf("expected no-op for non-Bash tools, got: %s", out.String())
	}
}

func TestRunHookAntigravityPassEmitsDecisionAllow(t *testing.T) {
	os.Unsetenv("AGENT_SYNC_PRETOOLUSE_VALIDATE")

	// Payload Antigravity real (toolCall objeto): PreToolUse exige
	// "decision". {} vazio vira deny e trava todas as tools
	// (verificado em runtime 2026-09-22 no antigravity-cli).
	payload := []byte(`{"toolCall":{"name":"view_file","args":{"file_path":"/x"}}}`)
	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != `{"decision":"allow"}` {
		t.Errorf("expected decision allow for antigravity pass, got: %s", out.String())
	}
}

func TestRunHookAntigravityShape(t *testing.T) {
	os.Setenv("AGENT_SYNC_PRETOOLUSE_VALIDATE", "1")
	defer os.Unsetenv("AGENT_SYNC_PRETOOLUSE_VALIDATE")

	payload, _ := json.Marshal(preToolUsePayload{
		ToolCall: &struct {
			Name string          `json:"name"`
			Args json.RawMessage `json:"args"`
		}{Name: "Bash", Args: json.RawMessage(`{"command":"go"}`)},
	})
	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "additionalContext") {
		t.Errorf("expected additionalContext for antigravity shape, got: %s", out.String())
	}
}
