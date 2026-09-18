package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCodeNudgeFromSQLite(t *testing.T) {
	withTempCache(t)
	t.Setenv("AGENT_SYNC_CTX_NUDGE_TOKENS", "1000")

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "opencode.db")
	t.Setenv("OPENCODE_DB_PATH", dbPath)

	// Cria o banco SQLite com a tabela session
	initSQL := `
CREATE TABLE session (
    id TEXT PRIMARY KEY,
    directory TEXT,
    tokens_input INTEGER,
    tokens_cache_read INTEGER,
    time_updated INTEGER
);
INSERT INTO session (id, directory, tokens_input, tokens_cache_read, time_updated)
VALUES ('ses-opencode-1', '/my/project', 800, 500, 100);
`
	cmd := exec.Command("sqlite3", dbPath, initSQL)
	if err := cmd.Run(); err != nil {
		t.Skipf("sqlite3 não disponível para teste: %v", err)
	}

	tokens, err := latestOpenCodeInputTokens("ses-opencode-1")
	if err != nil || tokens != 1300 {
		t.Fatalf("tokens=%d err=%v (expected 1300)", tokens, err)
	}

	nudge, err := checkOpenCodeNudge("ses-opencode-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(nudge, "1300 tokens") || !strings.Contains(nudge, "ctx-window summarize") {
		t.Fatalf("nudge malformed: %s", nudge)
	}

	// Segundo disparo deve ser vazio
	nudge2, err := checkOpenCodeNudge("ses-opencode-1")
	if err != nil {
		t.Fatal(err)
	}
	if nudge2 != "" {
		t.Fatalf("expected empty nudge on second check: got %s", nudge2)
	}
}

func TestOpenCodeNudgeBelowThreshold(t *testing.T) {
	withTempCache(t)
	t.Setenv("AGENT_SYNC_CTX_NUDGE_TOKENS", "5000")

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "opencode.db")
	t.Setenv("OPENCODE_DB_PATH", dbPath)

	initSQL := `
CREATE TABLE session (
    id TEXT PRIMARY KEY,
    directory TEXT,
    tokens_input INTEGER,
    tokens_cache_read INTEGER,
    time_updated INTEGER
);
INSERT INTO session (id, directory, tokens_input, tokens_cache_read, time_updated)
VALUES ('ses-opencode-low', '/my/project', 100, 50, 100);
`
	cmd := exec.Command("sqlite3", dbPath, initSQL)
	if err := cmd.Run(); err != nil {
		t.Skipf("sqlite3 não disponível para teste: %v", err)
	}

	nudge, err := checkOpenCodeNudge("ses-opencode-low")
	if err != nil {
		t.Fatal(err)
	}
	if nudge != "" {
		t.Fatalf("expected empty nudge when below threshold: got %s", nudge)
	}
}
