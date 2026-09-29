package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// benchResult é 1 linha da tabela de benchmark. Mantém-se simples: 6
// campos, todos serializáveis como tabela markdown.
type benchResult struct {
	Feature      string `json:"feature"`
	Dataset      string `json:"dataset"`
	OrigBytes    int    `json:"orig_bytes"`
	OutBytes     int    `json:"out_bytes"`
	ReductionPct int    `json:"reduction_pct"`
	LatencyUs    int64  `json:"latency_us"`
}

// runBenchmark é o Phase 0 do ADR-context-window-strategy: suite mínima
// que mede reduction% + latência de cada feature contra fixtures inline.
// Output: tabela markdown consumível em PR description. Sem gráficos,
// sem dataset sintético gerado — só o suficiente para validar que os
// defaults (K=5, threshold=4KB, loop window=10, skeletonMin=8KB) estão
// sensatos. Substitui o placeholder "not implemented yet".
func runBenchmark(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("benchmark", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit results as JSON instead of markdown")
	if err := fs.Parse(args); err != nil {
		return err
	}
	results := []benchResult{}
	results = append(results, benchLoopDetection()...)
	results = append(results, benchQuality()...)
	results = append(results, benchSkeleton()...)
	results = append(results, benchArchive()...)
	results = append(results, benchBashCompress()...)
	if *asJSON {
		return writeBenchJSON(stdout, results)
	}
	writeBenchMarkdown(stdout, results)
	return nil
}

func benchLoopDetection() []benchResult {
	cases := []struct {
		name  string
		turns []Turn
	}{
		{"empty", nil},
		{"small-5", makeTurns(5, "Edit: file.go")},
		{"medium-50-no-loop", makeTurns(50, "Read: foo.go")},
		{"loop-3x-5", append(makeTurns(2, "Edit: foo.go"), makeTurns(3, "Edit: foo.go")...)},
		{"loop-10x-30", append(makeTurns(20, "Bash: ls"), makeTurns(10, "Bash: ls")...)},
	}
	var out []benchResult
	for _, c := range cases {
		start := time.Now()
		sig := DetectLoop(c.turns, 10, 3)
		ns := time.Since(start).Nanoseconds()
		orig := sumContentBytes(c.turns)
		out = append(out, benchResult{
			Feature:      "loop_detect",
			Dataset:      c.name,
			OrigBytes:    orig,
			OutBytes:     orig,
			ReductionPct: 0,
			LatencyUs:    ns / 1000,
		})
		_ = sig
	}
	return out
}

func benchQuality() []benchResult {
	cases := []struct {
		name     string
		turns    []Turn
		versions int
	}{
		{"empty", nil, 0},
		{"normal-5", makeTurns(5, "Edit: file.go"), 0},
		{"bloated-1x4KB", []Turn{{Content: strings.Repeat("x", 8000)}}, 0},
		{"loop-heavy-10x", makeTurns(10, "Edit: foo.go"), 0},
		{"deep-compact-10", makeTurns(5, "Bash: test"), 10},
	}
	var out []benchResult
	for _, c := range cases {
		// usa Session fake in-memory (sem disco)
		s := &Session{ID: "bench", K: 5, Budget: 1000, Turns: c.turns, Version: c.versions}
		start := time.Now()
		report := ComputeQuality(s)
		ns := time.Since(start).Nanoseconds()
		orig := sumContentBytes(c.turns)
		out = append(out, benchResult{
			Feature:      "quality",
			Dataset:      c.name,
			OrigBytes:    orig,
			OutBytes:     report.Score, // score (0-100) como proxy de "info útil"
			ReductionPct: report.Score,
			LatencyUs:    ns / 1000,
		})
	}
	return out
}

func benchSkeleton() []benchResult {
	// fixtures: arquivo pequeno, médio, grande (> skeletonMinBytes)
	dir := mustTempDir("bench-skel")
	defer os.RemoveAll(dir)
	fixtures := []struct {
		name string
		body string
	}{
		{"tiny-1KB", strings.Repeat("# h\n", 100)},
		{"medium-12KB", makePythonFixture(12000)},
		{"big-30KB", makePythonFixture(30000)},
	}
	var out []benchResult
	for _, fx := range fixtures {
		path := filepath.Join(dir, fx.name+".py")
		if err := os.WriteFile(path, []byte(fx.body), 0o644); err != nil {
			continue
		}
		start := time.Now()
		res, err := Skeletonize(path, false)
		ns := time.Since(start).Nanoseconds()
		if err != nil || res == nil {
			continue
		}
		out = append(out, benchResult{
			Feature:      "skeleton",
			Dataset:      fx.name,
			OrigBytes:    res.Bytes,
			OutBytes:     res.SkeletonChars,
			ReductionPct: res.ReductionPct,
			LatencyUs:    ns / 1000,
		})
	}
	return out
}

func benchArchive() []benchResult {
	cases := []struct {
		name string
		size int
	}{
		{"small-3KB", 3000}, // abaixo threshold (4KB)
		{"medium-8KB", 8000},
		{"big-32KB", 32000},
	}
	var out []benchResult
	for _, c := range cases {
		body := strings.Repeat("a", c.size)
		start := time.Now()
		preview, id, archived := MaybeArchive(body)
		ns := time.Since(start).Nanoseconds()
		out = append(out, benchResult{
			Feature:      "archive",
			Dataset:      c.name,
			OrigBytes:    c.size,
			OutBytes:     len(preview),
			ReductionPct: pct(c.size, len(preview)),
			LatencyUs:    ns / 1000,
		})
		_ = id
		_ = archived
	}
	return out
}

func benchBashCompress() []benchResult {
	cases := []struct {
		name string
		cmd  string
		body string
	}{
		{"ls-short", "ls -la", "foo.go\nbar.go"},
		{"ls-medium", "ls -la", strings.Repeat("file.go\n", 50)},
		{"cat-big", "cat big.log", strings.Repeat("line\n", 200)},
		{"npm-skip", "npm install", strings.Repeat("info\n", 100)},
		{"pipe-skip", "ls | grep foo", strings.Repeat("foo\n", 50)},
	}
	var out []benchResult
	for _, c := range cases {
		start := time.Now()
		res := CompressBashOutput(c.cmd, c.body)
		ns := time.Since(start).Nanoseconds()
		out = append(out, benchResult{
			Feature:      "bash_compress",
			Dataset:      c.name,
			OrigBytes:    res.OrigBytes,
			OutBytes:     res.OutBytes,
			ReductionPct: res.ReductionPct,
			LatencyUs:    ns / 1000,
		})
	}
	return out
}

func writeBenchMarkdown(out io.Writer, results []benchResult) {
	fmt.Fprintln(out, "# ctx-window benchmark (Phase 0)")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Suite mínima: reduction% + latência (μs) por feature × dataset.")
	fmt.Fprintln(out, "Defaults validados: K=5, archiveThreshold=4KB, loop window=10, skeletonMin=8KB.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "| feature | dataset | orig_bytes | out_bytes | reduction_pct | latency_μs |")
	fmt.Fprintln(out, "| --- | --- | ---: | ---: | ---: | ---: |")
	for _, r := range results {
		fmt.Fprintf(out, "| %s | %s | %d | %d | %d | %d |\n",
			r.Feature, r.Dataset, r.OrigBytes, r.OutBytes, r.ReductionPct, r.LatencyUs)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Notas:")
	fmt.Fprintln(out, "- `quality` reporta score (0-100) na coluna out_bytes; reduction_pct = score.")
	fmt.Fprintln(out, "- `loop_detect` é zero-copy (não modifica), só mede latência.")
	fmt.Fprintln(out, "- `archive` abaixo de archiveThreshold (4KB) passa verbatim.")
	fmt.Fprintln(out, "- `bash_compress` skip para unsafe patterns (npm, pipe) — esperado.")
}

func writeBenchJSON(out io.Writer, results []benchResult) error {
	fmt.Fprintln(out, "[")
	for i, r := range results {
		comma := ","
		if i == len(results)-1 {
			comma = ""
		}
		fmt.Fprintf(out, `  {"feature":%q,"dataset":%q,"orig_bytes":%d,"out_bytes":%d,"reduction_pct":%d,"latency_us":%d}%s`+"\n",
			r.Feature, r.Dataset, r.OrigBytes, r.OutBytes, r.ReductionPct, r.LatencyUs, comma)
	}
	fmt.Fprintln(out, "]")
	return nil
}

// helpers — keep allocation in benchmark paths visible

func makeTurns(n int, content string) []Turn {
	turns := make([]Turn, n)
	for i := range turns {
		turns[i] = Turn{Content: content}
	}
	return turns
}

func sumContentBytes(turns []Turn) int {
	total := 0
	for _, t := range turns {
		total += len(t.Content)
	}
	return total
}

func makePythonFixture(targetBytes int) string {
	var b strings.Builder
	for b.Len() < targetBytes {
		b.WriteString("def function_")
		b.WriteString(strings.Repeat("a", 5))
		b.WriteString("():\n")
		b.WriteString("    x = ")
		b.WriteString(strings.Repeat("1", 50))
		b.WriteString("\n")
		b.WriteString("    return x\n\n")
	}
	return b.String()
}

func mustTempDir(prefix string) string {
	d, err := os.MkdirTemp("", prefix+"-*")
	if err != nil {
		return os.TempDir()
	}
	return d
}

func pct(orig, reduced int) int {
	if orig <= 0 {
		return 0
	}
	p := (orig - reduced) * 100 / orig
	if p < 0 {
		return 0
	}
	return p
}