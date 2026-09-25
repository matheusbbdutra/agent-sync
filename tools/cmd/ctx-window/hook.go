// Handles the PostToolUse (or equivalent) hook payload for the CLIs that
// invoke hooks as a plain shell command (claude, codex, cursor, antigravity)
// instead of a native plugin runtime (OpenCode is the exception: its hook
// IS a TS plugin, so it stays as hooks/ctx-compact.opencode.ts — there is
// no Go equivalent to swap in there).
//
// This keeps the whole non-OpenCode path in one language (Go, same as the
// rest of ctx-window) instead of spreading the same logic across bash +
// Python per CLI, which made it harder to follow and debug end to end.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const maxHookContentChars = 16000

// toolFailurePattern is a coarse signal of a failed/errored tool call. It is
// intentionally permissive (some false positives on normal output mentioning
// "error" are acceptable) because this only annotates context fed to the LLM
// summarizer, never blocks anything.
var toolFailurePattern = regexp.MustCompile(`(?i)\b(error|exception|traceback|failed|panic|timeout|timed out)\b`)

// toolStatusMarker mirrors the mitigation from arXiv 2609.14758 (Fabrication
// After Tool Failure): explicitly signaling tool status (OK/FAILED/EMPTY)
// dropped the paper's measured fabrication rate from 45.3% to 0.87%, far
// more than any prompt-level plea for honesty. Tagging the raw tool
// response before it reaches the working-memory turn (and, downstream, the
// LLM summarizer) makes the failure visible instead of leaving the model to
// infer status from prose.
func toolStatusMarker(responseText string) string {
	if strings.TrimSpace(responseText) == "" {
		return "[TOOL_STATUS: EMPTY]"
	}
	return toolFailureMarker(responseText)
}

// toolFailureMarker only checks for failure keywords, without the empty-body
// check from toolStatusMarker. Used where an empty string is ambiguous
// between "tool genuinely returned nothing" and "we failed to read the
// result at all" (antigravity's transcript lookup) — claiming EMPTY in the
// second case would itself be an unverified assertion.
func toolFailureMarker(responseText string) string {
	if toolFailurePattern.MatchString(responseText) {
		return "[TOOL_STATUS: FAILED]"
	}
	return ""
}

// runHook reads a raw hook payload from stdin, extracts the tool call +
// result for the given CLI's schema, and forwards it to the existing
// on-tool-call-llm path, which only records the turn. For Claude it also
// checks transcript token usage and may return one manual-summary reminder.
func runHook(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) < 1 {
		return errors.New("hook requires <cli> (claude, codex, cursor, antigravity)")
	}
	cli := args[0]

	raw, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("ctx-window: read hook payload: %w", err)
	}

	if cli == "cursor" && len(args) > 1 && args[1] == "precompact" {
		return runCursorPreCompactHook(raw, stdout, stderr)
	}
	if cli == "antigravity" && len(args) > 1 && args[1] == "preinvocation" {
		return runAntigravityPreInvocationHook(raw, stdout, stderr)
	}

	var sessionID, toolName, content string
	switch cli {
	case "claude", "codex":
		sessionID, toolName, content = parseClaudeCodexPayload(raw)
	case "cursor":
		sessionID, toolName, content = parseCursorPayload(raw)
	case "antigravity":
		sessionID, toolName, content = parseAntigravityPayload(raw)
	default:
		return fmt.Errorf("ctx-window: unknown hook cli %q", cli)
	}
	if sessionID == "" {
		sessionID = "default"
	}
	if toolName == "" {
		toolName = "unknown"
	}
	var common struct {
		CWD            string   `json:"cwd"`
		WorkspacePaths []string `json:"workspacePaths"`
	}
	_ = json.Unmarshal(raw, &common)
	projectPath := common.CWD
	if cli == "antigravity" && len(common.WorkspacePaths) == 1 {
		projectPath = common.WorkspacePaths[0]
	}
	if cli == "antigravity" && len(common.WorkspacePaths) > 1 {
		projectPath = ""
	}
	if cli == "cursor" && projectPath == "" {
		projectPath = os.Getenv("CURSOR_PROJECT_DIR")
	}
	if projectPath == "" && cli != "cursor" && cli != "antigravity" {
		projectPath, _ = os.Getwd()
	}
	if projectPath != "" {
		projectPath, _ = filepath.Abs(projectPath)
	}

	hookArgs := []string{sessionID, "--cli", cli, "--tool", toolName, "--project", projectPath}
	if content != "" {
		hookArgs = append(hookArgs, "--input", content)
	}
	if err := runOnToolCallLLM(hookArgs, io.Discard, stderr); err != nil {
		fmt.Fprintf(stderr, "ctx-window: hook record: %v\n", err)
		fmt.Fprint(stdout, "{}")
		return nil
	}
	if cli == "claude" {
		if err := writeClaudeNudge(raw, sessionID, stdout); err != nil {
			fmt.Fprintf(stderr, "ctx-window: claude nudge: %v\n", err)
			fmt.Fprint(stdout, "{}")
		}
		return nil
	}
	if cli == "codex" {
		if err := writeCodexNudge(raw, sessionID, stdout); err != nil {
			fmt.Fprintf(stderr, "ctx-window: codex nudge: %v\n", err)
			fmt.Fprint(stdout, "{}")
		}
		return nil
	}
	fmt.Fprint(stdout, "{}")
	return nil
}

// asText mirrors the equivalent helper previously duplicated in
// hooks/ctx-compact.py / ctx-compact.cursor.py: pulls a human-readable
// string out of a tool_response-shaped JSON value.
func asText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err == nil {
		for _, key := range []string{"result", "content", "text", "output", "stdout"} {
			if v, ok := m[key].(string); ok {
				return v
			}
		}
		return string(raw)
	}
	return ""
}

func compactRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, raw); err != nil {
		return string(raw)
	}
	return compacted.String()
}

// truncate caps s at maxHookContentChars, keeping head and tail instead of
// only the head. Tool output commonly has the actionable part (an error,
// exit status, final result) at the end — a head-only cut throws that away
// and keeps only setup noise. Mirrors the ACI idea from SWE-agent
// (arXiv 2405.15793): structure what reaches the model instead of a blind
// cut.
func truncate(s string) string {
	if len(s) <= maxHookContentChars {
		return s
	}
	const marker = " ...[truncated]... "
	half := (maxHookContentChars - len(marker)) / 2
	if half < 0 {
		half = 0
	}
	return s[:half] + marker + s[len(s)-half:]
}

func joinContent(parts ...string) string {
	nonEmpty := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return truncate(strings.Join(nonEmpty, " | "))
}

// claudeCodexPayload is the schema shared by Claude Code and Codex,
// confirmed against hooks/docs-cache.py (already relied on the same
// fields, live-tested for both CLIs before this change existed).
type claudeCodexPayload struct {
	SessionID    string          `json:"session_id"`
	ToolName     string          `json:"tool_name"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response"`
}

func parseClaudeCodexPayload(raw []byte) (sessionID, toolName, content string) {
	var p claudeCodexPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", "", ""
	}
	responseText := asText(p.ToolResponse)
	content = joinContent(compactRaw(p.ToolInput), responseText, toolStatusMarker(responseText))
	return p.SessionID, p.ToolName, content
}

// cursorPayload is the schema confirmed against hooks/docs-cache.cursor.py:
// tool_input may arrive either as an object or as a JSON-stringified value;
// the result comes back as tool_output or tool_response.
type cursorPayload struct {
	SessionID      string          `json:"session_id"`
	ConversationID string          `json:"conversation_id"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	ToolOutput     json.RawMessage `json:"tool_output"`
	ToolResponse   json.RawMessage `json:"tool_response"`
}

func parseCursorPayload(raw []byte) (sessionID, toolName, content string) {
	var p cursorPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", "", ""
	}
	sessionID = p.SessionID
	if sessionID == "" {
		sessionID = p.ConversationID
	}
	inputRaw := p.ToolInput
	// tool_input can be a JSON-encoded string instead of a raw object.
	var asStr string
	if json.Unmarshal(inputRaw, &asStr) == nil && asStr != "" {
		inputRaw = json.RawMessage(asStr)
	}
	response := p.ToolOutput
	if len(response) == 0 {
		response = p.ToolResponse
	}
	responseText := asText(response)
	content = joinContent(compactRaw(inputRaw), responseText, toolStatusMarker(responseText))
	return sessionID, p.ToolName, content
}

// antigravityPayload is the schema confirmed against
// hooks/docs-cache.antigravity.py: the payload itself carries no result
// (documented upstream limitation) — it has to be read back from the
// transcript JSONL at step_index == stepIdx + 1.
type antigravityPayload struct {
	SessionID      string `json:"session_id"`
	SessionIDCamel string `json:"sessionId"`
	ConversationID string `json:"conversationId"`
	ToolCall       struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	} `json:"toolCall"`
	StepIdx        *int   `json:"stepIdx"`
	TranscriptPath string `json:"transcriptPath"`
}

func parseAntigravityPayload(raw []byte) (sessionID, toolName, content string) {
	var p antigravityPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", "", ""
	}
	sessionID = p.SessionIDCamel
	if sessionID == "" {
		sessionID = p.SessionID
	}
	if sessionID == "" {
		sessionID = p.ConversationID
	}
	var result string
	if p.StepIdx != nil && p.TranscriptPath != "" {
		result = findAntigravityResult(p.TranscriptPath, *p.StepIdx+1)
	}
	content = joinContent(compactRaw(p.ToolCall.Args), result, toolFailureMarker(result))
	return sessionID, p.ToolCall.Name, content
}

type antigravityTranscriptEntry struct {
	StepIndex int    `json:"step_index"`
	Type      string `json:"type"`
	Content   string `json:"content"`
}

func findAntigravityResult(path string, targetStep int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry antigravityTranscriptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.StepIndex == targetStep && entry.Type == "GENERIC" {
			return entry.Content
		}
	}
	return ""
}

func runCursorPreCompactHook(raw []byte, stdout, stderr io.Writer) error {
	if len(raw) == 0 {
		fmt.Fprint(stdout, "{}")
		return nil
	}
	var p struct {
		ContextTokens  int    `json:"context_tokens"`
		SessionID      string `json:"session_id"`
		ConversationID string `json:"conversation_id"`
	}
	_ = json.Unmarshal(raw, &p)
	tokens := p.ContextTokens
	var msg string
	if tokens > 0 {
		msg = fmt.Sprintf("[AVISO agent-sync] Limite de contexto atingido (%d tokens). Para evitar perda de contexto e respeitar o sliding window, considere cancelar a compactação automática, executar `ctx-window summarize` e iniciar nova sessão com o resumo.", tokens)
	} else {
		msg = "[AVISO agent-sync] Compactação nativa acionada. Para manter o fluxo estruturado, considere executar `ctx-window summarize` e iniciar nova sessão."
	}
	return json.NewEncoder(stdout).Encode(map[string]string{
		"user_message": msg,
	})
}

func runAntigravityPreInvocationHook(raw []byte, stdout, stderr io.Writer) error {
	if len(raw) == 0 {
		fmt.Fprint(stdout, "{}")
		return nil
	}
	var p struct {
		ConversationID  string `json:"conversationId"`
		InvocationNum   int    `json:"invocationNum"`
		InitialNumSteps int    `json:"initialNumSteps"`
		TranscriptPath  string `json:"transcriptPath"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		fmt.Fprint(stdout, "{}")
		return nil
	}
	threshold := 30
	if envVal := strings.TrimSpace(os.Getenv("AGENT_SYNC_CTX_NUDGE_INVOCATIONS")); envVal != "" {
		if n, err := strconv.Atoi(envVal); err == nil && n > 0 {
			threshold = n
		}
	}
	sessionID := p.ConversationID
	if sessionID == "" {
		sessionID = "default"
	}
	s, err := Load(sessionID)
	if err != nil || s.NudgeSent {
		fmt.Fprint(stdout, "{}")
		return nil
	}
	if p.InvocationNum < threshold && p.InitialNumSteps < threshold*2 {
		fmt.Fprint(stdout, "{}")
		return nil
	}
	s.NudgeSent = true
	_ = s.Save()

	msg := fmt.Sprintf("[AVISO agent-sync] Limite de interações atingido (%d invocações). Considere executar `ctx-window summarize` e iniciar uma nova sessão.", p.InvocationNum)
	return json.NewEncoder(stdout).Encode(map[string]any{
		"injectSteps": []map[string]string{
			{"ephemeralMessage": msg},
		},
	})
}
