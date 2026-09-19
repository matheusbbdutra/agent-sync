package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTranscript(t *testing.T, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunHookFlagsUnverifiedSuccess(t *testing.T) {
	entry := `{"message":{"role":"assistant","content":[{"type":"text","text":"Pronto, funcionalidade implementada com sucesso."}]}}`
	path := writeTranscript(t, []string{entry})
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: path})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("additionalContext")) {
		t.Errorf("expected flagged nudge, got: %s", out.String())
	}
}

func TestRunHookIgnoresVerifiedSuccess(t *testing.T) {
	entry := `{"message":{"role":"assistant","content":[{"type":"text","text":"Implementado, ver main.go:10 e o teste passou (exit code 0)."}]}}`
	path := writeTranscript(t, []string{entry})
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: path})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{}" {
		t.Errorf("expected empty hook response, got: %s", out.String())
	}
}

func TestRunHookHandlesMissingTranscript(t *testing.T) {
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: "/nonexistent/path.jsonl"})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{}" {
		t.Errorf("expected empty hook response for missing transcript, got: %s", out.String())
	}
}

func TestRunHookIgnoresOldToolErrorWhenCurrentTurnClean(t *testing.T) {
	lines := []string{
		`{"message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{}}]}}`,
		`{"message":{"role":"user","content":[{"type":"tool_result","isError":true,"content":"exit code 1"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"text","text":"vou tentar de novo"}]}}`,
		`{"message":{"role":"user","content":[{"type":"text","text":"agora tenta de novo"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{}}]}}`,
		`{"message":{"role":"user","content":[{"type":"tool_result","isError":false,"content":"exit code 0"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"text","text":"implementado com sucesso"}]}}`,
	}
	path := writeTranscript(t, lines)
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: path})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{}" {
		t.Errorf("expected empty hook response (old tool error must not persist), got: %s", out.String())
	}
}

func TestRunHookStillFlagsCurrentTurnError(t *testing.T) {
	lines := []string{
		`{"message":{"role":"user","content":[{"type":"text","text":"implementa X"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{}}]}}`,
		`{"message":{"role":"user","content":[{"type":"tool_result","isError":true,"content":"exit code 1"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"text","text":"implementado com sucesso"}]}}`,
	}
	path := writeTranscript(t, lines)
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: path})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("additionalContext")) {
		t.Errorf("expected flagged nudge for current-turn tool error, got: %s", out.String())
	}
}

func TestRunHookResetsOnUserTextMixedWithToolResult(t *testing.T) {
	lines := []string{
		`{"message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{}}]}}`,
		`{"message":{"role":"user","content":[{"type":"tool_result","isError":true,"content":"exit code 1"}]}}`,
		`{"message":{"role":"user","content":[{"type":"text","text":"tenta de novo"},{"type":"tool_result","isError":false,"content":"exit code 0"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{}}]}}`,
		`{"message":{"role":"user","content":[{"type":"tool_result","isError":false,"content":"exit code 0"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"text","text":"implementado com sucesso"}]}}`,
	}
	path := writeTranscript(t, lines)
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: path})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{}" {
		t.Errorf("expected empty hook response (mixed text+tool_result must still reset), got: %s", out.String())
	}
}
