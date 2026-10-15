// Package main — fonte única da verdade do secret-guard (regra §4:
// Go é fonte da verdade, TS/shell nunca reimplementam deny-list).
//
// Portado de hooks/secret-guard.pretooluse.sh + posttooluse.sh (que viram
// adapters finos) e de cline-plugin/src/hooks/secret-guard.ts (que agora
// delega via bridge). Comportamento preservado 1:1, incluindo o drift
// pré-existente entre pre (sem `/config.json` nu, removido em 2026-09-29)
// e post (com `/config.json`): o padrão canônico aqui é o do PRE.
//
// Subcommands (sempre stdout JSON + exit 0 — fail-open, nunca derruba hook):
//   check  [--format decision|auto]  lê payload PreToolUse via stdin.
//   redact [--format decision|auto]  lê payload PostToolUse via stdin.
// Formato auto detecta Antigravity pela presença de `toolCall` (mesmo padrão
// do .sh) e emite o contrato de cada host; `decision` emite envelope neutro
// para o plugin Cline.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const usage = `secret-scan — guard de segredos (fonte única da verdade do secret-guard)

Usage:
  secret-scan check [--format decision|auto]   avalia payload PreToolUse (stdin)
  secret-scan redact [--format decision|auto]  redige output PostToolUse (stdin)
`

// ---------- envelope decision (neutro, para o plugin Cline) ----------

type checkDecision struct {
	Decision string `json:"decision"` // allow|deny
	Reason   string `json:"reason,omitempty"`
}

type redactDecision struct {
	Action string `json:"action"` // none|redact-total|redact-inline
	Text   string `json:"text,omitempty"`
	Detail string `json:"detail,omitempty"`
}

func isAntigravity(raw []byte) bool {
	return strings.Contains(string(raw), `"toolCall"`)
}

func runCheck(args []string, stdin io.Reader, stdout io.Writer) {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	format := fs.String("format", "auto", "decision|auto")
	_ = fs.Parse(args)

	raw, err := io.ReadAll(stdin)
	if err != nil {
		raw = nil
	}
	deny, reason := Check(raw)

	if *format == "decision" {
		d := checkDecision{Decision: "allow"}
		if deny {
			d = checkDecision{Decision: "deny", Reason: reason}
		}
		emit(stdout, d)
		return
	}
	if isAntigravity(raw) {
		if deny {
			emit(stdout, map[string]string{"decision": "deny", "reason": reason})
			return
		}
		emit(stdout, map[string]string{"decision": "allow"})
		return
	}
	if deny {
		emit(stdout, map[string]any{"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": reason,
		}})
		return
	}
	emit(stdout, map[string]any{})
}

func runRedact(args []string, stdin io.Reader, stdout io.Writer) {
	fs := flag.NewFlagSet("redact", flag.ContinueOnError)
	format := fs.String("format", "auto", "decision|auto")
	_ = fs.Parse(args)

	raw, err := io.ReadAll(stdin)
	if err != nil {
		raw = nil
	}
	res := Redact(raw)

	if *format == "decision" {
		emit(stdout, redactDecision{Action: res.Action, Text: res.Text, Detail: res.Detail})
		return
	}
	// Contrato Claude/Codex (o .sh nunca teve branch Antigravity no Post).
	switch res.Action {
	case "redact-total":
		emit(stdout, map[string]any{"hookSpecificOutput": map[string]any{
			"hookEventName":     "PostToolUse",
			"additionalContext": "<REDACTED:FILE_IN_DENYLIST:" + res.Detail + ">",
		}})
	case "redact-inline":
		emit(stdout, map[string]any{"hookSpecificOutput": map[string]any{
			"hookEventName":     "PostToolUse",
			"additionalContext": res.Text,
		}})
	default:
		emit(stdout, map[string]any{})
	}
}

func emit(stdout io.Writer, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintln(stdout, "{}")
		return
	}
	fmt.Fprintln(stdout, string(data))
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		return
	}
	switch os.Args[1] {
	case "check":
		runCheck(os.Args[2:], os.Stdin, os.Stdout)
	case "redact":
		runRedact(os.Args[2:], os.Stdin, os.Stdout)
	case "-help", "--help", "help":
		fmt.Fprint(os.Stderr, usage)
	default:
		fmt.Fprintf(os.Stderr, "secret-scan: unknown subcommand %q\n", os.Args[1])
	}
}
