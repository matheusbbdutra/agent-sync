package graph

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAggregateTelemetryEmptyAndNonExistent(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "nonexistent.jsonl")
	res, err := AggregateTelemetry(nonExistent, 0)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if res.TotalInvocations != 0 {
		t.Errorf("esperava 0 invocações, obteve %d", res.TotalInvocations)
	}
	if !res.GhostWiringWarn {
		t.Errorf("esperava ghost wiring warning ativo para arquivo inexistente")
	}

	emptyFile := filepath.Join(t.TempDir(), "empty.jsonl")
	if err := os.WriteFile(emptyFile, []byte("\n  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res2, err := AggregateTelemetry(emptyFile, 0)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if res2.TotalInvocations != 0 {
		t.Errorf("esperava 0 invocações, obteve %d", res2.TotalInvocations)
	}
	if !res2.GhostWiringWarn {
		t.Errorf("esperava ghost wiring warning ativo para arquivo vazio")
	}
}

func TestAggregateTelemetryAggregation(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "telemetry.jsonl")

	now := time.Now().UTC()
	entries := []GraphTelemetryEntry{
		{
			TS:               now.Add(-4 * time.Minute),
			SessionID:        "ses-1",
			CLI:              "claude",
			ToolName:         "get_file_impact",
			ArgsPathOrSymbol: "main.go",
			DurationMs:       10,
			OutputBytes:      500,
			CacheHit:         true,
		},
		{
			TS:               now.Add(-3 * time.Minute),
			SessionID:        "ses-1",
			CLI:              "claude",
			ToolName:         "get_file_impact",
			ArgsPathOrSymbol: "calc.go",
			DurationMs:       20,
			OutputBytes:      300,
			CacheHit:         true,
		},
		{
			TS:               now.Add(-2 * time.Minute),
			SessionID:        "ses-2",
			CLI:              "codex",
			ToolName:         "get_symbol_callers",
			ArgsPathOrSymbol: "Add",
			DurationMs:       30,
			OutputBytes:      150,
			CacheHit:         false,
		},
		{
			TS:               now.Add(-1 * time.Minute),
			SessionID:        "ses-2",
			CLI:              "codex",
			ToolName:         "repo_summary",
			ArgsPathOrSymbol: "(all)",
			DurationMs:       100,
			OutputBytes:      1200,
			CacheHit:         true,
		},
	}

	var buf bytes.Buffer
	for _, e := range entries {
		data, _ := json.Marshal(e)
		buf.Write(data)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(logFile, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := AggregateTelemetry(logFile, 0)
	if err != nil {
		t.Fatalf("AggregateTelemetry falhou: %v", err)
	}

	if res.TotalInvocations != 4 {
		t.Errorf("TotalInvocations esperado 4, obteve %d", res.TotalInvocations)
	}
	if res.UniqueSessions != 2 {
		t.Errorf("UniqueSessions esperado 2, obteve %d", res.UniqueSessions)
	}
	if res.ByTool["get_file_impact"] != 2 || res.ByTool["get_symbol_callers"] != 1 || res.ByTool["repo_summary"] != 1 {
		t.Errorf("ByTool incorreto: %+v", res.ByTool)
	}
	if res.ByCLI["claude"] != 2 || res.ByCLI["codex"] != 2 {
		t.Errorf("ByCLI incorreto: %+v", res.ByCLI)
	}
	if res.CacheHitPct != 75.0 {
		t.Errorf("CacheHitPct esperado 75.0, obteve %.2f", res.CacheHitPct)
	}
	if res.DurationP50Ms != 30 {
		t.Errorf("DurationP50Ms esperado 30, obteve %d", res.DurationP50Ms)
	}
	if res.GhostWiringWarn {
		t.Errorf("GhostWiringWarn não deveria estar ativo com invocações")
	}

	// Teste com -last 2
	resLast, err := AggregateTelemetry(logFile, 2)
	if err != nil {
		t.Fatalf("AggregateTelemetry com last falhou: %v", err)
	}
	if resLast.TotalInvocations != 2 {
		t.Errorf("TotalInvocations esperado 2 para last=2, obteve %d", resLast.TotalInvocations)
	}
}

func TestPrintTextStats(t *testing.T) {
	stats := GraphStatsResult{
		TotalInvocations: 10,
		UniqueSessions:   2,
		CacheHitPct:      80.0,
		DurationP50Ms:    15,
		DurationP95Ms:    45,
		ByTool:           map[string]int{"get_file_impact": 7, "repo_summary": 3},
		ByCLI:            map[string]int{"claude": 10},
		TopFiles: []FileCount{
			{Path: "main.go", Count: 5},
			{Path: "calc.go", Count: 2},
		},
	}

	var buf bytes.Buffer
	printTextStats(&buf, stats, "test.jsonl")
	out := buf.String()

	if !strings.Contains(out, "Total de invocações: 10 (em 2 sessões)") {
		t.Errorf("saída de texto inesperada: %s", out)
	}
	if !strings.Contains(out, "80.0%") {
		t.Errorf("esperava 80.0%% na saída: %s", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Errorf("esperava main.go no top alvos: %s", out)
	}
}
