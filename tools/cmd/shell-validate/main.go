// Command shell-validate — lightweight PreToolUse validator that flags
// shell commands likely to fail because they are missing a required
// positional argument. Pattern from arXiv 2609.11957 ("Look Before You
// Leap"); deliberately small to keep false positives low (paper's full
// validator has ~10% FP).
//
// Advisory, never a gate: emits only additionalContext, never blocks.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const usage = `shell-validate — avisador de comandos shell provavelmente inválidos (PreToolUse, arXiv 2609.11957)

Usage:
  shell-validate check --cmd "<comando>"     classifica um comando (lê de --cmd ou stdin)
  shell-validate hook                        lê payload PreToolUse do stdin; só emite aviso se
                                              AGENT_SYNC_PRETOOLUSE_VALIDATE=1
  shell-validate -help
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
	cmd := fs.String("cmd", "", "comando a classificar (se vazio, lê do stdin)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	input := *cmd
	if input == "" {
		raw, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("shell-validate: read stdin: %w", err)
		}
		input = string(raw)
	}
	if strings.TrimSpace(input) == "" {
		return errors.New("check requires --cmd ou texto via stdin")
	}
	verdict := Classify(input)
	fmt.Fprintf(stdout, `{"valid":%v,"reason":%q}`+"\n", verdict.Valid, verdict.Reason)
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "shell-validate:", err)
		os.Exit(1)
	}
}
