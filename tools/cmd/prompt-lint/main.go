// Command prompt-lint faz lint mecanico de prompt de aplicacao.
//
//	prompt-lint check <arquivo.md|-> [--json] [--severity error|warning] [--quiet]
//
// Ele procura defeitos estruturais que o proprio texto revela: proibicao sem
// alternativa positiva (p.102), prompt longo sem marcador de fim (p.161-162),
// pedido de saida sem formato declarado (p.143), rotulos de classificacao nao
// disjuntos (p.165) e placeholder nao preenchido. Nao avalia qualidade
// semantica e nao chama modelo: a entrada e texto, a saida e deterministica.
//
// Exit codes: 0 sem achado, 1 com achado >= --severity, 2 uso ou erro de leitura.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/matheusdutra/token-tools/internal/promptlint"
)

const (
	exitOK       = 0
	exitFindings = 1
	exitUsage    = 2
)

const usage = `prompt-lint — lint mecanico de prompt (defeitos estruturais, sem modelo)

Usage:
  prompt-lint check <arquivo.md|-> [flags]

Flags de check:
  --json                 saida JSON estruturada
  --severity <nivel>     minimo que reprova: error|warning (default warning)
  --quiet                omite cabecalho e contagem por check (1 linha por achado)
  --help                 esta ajuda

O caminho vem antes das flags de valor (ex.: prompt-lint check p.md --severity error).
Use "-" como caminho para ler o prompt de stdin.

Checks: negation, no-terminator, undeclared-output-format,
        non-disjoint-classes, placeholder-leak

Exit codes: 0 sem achado, 1 com achado >= severity, 2 uso ou erro de leitura.
`

func main() {
	os.Exit(runMain(os.Args[1:], os.Stdout, os.Stderr, os.Stdin))
}

// runMain e o ponto de entrada testavel: devolve exit code em vez de chamar
// os.Exit, para que os testes exercitem o contrato de codigos.
func runMain(args []string, stdout, stderr io.Writer, stdin io.Reader) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], stdout, stderr, stdin)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return exitOK
	default:
		fmt.Fprintf(stderr, "prompt-lint: subcomando desconhecido %q\n\n%s", args[0], usage)
		return exitUsage
	}
}

// checkReport e o payload de --json.
type checkReport struct {
	Prompt   string               `json:"prompt"`
	Chars    int                  `json:"chars"`
	Lines    int                  `json:"lines"`
	Severity string               `json:"severity"`
	Summary  promptlint.Summary   `json:"summary"`
	Findings []promptlint.Finding `json:"findings"`
}

// runCheck despacha `prompt-lint check`.
func runCheck(args []string, stdout, stderr io.Writer, stdin io.Reader) int {
	path, rest := splitPositional(args)
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "saida JSON")
	severity := fs.String("severity", promptlint.SeverityWarning, "severidade minima que reprova")
	quiet := fs.Bool("quiet", false, "omite cabecalho e contagem por check")
	if err := fs.Parse(rest); err != nil {
		return exitUsage
	}
	if path == "" {
		fmt.Fprintf(stderr, "prompt-lint check: informe o arquivo do prompt (ou - para stdin)\n\n%s", usage)
		return exitUsage
	}
	if _, ok := promptlint.SeverityRank(*severity); !ok {
		fmt.Fprintf(stderr, "prompt-lint: severidade invalida %q (use error ou warning)\n", *severity)
		return exitUsage
	}

	content, err := readPrompt(path, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "prompt-lint: %v\n", err)
		return exitUsage
	}
	findings, err := promptlint.FilterBySeverity(promptlint.Check(content), *severity)
	if err != nil {
		fmt.Fprintf(stderr, "prompt-lint: %v\n", err)
		return exitUsage
	}

	label := path
	if path == "-" {
		label = "<stdin>"
	}
	summary := promptlint.Summarize(findings)
	report := checkReport{
		Prompt:   label,
		Chars:    len(content),
		Lines:    countLines(content),
		Severity: *severity,
		Summary:  summary,
		Findings: findings,
	}
	if report.Findings == nil {
		report.Findings = []promptlint.Finding{}
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(stderr, "prompt-lint: serializar saida: %v\n", err)
			return exitUsage
		}
	} else {
		printText(stdout, report, *quiet)
	}

	if summary.Total > 0 {
		return exitFindings
	}
	return exitOK
}

// printText imprime o resultado no formato do `agent-sync skills lint`:
// cabecalho com contadores, tabela de achados e contagem por check no final.
func printText(stdout io.Writer, report checkReport, quiet bool) {
	if !quiet {
		fmt.Fprintf(stdout, "prompt: %s (%d chars, %d linhas)\n", report.Prompt, report.Chars, report.Lines)
		fmt.Fprintf(stdout, "checks: %d  achados: %d (errors=%d, warnings=%d, severity>=%s)\n",
			len(promptlint.CheckCodes()), report.Summary.Total, report.Summary.Errors,
			report.Summary.Warnings, report.Severity)
	}
	if report.Summary.Total == 0 {
		if !quiet {
			fmt.Fprintln(stdout, "\n✅ Nenhum achado.")
			printCounts(stdout, report.Summary)
		}
		return
	}

	if !quiet {
		fmt.Fprintln(stdout)
		fmt.Fprintf(stdout, "%-6s %-9s %-25s %s\n", "linha", "severity", "check", "mensagem")
		fmt.Fprintln(stdout, strings.Repeat("-", 78))
	}
	for _, f := range report.Findings {
		fmt.Fprintf(stdout, "%-6d %-9s %-25s %s\n", f.Line, f.Severity, f.Check, f.Message)
		if !quiet {
			fmt.Fprintf(stdout, "%sfix: %s\n", strings.Repeat(" ", 6), f.Fix)
		}
	}
	if !quiet {
		printCounts(stdout, report.Summary)
	}
}

// printCounts imprime a contagem por check (sempre as 5 linhas, inclusive 0).
func printCounts(stdout io.Writer, summary promptlint.Summary) {
	fmt.Fprintln(stdout, "\ncontagem por check:")
	for _, code := range promptlint.CheckCodes() {
		fmt.Fprintf(stdout, "  %-26s %d\n", code, summary.ByCheck[code])
	}
}

// readPrompt le o arquivo do prompt, ou stdin quando path == "-".
func readPrompt(path string, stdin io.Reader) (string, error) {
	if path == "-" {
		raw, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("ler stdin: %w", err)
		}
		return string(raw), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("ler prompt: %w", err)
	}
	return string(raw), nil
}

// countLines conta linhas sem contar o \n final como linha vazia.
func countLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(strings.TrimSuffix(content, "\n"), "\n") + 1
}

// valueFlags precisam do argumento seguinte; splitPositional os pula para o
// valor nao ser confundido com o caminho posicional.
var valueFlags = map[string]bool{
	"--severity": true,
	"-severity":  true,
}

// splitPositional separa o primeiro argumento posicional (o arquivo) dos
// demais, permitindo que os flags venham antes ou depois dele.
func splitPositional(args []string) (string, []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if valueFlags[a] {
			i++
			continue
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			continue
		}
		rest := make([]string, 0, len(args)-1)
		rest = append(rest, args[:i]...)
		rest = append(rest, args[i+1:]...)
		return a, rest
	}
	return "", args
}
