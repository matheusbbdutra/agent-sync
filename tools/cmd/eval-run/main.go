// Command eval-run avalia um artefato (skill, prompt, AGENTS.md) contra uma
// suite de casos em JSONL, compara com um baseline e registra cada execucao.
//
//	eval-run suite check <suite.jsonl> [--json]     valida a suite
//	eval-run run --suite <s> --target <t> [flags]   pontua e registra
//	eval-run report [--run <id>] [--json]           le um registro gravado
//	eval-run list [--limit N] [--json]              lista execucoes gravadas
//
// O scoring deterministico (must_match, must_not_match, json_schema) nao
// depende de modelo. O juiz externo (--judge-cmd) e opcional e opaco: o binario
// so escreve {"case":...,"output":...} em stdin e le {"score":N,"reason":"..."}
// em stdout — sem SDK de provider e sem HTTP.
//
// Exit codes: 0 ok, 1 falhas, 2 uso/config, 3 incomparavel (hash).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/matheusdutra/token-tools/internal/evalrun"
	"github.com/matheusdutra/token-tools/internal/evalsuite"
)

const (
	exitOK           = 0
	exitFailures     = 1
	exitUsage        = 2
	exitIncomparable = 3
)

const usage = `eval-run — avaliacao deterministica de skills e artefatos

Usage:
  eval-run suite check <suite.jsonl> [--json]
  eval-run run --suite <suite.jsonl> --target <arquivo> [flags]
  eval-run report [--run <run_id>] [--record <arquivo>] [--json]
  eval-run list [--record <arquivo>] [--limit N] [--json]

Flags de run:
  --suite <arquivo>    suite JSONL de casos
  --target <arquivo>   artefato avaliado (o conteudo do arquivo e o output de cada caso)
  --runner-cmd <cmd>   processo externo que gera a saida de cada caso
                       (stdin {"case"} -> stdout = saida). Sem default: quem roda
                       escolhe o CLI. Com runner, --target passa a ser opcional.
  --baseline <valor>   caminho de registro ou run_id usado como baseline
  --tolerance <float>  queda de pass_rate que conta como regressao (default 0.02)
  --judge-cmd <cmd>    processo externo de judge (stdin {"case","output"} -> stdout {"score","reason"})
  --slice <nome>       restringe a execucao a um slice
  --model <nome>       modelo registrado no run (default vazio)
  --record <arquivo>   registro append-only (default .agent-sync/eval-runs.jsonl)
  --no-record          nao grava o registro
  --json               saida JSON

Exit codes: 0 ok, 1 falhas, 2 uso/config, 3 incomparavel (hash)
`

func main() {
	os.Exit(runMain(os.Args[1:], os.Stdout, os.Stderr))
}

// runMain e o ponto de entrada testavel: devolve exit code em vez de chamar
// os.Exit, para que os testes exercitem o contrato de codigos.
func runMain(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	switch args[0] {
	case "suite":
		return runSuite(args[1:], stdout, stderr)
	case "run":
		return runEval(args[1:], stdout, stderr)
	case "report":
		return runReport(args[1:], stdout, stderr)
	case "list":
		return runList(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return exitOK
	default:
		fmt.Fprintf(stderr, "eval-run: subcomando desconhecido %q\n\n%s", args[0], usage)
		return exitUsage
	}
}

// runSuite despacha `eval-run suite check`.
func runSuite(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "check" {
		fmt.Fprintf(stderr, "eval-run suite: esperado 'check <suite.jsonl>'\n\n%s", usage)
		return exitUsage
	}
	path, rest := splitPositional(args[1:])
	fs := flag.NewFlagSet("suite check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "saida JSON")
	if err := fs.Parse(rest); err != nil {
		return exitUsage
	}
	if path == "" {
		fmt.Fprintf(stderr, "eval-run suite check: informe o arquivo da suite\n")
		return exitUsage
	}

	cases, err := evalsuite.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "eval-run suite check: %v\n", err)
		return exitUsage
	}
	slices := map[string]bool{}
	for _, c := range cases {
		slices[c.Slice] = true
	}
	report := suiteCheckReport{Suite: path, Cases: len(cases), Slices: len(slices), Valid: true}
	if *jsonOut {
		return emitJSON(stdout, stderr, report, exitOK)
	}
	fmt.Fprintf(stdout, "suite %s\n", path)
	fmt.Fprintf(stdout, "casos %d  slices %d\nok\n", report.Cases, report.Slices)
	return exitOK
}

type suiteCheckReport struct {
	Suite  string `json:"suite"`
	Cases  int    `json:"cases"`
	Slices int    `json:"slices"`
	Valid  bool   `json:"valid"`
}

// runEval despacha `eval-run run`.
func runEval(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	suitePath := fs.String("suite", "", "suite JSONL de casos")
	targetPath := fs.String("target", "", "artefato avaliado")
	baseline := fs.String("baseline", "", "caminho de registro ou run_id do baseline")
	tolerance := fs.Float64("tolerance", 0.02, "queda de pass_rate que conta como regressao")
	judgeCmd := fs.String("judge-cmd", "", "processo externo de judge")
	runnerCmd := fs.String("runner-cmd", "", "processo externo que gera a saida de cada caso (sem default)")
	sliceName := fs.String("slice", "", "restringe a execucao a um slice")
	model := fs.String("model", "", "modelo registrado no run")
	recordPath := fs.String("record", evalrun.DefaultRecordPath, "registro append-only")
	noRecord := fs.Bool("no-record", false, "nao grava o registro")
	jsonOut := fs.Bool("json", false, "saida JSON")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "eval-run run: argumento inesperado %q\n", fs.Arg(0))
		return exitUsage
	}
	if *suitePath == "" || (*targetPath == "" && *runnerCmd == "") {
		fmt.Fprintf(stderr, "eval-run run: --suite e (--target ou --runner-cmd) sao obrigatorios\n\n%s", usage)
		return exitUsage
	}
	if *tolerance < 0 {
		fmt.Fprintf(stderr, "eval-run run: --tolerance nao pode ser negativo\n")
		return exitUsage
	}

	cases, err := evalsuite.Load(*suitePath)
	if err != nil {
		fmt.Fprintf(stderr, "eval-run run: %v\n", err)
		return exitUsage
	}
	if *sliceName != "" {
		filtered := cases[:0:0]
		for _, c := range cases {
			if c.Slice == *sliceName {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) == 0 {
			fmt.Fprintf(stderr, "eval-run run: slice %q nao existe na suite\n", *sliceName)
			return exitUsage
		}
		cases = filtered
	}

	suiteHash, err := evalrun.HashFile(*suitePath)
	if err != nil {
		fmt.Fprintf(stderr, "eval-run run: %v\n", err)
		return exitUsage
	}
	// --target e opcional quando ha runner: no modo offline o conteudo do
	// artefato e a saida de todo caso; no modo runner a saida vem do comando.
	var output, targetHash string
	if *targetPath != "" {
		raw, err := os.ReadFile(*targetPath)
		if err != nil {
			fmt.Fprintf(stderr, "eval-run run: %v\n", err)
			return exitUsage
		}
		output = string(raw)
		targetHash, err = evalrun.HashFile(*targetPath)
		if err != nil {
			fmt.Fprintf(stderr, "eval-run run: %v\n", err)
			return exitUsage
		}
	}

	var runner evalrun.Runner
	if *runnerCmd != "" {
		runner = evalrun.ExecRunner{Command: *runnerCmd}
	}
	var judge evalrun.Judge
	if *judgeCmd != "" {
		judge = evalrun.ExecJudge{Command: *judgeCmd}
	}

	results := make([]evalrun.CaseResult, 0, len(cases))
	for _, c := range cases {
		caseOutput := output
		if runner != nil {
			out, rerr := runner.Run(c)
			if rerr != nil {
				fmt.Fprintf(stderr, "eval-run run: caso %s: %v\n", c.ID, rerr)
				return exitUsage
			}
			caseOutput = out
		}
		res, err := evalrun.ScoreCase(c, caseOutput, judge)
		if err != nil {
			fmt.Fprintf(stderr, "eval-run run: caso %s: %v\n", c.ID, err)
			return exitUsage
		}
		results = append(results, res)
	}
	totals, bySlices := evalrun.Aggregate(results)

	now := time.Now().UTC()
	runID, err := evalrun.NewRunID(now)
	if err != nil {
		fmt.Fprintf(stderr, "eval-run run: %v\n", err)
		return exitUsage
	}
	record := evalrun.RunRecord{
		SchemaVersion: evalrun.RecordSchemaVersion,
		RunID:         runID,
		TS:            now.Format(time.RFC3339),
		Suite:         *suitePath,
		SuiteHash:     suiteHash,
		Target:        *targetPath,
		TargetHash:    targetHash,
		Model:         *model,
		Runner:        *runnerCmd,
		Totals:        totals,
		BySlice:       bySlices,
	}

	var cmp *evalrun.Comparison
	if *baseline != "" {
		baseRec, err := evalrun.ResolveBaseline(*baseline, *recordPath)
		if err != nil {
			fmt.Fprintf(stderr, "eval-run run: %v\n", err)
			return exitUsage
		}
		c := evalrun.CompareBaseline(baseRec, &record, *tolerance)
		cmp = &c
		comparable := c.Comparable
		record.Comparable = &comparable
		record.Baseline = &evalrun.BaselineRef{
			RunID:    baseRec.RunID,
			TS:       baseRec.TS,
			PassRate: evalrun.OverallPassRate(baseRec.BySlice),
		}
		if c.Comparable {
			record.Delta = &evalrun.Delta{PassRate: c.DeltaPassRate, Regressions: c.Regressions}
		}
	}

	if !*noRecord {
		if err := evalrun.AppendRecord(*recordPath, record); err != nil {
			fmt.Fprintf(stderr, "eval-run run: %v\n", err)
			return exitUsage
		}
	}

	if *jsonOut {
		payload := runOutput{Record: record, Results: results, Comparison: cmp}
		code := exitCodeFor(totals, cmp)
		return emitJSON(stdout, stderr, payload, code)
	}
	printRun(stdout, record, results, cmp, *recordPath, *noRecord)
	return exitCodeFor(totals, cmp)
}

type runOutput struct {
	Record     evalrun.RunRecord    `json:"record"`
	Results    []evalrun.CaseResult `json:"results"`
	Comparison *evalrun.Comparison  `json:"comparison,omitempty"`
}

// exitCodeFor aplica a precedencia dos codigos: incomparavel vence falhas,
// porque delta medido contra suite/target diferentes nao significa nada.
func exitCodeFor(totals evalrun.Totals, cmp *evalrun.Comparison) int {
	if cmp != nil && !cmp.Comparable {
		return exitIncomparable
	}
	if totals.Fail > 0 {
		return exitFailures
	}
	return exitOK
}

func printRun(w io.Writer, rec evalrun.RunRecord, results []evalrun.CaseResult, cmp *evalrun.Comparison, recordPath string, noRecord bool) {
	fmt.Fprintf(w, "run %s\n", rec.RunID)
	fmt.Fprintf(w, "suite   %s (%s)\n", rec.Suite, rec.SuiteHash)
	fmt.Fprintf(w, "target  %s (%s)\n", rec.Target, rec.TargetHash)
	if rec.Model != "" {
		fmt.Fprintf(w, "model   %s\n", rec.Model)
	}
	t := rec.Totals
	fmt.Fprintf(w, "casos %d  scored %d  skipped %d  pass %d  fail %d\n", t.Cases, t.Scored, t.Skipped, t.Pass, t.Fail)
	printSlices(w, rec.BySlice)
	for _, r := range results {
		if r.Status == evalrun.StatusFail {
			fmt.Fprintf(w, "fail  %s (%s): %s\n", r.CaseID, r.Slice, r.Reason)
		}
	}
	if cmp != nil {
		if cmp.Comparable {
			fmt.Fprintf(w, "baseline %s  delta_pass_rate %+.3f  regressions %d\n",
				rec.Baseline.RunID, rec.Delta.PassRate, rec.Delta.Regressions)
			for _, sc := range cmp.Slices {
				if sc.Regression {
					fmt.Fprintf(w, "  regressao em %s: %+.3f\n", sc.Slice, *sc.DeltaPassRate)
				}
			}
		} else {
			fmt.Fprintf(w, "incomparavel: %s\n", cmp.Reason)
		}
	}
	if noRecord {
		fmt.Fprintf(w, "registro nao gravado (--no-record)\n")
	} else {
		fmt.Fprintf(w, "registro %s\n", recordPath)
	}
}

func printSlices(w io.Writer, slices []evalrun.SliceStats) {
	fmt.Fprintf(w, "\n%-20s %6s %7s %6s %5s %9s\n", "slice", "casos", "scored", "pass", "fail", "pass_rate")
	for _, st := range slices {
		fmt.Fprintf(w, "%-20s %6d %7d %6d %5d %9.3f\n", st.Slice, st.Cases, st.Scored, st.Pass, st.Fail, st.PassRate)
	}
}

// runReport imprime um registro ja gravado (o ultimo por default).
func runReport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	runID := fs.String("run", "", "run_id a exibir (default: ultimo registro)")
	recordPath := fs.String("record", evalrun.DefaultRecordPath, "registro append-only")
	jsonOut := fs.Bool("json", false, "saida JSON")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	records, err := evalrun.ReadRecords(*recordPath)
	if err != nil {
		fmt.Fprintf(stderr, "eval-run report: %v\n", err)
		return exitUsage
	}
	if len(records) == 0 {
		fmt.Fprintf(stderr, "eval-run report: nenhum registro em %s\n", *recordPath)
		return exitUsage
	}
	rec := records[len(records)-1]
	if *runID != "" {
		found, ok := evalrun.FindRecord(records, *runID)
		if !ok {
			fmt.Fprintf(stderr, "eval-run report: run_id %q nao encontrado em %s\n", *runID, *recordPath)
			return exitUsage
		}
		rec = *found
	}

	if *jsonOut {
		return emitJSON(stdout, stderr, rec, exitOK)
	}
	fmt.Fprintf(stdout, "run %s  ts %s\n", rec.RunID, rec.TS)
	fmt.Fprintf(stdout, "suite   %s (%s)\n", rec.Suite, rec.SuiteHash)
	fmt.Fprintf(stdout, "target  %s (%s)\n", rec.Target, rec.TargetHash)
	if rec.Model != "" {
		fmt.Fprintf(stdout, "model   %s\n", rec.Model)
	}
	t := rec.Totals
	fmt.Fprintf(stdout, "casos %d  scored %d  skipped %d  pass %d  fail %d\n", t.Cases, t.Scored, t.Skipped, t.Pass, t.Fail)
	printSlices(stdout, rec.BySlice)
	if rec.Baseline != nil {
		if rec.Comparable != nil && *rec.Comparable && rec.Delta != nil {
			fmt.Fprintf(stdout, "baseline %s  delta_pass_rate %+.3f  regressions %d\n",
				rec.Baseline.RunID, rec.Delta.PassRate, rec.Delta.Regressions)
		} else {
			fmt.Fprintf(stdout, "baseline %s  incomparavel (hash divergente)\n", rec.Baseline.RunID)
		}
	}
	return exitOK
}

// runList lista as execucoes gravadas na ordem do arquivo (append-only).
func runList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	recordPath := fs.String("record", evalrun.DefaultRecordPath, "registro append-only")
	limit := fs.Int("limit", 20, "maximo de runs listados (0 = todos)")
	jsonOut := fs.Bool("json", false, "saida JSON")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if *limit < 0 {
		fmt.Fprintf(stderr, "eval-run list: --limit nao pode ser negativo\n")
		return exitUsage
	}
	records, err := evalrun.ReadRecords(*recordPath)
	if err != nil {
		fmt.Fprintf(stderr, "eval-run list: %v\n", err)
		return exitUsage
	}
	if *limit > 0 && len(records) > *limit {
		records = records[len(records)-*limit:]
	}
	if *jsonOut {
		return emitJSON(stdout, stderr, records, exitOK)
	}
	if len(records) == 0 {
		fmt.Fprintf(stdout, "nenhum registro em %s\n", *recordPath)
		return exitOK
	}
	fmt.Fprintf(stdout, "%-32s %-20s %8s %6s %6s %s\n", "run_id", "ts", "pass", "fail", "skip", "target")
	for _, rec := range records {
		fmt.Fprintf(stdout, "%-32s %-20s %8d %6d %6d %s\n",
			rec.RunID, rec.TS, rec.Totals.Pass, rec.Totals.Fail, rec.Totals.Skipped, rec.Target)
	}
	return exitOK
}

// emitJSON escreve v como JSON indentado e devolve o exit code recebido.
func emitJSON(stdout, stderr io.Writer, v any, code int) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(stderr, "eval-run: serializar saida: %v\n", err)
		return exitUsage
	}
	return code
}

// splitPositional separa o primeiro argumento posicional (se houver) dos
// demais, permitindo que os flags venham antes ou depois dele.
func splitPositional(args []string) (string, []string) {
	for i, a := range args {
		if !strings.HasPrefix(a, "-") {
			rest := make([]string, 0, len(args)-1)
			rest = append(rest, args[:i]...)
			rest = append(rest, args[i+1:]...)
			return a, rest
		}
	}
	return "", args
}
