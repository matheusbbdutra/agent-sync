package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandoffUsesLatestSummaryForProject(t *testing.T) {
	withTempCache(t)
	project := filepath.Join(t.TempDir(), "project")
	otherProject := filepath.Join(t.TempDir(), "other")
	for _, item := range []struct{ id, path, summary string }{
		{"old", project, "old decision"},
		{"other", otherProject, "private other decision"},
		{"new", project, "new decision"},
	} {
		s, err := Load(item.id)
		if err != nil {
			t.Fatal(err)
		}
		s.ProjectPath = item.path
		if item.id == "new" {
			_ = s.AddTurn(Turn{Role: "tool", Content: "git status"})
		}
		if err := s.AppendVersionedSummary(item.summary); err != nil {
			t.Fatal(err)
		}
		if err := s.Save(); err != nil {
			t.Fatal(err)
		}
	}
	// Set an explicit modification time so the test does not rely on clock resolution.
	newDir, _ := SessionDir("new")
	latestTime := time.Now().Add(time.Hour)
	if err := os.Chtimes(summaryPath(newDir), latestTime, latestTime); err != nil {
		t.Fatal(err)
	}
	var output, stderr bytes.Buffer
	if err := runHandoff([]string{"codex"}, strings.NewReader(`{"cwd":"`+project+`"}`), &output, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "new decision") || strings.Contains(output.String(), "private other decision") {
		t.Fatalf("wrong project summary: %s", output.String())
	}
	if !strings.Contains(output.String(), "Últimas interações da sessão anterior") || !strings.Contains(output.String(), "git status") {
		t.Fatalf("missing working memory turns in handoff: %s", output.String())
	}
	var result map[string]any
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	hook := result["hookSpecificOutput"].(map[string]any)
	if hook["hookEventName"] != "SessionStart" {
		t.Fatalf("wrong event: %v", hook)
	}
}

func TestCursorHandoffUsesProjectEnvironment(t *testing.T) {
	withTempCache(t)
	project := t.TempDir()
	t.Setenv("CURSOR_PROJECT_DIR", project)
	s, _ := Load("cursor-old")
	s.ProjectPath = project
	if err := s.AppendVersionedSummary("cursor decision"); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	var output, stderr bytes.Buffer
	if err := runHandoff([]string{"cursor"}, strings.NewReader(`{"session_id":"new"}`), &output, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"additional_context"`) || !strings.Contains(output.String(), "cursor decision") {
		t.Fatalf("cursor handoff missing: %s", output.String())
	}
}

func TestAntigravityHandoffUsesWorkspacePaths(t *testing.T) {
	withTempCache(t)
	project := t.TempDir()
	s, _ := Load("agy-old")
	s.ProjectPath = project
	if err := s.AppendVersionedSummary("agy decision"); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	var output, stderr bytes.Buffer
	if err := runHandoff([]string{"antigravity"}, strings.NewReader(`{"workspacePaths":["`+project+`"]}`), &output, &stderr); err != nil {
		t.Fatal(err)
	}
	// Antigravity PreInvocation NAO aceita `injectSteps + ephemeralMessage`:
	// o Gemini CLI interpreta esse schema como "tool call denied by
	// pre-tool hook" (ver handoff.go comentario). O summary fica
	// disponivel em `.agent-sync/summary.md` (lido pelo agente via
	// tool Read). Aqui validamos: (a) stdout vazio no contrato
	// PreInvocation e (b) stderr com a auditoria local do payload.
	if output.String() != "{}" {
		t.Fatalf("antigravity handoff deve emitir {} no stdout, got %s", output.String())
	}
	if !strings.Contains(stderr.String(), "agy decision") {
		t.Fatalf("antigravity handoff deve logar o summary em stderr, got %s", stderr.String())
	}

	// Invocação subsequente (invocationNum > 1) deve retornar "{}"
	output.Reset()
	stderr.Reset()
	if err := runHandoff([]string{"antigravity"}, strings.NewReader(`{"workspacePaths":["`+project+`"],"invocationNum":2}`), &output, &stderr); err != nil {
		t.Fatal(err)
	}
	if output.String() != "{}" {
		t.Fatalf("expected {} for invocationNum > 1, got %s", output.String())
	}
}

func TestHandoffPrioritizesLocalProjectSummary(t *testing.T) {
	withTempCache(t)
	project := t.TempDir()

	// 1. Cria resumo em cache global
	s, _ := Load("cache-session")
	s.ProjectPath = project
	if err := s.AppendVersionedSummary("cached decision"); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	// 2. Cria resumo local em .agent-sync/summary.md
	if err := SaveProjectSummary(project, "local project decision"); err != nil {
		t.Fatal(err)
	}

	// 3. Executa handoff e confirma que o local tem precedência
	var output, stderr bytes.Buffer
	if err := runHandoff([]string{"claude"}, strings.NewReader(`{"cwd":"`+project+`"}`), &output, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "local project decision") {
		t.Fatalf("expected local project decision, got %s", output.String())
	}
	if strings.Contains(output.String(), "cached decision") {
		t.Fatalf("cached decision should not override local summary: %s", output.String())
	}
}

func TestHandoffDefensiveOnEmptyOrInvalidPayload(t *testing.T) {
	withTempCache(t)
	origWd, err := os.Getwd()
	if err == nil {
		_ = os.Chdir(t.TempDir())
		t.Cleanup(func() { _ = os.Chdir(origWd) })
	}
	var output, stderr bytes.Buffer

	// Empty input returns "{}" and no error.
	if err := runHandoff([]string{"claude"}, strings.NewReader(""), &output, &stderr); err != nil {
		t.Fatalf("unexpected error on empty stdin: %v", err)
	}
	if output.String() != "{}" {
		t.Fatalf("expected {}, got %q", output.String())
	}

	// Invalid JSON logs to stderr and returns "{}" without error.
	output.Reset()
	stderr.Reset()
	if err := runHandoff([]string{"codex"}, strings.NewReader("{bad json"), &output, &stderr); err != nil {
		t.Fatalf("unexpected error on invalid JSON: %v", err)
	}
	if output.String() != "{}" {
		t.Fatalf("expected {}, got %q", output.String())
	}
	if !strings.Contains(stderr.String(), "parse handoff payload") {
		t.Fatalf("expected stderr warning, got %q", stderr.String())
	}

	// Unknown project (no summary) returns "{}".
	output.Reset()
	stderr.Reset()
	if err := runHandoff([]string{"cursor"}, strings.NewReader(`{"cwd":"/nonexistent/project"}`), &output, &stderr); err != nil {
		t.Fatalf("unexpected error on missing summary: %v", err)
	}
	if output.String() != "{}" {
		t.Fatalf("expected {}, got %q", output.String())
	}
}

func TestClineHandoffEmitsClaudeContract(t *testing.T) {
	withTempCache(t)
	project := t.TempDir()
	if err := SaveProjectSummary(project, "cline decision"); err != nil {
		t.Fatal(err)
	}
	var output, stderr bytes.Buffer
	if err := runHandoff([]string{"cline"}, strings.NewReader(`{"cwd":"`+project+`"}`), &output, &stderr); err != nil {
		t.Fatalf("runHandoff(cline): %v", err)
	}
	if !strings.Contains(output.String(), "cline decision") {
		t.Fatalf("handoff cline sem o resumo do projeto: %s", output.String())
	}
	var result map[string]any
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("output nao e JSON: %v\n%s", err, output.String())
	}
	hook, ok := result["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("hookSpecificOutput ausente: %s", output.String())
	}
	if hook["hookEventName"] != "SessionStart" {
		t.Fatalf("hookEventName=%v, esperado SessionStart", hook["hookEventName"])
	}
}

func TestHandoffRejectsUnsupportedCLI(t *testing.T) {
	withTempCache(t)
	var output, stderr bytes.Buffer
	err := runHandoff([]string{"copilot"}, strings.NewReader(`{}`), &output, &stderr)
	if err == nil || !strings.Contains(err.Error(), "unsupported handoff cli") {
		t.Fatalf("esperava erro de CLI nao suportada, obteve %v", err)
	}
}

func TestOnToolCallLLMOnlyRecords(t *testing.T) {
	withTempCache(t)
	var output bytes.Buffer
	if err := runOnToolCallLLM([]string{"no-llm", "--cli", "unknown", "--tool", "Bash", "--input", strings.Repeat("x", 500)}, &output, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	s, err := Load("no-llm")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Turns) != 1 || s.Version != 0 {
		t.Fatalf("expected tracking without summary: turns=%d version=%d", len(s.Turns), s.Version)
	}
}
