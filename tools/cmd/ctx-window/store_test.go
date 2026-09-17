package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func withTempCache(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev := cacheRoot
	cacheRoot = dir
	t.Cleanup(func() { cacheRoot = prev })
	return dir
}

func TestSessionLoadCreatesNew(t *testing.T) {
	withTempCache(t)
	s, err := Load("test-session")
	if err != nil {
		t.Fatalf("error creating session: %v", err)
	}
	if s.ID != "test-session" {
		t.Errorf("wrong ID: %q", s.ID)
	}
	if s.K != DefaultK() {
		t.Errorf("wrong default K: %d, want %d", s.K, DefaultK())
	}
	if s.Version != 0 {
		t.Errorf("initial version should be 0, got %d", s.Version)
	}
}

func TestSessionLoadRejectsInvalidID(t *testing.T) {
	withTempCache(t)
	cases := []string{"", "../escape", "a/b", "a\\b", "line1\nline2", "x\x00y"}
	for _, id := range cases {
		if _, err := Load(id); err == nil {
			t.Errorf("id %q should be rejected", id)
		}
	}
}

func TestSessionAddTurnTrimsToK(t *testing.T) {
	withTempCache(t)
	s, err := Load("trim-session")
	if err != nil {
		t.Fatal(err)
	}
	s.K = 3
	for i := 0; i < 7; i++ {
		if err := s.AddTurn(Turn{Content: "turn " + string(rune('A'+i))}); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.Turns) != 3 {
		t.Fatalf("expected 3 turns after trim, got %d", len(s.Turns))
	}
	if !strings.Contains(s.Turns[0].Content, "E") {
		t.Errorf("first turn should be E (kept last 3 of A..G), got %q", s.Turns[0].Content)
	}
	if !strings.Contains(s.Turns[2].Content, "G") {
		t.Errorf("last turn should be G, got %q", s.Turns[2].Content)
	}
}

func TestSessionAppendVersionedSummary(t *testing.T) {
	withTempCache(t)
	s, err := Load("version-session")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AppendVersionedSummary("decisions:\n  - foo\n"); err != nil {
		t.Fatal(err)
	}
	if s.Version != 1 {
		t.Errorf("expected version 1, got %d", s.Version)
	}
	if err := s.AppendVersionedSummary("decisions:\n  - bar\n"); err != nil {
		t.Fatal(err)
	}
	if s.Version != 2 {
		t.Errorf("expected version 2, got %d", s.Version)
	}
	dir, _ := SessionDir("version-session")
	for _, name := range []string{"summary.md", "summary_v1.md", "summary_v2.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("file %s missing: %v", name, err)
		}
	}
	body, _ := os.ReadFile(filepath.Join(dir, "summary.md"))
	if !strings.Contains(string(body), "bar") {
		t.Errorf("summary.md should contain 'bar', got %s", body)
	}
}

func TestSessionListVersionsOrdered(t *testing.T) {
	withTempCache(t)
	s, _ := Load("list-session")
	s.AppendVersionedSummary("v1")
	s.AppendVersionedSummary("v2")
	s.AppendVersionedSummary("v3")
	versions, err := s.ListVersions()
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}
	for i, v := range versions {
		if !strings.Contains(v, "summary_v") {
			t.Errorf("entry %d does not look like a version: %s", i, v)
		}
	}
}

func TestSessionSaveRoundtrip(t *testing.T) {
	withTempCache(t)
	s, _ := Load("rt-session")
	s.K = 7
	s.Summarizer = "ollama:qwen2.5:3b"
	s.Budget = 500
	s.AddTurn(Turn{Content: "hello"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	s2, err := Load("rt-session")
	if err != nil {
		t.Fatal(err)
	}
	if s2.K != 7 || s2.Summarizer != "ollama:qwen2.5:3b" || s2.Budget != 500 {
		t.Errorf("metadata did not persist: %+v", s2)
	}
	if len(s2.Turns) != 1 || s2.Turns[0].Content != "hello" {
		t.Errorf("turns did not persist: %+v", s2.Turns)
	}
}

func TestSessionWriteReport(t *testing.T) {
	withTempCache(t)
	s, _ := Load("report-session")
	s.K = 5
	s.AppendVersionedSummary(ExtractedSummary{Decisoes: []string{"use K=5"}}.ToYAML())
	s.AddTurn(Turn{Content: "latest tool call"})
	var buf bytes.Buffer
	if err := s.WriteReport(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"report-session", "K=5", "Current summary", "Working memory", "latest tool call", "summary_v1.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q", want)
		}
	}
}

func TestInvalidMetaJSONFails(t *testing.T) {
	withTempCache(t)
	dir, _ := EnsureSessionDir("broken")
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("broken"); err == nil {
		t.Fatal("expected error loading invalid meta.json")
	}
}

func TestCompactFlowIntegrated(t *testing.T) {
	withTempCache(t)
	s, _ := Load("flow-session")
	s.K = 5
	s.AddTurn(Turn{Content: "We decided to use sliding window with K=5."})
	s.AddTurn(Turn{Content: "I will test heuristic on tools/cmd/ctx-window/main.go."})
	s.AddTurn(Turn{Content: "Error: hook failed with exit 127."})
	s.AddTurn(Turn{Content: "Next step: calibrate with mini-projects."})

	summary := HeuristicExtract(s.Turns)
	yaml := summary.ToYAML()
	if err := s.AppendVersionedSummary(yaml); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	// reload and verify summary persisted
	reloaded, _ := Load("flow-session")
	if reloaded.Version != 1 {
		t.Errorf("version after compact should be 1, got %d", reloaded.Version)
	}
	dir, _ := SessionDir("flow-session")
	body, _ := os.ReadFile(summaryPath(dir))
	if !strings.Contains(string(body), "decisions:") {
		t.Errorf("persisted summary without decisions section: %s", body)
	}
}

func TestTurnJSONLPersists(t *testing.T) {
	withTempCache(t)
	s, _ := Load("jsonl-session")
	s.AddTurn(Turn{Content: "first", At: time.Now().UTC()})
	s.AddTurn(Turn{Content: "second", At: time.Now().UTC()})
	dir, _ := SessionDir("jsonl-session")
	body, err := os.ReadFile(filepath.Join(dir, "turns.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(body), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSONL lines, got %d", len(lines))
	}
	for _, line := range lines {
		var turn Turn
		if err := json.Unmarshal(line, &turn); err != nil {
			t.Errorf("invalid line: %v (%s)", err, line)
		}
	}
}

func TestDefaultKAndBudgetFromEnv(t *testing.T) {
	t.Setenv("AGENT_SYNC_CTX_K", "8")
	t.Setenv("AGENT_SYNC_CTX_BUDGET", "1500")
	if got := DefaultK(); got != 8 {
		t.Errorf("DefaultK()=%d, want 8", got)
	}
	if got := DefaultBudget(); got != 1500 {
		t.Errorf("DefaultBudget()=%d, want 1500", got)
	}
}

func TestSummarizerFromConfigFallsBack(t *testing.T) {
	t.Setenv("AGENT_SYNC_SUMMARIZER", "")
	if got := SummarizerFromConfig(); got != "agent" {
		t.Errorf("fallback should be 'agent', got %q", got)
	}
	t.Setenv("AGENT_SYNC_SUMMARIZER", "ollama:qwen2.5:3b")
	if got := SummarizerFromConfig(); got != "ollama:qwen2.5:3b" {
		t.Errorf("SummarizerFromConfig()=%q, want ollama:qwen2.5:3b", got)
	}
}

func TestRunUsageAndUnknown(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{}, &stdout, &stderr); err != nil {
		t.Fatalf("no args should not fail: %v", err)
	}
	if !strings.Contains(stdout.String(), "ctx-window") {
		t.Errorf("stdout should contain usage, got %q", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if err := run([]string{"unknown"}, &stdout, &stderr); err == nil {
		t.Fatal("unknown subcommand should fail")
	}
}

func TestRunCompactEmptySession(t *testing.T) {
	withTempCache(t)
	var stdout, stderr bytes.Buffer
	err := run([]string{"compact", "empty-session"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "nothing to compact") {
		t.Fatalf("expected 'nothing to compact' error, got %v", err)
	}
}

func TestRunCompactWithTurns(t *testing.T) {
	withTempCache(t)
	s, _ := Load("comp-session")
	s.AddTurn(Turn{Content: "We decided to use K=5 for the context window."})
	s.AddTurn(Turn{Content: "I will test the heuristic on tools/foo.go."})
	var stdout, stderr bytes.Buffer
	if err := run([]string{"compact", "comp-session"}, &stdout, &stderr); err != nil {
		t.Fatalf("compact failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "compacted") {
		t.Errorf("stdout should indicate compaction: %q", stdout.String())
	}
	reloaded, _ := Load("comp-session")
	if reloaded.Version != 1 {
		t.Errorf("version should be 1, got %d", reloaded.Version)
	}
}

func TestRunSetK(t *testing.T) {
	withTempCache(t)
	s, _ := Load("setk-session")
	s.AddTurn(Turn{Content: "a"})
	s.AddTurn(Turn{Content: "b"})
	s.AddTurn(Turn{Content: "c"})
	var stdout, stderr bytes.Buffer
	if err := run([]string{"set-k", "setk-session", "2"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := Load("setk-session")
	if reloaded.K != 2 || len(reloaded.Turns) != 2 {
		t.Errorf("set-k failed: K=%d turns=%d", reloaded.K, len(reloaded.Turns))
	}
}

func TestRunShow(t *testing.T) {
	withTempCache(t)
	s, _ := Load("show-session")
	s.AppendVersionedSummary(ExtractedSummary{Decisoes: []string{"use K=5"}}.ToYAML())
	var stdout, stderr bytes.Buffer
	if err := run([]string{"show", "show-session"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "show-session") {
		t.Errorf("show should print the ID, got %q", stdout.String())
	}
}

func TestRunDoctor(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"doctor"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	for _, want := range []string{"configured summarizer", "default K", "default budget"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor missing %q: %s", want, out)
		}
	}
}

func TestRunOnToolCallBelowThreshold(t *testing.T) {
	withTempCache(t)
	var stdout, stderr bytes.Buffer
	t.Setenv("AGENT_SYNC_CTX_COMPACT_AT", "10000") // high threshold so no auto-compact
	if err := run([]string{"on-tool-call", "otc-session", "--tool", "Bash"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"auto_compacted":false`) {
		t.Errorf("expected no auto-compaction, got %s", stdout.String())
	}
	reloaded, _ := Load("otc-session")
	if len(reloaded.Turns) != 1 {
		t.Errorf("expected 1 turn, got %d", len(reloaded.Turns))
	}
	if !strings.Contains(reloaded.Turns[0].Content, "Bash") {
		t.Errorf("turn should contain tool name, got %q", reloaded.Turns[0].Content)
	}
}

func TestRunOnToolCallAboveThreshold(t *testing.T) {
	withTempCache(t)
	t.Setenv("AGENT_SYNC_CTX_COMPACT_AT", "50") // low threshold
	var stdout, stderr bytes.Buffer
	if err := run([]string{"on-tool-call", "otc-threshold", "--tool", "Read", "--input", "long content here to exceed threshold"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"auto_compacted":true`) {
		t.Errorf("expected auto-compaction, got %s", stdout.String())
	}
	reloaded, _ := Load("otc-threshold")
	if reloaded.Version != 1 {
		t.Errorf("expected version 1 after auto-compact, got %d", reloaded.Version)
	}
}

func TestRunOnToolCallMissingTool(t *testing.T) {
	withTempCache(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"on-tool-call", "no-tool"}, &stdout, &stderr); err == nil {
		t.Fatal("expected error when --tool is missing")
	}
}

func TestEstimatedChars(t *testing.T) {
	withTempCache(t)
	s, _ := Load("est-session")
	s.AddTurn(Turn{Content: "hello"})
	s.AddTurn(Turn{Content: "world"})
	got := s.EstimatedChars()
	if got <= 0 {
		t.Errorf("estimated chars should be > 0, got %d", got)
	}
}

func TestCompactAtThresholdDefault(t *testing.T) {
	t.Setenv("AGENT_SYNC_CTX_COMPACT_AT", "")
	if got := compactAtThreshold(); got != 200 {
		t.Errorf("default threshold should be 200, got %d", got)
	}
	t.Setenv("AGENT_SYNC_CTX_COMPACT_AT", "500")
	if got := compactAtThreshold(); got != 500 {
		t.Errorf("env threshold should be 500, got %d", got)
	}
}
