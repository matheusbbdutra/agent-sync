package hooks

// command.go: fachada de runtime dos hooks (`agent-sync hook <cli>`), em
// oposicao a internal/hooks/apply_*.go, que *wirar* hooks nos arquivos de
// configuracao de cada CLI.
//
// Hoje existe um unico consumidor: `agent-sync hook cline`, a ponte que
// traduz o contrato de hooks do Cline v3 para os scripts agent-sync (A-80.1).

import (
	"fmt"
	"io"
	"os"
)

// RunCommand implementa `agent-sync hook <subcommand> [flags]`.
func RunCommand(args []string) error {
	if len(args) == 0 {
		return hookUsage(os.Stderr)
	}
	switch args[0] {
	case "cline":
		return RunClineBridge(args[1:], os.Stdin, os.Stdout, os.Stderr)
	case "help", "-h", "--help":
		return hookUsage(os.Stdout)
	default:
		return fmt.Errorf("hook: CLI desconhecida: %q (suportadas: cline)", args[0])
	}
}

func hookUsage(w io.Writer) error {
	fmt.Fprintf(w, "Uso: agent-sync hook <cli> [flags]\n\n")
	fmt.Fprintf(w, "Ponte de contrato de hooks em runtime (ver docs/investigations/cline-hooks-contract.md).\n\n")
	fmt.Fprintf(w, "Subcommands:\n")
	fmt.Fprintf(w, "  cline        Traduz payload Cline <-> scripts agent-sync (uso interno dos shims de ~/.cline/hooks)\n")
	fmt.Fprintf(w, "\nFlags (cline):\n")
	fmt.Fprintf(w, "  -event <E>      Evento Cline (PreToolUse, PostToolUse, TaskStart, TaskComplete) [obrigatorio]\n")
	fmt.Fprintf(w, "  -base-dir <D>   Raiz do repo agent-sync (default: AGENT_SYNC_HOME ou cwd)\n")
	fmt.Fprintf(w, "  -deny-mode <M>  stop (default, aborta o run) | warn (so injeta contexto)\n")
	fmt.Fprintf(w, "  -timeout <D>    Timeout por script (default: 10s)\n")
	return nil
}
