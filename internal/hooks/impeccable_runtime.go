package hooks

// impeccable_runtime.go: runtime bridge for the Impeccable design detector.
//
// Impeccable ships a native per-edit hook only for Claude Code, Codex, Cursor,
// GitHub Copilot, Grok Build and Hermes Agent. OpenCode and Antigravity are
// intentionally left out; this bridge fills that gap without duplicating the
// detector rules. It normalizes each host event to the Claude-shaped payload
// accepted by the installed skill launcher (`impeccable hook`), then:
//
//   - OpenCode: returns {"context": "..."} for the TS plugin to inject via
//     ctx.session.hook("context").
//   - Antigravity PostToolUse: writes the findings to a per-conversation
//     pending file and returns {} (the event contract only accepts {}).
//   - Antigravity PreInvocation: drains the pending file into
//     {"injectSteps":[{"systemMessage":{"systemMessage":"..."}}]}.
//
// The bridge is fail-open: missing skill, missing project opt-in, timeout,
// malformed JSON or detector error all return {} and never block the turn.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const (
	impeccablePendingTTL       = time.Hour
	impeccableMaxPendingBytes  = 8000
	impeccableMaxFilesPerEvent = 3
)

var impeccableApplyPatchRe = regexp.MustCompile(`(?m)^\*\*\* (?:Update|Add) File: (.+)$`)

type impeccableEvent struct {
	SessionID      string         `json:"session_id"`
	Cwd            string         `json:"cwd"`
	ToolName       string         `json:"tool_name"`
	ToolInput      map[string]any `json:"tool_input"`
	ConversationID string         `json:"conversationId"`
	InvocationNum  *int           `json:"invocationNum"`
	ToolCall       *struct {
		Name string         `json:"name"`
		Args map[string]any `json:"args"`
	} `json:"toolCall"`
	WorkspacePaths []string `json:"workspacePaths"`
	HookEventName  string   `json:"hookEventName"`
	Error          string   `json:"error"`
}

// runImpeccableHook implements `agent-sync hook impeccable [--host=<host>]`.
// It always exits successfully; unknown/invalid input is a silent no-op.
func runImpeccableHook(args []string) error {
	host := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--host" && i+1 < len(args):
			host = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--host="):
			host = strings.TrimPrefix(args[i], "--host=")
		}
	}

	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		_, _ = os.Stdout.WriteString("{}\n")
		return nil
	}
	_, _ = os.Stdout.Write(handleImpeccableEvent(host, stdin))
	_, _ = os.Stdout.WriteString("\n")
	return nil
}

func handleImpeccableEvent(host string, stdin []byte) []byte {
	var ev impeccableEvent
	if err := json.Unmarshal(stdin, &ev); err != nil {
		return []byte("{}")
	}

	host = normalizeImpeccableHost(host, &ev)
	if host == "" {
		return []byte("{}")
	}

	cwd := firstNonEmpty(ev.Cwd, firstString(ev.WorkspacePaths))
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	projectRoot := findImpeccableProject(cwd)
	if projectRoot == "" {
		return []byte("{}")
	}

	session := firstNonEmpty(ev.SessionID, ev.ConversationID, "default")
	skill := findImpeccableLauncher(host, projectRoot)
	if skill == "" {
		return []byte("{}")
	}

	if host == "antigravity" {
		return handleImpeccableAntigravity(ev, projectRoot, session, skill)
	}
	return handleImpeccableOpenCode(ev, projectRoot, session, skill)
}

func handleImpeccableOpenCode(ev impeccableEvent, projectRoot, session, skill string) []byte {
	toolName := ev.ToolName
	toolInput := ev.ToolInput
	if toolName == "" && ev.ToolCall != nil {
		toolName = ev.ToolCall.Name
	}
	if toolInput == nil && ev.ToolCall != nil {
		toolInput = ev.ToolCall.Args
	}

	findings := collectImpeccableFindings(skill, projectRoot, session, toolName, toolInput)
	if len(findings) == 0 {
		return []byte("{}")
	}
	return impeccableJSON(map[string]any{"context": strings.Join(findings, "\n\n---\n\n")})
}

func handleImpeccableAntigravity(ev impeccableEvent, projectRoot, session, skill string) []byte {
	eventName := strings.TrimSpace(ev.HookEventName)
	if eventName == "" {
		switch {
		case ev.ToolCall != nil:
			eventName = "PostToolUse"
		case ev.InvocationNum != nil:
			eventName = "PreInvocation"
		}
	}

	switch {
	case strings.EqualFold(eventName, "PostToolUse"):
		if ev.ToolCall == nil || ev.Error != "" {
			return []byte("{}")
		}
		findings := collectImpeccableFindings(skill, projectRoot, session, ev.ToolCall.Name, ev.ToolCall.Args)
		if len(findings) == 0 {
			return []byte("{}")
		}
		writeImpeccablePending(session, strings.Join(findings, "\n\n---\n\n"))
		return []byte("{}")
	case strings.EqualFold(eventName, "PreInvocation"):
		findings := readImpeccablePending(session)
		if findings == "" {
			return []byte("{}")
		}
		removeImpeccablePending(session)
		message := "[agent-sync] Impeccable design hook findings from a previous edit:\n\n" + findings
		return impeccableJSON(map[string]any{
			"injectSteps": []any{
				map[string]any{
					"systemMessage": map[string]any{"systemMessage": message},
				},
			},
		})
	default:
		return []byte("{}")
	}
}

func collectImpeccableFindings(skill, projectRoot, session, toolName string, toolInput map[string]any) []string {
	paths := extractImpeccablePaths(toolName, toolInput, projectRoot)
	if len(paths) == 0 {
		return nil
	}
	findings := make([]string, 0, len(paths))
	for _, file := range paths {
		contextText := runImpeccableDetector(skill, projectRoot, session, toolName, file)
		if contextText == "" || isImpeccableCleanAck(contextText) {
			continue
		}
		findings = append(findings, contextText)
	}
	return findings
}

func runImpeccableDetector(skill, projectRoot, session, toolName, file string) string {
	payload, err := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse",
		"session_id":      session,
		"tool_name":       firstNonEmpty(toolName, "Write"),
		"tool_input":      map[string]any{"file_path": file},
		"cwd":             projectRoot,
	})
	if err != nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), impeccableTimeout())
	defer cancel()

	cmd := exec.CommandContext(ctx, skill, "hook")
	cmd.Dir = projectRoot
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Env = append(impeccableEnvWithoutHookHarness(os.Environ()), "IMPECCABLE_HOOK_HARNESS=claude")

	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	var parsed struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.HookSpecificOutput.AdditionalContext)
}

func extractImpeccablePaths(toolName string, input map[string]any, projectRoot string) []string {
	if input == nil || !isImpeccableEditTool(toolName) {
		return nil
	}

	seen := map[string]bool{}
	paths := make([]string, 0, 2)
	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		if !filepath.IsAbs(raw) {
			raw = filepath.Join(projectRoot, raw)
		}
		raw = filepath.Clean(raw)
		if seen[raw] {
			return
		}
		if info, err := os.Stat(raw); err != nil || info.IsDir() {
			return
		}
		seen[raw] = true
		paths = append(paths, raw)
	}

	for _, key := range []string{
		"file_path", "filePath", "path", "Path", "filename", "fileName",
		"file", "File", "target_file", "TargetFile", "targetPath",
		"absolute_path", "AbsolutePath",
	} {
		if value, ok := input[key].(string); ok {
			add(value)
		}
	}

	for _, nestedKey := range []string{"args", "input"} {
		if nested, ok := input[nestedKey].(map[string]any); ok {
			for _, key := range []string{"file_path", "filePath", "path", "Path", "TargetFile", "AbsolutePath"} {
				if value, ok := nested[key].(string); ok {
					add(value)
				}
			}
		}
	}

	if len(paths) == 0 && strings.Contains(strings.ToLower(toolName), "patch") {
		for _, key := range []string{"patchText", "patch", "command", "input"} {
			if patch, ok := input[key].(string); ok {
				for _, match := range impeccableApplyPatchRe.FindAllStringSubmatch(patch, -1) {
					add(strings.TrimSpace(match[1]))
				}
			}
		}
	}

	if len(paths) > impeccableMaxFilesPerEvent {
		return paths[:impeccableMaxFilesPerEvent]
	}
	return paths
}

func isImpeccableEditTool(toolName string) bool {
	tool := strings.ToLower(strings.TrimSpace(toolName))
	if tool == "" {
		return true
	}
	for _, fragment := range []string{
		"read", "view", "list", "search", "grep", "glob", "browser",
		"run_command", "execute", "shell", "terminal", "fetch", "web", "download",
	} {
		if strings.Contains(tool, fragment) {
			return false
		}
	}
	for _, fragment := range []string{
		"write", "edit", "patch", "replace", "create", "save", "update", "apply",
	} {
		if strings.Contains(tool, fragment) {
			return true
		}
	}
	return false
}

func findImpeccableProject(cwd string) string {
	if cwd == "" {
		return ""
	}
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return ""
	}
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}

	for {
		if pathExists(filepath.Join(dir, ".impeccable")) ||
			fileExists(filepath.Join(dir, "PRODUCT.md")) ||
			fileExists(filepath.Join(dir, "DESIGN.md")) {
			return dir
		}
		if pathExists(filepath.Join(dir, ".git")) {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func findImpeccableLauncher(host, projectRoot string) string {
	if override := os.Getenv("AGENT_SYNC_IMPECCABLE_SKILL"); override != "" {
		if fileExists(override) {
			return override
		}
	}

	home, _ := os.UserHomeDir()
	candidates := make([]string, 0, 8)
	addProject := func(parts ...string) {
		if projectRoot == "" {
			return
		}
		candidates = append(candidates, filepath.Join(append([]string{projectRoot}, parts...)...))
	}
	addHome := func(parts ...string) {
		if home == "" {
			return
		}
		candidates = append(candidates, filepath.Join(append([]string{home}, parts...)...))
	}

	switch host {
	case "opencode":
		addProject(".opencode", "skills", "impeccable", "scripts", "impeccable")
		addProject(".agents", "skills", "impeccable", "scripts", "impeccable")
		addProject(".claude", "skills", "impeccable", "scripts", "impeccable")
		addProject("skills", "impeccable", "scripts", "impeccable")
		addHome(".config", "opencode", "skills", "impeccable", "scripts", "impeccable")
		addHome(".agents", "skills", "impeccable", "scripts", "impeccable")
		addHome(".claude", "skills", "impeccable", "scripts", "impeccable")
	case "antigravity":
		addProject(".agent", "skills", "impeccable", "scripts", "impeccable")
		addProject(".agents", "skills", "impeccable", "scripts", "impeccable")
		addProject("skills", "impeccable", "scripts", "impeccable")
		addHome(".gemini", "config", "skills", "impeccable", "scripts", "impeccable")
		addHome(".gemini", "antigravity-cli", "skills", "impeccable", "scripts", "impeccable")
		addHome(".agents", "skills", "impeccable", "scripts", "impeccable")
	default:
		return ""
	}

	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

func normalizeImpeccableHost(host string, ev *impeccableEvent) string {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "opencode":
		return "opencode"
	case "antigravity":
		return "antigravity"
	}
	if ev.ToolCall != nil || ev.InvocationNum != nil {
		return "antigravity"
	}
	if ev.SessionID != "" || ev.ToolName != "" {
		return "opencode"
	}
	return ""
}

func impeccableTimeout() time.Duration {
	if raw := os.Getenv("AGENT_SYNC_IMPECCABLE_TIMEOUT_MS"); raw != "" {
		var ms int
		if _, err := fmt.Sscanf(raw, "%d", &ms); err == nil && ms >= 100 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return 5 * time.Second
}

func isImpeccableCleanAck(text string) bool {
	return strings.Contains(text, "No deterministic design-quality issues found")
}

func impeccableEnvWithoutHookHarness(env []string) []string {
	out := env[:0]
	for _, item := range env {
		if strings.HasPrefix(item, "IMPECCABLE_HOOK_HARNESS=") {
			continue
		}
		out = append(out, item)
	}
	return out
}

func writeImpeccablePending(session, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if len(text) > impeccableMaxPendingBytes {
		text = text[:impeccableMaxPendingBytes]
	}

	dir := impeccablePendingDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	file := filepath.Join(dir, sanitizeImpeccableSession(session)+".txt")

	if existing, err := os.ReadFile(file); err == nil {
		joined := strings.TrimSpace(string(existing)) + "\n\n---\n\n" + text
		if len(joined) > impeccableMaxPendingBytes {
			joined = text
		}
		text = joined
	}

	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, []byte(text), 0o600); err != nil {
		return
	}
	if err := os.Rename(tmp, file); err != nil {
		_ = os.Remove(tmp)
	}
}

func readImpeccablePending(session string) string {
	file := filepath.Join(impeccablePendingDir(), sanitizeImpeccableSession(session)+".txt")
	info, err := os.Stat(file)
	if err != nil {
		return ""
	}
	if time.Since(info.ModTime()) > impeccablePendingTTL {
		_ = os.Remove(file)
		return ""
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func removeImpeccablePending(session string) {
	_ = os.Remove(filepath.Join(impeccablePendingDir(), sanitizeImpeccableSession(session)+".txt"))
}

func impeccablePendingDir() string {
	cache, err := os.UserCacheDir()
	if err != nil || cache == "" {
		cache = filepath.Join(os.Getenv("HOME"), ".cache")
	}
	return filepath.Join(cache, "agent-sync", "impeccable")
}

func sanitizeImpeccableSession(session string) string {
	session = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, session)
	session = strings.Trim(session, ".")
	if session == "" {
		return "default"
	}
	if len(session) > 120 {
		session = session[:120]
	}
	return session
}

func impeccableJSON(value any) []byte {
	out, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
