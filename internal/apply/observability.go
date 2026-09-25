package apply

// observability.go: relatorios de observability dos hooks (metricas
// de erro, latencia, contagem por stage).
//
// hookErrorEvent e' o registro append-only em .agent-sync/hook-errors.jsonl;
// printHookObservability imprime resumo agregado. Migrado sem renomear
// em 2026-09-21 (Fase 7b).
import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
)

type hookErrorEvent struct {
	Code       string `json:"code"`
	Stage      string `json:"stage"`
	CLI        string `json:"cli,omitempty"`
	Tool       string `json:"tool,omitempty"`
	DurationMs *int   `json:"duration_ms,omitempty"`
}

type keyCount struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type latencySummary struct {
	P50     int `json:"p50"`
	P95     int `json:"p95"`
	Max     int `json:"max"`
	Samples int `json:"samples"`
}

type hookEventSummary struct {
	ByStageCode []keyCount      `json:"by_stage_code"`
	ByCLI       []keyCount      `json:"by_cli"`
	ByTool      []keyCount      `json:"by_tool"`
	LatencyMs   *latencySummary `json:"latency_ms,omitempty"`
}

func resolveHookLogPath() (string, error) {
	if path := os.Getenv("AGENT_SYNC_HOOK_LOG"); path != "" {
		return path, nil
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "agent-sync", "hooks", "errors.jsonl"), nil
}

func readHookEvents(path string) ([]hookErrorEvent, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []hookErrorEvent
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event hookErrorEvent
		if json.Unmarshal(scanner.Bytes(), &event) == nil {
			events = append(events, event)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func aggregateHookEvents(events []hookErrorEvent) hookEventSummary {
	stageCodeCounts := map[string]int{}
	cliCounts := map[string]int{}
	toolCounts := map[string]int{}
	var latencies []int

	for _, ev := range events {
		stageCodeCounts[ev.Stage+":"+ev.Code]++
		if ev.CLI != "" {
			cliCounts[ev.CLI]++
		}
		if ev.Tool != "" {
			toolCounts[ev.Tool]++
		} else {
			toolCounts["(sem tool)"]++
		}
		if ev.DurationMs != nil {
			latencies = append(latencies, *ev.DurationMs)
		}
	}

	summary := hookEventSummary{
		ByStageCode: sortedCounts(stageCodeCounts),
		ByCLI:       sortedCounts(cliCounts),
		ByTool:      sortedCounts(toolCounts),
	}

	if len(latencies) > 0 {
		sort.Ints(latencies)
		summary.LatencyMs = &latencySummary{
			P50:     percentile(latencies, 0.5),
			P95:     percentile(latencies, 0.95),
			Max:     latencies[len(latencies)-1],
			Samples: len(latencies),
		}
	}
	return summary
}

func sortedCounts(counts map[string]int) []keyCount {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := counts[keys[i]], counts[keys[j]]
		if a != b {
			return a > b
		}
		return keys[i] < keys[j]
	})
	out := make([]keyCount, len(keys))
	for i, k := range keys {
		out[i] = keyCount{Key: k, Count: counts[k]}
	}
	return out
}

func percentile(values []int, p float64) int {
	if len(values) == 0 {
		return 0
	}
	if p < 0 || p > 1 {
		return 0
	}
	sorted := make([]int, len(values))
	copy(sorted, values)
	sort.Ints(sorted)
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func printHookObservability() error {
	return printHookObservabilityTo(os.Stdout, false)
}

func printHookObservabilityJSON() error {
	return printHookObservabilityTo(os.Stdout, true)
}

func printHookObservabilityTo(stdout io.Writer, jsonMode bool) error {
	path, err := resolveHookLogPath()
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		if jsonMode {
			empty := observabilityReport{
				Path:        path,
				ByStageCode: []keyCount{},
				ByCLI:       []keyCount{},
				ByTool:      []keyCount{},
			}
			data, _ := json.Marshal(empty)
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		fmt.Fprintln(stdout, "Nenhum erro de hook persistido.")
		return nil
	}

	events, err := readHookEvents(path)
	if err != nil {
		return err
	}
	summary := aggregateHookEvents(events)

	if jsonMode {
		report := observabilityReport{
			Path:        path,
			Total:       len(events),
			ByStageCode: summary.ByStageCode,
			ByCLI:       summary.ByCLI,
			ByTool:      summary.ByTool,
			LatencyMs:   summary.LatencyMs,
		}
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "📊 Erros de hooks (%s)\n", path)

	fmt.Fprintln(stdout, "\nPor stage:code:")
	if len(summary.ByStageCode) == 0 {
		fmt.Fprintln(stdout, "Nenhum evento válido encontrado.")
		return nil
	}
	for _, kc := range summary.ByStageCode {
		fmt.Fprintf(stdout, " - %-35s %d\n", kc.Key, kc.Count)
	}

	fmt.Fprintln(stdout, "\nPor cli:")
	for _, kc := range summary.ByCLI {
		fmt.Fprintf(stdout, " - %-35s %d\n", kc.Key, kc.Count)
	}

	fmt.Fprintln(stdout, "\nPor tool:")
	for _, kc := range summary.ByTool {
		fmt.Fprintf(stdout, " - %-35s %d\n", kc.Key, kc.Count)
	}

	if summary.LatencyMs != nil {
		fmt.Fprintf(stdout, "\nLatência (duration_ms presente em %d eventos):\n", summary.LatencyMs.Samples)
		fmt.Fprintf(stdout, " - p50: %dms\n", summary.LatencyMs.P50)
		fmt.Fprintf(stdout, " - p95: %dms\n", summary.LatencyMs.P95)
		fmt.Fprintf(stdout, " - max: %dms\n", summary.LatencyMs.Max)
	}

	return nil
}

type observabilityReport struct {
	Path        string          `json:"path"`
	Total       int             `json:"total"`
	ByStageCode []keyCount      `json:"by_stage_code"`
	ByCLI       []keyCount      `json:"by_cli"`
	ByTool      []keyCount      `json:"by_tool"`
	LatencyMs   *latencySummary `json:"latency_ms,omitempty"`
}
