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
//	ctx-window track-docs [--snapshot-dir DIR]  monitors CLI docs/forum/GitHub
//	                                              for token field reappearance (K reopen signal)
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
	"time"

	"github.com/matheusdutra/token-tools/cmd/ctx-window/track-docs"
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
  ctx-window quality [--json] <session>      computes 7-signal quality score (S/A/B/C/D/F) for the session
  ctx-window skeleton [--json] [--force] <file>  builds a structural skeleton (imports + signatures) for .py/.ts/.js/.go files; cached by fingerprint
  ctx-window archive [--tool <name>] [--content <text>] [--stdin] <session>  archive a tool result (≥ archiveDefaultAt bytes) to <session>/archive/
  ctx-window expand [--list | --search <q> | <id>] <session>   list/search/retrieve archived tool results
  ctx-window activity [--json] <session>      classify current activity mode (code/debug/review/infra/general) from last 10 tool calls
  ctx-window bash-compress [--json] --cmd <command> [--content <text>] [--stdin]  compress safe-readonly bash output (whitelist: ls/cat/find/grep/...)
  ctx-window benchmark <dataset>             runs empirical battery (placeholder)
  ctx-window track-docs [--snapshot-dir DIR] scans Cursor + Antigravity docs/forum/issues
                                               for token-field reappearance (K reopen signal).
                                               Exit 0 = no change, 1 = positive match (reopen),
                                               2 = network error.
  ctx-window -help

Environment variables:
  AGENT_SYNC_SUMMARIZER=agent|ollama:<model>|heuristic|auto (default: agent)
                         também lido de ~/.config/agent-sync/config.json ("summarizer")
                         auto = heuristic ate 20 turns E 24k chars, senao agent
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
	case "quality":
		return runQuality(rest, stdout, stderr)
	case "skeleton":
		return runSkeleton(rest, stdout, stderr)
	case "archive":
		return runArchive(rest, os.Stdin, stdout, stderr)
	case "expand":
		return runExpand(rest, stdout, stderr)
	case "activity":
		return runActivity(rest, stdout, stderr)
	case "bash-compress":
		return runBashCompress(rest, os.Stdin, stdout, stderr)
	case "benchmark":
		return runBenchmark(rest, stdout, stderr)
	case "gain":
		return runGain(rest, stdout, stderr)
	case "track-docs":
		return trackdocs.Run(rest, stdout, stderr)
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
	beforeChars := s.EstimatedChars()
	if err := s.AppendVersionedSummary(yaml); err != nil {
		return err
	}
	if err := s.Save(); err != nil {
		return err
	}
	afterChars := s.EstimatedChars()
	_ = AppendGainEntry(s.ID, GainEntry{
		Kind:        "heuristic_compact",
		Version:     s.Version,
		BeforeChars: beforeChars,
		AfterChars:  afterChars,
		SavedChars:  beforeChars - afterChars,
		Note:        fmt.Sprintf("manual compact; %d decisions, %d hypotheses, %d artifacts",
			len(summary.Decisoes), len(summary.Hipoteses), len(summary.Artefatos)),
	})
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
	// Detect loop in last K turns (fail-open: only annotates JSON, never blocks).
	loop := DetectLoop(s.Turns, loopWindow(), loopMinRepeats())
	loopSuffix := `,"loop_detected":false`
	if loop.Detected {
		loopSuffix = fmt.Sprintf(`,"loop_detected":true,"loop_count":%d`, loop.Count)
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
		_ = AppendGainEntry(session, GainEntry{
			Kind:        "auto_compact",
			Version:     s.Version,
			BeforeChars: estimated,
			AfterChars:  s.EstimatedChars(),
			SavedChars:  estimated - s.EstimatedChars(),
		})
		fmt.Fprintf(stdout, `{"auto_compacted":true,"version":%d,"previous":%d,"estimated_chars":%d,"turns":%d%s}`+"\n",
			s.Version, prev, estimated, len(s.Turns), loopSuffix)
		return nil
	}
	fmt.Fprintf(stdout, `{"auto_compacted":false,"estimated_chars":%d,"turns":%d,"threshold":%d%s}`+"\n",
		estimated, len(s.Turns), threshold, loopSuffix)
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

func runActivity(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("activity", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit Activity as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("activity requires <session>")
	}
	s, err := Load(fs.Arg(0))
	if err != nil {
		return err
	}
	a := ClassifyActivity(s.Turns)
	return a.Write(stdout, *asJSON)
}

func runGain(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("gain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit gain entries + totals as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("gain requires <session>")
	}
	entries, err := LoadGainEntries(fs.Arg(0))
	if err != nil {
		return err
	}
	if *asJSON {
		return WriteGainJSON(stdout, entries)
	}
	WriteGainMarkdown(stdout, entries)
	return nil
}

func runBashCompress(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("bash-compress", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit BashCompressResult as JSON")
	cmd := fs.String("cmd", "", "bash command (required)")
	content := fs.String("content", "", "command output to compress (or use --stdin)")
	useStdin := fs.Bool("stdin", false, "read output from stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*cmd) == "" {
		return errors.New("--cmd is required")
	}
	body := *content
	if *useStdin {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		body = string(b)
	}
	if body == "" {
		return errors.New("--content (or --stdin) is required and cannot be empty")
	}
	res := CompressBashOutput(*cmd, body)
	return res.Write(stdout, *asJSON)
}

func runQuality(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("quality", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit QualityReport as JSON instead of human-readable text")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("quality requires <session>")
	}
	s, err := Load(fs.Arg(0))
	if err != nil {
		return err
	}
	report := ComputeQuality(s)
	return report.Write(stdout, *asJSON)
}

func runSkeleton(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("skeleton", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit SkeletonResult as JSON instead of human-readable text")
	force := fs.Bool("force", false, "bypass the fingerprint cache and regenerate")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("skeleton requires <file>")
	}
	res, err := Skeletonize(fs.Arg(0), *force)
	if err != nil {
		return err
	}
	return res.Write(stdout, *asJSON)
}

func runArchive(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("archive", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tool := fs.String("tool", "", "tool name (required)")
	content := fs.String("content", "", "content to archive (or use --stdin)")
	useStdin := fs.Bool("stdin", false, "read content from stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("archive requires <session> (flags must come before <session> — stdlib flag stops at first positional)")
	}
	if strings.TrimSpace(*tool) == "" {
		return errors.New("--tool is required")
	}
	body := *content
	if *useStdin {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		body = string(b)
	}
	if body == "" {
		return errors.New("--content (or --stdin) is required and cannot be empty")
	}
	entry, err := ArchiveResult(fs.Arg(0), *tool, body)
	if err != nil {
		return err
	}
	if entry == nil {
		fmt.Fprintf(stdout, `{"archived":false,"reason":"below_threshold","threshold":%d}`+"\n", archiveThreshold())
		return nil
	}
	fmt.Fprintf(stdout, `{"archived":true,"id":%q,"tool":%q,"bytes":%d,"path":%q}`+"\n",
		entry.ID, entry.Tool, entry.Bytes, entry.Path)
	return nil
}

func runExpand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("expand", flag.ContinueOnError)
	fs.SetOutput(stderr)
	list := fs.Bool("list", false, "list all archived entries for the session")
	search := fs.String("search", "", "search archived entries by substring (case-insensitive)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errors.New("expand requires <session>")
	}
	session := fs.Arg(0)
	switch {
	case *list:
		entries, err := LoadArchiveIndex(session)
		if err != nil {
			return err
		}
		WriteArchiveList(stdout, entries, "")
		return nil
	case *search != "":
		matches, err := SearchArchive(session, *search)
		if err != nil {
			return err
		}
		WriteArchiveList(stdout, matches, *search)
		return nil
	default:
		if fs.NArg() < 2 {
			return errors.New("expand requires --list, --search <q>, or <id> as second positional")
		}
		body, entry, err := ExpandByID(session, fs.Arg(1))
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "# %s (%d bytes, %s, archived %s)\n",
			entry.ID, entry.Bytes, entry.Tool, entry.At.Format(time.RFC3339))
		io.WriteString(stdout, body)
		if !strings.HasSuffix(body, "\n") {
			io.WriteString(stdout, "\n")
		}
		return nil
	}
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "ctx-window:", err)
		os.Exit(1)
	}
}
