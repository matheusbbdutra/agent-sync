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
	case "help", "-h", "--help":
		return hookUsage(os.Stdout)
	default:
		// A-90: o subcomando `hook cline` foi removido. O plugin Cline agora é
		// TS autocontido (`cline-plugin/src/plugin.ts`) e chama os binários
		// core via `execFile` direto quando precisa. Este comando fica como
		// no-op explícito para detectar wiramentos legados remanescentes.
		return fmt.Errorf("hook: CLI/subcomando desconhecido: %q (A-90: removido — wiramento migrou para plugin TS em cline-plugin/)", args[0])
	}
}

func hookUsage(w io.Writer) error {
	fmt.Fprintf(w, "Uso: agent-sync hook <subcomando> [flags]\n\n")
	fmt.Fprintf(w, "Estado atual (A-90): nenhum subcomando ativo.\n")
	fmt.Fprintf(w, "O wiramento de hooks Cline migrou para o plugin TS autocontido em\n")
	fmt.Fprintf(w, "  cline-plugin/src/plugin.ts (entry: cline-plugin/scripts/install.ts).\n")
	fmt.Fprintf(w, "Hooks Cline chamam binários core Go via execFile direto quando necessário.\n")
	return nil
}
