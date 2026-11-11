package budget

// budget_status_test.go: testes para budget_status.go.
// Migrado de token_budget_test.go em 2026-09-21 (Fase 5).
import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestContextWindowLookup_ModelosConhecidos(t *testing.T) {
	cases := map[string]int{
		"claude-sonnet-4.5": 200000,
		"claude-opus-4.6":   200000,
		"claude-haiku-4.5":  200000,
		"gpt-5":             400000,
		"gpt-5-codex":       400000,
		"gemini-2.5-pro":    1000000,
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

func TestNudgeMessage_ReportaGatilhoReal(t *testing.T) {
	stale := filepath.Join(t.TempDir(), "summary.md")
	if err := os.WriteFile(stale, []byte("# handoff\n"), 0o644); err != nil {
		t.Fatalf("summary: %v", err)
	}
	old := time.Now().Add(-6 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	// Gatilho de tokens: unico caso em que o percentual aparece.
	s, err := BuildTokenBudgetStatus("claude", "s", "m", 1000000,
		900000, 0, 0, stale, 80, nil)
	if err != nil {
		t.Fatalf("build tokens: %v", err)
	}
	if msg := NudgeMessage(s); !strings.Contains(msg, "90% da janela") || strings.Contains(msg, "Handoff") {
		t.Errorf("gatilho de tokens deveria mostrar percentual, veio %q", msg)
	}

	// Gatilho de summary velho: sem percentual, com idade real.
	s, err = BuildTokenBudgetStatus("claude", "s", "m", 1000000,
		1000, 0, 0, stale, 80, nil)
	if err != nil {
		t.Fatalf("build summary: %v", err)
	}
	msg := NudgeMessage(s)
	if !strings.Contains(msg, "Handoff desatualizado") || strings.Contains(msg, "de utilizacao") {
		t.Errorf("gatilho de summary deveria reportar handoff, veio %q", msg)
	}
	if !strings.Contains(msg, "com 6h") {
		t.Errorf("idade do summary ausente em %q", msg)
	}

	// Sem nudge: mensagem vazia (o shell nao emite nada).
	s, err = BuildTokenBudgetStatus("claude", "s", "m", 1000000,
		1000, 0, 0, "", 80, nil)
	if err != nil {
		t.Fatalf("build missing: %v", err)
	}
	if msg := NudgeMessage(s); !strings.Contains(msg, "Handoff ausente") {
		t.Errorf("esperava aviso de handoff ausente, veio %q", msg)
	}
	s.ShouldNudge = false
	if msg := NudgeMessage(s); msg != "" {
		t.Errorf("sem nudge a mensagem deveria ser vazia, veio %q", msg)
	}
}

func TestBuildTokenBudgetStatus_ContextWindowNativoSobrepoeTabela(t *testing.T) {
	// Modelo fora da tabela (janela nativa 1M) nao pode cair no default 200k:
	// 45k tokens = 4% de 1M, nao 22% de 200k.
	summary := filepath.Join(t.TempDir(), "summary.md")
	if err := os.WriteFile(summary, []byte("# handoff\n"), 0o644); err != nil {
		t.Fatalf("preparar summary: %v", err)
	}
	s, err := BuildTokenBudgetStatus("pi", "sess-pi", "deepseek-v4.1-flash", 1000000,
		45525, 0, 0, summary, 80, nil)
	if err != nil {
		t.Fatalf("BuildTokenBudgetStatus: %v", err)
	}
	if s.ContextWindow != 1000000 {
		t.Errorf("ContextWindow = %d, esperado 1000000", s.ContextWindow)
	}
	if s.UtilizationPct != 4 {
		t.Errorf("UtilizationPct = %d, esperado 4", s.UtilizationPct)
	}
	if s.Trigger != "none" {
		t.Errorf("4%% de 1M nao deveria disparar nudge, trigger=%q", s.Trigger)
	}
}

func TestBuildTokenBudgetStatus_NormalizaActorAntigravity(t *testing.T) {
	// D-116: wiramento Antigravity passa -actor antigravity; o schema exige o
	// vocabulário de eventos ("agy"). Build deve normalizar, não falhar.
	s, err := BuildTokenBudgetStatus("antigravity", "sess-agy", "", 0,
		900000, 5000, 0, "", 80, nil)
	if err != nil {
		t.Fatalf("BuildTokenBudgetStatus(actor=antigravity): %v", err)
	}
	if s.Actor != "agy" {
		t.Errorf("Actor = %q, esperado agy (alias normalizado)", s.Actor)
	}
	if !s.ShouldNudge {
		t.Errorf("905k tokens em janela 200k deveria dar nudge")
	}
}

func TestBuildTokenBudgetStatus_HighUtilizationTriggersNudge(t *testing.T) {
	s, err := BuildTokenBudgetStatus("claude", "sess-test", "claude-sonnet-4.5", 0,
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
	s, err := BuildTokenBudgetStatus("codex", "sess-2", "gpt-5", 0,
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
	s, err := BuildTokenBudgetStatus("agy", "sess-3", "", 0,
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
	s, err := BuildTokenBudgetStatus("cline", "sess-4", "", 0,
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
