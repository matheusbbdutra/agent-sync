// Command repo-map mantém um cache estrutural incremental de um repositório,
// inspirado no Aider RepoMap. Três modos:
//
//	repo-map --update [--quiet]                  // re-parseia deltas (mtime/size)
//	repo-map --focus <caminho> [--depth N]       // subgrafo focado em um arquivo
//	repo-map --summary [--max-tokens N]          // top hubs de chamadas/imports
//
// O cache fica em <root>/.agent-sync/cache/repomap.json por padrão e é
// versionado (campo "version": 1).
//
// O modo --update é silencioso por design (--quiet) e ideal para ser
// chamado de hooks de inicialização sem poluir o prompt.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/matheusdutra/token-tools/internal/repomap"
)

type config struct {
	mode        string
	update      bool
	quiet       bool
	root        string
	cacheDir    string
	focus       string
	depth       int
	summary     bool
	maxTokens   int
	showVersion bool
}

func parseFlags(args []string) (*config, error) {
	fs := flag.NewFlagSet("repo-map", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	update := fs.Bool("update", false, "Atualiza o cache incremental (delega a git ls-files e re-parseia apenas modificados)")
	quiet := fs.Bool("quiet", false, "Em --update, suprime saida em stdout; stderr continua livre para erros")
	root := fs.String("root", "", "Raiz do repositorio (default: diretório atual)")
	cacheDir := fs.String("cache-dir", "", "Diretório do cache (default: <root>/.agent-sync/cache)")
	focus := fs.String("focus", "", "Retorna subgrafo textual focado em <caminho/arquivo>")
	depth := fs.Int("depth", 1, "Profundidade do subgrafo (1 = direto, 2 = inclui importadores dos importadores)")
	summary := fs.Bool("summary", false, "Retorna os símbolos centrais do projeto")
	maxTokens := fs.Int("max-tokens", 0, "Em --summary, limita a saída a ~N tokens (0 = sem limite)")
	showVersion := fs.Bool("version", false, "Mostra a versão do cache e sai")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Uso: repo-map [flags]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Modos:")
		fmt.Fprintln(os.Stderr, "  --update                  Re-parseia deltas e reescreve o cache")
		fmt.Fprintln(os.Stderr, "  --focus <caminho>         Subgrafo em torno de <caminho>")
		fmt.Fprintln(os.Stderr, "  --summary                 Top hubs de chamadas/imports")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Flags:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg := &config{
		update:      *update,
		quiet:       *quiet,
		root:        *root,
		cacheDir:    *cacheDir,
		focus:       *focus,
		depth:       *depth,
		summary:     *summary,
		maxTokens:   *maxTokens,
		showVersion: *showVersion,
	}
	if cfg.root == "" {
		if wd, err := os.Getwd(); err == nil {
			cfg.root = wd
		}
	}
	if cfg.cacheDir == "" {
		cfg.cacheDir = repomap.DefaultCacheDir(cfg.root)
	}
	return cfg, nil
}

func main() {
	os.Exit(runMain(os.Args[1:]))
}

// runMain é o ponto de entrada testável. Despacha para o modo correto e
// retorna o exit code; nunca chama os.Exit diretamente.
func runMain(args []string) int {
	cfg, err := parseFlags(args)
	if err != nil {
		return 2
	}
	switch {
	case cfg.showVersion:
		fmt.Fprintf(os.Stdout, "repo-map cache v%d\n", repomap.CacheVersion)
		return 0
	case cfg.update:
		return runUpdate(cfg)
	case cfg.focus != "":
		return runFocus(cfg)
	case cfg.summary:
		return runSummary(cfg)
	default:
		fmt.Fprintln(os.Stderr, "Erro: nenhum modo informado. Use --update, --focus ou --summary.")
		fmt.Fprintln(os.Stderr, "")
		printMainUsage()
		return 2
	}
}

func printMainUsage() {
	fmt.Fprintln(os.Stderr, "Uso: repo-map [flags]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Modos:")
	fmt.Fprintln(os.Stderr, "  --update                  Re-parseia deltas e reescreve o cache")
	fmt.Fprintln(os.Stderr, "  --focus <caminho>         Subgrafo em torno de <caminho>")
	fmt.Fprintln(os.Stderr, "  --summary                 Top hubs de chamadas/imports")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Flags comuns:")
	fmt.Fprintln(os.Stderr, "  --root <path>             Raiz do repositorio (default: cwd)")
	fmt.Fprintln(os.Stderr, "  --cache-dir <path>        Diretorio do cache (default: <root>/.agent-sync/cache)")
	fmt.Fprintln(os.Stderr, "  --quiet                   Em --update, suprime stdout")
	fmt.Fprintln(os.Stderr, "  --version                 Imprime versao do cache e sai")
}

func runUpdate(cfg *config) int {
	cache, stats, err := repomap.Update(cfg.root, cfg.cacheDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "repo-map: %v\n", err)
		return 1
	}
	if !cfg.quiet {
		fmt.Fprintf(os.Stdout, "repo-map: %d arquivos (%d reusados, %d re-parseados, %d removidos) em %dms — cache=%d arestas\n",
			stats.Visited, stats.Reused, stats.Reparsed, stats.Removed, stats.ElapsedMs, stats.EdgesNow)
		_ = cache
	}
	return 0
}

func runFocus(cfg *config) int {
	cache, err := repomap.Load(cfg.cacheDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "repo-map: %v\n", err)
		return 1
	}
	if cache == nil {
		fmt.Fprintln(os.Stderr, "(cache ausente — execute repo-map --update primeiro)")
		return 1
	}
	out := repomap.Focus(cache, cfg.focus, cfg.depth)
	fmt.Print(out)
	return 0
}

func runSummary(cfg *config) int {
	cache, err := repomap.Load(cfg.cacheDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "repo-map: %v\n", err)
		return 1
	}
	if cache == nil {
		fmt.Fprintln(os.Stderr, "(cache ausente — execute repo-map --update primeiro)")
		return 1
	}
	out := repomap.Summary(cache, cfg.maxTokens)
	fmt.Print(out)
	return 0
}