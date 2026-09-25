package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func findOpenCodeDBPath() string {
	if custom := strings.TrimSpace(os.Getenv("OPENCODE_DB_PATH")); custom != "" {
		return custom
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dbPath := filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	if _, err := os.Stat(dbPath); err == nil {
		return dbPath
	}
	return ""
}

func latestOpenCodeInputTokens(sessionID string) (int, error) {
	dbPath := findOpenCodeDBPath()
	if dbPath == "" {
		return 0, fmt.Errorf("opencode db not found")
	}
	cleanID := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, sessionID)

	var query string
	if cleanID != "" && cleanID != "default" {
		query = fmt.Sprintf("SELECT COALESCE(tokens_input, 0) + COALESCE(tokens_cache_read, 0) FROM session WHERE id = '%s' ORDER BY time_updated DESC LIMIT 1;", cleanID)
	} else {
		query = "SELECT COALESCE(tokens_input, 0) + COALESCE(tokens_cache_read, 0) FROM session ORDER BY time_updated DESC LIMIT 1;"
	}

	cmd := exec.Command("sqlite3", "-batch", "-noheader", dbPath, query)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	if err := cmd.Run(); err != nil {
		return 0, err
	}
	valStr := strings.TrimSpace(outBuf.String())
	if valStr == "" {
		return 0, nil
	}
	return strconv.Atoi(valStr)
}

func checkOpenCodeNudge(sessionID string) (string, error) {
	s, err := Load(sessionID)
	if err != nil {
		return "", err
	}
	if s.NudgeSent {
		return "", nil
	}
	tokens, err := latestOpenCodeInputTokens(sessionID)
	if err != nil || tokens < claudeNudgeThreshold() {
		return "", nil
	}
	s.NudgeSent = true
	if err := s.Save(); err != nil {
		return "", err
	}
	return fmt.Sprintf("Contexto atual do OpenCode: cerca de %d tokens de entrada. Considere executar `ctx-window summarize` manualmente e iniciar uma nova sessão para recuperar o resumo.", tokens), nil
}
