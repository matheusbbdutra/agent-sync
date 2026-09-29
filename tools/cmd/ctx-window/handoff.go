package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/matheusdutra/token-tools/internal/agentmemory"
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

// handoffCLIs são as CLIs cujo handoff `ctx-window handoff <cli>` sabe emitir
// no contrato nativo. `cline` usa o default (hookSpecificOutput.additionalContext),
// o mesmo de Claude Code/Codex — o bridge do Cline traduz de volta (A-80.4).
var handoffCLIs = map[string]bool{
	"claude":      true,
	"codex":       true,
	"cursor":      true,
	"antigravity": true,
	"cline":       true,
}

// coldResumeEnabled devolve true se AGENT_SYNC_CTX_COLD_RESUME=1
// (opt-in: cold-resume cross-PC consome I/O e round-trips ao memory-mcp).
func coldResumeEnabled() bool {
	return strings.TrimSpace(os.Getenv("AGENT_SYNC_CTX_COLD_RESUME")) == "1"
}

// coldResumeProjectID devolve um ProjectID estável do path: o git toplevel.
// Mesmo esquema que UpsertSummary usa para gravar (resumos viram memórias
// com ProjectID = git toplevel). Vazio se path inválido.
func coldResumeProjectID(projectPath string) string {
	if projectPath == "" {
		return ""
	}
	return FindProjectRoot(projectPath)
}

// coldResumeFromMemory reconstrói um YAML de summary a partir das memórias
// type="project" do ProjectID que tenham Name prefix "ctx-" (gravadas pelo
// UpsertSummary de uma sessão anterior). Fail-open: qualquer erro → "" sem
// propagar (handoff deve sempre devolver algo útil ou vazio, nunca falhar).
//
// Inspirado no resume-lean do alexgreensh/token-optimizer: zero chamada LLM,
// reconstrução token-free direto do SQLite (que tem sync Turso cross-PC).
func coldResumeFromMemory(projectID string) (string, error) {
	if projectID == "" {
		return "", nil
	}
	dbPath, err := agentmemory.DefaultDBPath()
	if err != nil {
		return "", nil // silencioso: cold-resume é opt-in, melhor no
	}
	store, err := agentmemory.Open(dbPath)
	if err != nil {
		return "", nil
	}
	defer store.Close()
	// busca ampla: type=project, ProjectID match, limite generoso
	mems, err := store.List("", "project", 200, agentmemory.ScopeFilter{ProjectID: projectID})
	if err != nil {
		return "", nil
	}
	// agrupa por section (extraída do Name "ctx-<section>-<hash>")
	bySection := make(map[string][]string, 6)
	for _, m := range mems {
		if !strings.HasPrefix(m.Name, "ctx-") {
			continue
		}
		rest := strings.TrimPrefix(m.Name, "ctx-")
		idx := strings.LastIndex(rest, "-")
		if idx < 1 {
			continue
		}
		section := rest[:idx]
		if section == "" || m.Content == "" {
			continue
		}
		bySection[section] = append(bySection[section], m.Content)
	}
	if len(bySection) == 0 {
		return "", nil
	}
	// reconstrói YAML mínimo (mesma forma do ExtractedSummary.ToYAML())
	var b strings.Builder
	for _, section := range []string{"decisions", "active_hypotheses", "artifacts", "resolved_errors", "next_steps", "constraints"} {
		items, ok := bySection[section]
		if !ok || len(items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s:\n", section)
		for _, it := range items {
			// YAML-safe escape básico
			escaped := strings.ReplaceAll(it, `"`, `\"`)
			fmt.Fprintf(&b, "  - %q\n", escaped)
		}
	}
	return b.String(), nil
}

func runHandoff(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("handoff requires <cli>")
	}
	cli := args[0]
	if !handoffCLIs[cli] {
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
	// Cold-resume cross-PC: se nada local e env ligada, tenta memory-mcp.
	// Fail-open silencioso; só loga em stderr.
	if summary == "" && coldResumeEnabled() {
		pid := coldResumeProjectID(projectPath)
		cold, cerr := coldResumeFromMemory(pid)
		if cerr != nil {
			fmt.Fprintf(stderr, "ctx-window: cold-resume: %v\n", cerr)
		}
		if cold != "" {
			summary = cold
		}
	}
	if summary == "" {
		fmt.Fprint(stdout, "{}")
		return nil
	}
	// Sumário exposto via `.agent-sync/summary.md` (lido pelo agente via
	// tool Read) ou wirado em GEMINI.md via syncRules em ~/.gemini/.
	// NUNCA emitir `injectSteps + ephemeralMessage` no PreInvocation:
	// o Gemini CLI interpreta esse schema como "tool call denied by
	// pre-tool hook" (verificado em runtime 2026-09-22, mesmo motivo
	// que tornou `agent-react-nudge.antigravity.sh` no-op).
	context := "Resumo anterior do projeto (confirme no código antes de agir):\n" + summary + formatTurnsForHandoff(turns)
	var output any
	switch cli {
	case "cursor":
		output = map[string]any{"additional_context": context}
	case "antigravity":
		// Stderr apenas (auditoria local; nada no contrato PreInvocation
		// do Gemini, que exige `{}` para nao bloquear tool calls).
		fmt.Fprintf(stderr, "ctx-window: antigravity handoff carregado\n%s\n", context)
		fmt.Fprint(stdout, "{}")
		return nil
	default:
		output = map[string]any{"hookSpecificOutput": map[string]string{
			"hookEventName": "SessionStart", "additionalContext": context,
		}}
	}
	return json.NewEncoder(stdout).Encode(output)
}
