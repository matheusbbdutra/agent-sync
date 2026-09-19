// Command false-success-guard is a lightweight, non-LLM detector for
// unverified success claims in agent-facing text — the pattern described
// in arXiv 2606.09863v1 ("false success" in LLM agents): a regex/keyword
// classifier flags confident success language lacking evidence far more
// reliably, and orders of magnitude cheaper, than using an LLM as judge.
//
// It is advisory only: never gate or block on its verdict.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

const usage = `false-success-guard — sinaliza alegações de sucesso sem evidência (arXiv 2606.09863v1)

Usage:
  false-success-guard check --text "<mensagem>"   classifica um texto (lê de --text ou stdin)
  false-success-guard hook                        lê um payload de Stop hook (Claude Code) do stdin
  false-success-guard -help
`

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)
		return nil
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], stdin, stdout, stderr)
	case "hook":
		return runHook(stdin, stdout, stderr)
	case "-help", "--help", "help":
		fmt.Fprint(stdout, usage)
		return nil
	default:
		return fmt.Errorf("unknown subcommand: %q", args[0])
	}
}

func runCheck(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	text := fs.String("text", "", "texto a classificar (se vazio, lê do stdin)")
	transcript := fs.String("transcript", "", "caminho opcional para o transcript JSONL para extrair evidência de TraceSteps (HarnessFix)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	input := *text
	var ev ExecutionEvidence
	if *transcript != "" {
		extractedText, traceEv := inspectTranscript(*transcript)
		ev = traceEv
		if input == "" {
			input = extractedText
		}
	}

	if input == "" {
		raw, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("false-success-guard: read stdin: %w", err)
		}
		input = string(raw)
	}
	if input == "" {
		return errors.New("check requires --text, --transcript ou texto via stdin")
	}

	verdict := ClassifyWithTrace(input, ev)
	fmt.Fprintf(stdout, `{"flagged":%v,"reason":%q}`+"\n", verdict.Flagged, verdict.Reason)
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "false-success-guard:", err)
		os.Exit(1)
	}
}
