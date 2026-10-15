// Package main — extração de {url,text} do payload cru do hook PostToolUse.
//
// Portado de hooks/docs-cache.py + hooks/docs-cache.antigravity.py (removidos,
// regra §4 Python-zero). A lógica de parse vive aqui, em Go, junto do writer —
/// o wrapper shell só traduz o contrato e repassa o stdin.
package main

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
)

// hookAsText espelha o helper as_text do antigo docs-cache.py:
// string vira texto direto; objeto tenta result/content/text/output;
// caso contrário devolve o JSON cru.
func hookAsText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err == nil {
		for _, key := range []string{"result", "content", "text", "output"} {
			if v, ok := m[key]; ok {
				var vs string
				if err := json.Unmarshal(v, &vs); err == nil {
					return vs
				}
			}
		}
		return string(raw)
	}
	return ""
}

func hookStringField(obj map[string]json.RawMessage, keys ...string) string {
	for _, k := range keys {
		if v, ok := obj[k]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err == nil && s != "" {
				return s
			}
		}
	}
	return ""
}

// extractClaudeCodex extrai {url,text} do payload PostToolUse de Claude/Codex:
// {tool_name, tool_input, tool_response}. Retorna "" quando não é doc cacheável.
func extractClaudeCodex(raw []byte) (url, text string) {
	var p struct {
		ToolName     string          `json:"tool_name"`
		ToolInput    json.RawMessage `json:"tool_input"`
		ToolResponse json.RawMessage `json:"tool_response"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", ""
	}
	var toolInput map[string]json.RawMessage
	if len(p.ToolInput) > 0 {
		_ = json.Unmarshal(p.ToolInput, &toolInput)
	}
	if toolInput == nil {
		toolInput = map[string]json.RawMessage{}
	}
	if p.ToolName == "WebFetch" {
		u := hookStringField(toolInput, "url")
		t := hookAsText(p.ToolResponse)
		if u == "" || t == "" {
			return "", ""
		}
		return u, t
	}
	if strings.HasPrefix(p.ToolName, "mcp__context7__query-docs") || strings.HasSuffix(p.ToolName, "query-docs") {
		lib := hookStringField(toolInput, "libraryId", "context7CompatibleLibraryID")
		if lib == "" {
			lib = "unknown-library"
		}
		query := hookStringField(toolInput, "query", "topic")
		if query == "" {
			query = "index"
		}
		t := hookAsText(p.ToolResponse)
		if t == "" {
			return "", ""
		}
		return "context7:/" + lib + "/" + query, t
	}
	return "", ""
}

// findHookTranscriptResult lê o resultado da tool no transcript JSONL do
// Antigravity (entrada GENERIC com step_index == targetStep).
func findHookTranscriptResult(path string, targetStep int) string {
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
		var entry struct {
			StepIndex int    `json:"step_index"`
			Type      string `json:"type"`
			Content   string `json:"content"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.StepIndex == targetStep && entry.Type == "GENERIC" {
			return entry.Content
		}
	}
	return ""
}

// extractAntigravity extrai {url,text} do payload PostToolUse do Antigravity:
// {toolCall:{name,args}, stepIdx, transcriptPath}. O resultado não vem no
// payload — é lido de volta do transcript (limitação documentada upstream).
func extractAntigravity(raw []byte) (url, text string) {
	var p struct {
		ToolCall struct {
			Name string          `json:"name"`
			Args json.RawMessage `json:"args"`
		} `json:"toolCall"`
		StepIdx        *int   `json:"stepIdx"`
		TranscriptPath string `json:"transcriptPath"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", ""
	}
	var args map[string]json.RawMessage
	if len(p.ToolCall.Args) > 0 {
		_ = json.Unmarshal(p.ToolCall.Args, &args)
	}
	if args == nil {
		args = map[string]json.RawMessage{}
	}
	u := ""
	switch p.ToolCall.Name {
	case "read_url_content":
		u = hookStringField(args, "Url", "URL", "url")
	case "call_mcp_tool":
		if hookStringField(args, "ServerName") != "context7" {
			return "", ""
		}
		if hookStringField(args, "ToolName") != "query-docs" {
			return "", ""
		}
		var inner map[string]json.RawMessage
		if v, ok := args["Arguments"]; ok {
			if err := json.Unmarshal(v, &inner); err != nil {
				var s string
				if err2 := json.Unmarshal(v, &s); err2 == nil && s != "" {
					_ = json.Unmarshal([]byte(s), &inner)
				}
			}
		}
		if inner == nil {
			inner = map[string]json.RawMessage{}
		}
		lib := hookStringField(inner, "libraryId")
		if lib == "" {
			lib = "unknown-library"
		}
		query := hookStringField(inner, "query")
		if query == "" {
			query = "index"
		}
		u = "context7:/" + lib + "/" + query
	default:
		return "", ""
	}
	if u == "" || p.StepIdx == nil || p.TranscriptPath == "" {
		return "", ""
	}
	t := findHookTranscriptResult(p.TranscriptPath, *p.StepIdx+1)
	if t == "" {
		return "", ""
	}
	return u, t
}

// extractFromHook escolhe o parser pelo --cli (auto detecta via "toolCall").
func extractFromHook(raw []byte, cli string) (url, text string) {
	c := strings.ToLower(strings.TrimSpace(cli))
	if c == "" || c == "auto" {
		if strings.Contains(string(raw), `"toolCall"`) {
			return extractAntigravity(raw)
		}
		return extractClaudeCodex(raw)
	}
	if c == "antigravity" {
		return extractAntigravity(raw)
	}
	return extractClaudeCodex(raw)
}
