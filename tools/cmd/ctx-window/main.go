// Command ctx-window manages the agent's context window: working memory
// (last K tool calls verbatim) + incremental summary.
//
// Subcommands:
//
//	ctx-window show <session>         shows summary + working memory
//	ctx-window compact <session>      forces compaction now (local heuristic)
//	ctx-window set-k <session> <N>    adjusts K (working memory)
//	ctx-window doctor                 detects summarizer/configuration
//	ctx-window benchmark <dataset>    runs empirical battery (Phase 0)
//
// The default summarizer is the session's own model; this CLI implements
// the pure local heuristic fallback (no external LLM dependency).
//
// Pattern aligned with arXiv 2606.10209v1.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const usage = `ctx-window — manages context window (sliding window + summary)

Usage:
  ctx-window show <session>                  shows summary + working memory + versions
  ctx-window compact <session>               forces compaction now (local heuristic)
  ctx-window set-k <session> <N>             adjusts K (working memory)
  ctx-window on-tool-call <session>          records a tool call and compacts (local heuristic) if needed
      --tool <name>                          tool name (required)
      [--input <text>]                       truncated input (optional)
      [--cli <name>]                         claude|codex|opencode|cursor|antigravity (persisted on the session)
  ctx-window on-tool-call-llm <session>      records a tool call without automatic LLM summarization
      --cli <name>                           claude|codex|opencode|cursor|antigravity (required first call)
  ctx-window hook <cli>                      reads a PostToolUse-style payload from stdin, extracts session/tool/
                                              input/result for that CLI's schema, and calls on-tool-call-llm.
                                              cli: claude|codex|cursor|antigravity. Installed directly as the hook
                                              command by cmd/agent-sync (syncCtxCompactHook) — no intermediate
                                              script file needed. OpenCode stays on hooks/ctx-compact.opencode.ts
                                              (its hook IS a TS plugin, no Go equivalent to swap in there).
  ctx-window summarize --cli <name> <session>  forces LLM summarization now (flags must come before <session>: the
                                                stdlib flag parser stops at the first positional argument)
  ctx-window handoff <cli>                   reads a SessionStart payload from stdin and returns the latest project summary
  ctx-window doctor                          detects available configuration
  ctx-window benchmark <dataset>             runs empirical battery (placeholder)
  ctx-window -help

Environment variables:
  AGENT_SYNC_SUMMARIZER=agent|ollama:<model>|heuristic (default: agent)
  AGENT_SYNC_CTX_K=5                       working memory size
  AGENT_SYNC_CTX_BUDGET=1000               summary token budget
  AGENT_SYNC_CTX_COMPACT_AT=200            estimated chars threshold for auto-compaction
  AGENT_SYNC_CTX_NUDGE_TOKENS=150000        Claude Code token threshold for one manual-summary reminder

Persistent config:
  ~/.config/agent-sync/config.json (fields "summarizer", "ctx_k", "ctx_budget")
`

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)
		return nil
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "show":
		return runShow(rest, stdout, stderr)
	case "compact":
		return runCompact(rest, stdout, stderr)
	case "set-k":
		return runSetK(rest, stdout, stderr)
	case "on-tool-call":
		return runOnToolCall(rest, stdout, stderr)
	case "on-tool-call-llm":
		return runOnToolCallLLM(rest, stdout, stderr)
	case "hook":
		return runHook(rest, os.Stdin, stdout, stderr)
	case "handoff":
		return runHandoff(rest, os.Stdin, stdout, stderr)
	case "summarize":
		return runSummarize(rest, stdout, stderr)
	case "doctor":
		return runDoctor(stdout, stderr)
	case "benchmark":
		return runBenchmark(rest, stdout, stderr)
	case "-help", "--help", "help":
		fmt.Fprint(stdout, usage)
		return nil
	default:
		return fmt.Errorf("unknown subcommand: %q", sub)
	}
}

func runShow(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("show requires <session>")
	}
	s, err := Load(fs.Arg(0))
	if err != nil {
		return err
	}
	return s.WriteReport(stdout)
}

func runCompact(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("compact", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("compact requires <session>")
	}
	s, err := Load(fs.Arg(0))
	if err != nil {
		return err
	}
	if len(s.Turns) == 0 {
		return errors.New("session has no tool calls — nothing to compact")
	}
	summary := HeuristicExtract(s.Turns)
	yaml := summary.ToYAML()
	prevVersion := s.Version
	if err := s.AppendVersionedSummary(yaml); err != nil {
		return err
	}
	if err := s.Save(); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "compacted: version %d (previous %d); %d decisions, %d hypotheses, %d artifacts\n",
		s.Version, prevVersion, len(summary.Decisoes), len(summary.Hipoteses), len(summary.Artefatos))
	return nil
}

func runSetK(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("set-k", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return errors.New("set-k requires <session> <N>")
	}
	var k int
	if _, err := fmt.Sscanf(fs.Arg(1), "%d", &k); err != nil || k < 1 {
		return fmt.Errorf("invalid K: %q", fs.Arg(1))
	}
	s, err := Load(fs.Arg(0))
	if err != nil {
		return err
	}
	prev := s.K
	s.K = k
	s.TrimTurns()
	if err := s.Save(); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "K updated: %d → %d (turns kept: %d)\n", prev, s.K, len(s.Turns))
	return nil
}

func runOnToolCall(args []string, stdout, stderr io.Writer) error {
	if len(args) < 1 {
		return errors.New("on-tool-call requires <session>")
	}
	session := args[0]
	fs := flag.NewFlagSet("on-tool-call", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tool := fs.String("tool", "", "tool name (required)")
	input := fs.String("input", "", "truncated tool input (optional)")
	cliFlag := fs.String("cli", "", "CLI that owns this session: claude, codex, opencode, cursor, antigravity")
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
	content := *tool
	if *input != "" {
		// cap input to avoid bloat in working memory
		const maxInput = 16000
		if len(*input) > maxInput {
			content = *tool + ": " + (*input)[:maxInput] + "..."
		} else {
			content = *tool + ": " + *input
		}
	}
	if err := s.AddTurn(Turn{Role: "tool", Content: content}); err != nil {
		return err
	}
	if err := s.Save(); err != nil {
		return err
	}
	if cli := strings.TrimSpace(*cliFlag); cli == "opencode" {
		if nudge, _ := checkOpenCodeNudge(session); nudge != "" {
			fmt.Fprintf(stdout, "[AVISO agent-sync] %s\n", nudge)
			return nil
		}
	}
	// Auto-compact if estimated size exceeds budget
	estimated := s.EstimatedChars()
	threshold := compactAtThreshold()
	if estimated >= threshold {
		summary := HeuristicExtract(s.Turns)
		yaml := summary.ToYAML()
		prev := s.Version
		if err := s.AppendVersionedSummary(yaml); err != nil {
			return err
		}
		if err := s.Save(); err != nil {
			return err
		}
		fmt.Fprintf(stdout, `{"auto_compacted":true,"version":%d,"previous":%d,"estimated_chars":%d,"turns":%d}`+"\n",
			s.Version, prev, estimated, len(s.Turns))
		return nil
	}
	fmt.Fprintf(stdout, `{"auto_compacted":false,"estimated_chars":%d,"turns":%d,"threshold":%d}`+"\n",
		estimated, len(s.Turns), threshold)
	return nil
}

func runDoctor(stdout, stderr io.Writer) error {
	fmt.Fprintln(stdout, "ctx-window doctor")
	fmt.Fprintf(stdout, "  configured summarizer: %s\n", SummarizerFromConfig())
	fmt.Fprintf(stdout, "  default K: %d\n", DefaultK())
	fmt.Fprintf(stdout, "  default budget: %d tokens\n", DefaultBudget())
	fmt.Fprintf(stdout, "  ollama on PATH: %v\n", ollamaAvailable())
	return nil
}

func runBenchmark(args []string, stdout, stderr io.Writer) error {
	fmt.Fprintln(stdout, "benchmark: not implemented yet (Phase 0 of the plan)")
	fmt.Fprintln(stdout, "see skills/context-window-strategy/SKILL.md and docs/ADR-context-window-strategy.md")
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "ctx-window:", err)
		os.Exit(1)
	}
}
