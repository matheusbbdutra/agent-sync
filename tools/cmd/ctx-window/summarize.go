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

// runSummarize asks the opencode CLI (already authenticated) to generate a
// summary for the session via the same LLM the user is running. We invoke
// `opencode run --pure` to avoid plugin recursion (our own hook should not
// fire while we're inside a summarize call).
func runSummarize(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("summarize", flag.ContinueOnError)
	fs.SetOutput(stderr)
	model := fs.String("model", "", "model in provider/model format (default: minimax/MiniMax-M3)")
	timeout := fs.Duration("timeout", 60*time.Second, "timeout for the opencode call")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("summarize requires <session>")
	}
	s, err := Load(fs.Arg(0))
	if err != nil {
		return err
	}
	if len(s.Turns) == 0 {
		return errors.New("session has no tool calls — nothing to summarize")
	}
	promptModel := *model
	if promptModel == "" {
		promptModel = strings.TrimSpace(os.Getenv("AGENT_SYNC_CTX_MODEL"))
	}
	if promptModel == "" {
		promptModel = "minimax/MiniMax-M3"
	}
	history := formatHistoryForPrompt(s.Turns)
	prompt := buildSummarizePrompt(history, s.Budget)
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "opencode", "run", "--pure", "-m", promptModel, prompt)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("opencode run failed: %w (stderr: %s)", err, strings.TrimSpace(stderrBuf.String()))
	}
	yaml := extractYAML(stdoutBuf.String())
	if yaml == "" {
		yaml = "# empty summary returned by opencode\n" + stdoutBuf.String()
	}
	prev := s.Version
	if err := s.AppendVersionedSummary(yaml); err != nil {
		return err
	}
	if err := s.Save(); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "summarized: version %d (previous %d); model=%s; turns=%d\n",
		s.Version, prev, promptModel, len(s.Turns))
	return nil
}

// runOnToolCallLLM is like runOnToolCall but uses the LLM summarizer (via
// opencode run) instead of the local heuristic. Used by the OpenCode hook
// to keep the default summarizer = own model working end-to-end.
func runOnToolCallLLM(args []string, stdout, stderr io.Writer) error {
	if len(args) < 1 {
		return errors.New("on-tool-call-llm requires <session>")
	}
	session := args[0]
	fs := flag.NewFlagSet("on-tool-call-llm", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tool := fs.String("tool", "", "tool name (required)")
	input := fs.String("input", "", "truncated tool input (optional)")
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
	if s.EstimatedChars() >= compactAtThreshold() {
		return runSummarize([]string{session}, stdout, stderr)
	}
	fmt.Fprintf(stdout, `{"auto_summarized":false,"estimated_chars":%d,"turns":%d}`+"\n",
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
