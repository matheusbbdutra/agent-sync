package budget

// budget_status_test.go: testes para budget_status.go.
// Migrado de token_budget_test.go em 2026-09-21 (Fase 5).
import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextWindowLookup_ModelosConhecidos(t *testing.T) {
	cases := map[string]int{
		"claude-sonnet-4.5":   200000,
		"claude-opus-4.6":     200000,
		"claude-haiku-4.5":    200000,
		"gpt-5":               400000,
		"gpt-5-codex":         400000,
		"gemini-2.5-pro":      1000000,
	}
	for model, want := range cases {
		got := ContextWindowLookup(model)
		if got != want {
			t.Errorf("model=%s esperava cw=%d, veio %d", model, want, got)
		}
	}
}

func TestContextWindowLookup_DesconhecidoDefault200k(t *testing.T) {
	got := ContextWindowLookup("modelo-futuro-desconhecido")
	if got != 200000 {
		t.Errorf("modelo desconhecido esperava default 200000, veio %d", got)
	}
}

func TestBuildTokenBudgetStatus_HighUtilizationTriggersNudge(t *testing.T) {
	s, err := BuildTokenBudgetStatus("claude", "sess-test", "claude-sonnet-4.5",
		155000, 12000, 0, "", 80, nil)
	if err != nil {
		t.Fatalf("BuildTokenBudgetStatus: %v", err)
	}
	if !s.ShouldNudge {
		t.Errorf("utilization_pct=%d >= threshold=80 deveria dar nudge", s.UtilizationPct)
	}
	if s.Trigger != "tokens_in>=threshold" {
		t.Errorf("esperava trigger tokens_in>=threshold, veio %q", s.Trigger)
	}
}

func TestBuildTokenBudgetStatus_LowUtilizationNoNudge(t *testing.T) {
	s, err := BuildTokenBudgetStatus("codex", "sess-2", "gpt-5",
		1000, 200, 0, "/tmp/some/summary.md", 80, nil)
	if err != nil {
		t.Fatalf("BuildTokenBudgetStatus: %v", err)
	}
	if s.ShouldNudge {
		t.Errorf("utilization_pct=%d baixo NAO deveria nudgar", s.UtilizationPct)
	}
	if s.Trigger != "none" {
		t.Errorf("esperava trigger=none, veio %q", s.Trigger)
	}
}

func TestBuildTokenBudgetStatus_LegacyHeuristicToolCalls(t *testing.T) {
	// tool_calls >= 80 sem tokens deve disparar heuristica legada.
	s, err := BuildTokenBudgetStatus("agy", "sess-3", "",
		0, 0, 100, "/tmp/some/summary.md", 80, nil)
	if err != nil {
		t.Fatalf("BuildTokenBudgetStatus: %v", err)
	}
	if !s.ShouldNudge {
		t.Errorf("tool_calls=100 (>=80) deveria nudgar via heuristica legada")
	}
	if s.Trigger != "tool_calls>=min" {
		t.Errorf("esperava trigger tool_calls>=min, veio %q", s.Trigger)
	}
}

func TestBuildTokenBudgetStatus_LegacyHeuristicSummaryMissing(t *testing.T) {
	s, err := BuildTokenBudgetStatus("cursor", "sess-4", "",
		0, 0, 5, "", 80, nil) // summary vazio = missing
	if err != nil {
		t.Fatalf("BuildTokenBudgetStatus: %v", err)
	}
	if !s.ShouldNudge {
		t.Errorf("summary ausente deveria nudgar via heuristica legada")
	}
	if s.Trigger != "summary_missing" {
		t.Errorf("esperava trigger summary_missing, veio %q", s.Trigger)
	}
}

func TestExtractTokensFromTranscript_TipoAssistant(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "transcript.jsonl")
	data := `{"type":"user","message":{"role":"user","content":"q1"}}
{"type":"assistant","message":{"role":"assistant","content":"r1","usage":{"input_tokens":100,"output_tokens":50}}}
{"type":"assistant","message":{"role":"assistant","content":"r2","usage":{"input_tokens":200,"output_tokens":80}}}
`
	if err := os.WriteFile(transcript, []byte(data), 0o644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	in, out, ok := extractTokensFromTranscript(transcript)
	if !ok {
		t.Fatal("esperava extrair tokens")
	}
	if in != 200 || out != 80 {
		t.Errorf("esperava ULTIMO assistant (in=200, out=80), veio in=%d out=%d", in, out)
	}
}

func TestExtractTokensFromTranscript_PromptCompletionAliases(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "transcript.jsonl")
	data := `{"type":"assistant","message":{"role":"assistant","content":"r","usage":{"prompt_tokens":500,"completion_tokens":150}}}`
	if err := os.WriteFile(transcript, []byte(data), 0o644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	in, out, ok := extractTokensFromTranscript(transcript)
	if !ok || in != 500 || out != 150 {
		t.Errorf("aliases nao funcionaram: ok=%v in=%d out=%d", ok, in, out)
	}
}

func TestExtractTokensFromTranscript_NenhumAssistant(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "transcript.jsonl")
	if err := os.WriteFile(transcript, []byte(`{"type":"user"}`), 0o644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	in, out, ok := extractTokensFromTranscript(transcript)
	if ok || in != 0 || out != 0 {
		t.Errorf("sem assistant: esperava false/0/0, veio ok=%v in=%d out=%d", ok, in, out)
	}
}

func TestExtractTokensFromTranscript_ArquivoInexistente(t *testing.T) {
	_, _, ok := extractTokensFromTranscript("/tmp/nao-existe-agent-sync-test-xyz")
	if ok {
		t.Error("arquivo inexistente deveria retornar false")
	}
}

func TestRunBudgetNudge_SemTranscriptTokensZero(t *testing.T) {
	dir := t.TempDir()
	// Sem transcript e sem tokens: ShouldNudge=false (heuristica legada
	// nao dispara com tool_calls=0 e summary fornecido).
	summary := filepath.Join(dir, "summary.md")
	if err := os.WriteFile(summary, []byte("# ok"), 0o644); err != nil {
		t.Fatalf("writeFile summary: %v", err)
	}
	out, err := captureStdout(t, func() error {
		return runBudgetNudge([]string{
			"-actor", "claude",
			"-session-id", "sess-test",
			"-model", "claude-sonnet-4.5",
			"-summary", summary,
			"-tool-calls", "0",
			"-root", dir,
		})
	})
	if err != nil {
		t.Fatalf("runBudgetNudge: %v (out=%q)", err, out)
	}
	var status TokenBudgetStatus
	if err := json.Unmarshal([]byte(out), &status); err != nil {
		t.Fatalf("JSON invalido: %v (out=%q)", err, out)
	}
	if status.ShouldNudge {
		t.Errorf("sem tokens e sem heuristica deveria nao nudgar, veio should_nudge=true")
	}
	if status.Trigger != "none" {
		t.Errorf("esperava trigger=none, veio %q", status.Trigger)
	}
}

func TestBudgetUsageIncluiNudge(t *testing.T) {
	var buf strings.Builder
	if err := budgetUsage(&buf); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"nudge", "-threshold", "-transcript", "-actor"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("usage nao cita %q", want)
		}
	}
}
