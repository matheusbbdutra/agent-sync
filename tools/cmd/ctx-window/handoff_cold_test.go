package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matheusdutra/token-tools/internal/agentmemory"
)

func TestColdResumeEnabled(t *testing.T) {
	t.Setenv("AGENT_SYNC_CTX_COLD_RESUME", "")
	if coldResumeEnabled() {
		t.Error("default should be disabled")
	}
	t.Setenv("AGENT_SYNC_CTX_COLD_RESUME", "1")
	if !coldResumeEnabled() {
		t.Error("env=1 should enable")
	}
}

func TestColdResumeProjectID(t *testing.T) {
	// sem path → vazio
	if got := coldResumeProjectID(""); got != "" {
		t.Errorf("empty path should give empty ID, got %q", got)
	}
	// path sem git → devolve o próprio path (FindProjectRoot fallback)
	got := coldResumeProjectID("/tmp")
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
}

func TestColdResumeFromMemoryEmpty(t *testing.T) {
	// sem memória nenhuma: retorna ""
	withTempCache(t)
	if got, err := coldResumeFromMemory("nonexistent-project"); got != "" || err != nil {
		t.Errorf("expected empty result, got %q (err=%v)", got, err)
	}
}

func TestColdResumeFromMemoryReconstructsYAML(t *testing.T) {
	// Setup: cria DB fake via XDG_CACHE_HOME, popula com memórias
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)

	dbPath, err := agentmemory.DefaultDBPath()
	if err != nil {
		t.Fatal(err)
	}
	store, err := agentmemory.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	pid := "test-project-cold-resume"
	// popula: 2 decisions, 1 artifact, 1 hypothesis (cross-PC, scratch=false)
	memories := []agentmemory.Memory{
		{Agent: "claude-code", SessionID: "old-sess", Type: "project", ProjectID: pid,
			Name: "ctx-decisions-abc12345", Description: "ctx-window: decisions",
			Content: "use K=5"},
		{Agent: "claude-code", SessionID: "old-sess", Type: "project", ProjectID: pid,
			Name: "ctx-decisions-def67890", Description: "ctx-window: decisions",
			Content: "use Go"},
		{Agent: "claude-code", SessionID: "old-sess", Type: "project", ProjectID: pid,
			Name: "ctx-artifacts-11111111", Description: "ctx-window: artifacts",
			Content: "main.go:42"},
		{Agent: "claude-code", SessionID: "old-sess", Type: "project", ProjectID: pid,
			Name: "ctx-active_hypotheses-22222222", Description: "ctx-window: active_hypotheses",
			Content: "hypothesis X"},
		// não deve entrar: type errado
		{Agent: "claude-code", Type: "feedback", ProjectID: pid,
			Name: "ctx-decisions-33333333", Content: "wrong type"},
		// não deve entrar: ProjectID diferente
		{Agent: "claude-code", Type: "project", ProjectID: "other-project",
			Name: "ctx-decisions-44444444", Content: "wrong project"},
		// não deve entrar: Name sem prefix
		{Agent: "claude-code", Type: "project", ProjectID: pid,
			Name: "no-prefix-55555555", Content: "no prefix"},
	}
	for _, m := range memories {
		if err := store.Upsert(m); err != nil {
			t.Fatalf("upsert %s: %v", m.Name, err)
		}
	}
	store.Close()

	got, err := coldResumeFromMemory(pid)
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("expected non-empty YAML reconstruction")
	}
	// ordem de seções é fixa (decisions primeiro); verifica conteúdo
	for _, want := range []string{"decisions:", "use K=5", "use Go", "artifacts:", "main.go:42", "active_hypotheses:", "hypothesis X"} {
		if !strings.Contains(got, want) {
			t.Errorf("reconstructed YAML missing %q\n--- output ---\n%s", want, got)
		}
	}
	// itens rejeitados não devem aparecer
	for _, banned := range []string{"wrong type", "wrong project", "no prefix"} {
		if strings.Contains(got, banned) {
			t.Errorf("reconstructed YAML should not contain %q\n--- output ---\n%s", banned, got)
		}
	}
}

func TestColdResumeFromMemoryEmptyProjectID(t *testing.T) {
	withTempCache(t)
	got, err := coldResumeFromMemory("")
	if err != nil || got != "" {
		t.Errorf("empty projectID should give empty, got %q (err=%v)", got, err)
	}
}

func TestColdResumeFromMemoryDBUnavailable(t *testing.T) {
	// XDG_CACHE_HOME aponta para dir que não pode ser criado
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "readonly"))
	// cria arquivo "readonly" para forçar MkdirAll a falhar
	if err := os.WriteFile(filepath.Join(tmp, "readonly"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := coldResumeFromMemory("any")
	if err != nil {
		t.Errorf("fail-open should swallow error, got %v", err)
	}
	if got != "" {
		t.Errorf("expected empty on failure, got %q", got)
	}
}

func TestRunHandoffColdResumeIntegration(t *testing.T) {
	// E2E: setup memory, garante que sem AGENT_SYNC_CTX_COLD_RESUME não dispara,
	// e com a env ligada, devolve summary reconstruído.
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)

	dbPath, err := agentmemory.DefaultDBPath()
	if err != nil {
		t.Fatal(err)
	}
	store, err := agentmemory.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	pid := "e2e-cold-resume"
	store.Upsert(agentmemory.Memory{
		Agent: "claude-code", Type: "project", ProjectID: pid,
		Name: "ctx-decisions-abcdef00", Description: "ctx-window: decisions",
		Content: "from cold-resume e2e",
	})
	store.Close()

	// opt-out: sem env, devolve {}
	t.Setenv("AGENT_SYNC_CTX_COLD_RESUME", "")
	input := `{"cwd":"` + t.TempDir() + `"}` // path sem summary local
	stdout, stderr := captureHandoff(t, "claude", input)
	if !strings.Contains(stdout, "{}") {
		t.Errorf("without env, should return {}, got %s", stdout)
	}

	// opt-in: com env=1 e projectID correspondente, devolve summary reconstruído
	// (mas projectPath não está em git repo do pid real; usamos cwd = tmp)
	// truque: fazemos o cwd ser t.TempDir() que vira o ProjectID
	cwd := t.TempDir()
	t.Setenv("AGENT_SYNC_CTX_COLD_RESUME", "1")
	input = `{"cwd":"` + cwd + `"}`
	// hack simples: cwd não é git repo, então FindProjectRoot devolve o próprio cwd
	// e o ProjectID == cwd. Para o teste casar, vamos popular com ProjectID == cwd.
	store, _ = agentmemory.Open(dbPath)
	store.Upsert(agentmemory.Memory{
		Agent: "claude-code", Type: "project", ProjectID: cwd,
		Name: "ctx-decisions-99999999", Description: "ctx-window: decisions",
		Content: "matched by cwd",
	})
	store.Close()

	stdout, stderr = captureHandoff(t, "claude", input)
	if !strings.Contains(stdout, "matched by cwd") {
		t.Errorf("expected cold-resume content, got stdout=%s stderr=%s", stdout, stderr)
	}
}

func captureHandoff(t *testing.T, cli, input string) (string, string) {
	t.Helper()
	r, w, _ := os.Pipe()
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	w.Close()
	stdoutBuf := &strings.Builder{}
	stderrBuf := &strings.Builder{}
	if err := runHandoff([]string{cli}, r, stdoutBuf, stderrBuf); err != nil {
		t.Fatalf("runHandoff: %v", err)
	}
	return stdoutBuf.String(), stderrBuf.String()
}
