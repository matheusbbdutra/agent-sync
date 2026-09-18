package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type codexEventMsgTokenCount struct {
	Type    string `json:"type"`
	Payload struct {
		Type string `json:"type"`
		Info struct {
			TotalTokenUsage struct {
				InputTokens       int `json:"input_tokens"`
				CachedInputTokens int `json:"cached_input_tokens"`
			} `json:"total_token_usage"`
		} `json:"info"`
	} `json:"payload"`
}

type codexTokenUsageRecord struct {
	Type    string `json:"type"`
	Payload struct {
		Usage struct {
			InputTokens       int `json:"input_tokens"`
			CachedInputTokens int `json:"cached_input_tokens"`
			TotalTokens       int `json:"total_tokens"`
		} `json:"usage"`
	} `json:"payload"`
}

// findCodexRolloutPath tenta encontrar o arquivo rollout-*.jsonl para a sessão do Codex.
func findCodexRolloutPath(sessionID, explicitTranscript string) (string, error) {
	if explicitTranscript != "" {
		if _, err := os.Stat(explicitTranscript); err == nil {
			return explicitTranscript, nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	sessionsDir := filepath.Join(home, ".codex", "sessions")
	now := time.Now().UTC()

	// Procura nas pastas de hoje, ontem e anteontem
	days := []time.Time{now, now.AddDate(0, 0, -1), now.AddDate(0, 0, -2)}
	for _, day := range days {
		dayDir := filepath.Join(sessionsDir, day.Format("2006/01/02"))
		entries, err := os.ReadDir(dayDir)
		if err != nil {
			continue
		}
		for i := len(entries) - 1; i >= 0; i-- {
			name := entries[i].Name()
			if strings.HasPrefix(name, "rollout-") && strings.HasSuffix(name, ".jsonl") {
				if sessionID != "" && strings.Contains(name, sessionID) {
					return filepath.Join(dayDir, name), nil
				}
			}
		}
	}

	// Se não achou por sessionID exato nos dias recentes, busca o rollout mais recente
	for _, day := range days {
		dayDir := filepath.Join(sessionsDir, day.Format("2006/01/02"))
		entries, err := os.ReadDir(dayDir)
		if err != nil {
			continue
		}
		for i := len(entries) - 1; i >= 0; i-- {
			name := entries[i].Name()
			if strings.HasPrefix(name, "rollout-") && strings.HasSuffix(name, ".jsonl") {
				return filepath.Join(dayDir, name), nil
			}
		}
	}

	return "", fmt.Errorf("rollout file not found for session %s", sessionID)
}

// latestCodexInputTokens lê os últimos 2 MiB do rollout do Codex e extrai os tokens de entrada.
func latestCodexInputTokens(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	start := info.Size() - transcriptTailBytes
	if start < 0 {
		start = 0
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return 0, err
	}
	content, err := io.ReadAll(io.LimitReader(file, transcriptTailBytes))
	if err != nil {
		return 0, err
	}
	lines := bytes.Split(content, []byte{'\n'})
	for i := len(lines) - 1; i >= 0; i-- {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		// Tenta event_msg com token_count
		var event codexEventMsgTokenCount
		if json.Unmarshal(line, &event) == nil && event.Type == "event_msg" && event.Payload.Type == "token_count" {
			usage := event.Payload.Info.TotalTokenUsage
			tokens := usage.InputTokens + usage.CachedInputTokens
			if tokens > 0 {
				return tokens, nil
			}
		}
		// Tenta token_usage_record
		var record codexTokenUsageRecord
		if json.Unmarshal(line, &record) == nil && record.Type == "token_usage_record" {
			usage := record.Payload.Usage
			tokens := usage.InputTokens + usage.CachedInputTokens
			if tokens > 0 {
				return tokens, nil
			}
		}
	}
	return 0, nil
}

func writeCodexNudge(raw []byte, sessionID string, stdout io.Writer) error {
	var payload struct {
		TranscriptPath string `json:"transcript_path"`
	}
	_ = json.Unmarshal(raw, &payload)

	s, err := Load(sessionID)
	if err != nil {
		_, err := fmt.Fprint(stdout, "{}")
		return err
	}
	if s.NudgeSent {
		_, err := fmt.Fprint(stdout, "{}")
		return err
	}

	rolloutPath, err := findCodexRolloutPath(sessionID, payload.TranscriptPath)
	if err != nil {
		_, err := fmt.Fprint(stdout, "{}")
		return err
	}

	tokens, err := latestCodexInputTokens(rolloutPath)
	if err != nil || tokens < claudeNudgeThreshold() {
		_, err := fmt.Fprint(stdout, "{}")
		return err
	}

	s.NudgeSent = true
	if err := s.Save(); err != nil {
		return err
	}

	message := fmt.Sprintf("Contexto atual do Codex: cerca de %d tokens de entrada. Considere executar `ctx-window summarize` manualmente e iniciar uma nova sessão para recuperar o resumo.", tokens)
	return json.NewEncoder(stdout).Encode(map[string]any{
		"hookSpecificOutput": map[string]string{
			"hookEventName":     "PostToolUse",
			"additionalContext": message,
		},
	})
}
