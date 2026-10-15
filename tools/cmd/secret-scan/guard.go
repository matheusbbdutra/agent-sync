// Lógica do secret-guard portada 1:1 do shell
// (hooks/secret-guard.pretooluse.sh + posttooluse.sh).
//
// O padrão canônico é o do PRE.
package main

import (
	"encoding/json"
	"regexp"
	"strings"
)

// denyPattern: Camada 1, mesma do shell (case-insensitive).
var denyPattern = regexp.MustCompile(`(?i)__DENY_BODY__`)

// literalPattern: Camada 2, mesma do shell.
var literalPattern = regexp.MustCompile(`__LITERAL_RE__`)

var pathWordRe = regexp.MustCompile(`(/|~|\.)[^ "}{]+`)

// extractCandidates reproduz a extração do shell: Claude/Codex por
// tool_name; Antigravity via todas as strings de toolCall.args.
func extractCandidates(raw []byte, antigravity bool) (toolName string, candidates []string) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return "", nil
	}
	if antigravity {
		var tc struct {
			Name string          `json:"name"`
			Args json.RawMessage `json:"args"`
		}
		if args, ok := root["toolCall"]; ok {
			_ = json.Unmarshal(args, &tc)
		}
		toolName = tc.Name
		candidates = append(candidates, allStrings(tc.Args)...)
		return toolName, candidates
	}
	var tool string
	if v, ok := root["tool_name"]; ok {
		_ = json.Unmarshal(v, &tool)
	}
	toolName = tool
	strField := func(obj json.RawMessage, keys ...string) string {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(obj, &m); err != nil {
			return ""
		}
		for _, k := range keys {
			if v, ok := m[k]; ok {
				var s string
				if err := json.Unmarshal(v, &s); err == nil && s != "" {
					return s
				}
			}
		}
		return ""
	}
	toolInput := root["tool_input"]
	switch toolName {
	case "Read", "Edit", "Write", "MultiEdit", "NotebookEdit":
		if s := strField(toolInput, "file_path", "notebook_path", "path"); s != "" {
			candidates = append(candidates, s)
		}
	case "Grep", "Glob":
		if s := strField(toolInput, "path"); s != "" {
			candidates = append(candidates, s)
		}
	case "Bash":
		if m := pathWordRe.FindString(string(toolInput)); m != "" {
			candidates = append(candidates, m)
		}
	}
	return toolName, candidates
}

// allStrings coleta recursivamente todas as strings de um JSON.
func allStrings(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []string{s}
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		var out []string
		for _, e := range arr {
			out = append(out, allStrings(e)...)
		}
		return out
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		var out []string
		for _, v := range obj {
			out = append(out, allStrings(v)...)
		}
		return out
	}
	return nil
}

// Check avalia payload PreToolUse: retorna (deny, reason).
func Check(raw []byte) (bool, string) {
	antigravity := isAntigravity(raw)
	toolName, candidates := extractCandidates(raw, antigravity)
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if m := denyPattern.FindString(c); m != "" {
			return true, "path denylist (" + toolName + "): " + c + " casou '" + m + "' - arquivo pode conter secret; use env var ou arquivo fora da deny-list"
		}
	}
	if m := literalPattern.FindString(string(raw)); m != "" {
		kind := "SECRET"
		switch {
		case strings.HasPrefix(m, "eyJ"):
			kind = "JWT"
		case strings.HasPrefix(m, "AKIA"):
			kind = "AWS_ACCESS_KEY"
		case strings.HasPrefix(m, "gh"):
			kind = "GITHUB_PAT"
		}
		return true, "detectei " + kind + " literal nos args; use env var ou arquivo chmod 600 - nunca passe secrets inline"
	}
	return false, ""
}

// RedactResult é o veredito do PostToolUse.
type RedactResult struct {
	Action string
	Text   string
	Detail string
}

// Redact avalia payload PostToolUse: Camada 1 (path vira redação total)
// e Camada 2 (literal inline). Lê tool_output ou tool_response.
func Redact(raw []byte) RedactResult {
	output := hookString(raw, "tool_output", "tool_response")
	if output == "" {
		return RedactResult{Action: "none"}
	}
	toolName := hookString(raw, "tool_name")
	candidate := ""
	switch toolName {
	case "Read", "Edit", "Write", "MultiEdit", "NotebookEdit":
		candidate = hookStringIn(raw, "tool_input", "file_path", "notebook_path", "path")
	}
	if candidate != "" && denyPattern.MatchString(candidate) {
		return RedactResult{Action: "redact-total", Detail: candidate}
	}
	redacted := redactInline(output)
	if redacted == output {
		return RedactResult{Action: "none"}
	}
	return RedactResult{Action: "redact-inline", Text: redacted}
}

func hookString(raw []byte, keys ...string) string {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := root[k]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err == nil && s != "" {
				return s
			}
		}
	}
	return ""
}

func hookStringIn(raw []byte, obj string, keys ...string) string {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return ""
	}
	inner, ok := root[obj]
	if !ok {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(inner, &m); err != nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err == nil && s != "" {
				return s
			}
		}
	}
	return ""
}

func redactInline(s string) string {
	return literalPattern.ReplaceAllStringFunc(s, func(m string) string {
		switch {
		case strings.HasPrefix(m, "eyJ"):
			return "<REDACTED:JWT>"
		case strings.HasPrefix(m, "AKIA"):
			return "<REDACTED:AWS_ACCESS_KEY>"
		case strings.HasPrefix(m, "gh"):
			return "<REDACTED:GITHUB_PAT>"
		default:
			return m
		}
	})
}
