package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestGradeFromScore(t *testing.T) {
	cases := []struct {
		score int
		want  string
	}{
		{100, "S"},
		{95, "S"},
		{90, "S"},
		{89, "A"},
		{85, "A"},
		{80, "A"},
		{79, "B"},
		{75, "B"},
		{70, "B"},
		{69, "C"},
		{60, "C"},
		{55, "C"},
		{54, "D"},
		{45, "D"},
		{40, "D"},
		{39, "F"},
		{0, "F"},
	}
	for _, c := range cases {
		if got := GradeFromScore(c.score); got != c.want {
			t.Errorf("GradeFromScore(%d)=%q, want %q", c.score, got, c.want)
		}
	}
}

func TestComputeQualityEmptySession(t *testing.T) {
	withTempCache(t)
	s, _ := Load("empty-quality")
	report := ComputeQuality(s)
	if report.SessionID != "empty-quality" {
		t.Errorf("SessionID wrong: %q", report.SessionID)
	}
	if report.Score < 0 || report.Score > 100 {
		t.Errorf("Score should be 0-100, got %d", report.Score)
	}
	if report.Grade == "" {
		t.Errorf("Grade should not be empty")
	}
	if len(report.Signals) != 7 {
		t.Errorf("expected 7 signals, got %d", len(report.Signals))
	}
}

func TestSignalContextFill(t *testing.T) {
	withTempCache(t)
	t.Setenv("AGENT_SYNC_CTX_COMPACT_AT", "1000")
	s, _ := Load("ctx-fill")
	// sem turns: 0 chars → score ideal
	if sig := signalContextFill(s); sig.Score != 100 {
		t.Errorf("empty session should score 100 on context_fill, got %d", sig.Score)
	}
	// encher até ~50% do threshold (500 chars; vira 536 com padding de role/json)
	s.AddTurn(Turn{Content: strings.Repeat("a", 500)})
	if sig := signalContextFill(s); sig.Score != 73 { // 100 - (536/1000)*50 = 73.2 → 73
		t.Errorf("~50%% fill should score ~73, got %d (reason=%s)", sig.Score, sig.Reason)
	}
	// encher até 2x threshold (2000 chars) → score 0
	s.AddTurn(Turn{Content: strings.Repeat("b", 2000)})
	if sig := signalContextFill(s); sig.Score != 0 {
		t.Errorf("200%% fill should score 0, got %d", sig.Score)
	}
}

func TestSignalCompactionDepth(t *testing.T) {
	withTempCache(t)
	s, _ := Load("depth")
	// v=0 → 100
	if sig := signalCompactionDepth(s); sig.Score != 100 {
		t.Errorf("v=0 should score 100, got %d", sig.Score)
	}
	// v=3 → 40
	s.Version = 3
	if sig := signalCompactionDepth(s); sig.Score != 40 {
		t.Errorf("v=3 should score 40, got %d", sig.Score)
	}
	// v=10 → 0 (clamped)
	s.Version = 10
	if sig := signalCompactionDepth(s); sig.Score != 0 {
		t.Errorf("v=10 should score 0 (clamped), got %d", sig.Score)
	}
}

func TestSignalWasteTokens(t *testing.T) {
	withTempCache(t)
	t.Setenv("AGENT_SYNC_CTX_COMPACT_AT", "500")
	s, _ := Load("waste")
	// fill=0 → waste=0 → 100
	if sig := signalWasteTokens(s); sig.Score != 100 {
		t.Errorf("no waste should score 100, got %d", sig.Score)
	}
	// fill > threshold: 1032-500=532 chars de waste (1000+padding)
	s.AddTurn(Turn{Content: strings.Repeat("a", 1000)})
	if sig := signalWasteTokens(s); sig.Score != 64 { // ratio 532/1500=0.355 → 100-35.5=64.5 → 64
		t.Errorf("expected 64 with ~35%% waste, got %d (reason=%s)", sig.Score, sig.Reason)
	}
}

func TestSignalStaleReads(t *testing.T) {
	withTempCache(t)
	s, _ := Load("stale")
	// 0 turns → 100
	if sig := signalStaleReads(s); sig.Score != 100 {
		t.Errorf("empty should score 100, got %d", sig.Score)
	}
	// 3 turns, todos iguais: 1 dups (após 2ª ocorrência) → ratio 1/3=33% → 100-66=34
	s.AddTurn(Turn{Content: "Read: foo.go"})
	s.AddTurn(Turn{Content: "Read: foo.go"})
	s.AddTurn(Turn{Content: "Read: foo.go"})
	if sig := signalStaleReads(s); sig.Score != 33 { // 1 dup / 3 turns = 33% → 100-66.67 = 33.33 → 33
		t.Errorf("expected 33 for 1/3 dups, got %d (reason=%s)", sig.Score, sig.Reason)
	}
}

func TestSignalBloatedResults(t *testing.T) {
	withTempCache(t)
	s, _ := Load("bloated")
	s.AddTurn(Turn{Content: strings.Repeat("a", 5000)}) // bloated
	s.AddTurn(Turn{Content: "tiny"})
	if sig := signalBloatedResults(s); sig.Score != 0 { // 50% bloated → 100-100=0
		t.Errorf("50%% bloated should score 0, got %d", sig.Score)
	}
	s.AddTurn(Turn{Content: "tiny2"})
	if sig := signalBloatedResults(s); sig.Score != 33 { // 1/3 bloated → 100-66.67 = 33.33 → 33
		t.Errorf("expected ~33 for 1/3 bloated, got %d", sig.Score)
	}
}

func TestSignalDecisionDensity(t *testing.T) {
	withTempCache(t)
	s, _ := Load("decs")
	// sem summary → score 0 (não penaliza? é decisão: 0 = ruim mas explícito)
	if sig := signalDecisionDensity(s); sig.Score != 0 {
		t.Errorf("no summary should score 0, got %d (reason=%s)", sig.Score, sig.Reason)
	}
	// cria summary com 3 decisions
	summary := "decisions:\n  - use K=5\n  - use Go\n  - no Python\nactive_hypotheses:\n  - maybe\n"
	s.AppendVersionedSummary(summary)
	s.AddTurn(Turn{Content: "a"})
	s.AddTurn(Turn{Content: "b"})
	s.AddTurn(Turn{Content: "c"})
	s.AddTurn(Turn{Content: "d"})
	if sig := signalDecisionDensity(s); sig.Score <= 0 {
		t.Errorf("3 decisions / 4 turns should give positive score, got %d", sig.Score)
	}
}

func TestSignalAgentEfficiency(t *testing.T) {
	withTempCache(t)
	s, _ := Load("eff")
	// v=0 → 100
	if sig := signalAgentEfficiency(s); sig.Score != 100 {
		t.Errorf("v=0 should score 100, got %d", sig.Score)
	}
	// v=1, 10 turns → 10/compact → 100-(10-2)*15 = -20 → clamp 0
	s.Version = 1
	for i := 0; i < 10; i++ {
		s.Turns = append(s.Turns, Turn{Content: "x"})
	}
	if sig := signalAgentEfficiency(s); sig.Score != 0 {
		t.Errorf("10 turns/compact should clamp to 0, got %d", sig.Score)
	}
	// v=1, 3 turns → 3/compact → 100-(3-2)*15 = 85
	s.Turns = s.Turns[:3]
	if sig := signalAgentEfficiency(s); sig.Score != 85 {
		t.Errorf("3 turns/compact should score 85, got %d", sig.Score)
	}
}

func TestCountDecisionsInSummary(t *testing.T) {
	cases := []struct{
		name, input string
		want int
	}{
		{"empty", "", 0},
		{"no decisions", "active_hypotheses:\n  - x\n", 0},
		{"three items", "decisions:\n  - a\n  - b\n  - c\n", 3},
		{"with comments", "# header\ndecisions:\n  - a\n  # mid\n  - b\n", 2},
		{"followed by other section", "decisions:\n  - a\nactive_hypotheses:\n  - x\n  - y\n", 1},
		{"windows line endings", "decisions:\r\n  - a\r\n  - b\r\n", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := countDecisionsInSummary(c.input)
			if got != c.want {
				t.Errorf("countDecisionsInSummary(%q)=%d, want %d", c.input, got, c.want)
			}
		})
	}
}

func TestComputeQualityWeightsSumTo100(t *testing.T) {
	withTempCache(t)
	s, _ := Load("weights")
	report := ComputeQuality(s)
	var total int
	for _, sig := range report.Signals {
		total += sig.Weight
	}
	if total != 100 {
		t.Errorf("weights should sum to 100, got %d", total)
	}
}

func TestRunQualityPlain(t *testing.T) {
	withTempCache(t)
	s, _ := Load("plain-q")
	s.AddTurn(Turn{Content: "hello"})
	var stdout, stderr bytes.Buffer
	if err := run([]string{"quality", "plain-q"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	for _, want := range []string{"Quality report", "score:", "grade:", "context_fill", "compaction_depth"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output: %s", want, out)
		}
	}
}

func TestRunQualityJSON(t *testing.T) {
	withTempCache(t)
	s, _ := Load("json-q")
	s.AddTurn(Turn{Content: "hello"})
	var stdout, stderr bytes.Buffer
	if err := run([]string{"quality", "--json", "json-q"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	var report QualityReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\nout=%s", err, stdout.String())
	}
	if report.SessionID != "json-q" {
		t.Errorf("SessionID=%q, want json-q", report.SessionID)
	}
	if len(report.Signals) != 7 {
		t.Errorf("expected 7 signals in JSON, got %d", len(report.Signals))
	}
}

func TestRunQualityMissingSession(t *testing.T) {
	withTempCache(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"quality"}, &stdout, &stderr); err == nil {
		t.Fatal("expected error when session is missing")
	}
}