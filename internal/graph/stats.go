package graph

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GraphTelemetryEntry modela cada linha do log append-only JSONL de telemetria.
type GraphTelemetryEntry struct {
	TS               time.Time `json:"ts"`
	SessionID        string    `json:"session_id"`
	CLI              string    `json:"cli"`
	ToolName         string    `json:"tool_name"`
	ArgsPathOrSymbol string    `json:"args_path_or_symbol"`
	DurationMs       int64     `json:"duration_ms"`
	OutputBytes      int       `json:"output_bytes"`
	CacheHit         bool      `json:"cache_hit"`
}

// GraphStatsResult representa as métricas agregadas de telemetria do code-graph.
type GraphStatsResult struct {
	TotalInvocations  int            `json:"total_invocations"`
	ByTool            map[string]int `json:"by_tool"`
	ByCLI             map[string]int `json:"by_cli"`
	TopFiles          []FileCount    `json:"top_files"`
	DurationP50Ms     int64          `json:"duration_p50_ms"`
	DurationP95Ms     int64          `json:"duration_p95_ms"`
	CacheHitPct       float64        `json:"cache_hit_pct"`
	UniqueSessions    int            `json:"unique_sessions"`
	ZeroInvocations   int            `json:"sessions_zero_invocations,omitempty"`
	GhostWiringWarn   bool           `json:"ghost_wiring_warning"`
}

// FileCount associa caminho de arquivo com contagem de consultas.
type FileCount struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

// RunCommand despacha os subcomandos de `agent-sync graph <subcommand>`.
func RunCommand(args []string) error {
	if len(args) == 0 {
		return graphUsage(os.Stderr)
	}
	switch args[0] {
	case "stats":
		return runGraphStats(args[1:])
	case "help", "-h", "--help":
		return graphUsage(os.Stdout)
	default:
		return fmt.Errorf("graph: subcommand desconhecido: %q", args[0])
	}
}

func graphUsage(w io.Writer) error {
	fmt.Fprintf(w, "Uso: agent-sync graph <subcommand> [flags]\n\n")
	fmt.Fprintf(w, "Subcommands:\n")
	fmt.Fprintf(w, "  stats         Agrega métricas de telemetria do code-graph MCP\n")
	fmt.Fprintf(w, "\nFlags (stats):\n")
	fmt.Fprintf(w, "  -file <path>  Caminho do arquivo graph_telemetry.jsonl (default: ~/.cache/agent-sync/graph_telemetry.jsonl)\n")
	fmt.Fprintf(w, "  -json         Imprime resultado em formato JSON estruturado\n")
	fmt.Fprintf(w, "  -last N       Analisa apenas as últimas N invocações (0 = todas)\n")
	return nil
}

func defaultTelemetryPath() string {
	if envPath := strings.TrimSpace(os.Getenv("AGENT_SYNC_GRAPH_TELEMETRY_FILE")); envPath != "" {
		return envPath
	}
	if ucd, err := os.UserCacheDir(); err == nil && ucd != "" {
		return filepath.Join(ucd, "agent-sync", "graph_telemetry.jsonl")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".cache", "agent-sync", "graph_telemetry.jsonl")
	}
	return "graph_telemetry.jsonl"
}

func runGraphStats(args []string) error {
	fs := flag.NewFlagSet("graph stats", flag.ContinueOnError)
	filePath := fs.String("file", "", "Caminho do log JSONL de telemetria")
	asJSON := fs.Bool("json", false, "Exibe saída em formato JSON")
	last := fs.Int("last", 0, "Apenas as últimas N invocações (0 = todas)")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	target := *filePath
	if target == "" {
		target = defaultTelemetryPath()
	}

	stats, err := AggregateTelemetry(target, *last)
	if err != nil {
		return err
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(stats)
	}

	printTextStats(os.Stdout, stats, target)
	return nil
}

// AggregateTelemetry lê e agrega métricas de um arquivo JSONL.
func AggregateTelemetry(path string, last int) (GraphStatsResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return GraphStatsResult{
				ByTool:          map[string]int{},
				ByCLI:           map[string]int{},
				TopFiles:        []FileCount{},
				GhostWiringWarn: true,
			}, nil
		}
		return GraphStatsResult{}, fmt.Errorf("ler telemetria %s: %w", path, err)
	}

	rawLines := strings.Split(string(data), "\n")
	var entries []GraphTelemetryEntry

	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e GraphTelemetryEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}

	if last > 0 && len(entries) > last {
		entries = entries[len(entries)-last:]
	}

	res := GraphStatsResult{
		TotalInvocations: len(entries),
		ByTool:           make(map[string]int),
		ByCLI:            make(map[string]int),
		TopFiles:         []FileCount{},
	}

	if len(entries) == 0 {
		res.GhostWiringWarn = true
		return res, nil
	}

	fileFreq := make(map[string]int)
	sessions := make(map[string]int)
	durations := make([]int64, 0, len(entries))
	cacheHits := 0

	for _, e := range entries {
		res.ByTool[e.ToolName]++
		res.ByCLI[e.CLI]++
		sessions[e.SessionID]++

		if e.ArgsPathOrSymbol != "" && e.ArgsPathOrSymbol != "(all)" {
			fileFreq[e.ArgsPathOrSymbol]++
		}
		if e.CacheHit {
			cacheHits++
		}
		durations = append(durations, e.DurationMs)
	}

	res.UniqueSessions = len(sessions)
	res.CacheHitPct = (float64(cacheHits) / float64(len(entries))) * 100.0

	// Percentis de duração (P50 e P95)
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})
	res.DurationP50Ms = durations[len(durations)*50/100]
	res.DurationP95Ms = durations[len(durations)*95/100]

	// Top Files ordenados descrescente
	type kv struct {
		k string
		v int
	}
	var sortedFiles []kv
	for k, v := range fileFreq {
		sortedFiles = append(sortedFiles, kv{k, v})
	}
	sort.Slice(sortedFiles, func(i, j int) bool {
		if sortedFiles[i].v == sortedFiles[j].v {
			return sortedFiles[i].k < sortedFiles[j].k
		}
		return sortedFiles[i].v > sortedFiles[j].v
	})

	limit := 10
	if len(sortedFiles) < limit {
		limit = len(sortedFiles)
	}
	for i := 0; i < limit; i++ {
		res.TopFiles = append(res.TopFiles, FileCount{
			Path:  sortedFiles[i].k,
			Count: sortedFiles[i].v,
		})
	}

	return res, nil
}

func printTextStats(w io.Writer, s GraphStatsResult, file string) {
	fmt.Fprintf(w, "Telemetria Code-Graph (%s)\n", file)
	fmt.Fprintf(w, "Total de invocações: %d (em %d sessões)\n", s.TotalInvocations, s.UniqueSessions)
	if s.TotalInvocations == 0 {
		fmt.Fprintf(w, "⚠️  Nenhuma invocação registrada (possível wiramento fantasma)\n")
		return
	}

	fmt.Fprintf(w, "Cache Hit Rate:     %.1f%%\n", s.CacheHitPct)
	fmt.Fprintf(w, "Duração (ms):       p50=%dms, p95=%dms\n", s.DurationP50Ms, s.DurationP95Ms)

	fmt.Fprintf(w, "\nPor Ferramenta:\n")
	var tools []string
	for k := range s.ByTool {
		tools = append(tools, k)
	}
	sort.Strings(tools)
	for _, k := range tools {
		fmt.Fprintf(w, "  %-22s : %d\n", k, s.ByTool[k])
	}

	fmt.Fprintf(w, "\nPor CLI:\n")
	var clis []string
	for k := range s.ByCLI {
		clis = append(clis, k)
	}
	sort.Strings(clis)
	for _, k := range clis {
		fmt.Fprintf(w, "  %-12s : %d\n", k, s.ByCLI[k])
	}

	if len(s.TopFiles) > 0 {
		fmt.Fprintf(w, "\nTop Alvos / Arquivos:\n")
		for _, fc := range s.TopFiles {
			fmt.Fprintf(w, "  %-30s : %d\n", fc.Path, fc.Count)
		}
	}
}
