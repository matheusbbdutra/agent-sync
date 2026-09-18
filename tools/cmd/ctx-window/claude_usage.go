package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const transcriptTailBytes = 2 << 20
const defaultNudgeTokens = 150000

type claudeTranscriptEntry struct {
	Type    string `json:"type"`
	Message struct {
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

func claudeNudgeThreshold() int {
	n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("AGENT_SYNC_CTX_NUDGE_TOKENS")))
	if err == nil && n > 0 {
		return n
	}
	return defaultNudgeTokens
}

func latestClaudeInputTokens(path string) (int, error) {
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
		var entry claudeTranscriptEntry
		if json.Unmarshal(lines[i], &entry) != nil || entry.Type != "assistant" {
			continue
		}
		usage := entry.Message.Usage
		tokens := usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
		if tokens > 0 {
			return tokens, nil
		}
	}
	return 0, nil
}

func writeClaudeNudge(raw []byte, sessionID string, stdout io.Writer) error {
	var payload struct {
		TranscriptPath string `json:"transcript_path"`
	}
	if json.Unmarshal(raw, &payload) != nil || payload.TranscriptPath == "" {
		_, err := fmt.Fprint(stdout, "{}")
		return err
	}
	s, err := Load(sessionID)
	if err != nil {
		return err
	}
	if s.NudgeSent {
		_, err := fmt.Fprint(stdout, "{}")
		return err
	}
	tokens, err := latestClaudeInputTokens(payload.TranscriptPath)
	if err != nil {
		return err
	}
	if tokens < claudeNudgeThreshold() {
		_, err := fmt.Fprint(stdout, "{}")
		return err
	}
	s.NudgeSent = true
	if err := s.Save(); err != nil {
		return err
	}
	quotedSession := "'" + strings.ReplaceAll(sessionID, "'", "'\\''") + "'"
	message := fmt.Sprintf("Contexto atual do Claude Code: cerca de %d tokens de entrada. Considere executar `ctx-window summarize %s` manualmente e iniciar uma nova sessão para recuperar o resumo.", tokens, quotedSession)
	return json.NewEncoder(stdout).Encode(map[string]any{"hookSpecificOutput": map[string]string{
		"hookEventName": "PostToolUse", "additionalContext": message,
	}})
}
