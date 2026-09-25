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
	Type    string `json:"type"`
	Content string `json:"content"`
	IsError bool   `json:"isError"`
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

// readOnlyTools são tools cuja saída nunca deve alimentar HasToolError via o
// marcador textual [TOOL_STATUS: FAILED]: elas frequentemente devolvem o
// código-fonte do próprio guard/ADR/testes, que citam esse marcador como
// documentação — não como falha real. O ctx-window hook (matcher "*") tagueia
// toda tool_result, inclusive Read/Grep/Glob, então o marcador por si só não
// distingue "li um arquivo que menciona a string" de "uma tool mutável falhou".
var readOnlyTools = map[string]bool{
	"read": true, "grep": true, "glob": true, "ls": true,
	"websearch": true, "webfetch": true,
}

// transcriptBlock é um bloco de conteúdo de mensagem. Content é RawMessage
// porque tool_result.content varia por CLI: string simples (Claude Code) ou
// array de sub-blocos (Cursor "Shell"). IsError usa o nome real do campo no
// wire format do Claude Code 2.1.275 (is_error, snake_case) — a variante
// isError (camelCase) nunca ocorre nos transcripts reais observados.
type transcriptBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Name string `json:"name"`
	// ID identifica um bloco tool_use ("id"); ToolUseID é a referência de
	// volta usada por um bloco tool_result ("tool_use_id") — campos
	// distintos no wire format, nunca o mesmo nome.
	ID        string          `json:"id"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

func (b transcriptBlock) contentText() string {
	var s string
	if json.Unmarshal(b.Content, &s) == nil {
		return s
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(b.Content, &blocks) == nil {
		var out strings.Builder
		for _, sub := range blocks {
			out.WriteString(sub.Text)
		}
		return out.String()
	}
	return ""
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
	toolNameByID := map[string]string{}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var entry struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Message struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}

		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		role := entry.Role
		if role == "" {
			role = entry.Message.Role
		}

		// message.content pode ser string simples (prompt digitado sem
		// anexos) ou array de blocos (tool_use/tool_result/text misturados).
		// Um prompt de usuário em texto puro conta como reset de turno mesmo
		// sem vir em array — era esse caso que antes fazia a linha inteira
		// ser descartada por erro de tipo no Unmarshal.
		var plainText string
		var blocks []transcriptBlock
		if json.Unmarshal(entry.Message.Content, &plainText) == nil && plainText != "" {
			if role == "user" {
				ev = ExecutionEvidence{HasTraceData: false}
			}
			continue
		}
		_ = json.Unmarshal(entry.Message.Content, &blocks)

		if role == "assistant" || role == "MODEL" {
			for _, b := range blocks {
				if b.Type == "text" && b.Text != "" {
					lastText = b.Text
				}
				if b.Type == "tool_use" || b.Type == "call" {
					ev.HasTraceData = true
					toolName := strings.ToLower(b.Name)
					toolNameByID[b.ID] = toolName
					if strings.Contains(toolName, "write") || strings.Contains(toolName, "edit") ||
						strings.Contains(toolName, "replace") || strings.Contains(toolName, "patch") {
						ev.HasMutation = true
					}
					// "shell" cobre o tool Cursor/OpenCode "Shell"; "bash" cobre Claude Code.
					// "compact" cobre o tool interno disparado pelo hook PreCompact wirado em A-14
					// (syncContextSnapshotHook) — registrado por Claude Code no transcript como
					// tool_use "compact" quando a compactacao ocorre. Mesma logica do D-25 Shell:
					// sem reconhecer, alegacao de sucesso apos compactacao vira falso-positivo
					// (A-26 / ADR Decisao 5).
					if strings.Contains(toolName, "test") || strings.Contains(toolName, "bash") ||
						strings.Contains(toolName, "shell") || strings.Contains(toolName, "command") ||
						strings.Contains(toolName, "compact") {
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
					sourceTool := toolNameByID[b.ToolUseID]
					if !readOnlyTools[sourceTool] && strings.Contains(strings.ToLower(b.contentText()), "[tool_status: failed]") {
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
