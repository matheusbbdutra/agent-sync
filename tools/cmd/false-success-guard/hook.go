// Handles the Stop hook payload from Claude Code: reads the transcript,
// extracts the last assistant text message, classifies it, and — only if
// flagged — emits a non-blocking additionalContext nudge (same shape as
// hooks/context-guard-nudge.sh). It never blocks Stop: the paper's own
// numbers put precision at ~50% at a usable flag rate, so gating on this
// would be wrong more often than not.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type stopPayload struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
}

type transcriptContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type transcriptMessage struct {
	Role    string                   `json:"role"`
	Content []transcriptContentBlock `json:"content"`
}

type transcriptEntry struct {
	Message transcriptMessage `json:"message"`
}

type toolUseBlock struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type toolResultBlock struct {
	Type      string `json:"type"`
	Content   string `json:"content"`
	IsError   bool   `json:"isError"`
}

// runHook reads a Stop hook payload from stdin and writes a hook response
// to stdout. It never returns an error to the caller in practice — a hook
// must never fail the Stop event it's attached to.
func runHook(stdin io.Reader, stdout, stderr io.Writer) error {
	raw, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintln(stderr, "false-success-guard: read hook payload:", err)
		fmt.Fprint(stdout, "{}")
		return nil
	}

	var payload stopPayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.TranscriptPath == "" {
		fmt.Fprint(stdout, "{}")
		return nil
	}

	text, ev := inspectTranscript(payload.TranscriptPath)
	if text == "" {
		fmt.Fprint(stdout, "{}")
		return nil
	}

	verdict := ClassifyWithTrace(text, ev)
	if !verdict.Flagged {
		fmt.Fprint(stdout, "{}")
		return nil
	}

	fmt.Fprintf(stdout, `{"hookSpecificOutput":{"hookEventName":"Stop","additionalContext":"[agent-sync] false-success-guard: %s. Confirme com evidência real (tool/leitura) antes de manter essa conclusão."}}`,
		verdict.Reason)
	return nil
}

// inspectTranscript scans the transcript JSONL and returns the last assistant text
// alongside execution evidence from recent trace steps (HarnessFix TraceStep alignment).
func inspectTranscript(path string) (string, ExecutionEvidence) {
	f, err := os.Open(path)
	if err != nil {
		return "", ExecutionEvidence{}
	}
	defer f.Close()

	var lastText string
	ev := ExecutionEvidence{HasTraceData: false}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var entry struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Message struct {
				Role    string `json:"role"`
				Content []struct {
					Type    string `json:"type"`
					Text    string `json:"text"`
					Name    string `json:"name"`
					Content string `json:"content"`
					IsError bool   `json:"isError"`
				} `json:"content"`
			} `json:"message"`
		}

		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		role := entry.Role
		if role == "" {
			role = entry.Message.Role
		}

		blocks := entry.Message.Content
		if role == "assistant" || role == "MODEL" {
			for _, b := range blocks {
				if b.Type == "text" && b.Text != "" {
					lastText = b.Text
				}
				if b.Type == "tool_use" || b.Type == "call" {
					ev.HasTraceData = true
					toolName := strings.ToLower(b.Name)
					if strings.Contains(toolName, "write") || strings.Contains(toolName, "edit") ||
						strings.Contains(toolName, "replace") || strings.Contains(toolName, "patch") {
						ev.HasMutation = true
					}
					if strings.Contains(toolName, "test") || strings.Contains(toolName, "bash") ||
						strings.Contains(toolName, "command") {
						ev.RanTestCommand = true
					}
				}
			}
		} else if role == "user" || role == "tool" {
			hasUserText := false
			for _, b := range blocks {
				if b.Type == "text" {
					hasUserText = true
				}
				if b.Type == "tool_result" {
					ev.HasTraceData = true
					if b.IsError {
						ev.HasToolError = true
					}
					res := strings.ToLower(b.Content)
					if strings.Contains(res, "[tool_status: failed]") || strings.Contains(res, "exit code 1") {
						ev.HasToolError = true
					}
				}
			}
			if role == "user" && hasUserText {
				ev = ExecutionEvidence{HasTraceData: false}
			}
		}
	}

	return lastText, ev
}

