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
