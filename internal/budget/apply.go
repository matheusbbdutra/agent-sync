package budget

// budget_apply.go: subcommands CLI para .agent-sync/agent_tasks.jsonl (ADR-022).
//
// Subcommands: read, write (via stdin), stats. Migrado de budget_cli.go
// em 2026-09-21 (Fase 5). Renomeado: budget_cli.go -> budget_apply.go
// (mesma convencao state_apply / event_apply).
import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

func RunCommand(args []string) error {
	if len(args) == 0 {
		return budgetUsage(os.Stderr)
	}
	switch args[0] {
	case "read":
		return runBudgetRead(args[1:])
	case "stats":
		return runBudgetStats(args[1:])
	case "write":
		return runBudgetWrite(args[1:])
	case "nudge":
		return runBudgetNudge(args[1:])
	case "help", "-h", "--help":
		return budgetUsage(os.Stdout)
	default:
		return fmt.Errorf("budget: subcommand desconhecido: %q", args[0])
	}
}

func budgetUsage(w io.Writer) error {
	fmt.Fprintf(w, "Uso: agent-sync budget <subcommand> [-root <path>]\n\n")
	fmt.Fprintf(w, "Subcommands:\n")
	fmt.Fprintf(w, "  read          Imprime tasks do log agent_tasks.jsonl (JSONL por linha, ordem ts)\n")
	fmt.Fprintf(w, "  stats         Agrega tasks por CLI/status; soma tokens e cost_estimate\n")
	fmt.Fprintf(w, "  write         Anexa uma AgentTask lida do stdin (JSON); usado por hook Stop cross-CLI\n")
	fmt.Fprintf(w, "  nudge         Constroi status de tokens (token-budget-status.json); usado por hook postToolUse\n")
	fmt.Fprintf(w, "\nFlags (read/stats):\n")
	fmt.Fprintf(w, "  -last N       Apenas as ultimas N tasks (0 = todas)\n")
	fmt.Fprintf(w, "  -cli C        Filtra por CLI (claude|codex|opencode|cursor|agy)\n")
	fmt.Fprintf(w, "  -status S     Filtra por status (started|completed|failed|cancelled)\n")
	fmt.Fprintf(w, "  -since RFC    Apenas tasks com ts >= valor (RFC3339)\n")
	fmt.Fprintf(w, "\nFlags (write):\n")
	fmt.Fprintf(w, "  -root <path>  Project root (default: derivado do binario + AGENT_SYNC_HOME + cwd)\n")
	fmt.Fprintf(w, "  -dry-run      Valida e imprime o payload, sem escrever\n")
	fmt.Fprintf(w, "\nFlags (nudge):\n")
	fmt.Fprintf(w, "  -actor <c>          Origem (claude|codex|opencode|cursor|agy|agent-sync)\n")
	fmt.Fprintf(w, "  -session-id <s>     Sessao (default: derivado de session-state.json)\n")
	fmt.Fprintf(w, "  -model <m>          Modelo ativo (ex.: claude-sonnet-4.5)\n")
	fmt.Fprintf(w, "  -transcript <p>     Path para transcript JSONL (extracao automatica de tokens)\n")
	fmt.Fprintf(w, "  -tokens-in N        Tokens entrada (default: extrair do transcript)\n")
	fmt.Fprintf(w, "  -tokens-out N       Tokens saida (default: extrair do transcript)\n")
	fmt.Fprintf(w, "  -tool-calls N       Contador de tool calls (heuristica legada)\n")
	fmt.Fprintf(w, "  -threshold PCT      Limiar percentual para nudge (default 80)\n")
	fmt.Fprintf(w, "  -summary <path>     Path summary.md para heuristica legada (vazio desativa)\n")
	fmt.Fprintf(w, "  -root <path>        Project root\n")
	fmt.Fprintf(w, "  -dry-run            (alias de write) valida sem escrever\n")
	return nil
}

type budgetFlags struct {
	fs     *flag.FlagSet
	root   string
	last   int
	cli    string
	status string
	since  string
}

func newBudgetFlags(name string) *budgetFlags {
	f := &budgetFlags{}
	f.fs = flag.NewFlagSet(name, flag.ContinueOnError)
	f.fs.StringVar(&f.root, "root", "", "Project root")
	f.fs.IntVar(&f.last, "last", 0, "Ultimas N tasks (0 = todas)")
	f.fs.StringVar(&f.cli, "cli", "", "Filtra por CLI")
	f.fs.StringVar(&f.status, "status", "", "Filtra por status")
	f.fs.StringVar(&f.since, "since", "", "Filtra por ts >= RFC3339")
	return f
}

func (f *budgetFlags) parse(args []string) error {
	if err := f.fs.Parse(args); err != nil {
		return err
	}
	return nil
}

func (f *budgetFlags) readOpts() (AgentTaskReadOptions, error) {
	opts := AgentTaskReadOptions{Last: f.last, CLI: f.cli, Status: f.status}
	if f.since != "" {
		ts, err := time.Parse(time.RFC3339, f.since)
		if err != nil {
			return opts, fmt.Errorf("since invalido (esperado RFC3339): %w", err)
		}
		opts.SinceTS = ts
	}
	return opts, nil
}

func runBudgetRead(args []string) error {
	f := newBudgetFlags("agent-sync budget read")
	if err := f.parse(args); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	opts, err := f.readOpts()
	if err != nil {
		return err
	}
	tasks, err := ReadAgentTasks(root, opts)
	if err != nil {
		return err
	}
	for _, tk := range tasks {
		line, err := json.Marshal(&tk)
		if err != nil {
			return fmt.Errorf("budget read: marshal: %w", err)
		}
		fmt.Println(string(line))
	}
	return nil
}

func runBudgetStats(args []string) error {
	f := newBudgetFlags("agent-sync budget stats")
	if err := f.parse(args); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	opts, err := f.readOpts()
	if err != nil {
		return err
	}
	stats, err := StatsAgentTasks(root, opts)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(stats)
}

type budgetWriteFlags struct {
	fs     *flag.FlagSet
	root   string
	dryRun bool
}

func newBudgetWriteFlags(name string) *budgetWriteFlags {
	f := &budgetWriteFlags{}
	f.fs = flag.NewFlagSet(name, flag.ContinueOnError)
	f.fs.StringVar(&f.root, "root", "", "Project root")
	f.fs.BoolVar(&f.dryRun, "dry-run", false, "Validar sem escrever")
	return f
}

// runBudgetWrite le uma AgentTask do stdin em JSON e grava via AppendAgentTask.
// Usado pelo hook bash agent-task-record.stop.sh wirado no evento Stop de
// Claude Code, Codex, Antigravity e Cursor. Schema enforcement fica em
// AppendAgentTask via jsonschema.Validate.
func runBudgetWrite(args []string) error {
	f := newBudgetWriteFlags("agent-sync budget write")
	if err := f.fs.Parse(args); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("budget write: stdin: %w", err)
	}
	if len(raw) == 0 {
		return fmt.Errorf("budget write: stdin vazio (esperava JSON AgentTask)")
	}
	var t AgentTask
	if err := json.Unmarshal(raw, &t); err != nil {
		return fmt.Errorf("budget write: JSON invalido: %w", err)
	}
	if f.dryRun {
		data, _ := json.MarshalIndent(&t, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	return AppendAgentTask(root, t)
}
