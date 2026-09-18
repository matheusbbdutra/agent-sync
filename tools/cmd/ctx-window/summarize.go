package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/matheusdutra/token-tools/internal/agentmemory"
)

// summarizePromptTemplate is the prompt sent to the LLM to compress the
// session into a structured YAML summary. It mirrors
// skills/context-window-strategy/prompts/summarize.md (kept in sync; the
// Go side embeds it to avoid a filesystem dependency at runtime).
const summarizePromptTemplate = `You are a technical context summarizer. You receive the recent history of a working session (LLM ↔ tool interactions) and must produce a structured summary that allows the agent to continue the session without losing decisions, hypotheses, artifacts, errors, and next steps.

Rules:
- Keep ONLY stable, reusable information; discard noise (already-resolved debug logs, irrelevant back-and-forth, greetings).
- Preserve literal numbers (versions, hashes, exact paths, IDs) when they are essential for continuity.
- Mark still-open hypotheses with "(hypothesis)" — do not promote to fact.
- Total limit: {BUDGET_TOKENS} tokens. Be dense, not verbose.
- Respond EXCLUSIVELY in valid YAML, with no markdown comments or prose outside the YAML.

Expected format:

` + "```yaml" + `
decisions:
  - "..."
active_hypotheses:
  - "..."
artifacts:
  - path: "path:line"
    description: "..."
resolved_errors:
  - cause: "..."
    fix: "..."
next_steps:
  - "..."
constraints:
  - "..."
` + "```" + `

Recent history:
---
{HISTORY}
---
`

func buildSummarizePrompt(history string, budgetTokens int) string {
	prompt := summarizePromptTemplate
	prompt = strings.ReplaceAll(prompt, "{BUDGET_TOKENS}", fmt.Sprintf("%d", budgetTokens))
	prompt = strings.ReplaceAll(prompt, "{HISTORY}", history)
	return prompt
}

// knownCLIs maps a CLI name to the binary/flags that run it non-interactively
// with a single prompt and print the response to stdout. Verified live via
// `<bin> --help` on 2026-09-17 (see docs/ADR-context-window-strategy.md).
var knownCLIs = map[string]struct {
	bin        string
	staticArgs []string // args placed before the model flag/prompt
	modelFlag  string   // flag name used to select a model, "" if unsupported here
}{
	"claude":      {bin: "claude", staticArgs: []string{"-p"}, modelFlag: "--model"},
	"codex":       {bin: "codex", staticArgs: []string{"exec"}, modelFlag: "--model"},
	"opencode":    {bin: "opencode", staticArgs: []string{"run", "--pure"}, modelFlag: "-m"},
	"cursor":      {bin: "cursor-agent", staticArgs: []string{"-p"}, modelFlag: "--model"},
	"antigravity": {bin: "agy", staticArgs: []string{"-p"}, modelFlag: "--model"},
}

// summarizerCommand builds the exec.Cmd that runs the given CLI
// non-interactively with prompt as its single instruction. Recursion into
// our own hooks is not a concern for claude/codex/cursor/antigravity here
// because their hooks only fire on tool calls, and a plain "-p"/"exec" run
// with no tools available makes no tool calls; opencode is the exception
// (its plugin hook fires on tool.execute.after), so it keeps --pure.
func summarizerCommand(ctx context.Context, cliName, model, prompt string) (*exec.Cmd, error) {
	cli, ok := knownCLIs[cliName]
	if !ok {
		known := make([]string, 0, len(knownCLIs))
		for k := range knownCLIs {
			known = append(known, k)
		}
		return nil, fmt.Errorf("unknown CLI %q for summarization (known: %s)", cliName, strings.Join(known, ", "))
	}
	args := append([]string{}, cli.staticArgs...)
	if model != "" && cli.modelFlag != "" {
		args = append(args, cli.modelFlag, model)
	}
	args = append(args, prompt)
	return exec.CommandContext(ctx, cli.bin, args...), nil
}

// runSummarize asks the CLI that owns the session (claude, codex, opencode,
// cursor or antigravity — whichever hook triggered this session) to generate
// a summary via the same LLM/subscription the user is already running.
func runSummarize(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("summarize", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cliFlag := fs.String("cli", "", "CLI to run the summary through: claude, codex, opencode, cursor, antigravity (default: session's cli_name)")
	model := fs.String("model", "", "model to request from the CLI (default: CLI's own default, or $AGENT_SYNC_CTX_MODEL)")
	timeout := fs.Duration("timeout", 60*time.Second, "timeout for the CLI call")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var s *Session
	if fs.NArg() == 0 {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("summarize without <session> requires working directory: %w", err)
		}
		s, err = LatestSessionForProject(wd)
		if err != nil {
			return fmt.Errorf("summarize: %w (pass <session> explicitly)", err)
		}
	} else if fs.NArg() == 1 {
		var err error
		s, err = Load(fs.Arg(0))
		if err != nil {
			return err
		}
	} else {
		return errors.New("summarize accepts at most 1 argument: [session]")
	}
	if len(s.Turns) == 0 {
		return errors.New("session has no tool calls — nothing to summarize")
	}
	if s.ProjectPath == "" {
		projectPath, err := os.Getwd()
		if err != nil {
			return err
		}
		s.ProjectPath = projectPath
	}
	cliName := strings.TrimSpace(*cliFlag)
	if cliName == "" {
		cliName = strings.TrimSpace(s.CLIName)
	}
	if cliName == "" {
		return errors.New("no CLI known for this session — pass --cli (claude, codex, opencode, cursor, antigravity) or ensure the hook records cli_name")
	}
	promptModel := *model
	if promptModel == "" {
		promptModel = strings.TrimSpace(os.Getenv("AGENT_SYNC_CTX_MODEL"))
	}
	history := formatHistoryForPrompt(s.Turns)
	prompt := buildSummarizePrompt(history, s.Budget)
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	cmd, err := summarizerCommand(ctx, cliName, promptModel, prompt)
	if err != nil {
		return err
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s summarizer failed: %w (stderr: %s)", cliName, err, strings.TrimSpace(stderrBuf.String()))
	}
	yaml := extractYAML(stdoutBuf.String())
	if yaml == "" {
		yaml = fmt.Sprintf("# empty summary returned by %s\n", cliName) + stdoutBuf.String()
	}
	yaml = ensureHypothesesGuard(yaml)
	prev := s.Version
	if err := s.AppendVersionedSummary(yaml); err != nil {
		return err
	}
	if err := s.Save(); err != nil {
		return err
	}
	projPath := s.ProjectPath
	if projPath == "" {
		projPath, _ = os.Getwd()
	}
	if err := SaveProjectSummary(projPath, yaml); err != nil {
		fmt.Fprintf(stderr, "ctx-window: salvar resumo local do projeto: %v\n", err)
	}
	fmt.Fprintf(stdout, "summarized: version %d (previous %d); cli=%s; model=%s; turns=%d\n",
		s.Version, prev, cliName, promptModel, len(s.Turns))

	// Opt-in: persist the summary as individual memories in the shared
	// memory-mcp store, so other CLIs / future sessions can recall it via
	// search_memory. Gated by AGENT_SYNC_CTX_REMEMBER=1 because it leaks
	// session content into a table that is shared across PCs and synced
	// to Turso (see internal/agentmemory/sync.go). Default off.
	if strings.TrimSpace(os.Getenv("AGENT_SYNC_CTX_REMEMBER")) == "1" {
		remembered, err := rememberSummary(s.ID, cliName, yaml, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "ctx-window: remember summary: %v\n", err)
		} else if remembered > 0 {
			fmt.Fprintf(stdout, "remembered: %d items em memory-mcp\n", remembered)
		}
	}
	return nil
}

// rememberSummary abre o memory.db local, extrai itens do YAML e grava um
// por um como memória do tipo "project". Falha silenciosa por item: o
// Store.Upsert é independente, então uma linha com erro não aborta as
// outras.
//
// Limitação conhecida: usa o cwd de quem roda o summarizer como project path.
// Se a sessão original foi aberta numa CLI remota, o projectID pode divergir.
// Aceito por ora — corrigir exigiria persistir ProjectID no Session desde o
// hook, fora do escopo desta integração.
func rememberSummary(sessionID, cliName, yaml string, stderr io.Writer) (int, error) {
	yaml = ensureHypothesesGuard(yaml)
	dbPath, err := agentmemory.DefaultDBPath()
	if err != nil {
		return 0, err
	}
	store, err := agentmemory.Open(dbPath)
	if err != nil {
		return 0, err
	}
	defer store.Close()

	origin, err := agentmemory.ResolveOrigin("")
	if err != nil || origin.ProjectID == "" {
		// Sem project_id não conseguimos escopar a memória — não grava.
		if origin.ProjectID == "" {
			fmt.Fprintf(stderr, "ctx-window: remember: project sem remoto Git nem projects[%q] no config; memória não persistida\n", origin.ProjectPath)
		}
		return 0, err
	}
	return store.UpsertSummary(origin.ProjectID, cliName, sessionID, yaml)
}

// runOnToolCallLLM records tool calls without starting a nested LLM request.
// Summarization is only triggered explicitly by the summarize subcommand.
func runOnToolCallLLM(args []string, stdout, stderr io.Writer) error {
	if len(args) < 1 {
		return errors.New("on-tool-call-llm requires <session>")
	}
	session := args[0]
	fs := flag.NewFlagSet("on-tool-call-llm", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tool := fs.String("tool", "", "tool name (required)")
	input := fs.String("input", "", "truncated tool input (optional)")
	cliFlag := fs.String("cli", "", "CLI that owns this session: claude, codex, opencode, cursor, antigravity")
	projectFlag := fs.String("project", "", "project working directory")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if strings.TrimSpace(*tool) == "" {
		return errors.New("--tool is required")
	}
	s, err := Load(session)
	if err != nil {
		return err
	}
	if cli := strings.TrimSpace(*cliFlag); cli != "" {
		s.CLIName = cli
	}
	if project := strings.TrimSpace(*projectFlag); project != "" {
		s.ProjectPath = project
	}
	content := *tool
	if *input != "" {
		const maxInput = 16000
		inputText := *input
		if len(inputText) > maxInput {
			inputText = inputText[:maxInput] + "..."
		}
		content = *tool + ": " + inputText
	}
	if err := s.AddTurn(Turn{Role: "tool", Content: content}); err != nil {
		return err
	}
	if err := s.Save(); err != nil {
		return err
	}
	fmt.Fprintf(stdout, `{"recorded":true,"estimated_chars":%d,"turns":%d}`+"\n",
		s.EstimatedChars(), len(s.Turns))
	return nil
}

func formatHistoryForPrompt(turns []Turn) string {
	var b strings.Builder
	for i, t := range turns {
		fmt.Fprintf(&b, "[%d] (%s @ %s)\n%s\n\n",
			i+1, t.Role, t.At.Format(time.RFC3339), t.Content)
	}
	return b.String()
}

func extractYAML(s string) string {
	start := strings.Index(s, "```yaml")
	if start == -1 {
		// fallback: try generic fenced block
		start = strings.Index(s, "```")
		if start == -1 {
			return strings.TrimSpace(s)
		}
		start += len("```")
	} else {
		start += len("```yaml")
	}
	end := strings.Index(s[start:], "```")
	if end == -1 {
		return strings.TrimSpace(s[start:])
	}
	return strings.TrimSpace(s[start : start+end])
}

const (
	activeHypothesesBlockerText = `[BLOQUEIO agent-react] Existem hipóteses ativas pendentes de validação antes de declarar a tarefa pronta.`
	activeHypothesesBlockerLine = `- "` + activeHypothesesBlockerText + `"`
)

// ensureHypothesesGuard verifica se existem hipóteses ativas pendentes em active_hypotheses.
// Se existirem, garante que a seção next_steps contenha o marcador de bloqueio, sem duplicá-lo se já existir.
func ensureHypothesesGuard(summaryYAML string) string {
	if !hasActiveHypotheses(summaryYAML) || hasNextStepsBlocker(summaryYAML) {
		return summaryYAML
	}
	return insertBlockerIntoNextSteps(summaryYAML)
}

func hasActiveHypotheses(yaml string) bool {
	for _, rec := range agentmemory.ParseSummary(yaml) {
		if rec.Section == "active_hypotheses" {
			content := strings.TrimSpace(rec.Content)
			if content != "" && content != "[]" {
				return true
			}
		}
	}
	// Fallback para itens não indentados ou variações de formatação do YAML
	lines := strings.Split(yaml, "\n")
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		stripped := strings.TrimLeft(trimmed, " ")
		if !inSection {
			if stripped == "active_hypotheses:" || strings.HasPrefix(stripped, "active_hypotheses: ") {
				inSection = true
				rest := strings.TrimSpace(strings.TrimPrefix(stripped, "active_hypotheses:"))
				if rest != "" && rest != "[]" && !strings.HasPrefix(rest, "#") {
					return true
				}
			}
			continue
		}
		if trimmed == "" || strings.HasPrefix(stripped, "#") {
			continue
		}
		if strings.HasPrefix(stripped, "-") {
			itemContent := strings.TrimSpace(strings.TrimPrefix(stripped, "-"))
			itemContent = strings.Trim(itemContent, `"'`)
			if itemContent != "" && itemContent != "[]" {
				return true
			}
			continue
		}
		if strings.HasSuffix(stripped, ":") || strings.Contains(stripped, ": ") {
			break
		}
	}
	return false
}

func hasNextStepsBlocker(yaml string) bool {
	for _, rec := range agentmemory.ParseSummary(yaml) {
		if rec.Section == "next_steps" && strings.Contains(rec.Content, activeHypothesesBlockerText) {
			return true
		}
	}
	lines := strings.Split(yaml, "\n")
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		stripped := strings.TrimLeft(trimmed, " ")
		if !inSection {
			if stripped == "next_steps:" || strings.HasPrefix(stripped, "next_steps: ") {
				inSection = true
				if strings.Contains(stripped, activeHypothesesBlockerText) {
					return true
				}
			}
			continue
		}
		if trimmed == "" || strings.HasPrefix(stripped, "#") {
			continue
		}
		if strings.Contains(line, activeHypothesesBlockerText) {
			return true
		}
		if !strings.HasPrefix(stripped, "-") && (strings.HasSuffix(stripped, ":") || strings.Contains(stripped, ": ")) {
			break
		}
	}
	return false
}

func insertBlockerIntoNextSteps(yaml string) string {
	lines := strings.Split(yaml, "\n")
	nextStepsIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		stripped := strings.TrimLeft(trimmed, " ")
		if stripped == "next_steps:" || strings.HasPrefix(stripped, "next_steps: ") {
			nextStepsIdx = i
			break
		}
	}

	if nextStepsIdx == -1 {
		var sb strings.Builder
		trimmed := strings.TrimRight(yaml, "\r\n")
		sb.WriteString(trimmed)
		if len(trimmed) > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("next_steps:\n")
		sb.WriteString("  " + activeHypothesesBlockerLine + "\n")
		return sb.String()
	}

	headerLine := lines[nextStepsIdx]
	leadingSpaces := headerLine[:len(headerLine)-len(strings.TrimLeft(headerLine, " "))]
	stripped := strings.TrimLeft(strings.TrimRight(headerLine, " \t\r"), " ")
	rest := strings.TrimSpace(strings.TrimPrefix(stripped, "next_steps:"))

	if rest == "[]" || strings.HasPrefix(rest, "[]") || rest == "[ ]" {
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:nextStepsIdx]...)
		newLines = append(newLines, leadingSpaces+"next_steps:", leadingSpaces+"  "+activeHypothesesBlockerLine)
		newLines = append(newLines, lines[nextStepsIdx+1:]...)
		return strings.Join(newLines, "\n")
	}

	if strings.HasPrefix(rest, "[") && strings.HasSuffix(rest, "]") {
		inner := strings.TrimSuffix(strings.TrimPrefix(rest, "["), "]")
		var items []string
		for _, raw := range strings.Split(inner, ",") {
			item := strings.TrimSpace(raw)
			item = strings.Trim(item, `"`)
			if item != "" {
				items = append(items, item)
			}
		}
		newLines := make([]string, 0, len(lines)+len(items)+1)
		newLines = append(newLines, lines[:nextStepsIdx]...)
		newLines = append(newLines, leadingSpaces+"next_steps:", leadingSpaces+"  "+activeHypothesesBlockerLine)
		for _, it := range items {
			newLines = append(newLines, leadingSpaces+fmt.Sprintf("  - %q", it))
		}
		newLines = append(newLines, lines[nextStepsIdx+1:]...)
		return strings.Join(newLines, "\n")
	}

	if nextStepsIdx+1 < len(lines) && strings.TrimSpace(lines[nextStepsIdx+1]) == "[]" {
		newLines := make([]string, len(lines))
		copy(newLines, lines)
		newLines[nextStepsIdx+1] = leadingSpaces + "  " + activeHypothesesBlockerLine
		return strings.Join(newLines, "\n")
	}

	itemIndent := leadingSpaces + "  "
	for j := nextStepsIdx + 1; j < len(lines); j++ {
		s := strings.TrimSpace(lines[j])
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if strings.HasPrefix(s, "-") {
			dashPos := strings.Index(lines[j], "-")
			if dashPos >= 0 {
				itemIndent = lines[j][:dashPos]
			}
		}
		break
	}

	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:nextStepsIdx+1]...)
	newLines = append(newLines, itemIndent+activeHypothesesBlockerLine)
	newLines = append(newLines, lines[nextStepsIdx+1:]...)
	return strings.Join(newLines, "\n")
}
