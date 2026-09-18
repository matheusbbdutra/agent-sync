package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type sessionStartPayload struct {
	CWD            string   `json:"cwd"`
	WorkspacePaths []string `json:"workspacePaths"`
	InvocationNum  int      `json:"invocationNum"`
}

// latestProjectHandoff reads the local .agent-sync/summary.md or newest cached summary, plus recent turns.
func latestProjectHandoff(projectPath string) (string, []Turn, error) {
	if projectPath == "" {
		return "", nil, nil
	}
	projectPath = filepath.Clean(projectPath)

	// Abordagem B: Prioriza o arquivo local .agent-sync/summary.md do projeto
	localSummary, _ := LoadProjectSummary(projectPath)

	var latestSummaryPath string
	var latestTurns []Turn
	var latestTime time.Time
	root, err := SessionDir("placeholder")
	if err == nil {
		entries, err := os.ReadDir(filepath.Dir(root))
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				dir := filepath.Join(filepath.Dir(root), entry.Name())
				meta, err := os.ReadFile(metaPath(dir))
				if err != nil {
					continue
				}
				var session Session
				if json.Unmarshal(meta, &session) != nil || filepath.Clean(session.ProjectPath) != projectPath {
					continue
				}
				info, err := os.Stat(summaryPath(dir))
				if err != nil || !info.ModTime().After(latestTime) {
					continue
				}
				latestSummaryPath = summaryPath(dir)
				latestTurns = session.Turns
				latestTime = info.ModTime()
			}
		}
	}

	summary := localSummary
	if summary == "" && latestSummaryPath != "" {
		content, err := os.ReadFile(latestSummaryPath)
		if err == nil {
			summary = strings.TrimSpace(string(content))
		}
	}
	if summary == "" {
		return "", nil, nil
	}
	return summary, latestTurns, nil
}

func formatTurnsForHandoff(turns []Turn) string {
	if len(turns) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\nÚltimas interações da sessão anterior (working memory):\n")
	for i, t := range turns {
		fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, t.Role, oneLine(t.Content))
	}
	return b.String()
}

func runHandoff(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("handoff requires <cli>")
	}
	cli := args[0]
	if cli != "claude" && cli != "codex" && cli != "cursor" && cli != "antigravity" {
		return fmt.Errorf("unsupported handoff cli %q", cli)
	}
	input, err := io.ReadAll(io.LimitReader(stdin, 1<<20))
	if err != nil {
		fmt.Fprintf(stderr, "ctx-window: read handoff payload: %v\n", err)
		fmt.Fprint(stdout, "{}")
		return nil
	}
	var payload sessionStartPayload
	if len(input) > 0 {
		if err := json.Unmarshal(input, &payload); err != nil {
			fmt.Fprintf(stderr, "ctx-window: parse handoff payload: %v\n", err)
			fmt.Fprint(stdout, "{}")
			return nil
		}
	}
	// Antigravity dispara handoff via PreInvocation. Só injeta no primeiro turno.
	if cli == "antigravity" && payload.InvocationNum > 1 {
		fmt.Fprint(stdout, "{}")
		return nil
	}
	projectPath := payload.CWD
	if cli == "antigravity" && len(payload.WorkspacePaths) == 1 {
		projectPath = payload.WorkspacePaths[0]
	}
	if cli == "cursor" && projectPath == "" {
		projectPath = os.Getenv("CURSOR_PROJECT_DIR")
	}
	if projectPath == "" {
		var err error
		projectPath, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "ctx-window: getwd handoff: %v\n", err)
			fmt.Fprint(stdout, "{}")
			return nil
		}
	}
	summary, turns, err := latestProjectHandoff(projectPath)
	if err != nil {
		fmt.Fprintf(stderr, "ctx-window: latest project handoff: %v\n", err)
		fmt.Fprint(stdout, "{}")
		return nil
	}
	if summary == "" {
		fmt.Fprint(stdout, "{}")
		return nil
	}
	context := "Resumo anterior do projeto (confirme no código antes de agir):\n" + summary + formatTurnsForHandoff(turns)
	var output any
	switch cli {
	case "cursor":
		output = map[string]any{"additional_context": context}
	case "antigravity":
		output = map[string]any{"injectSteps": []any{map[string]string{"ephemeralMessage": context}}}
	default:
		output = map[string]any{"hookSpecificOutput": map[string]string{
			"hookEventName": "SessionStart", "additionalContext": context,
		}}
	}
	return json.NewEncoder(stdout).Encode(output)
}
