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

	text := lastAssistantText(payload.TranscriptPath)
	if text == "" {
		fmt.Fprint(stdout, "{}")
		return nil
	}

	verdict := Classify(text)
	if !verdict.Flagged {
		fmt.Fprint(stdout, "{}")
		return nil
	}

	fmt.Fprintf(stdout, `{"hookSpecificOutput":{"hookEventName":"Stop","additionalContext":"[agent-sync] false-success-guard: %s. Confirme com evidência real (tool/leitura) antes de manter essa conclusão."}}`,
		verdict.Reason)
	return nil
}

// lastAssistantText scans the transcript JSONL and returns the text of the
// last assistant message with a "text" content block (thinking/tool_use
// blocks are skipped).
func lastAssistantText(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	var last string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var entry transcriptEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Message.Role != "assistant" {
			continue
		}
		for _, block := range entry.Message.Content {
			if block.Type == "text" && block.Text != "" {
				last = block.Text
			}
		}
	}
	return last
}
