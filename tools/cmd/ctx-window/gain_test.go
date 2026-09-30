package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAppendGainEntry(t *testing.T) {
	withTempCache(t)
	err := AppendGainEntry("gain-sess", GainEntry{
		Kind:        "heuristic_compact",
		Version:     1,
		BeforeChars: 1000,
		AfterChars:  200,
		SavedChars:  800,
	})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := LoadGainEntries("gain-sess")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Kind != "heuristic_compact" {
		t.Errorf("Kind=%q, want heuristic_compact", e.Kind)
	}
	if e.SavedTokens != 200 {
		t.Errorf("SavedTokens=%d, want 200 (800/4)", e.SavedTokens)
	}
	if e.At.IsZero() {
		t.Error("At should be auto-populated")
	}
}

func TestAppendGainEntryMultiple(t *testing.T) {
	withTempCache(t)
	for i := 1; i <= 3; i++ {
		err := AppendGainEntry("multi", GainEntry{
			Kind: "auto_compact", Version: i,
			BeforeChars: 1000, AfterChars: 200, SavedChars: 800,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	entries, err := LoadGainEntries("multi")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestLoadGainEntriesEmpty(t *testing.T) {
	withTempCache(t)
	entries, err := LoadGainEntries("none-sess")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty, got %d", len(entries))
	}
}

func TestLoadGainEntriesSkipsCorrupt(t *testing.T) {
	withTempCache(t)
	// pre-popula com 1 válida + 1 corrupta
	err := AppendGainEntry("corrupt-sess", GainEntry{Kind: "test", Version: 1, SavedChars: 100})
	if err != nil {
		t.Fatal(err)
	}
	path, _ := gainPath("corrupt-sess")
	f := osOpenFileAppend(path)
	f.WriteString("not json\n")
	f.Close()
	entries, err := LoadGainEntries("corrupt-sess")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 valid (corrupt skipped), got %d", len(entries))
	}
}

func TestAggregateGain(t *testing.T) {
	now := time.Now().UTC()
	entries := []GainEntry{
		{Kind: "heuristic_compact", At: now.Add(-2 * time.Hour), SavedChars: 800, SavedTokens: 200},
		{Kind: "auto_compact", At: now.Add(-1 * time.Hour), SavedChars: 500, SavedTokens: 125},
		{Kind: "auto_compact", At: now, SavedChars: 300, SavedTokens: 75},
	}
	totals := aggregateGain(entries)
	if totals.Events != 3 {
		t.Errorf("Events=%d, want 3", totals.Events)
	}
	if totals.TotalChars != 1600 {
		t.Errorf("TotalChars=%d, want 1600", totals.TotalChars)
	}
	if totals.TotalTokens != 400 {
		t.Errorf("TotalTokens=%d, want 400", totals.TotalTokens)
	}
	if totals.ByKind["auto_compact"] != 800 {
		t.Errorf("auto_compact total=%d, want 800", totals.ByKind["auto_compact"])
	}
}

func TestWriteGainMarkdownEmpty(t *testing.T) {
	var buf bytes.Buffer
	WriteGainMarkdown(&buf, nil)
	if !strings.Contains(buf.String(), "no compaction events") {
		t.Errorf("expected empty-state message, got %q", buf.String())
	}
}

func TestWriteGainMarkdownWithData(t *testing.T) {
	now := time.Now().UTC()
	entries := []GainEntry{
		{Kind: "heuristic_compact", At: now, Version: 1, BeforeChars: 1000, AfterChars: 200, SavedChars: 800, SavedTokens: 200},
		{Kind: "auto_compact", At: now.Add(time.Minute), Version: 2, BeforeChars: 600, AfterChars: 100, SavedChars: 500, SavedTokens: 125},
	}
	var buf bytes.Buffer
	WriteGainMarkdown(&buf, entries)
	out := buf.String()
	for _, want := range []string{"# ctx-window gain", "| at | kind |", "heuristic_compact", "auto_compact", "events: 2", "1300 chars", "by kind:"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in markdown output:\n%s", want, out)
		}
	}
}

func TestRunGainMissingSession(t *testing.T) {
	withTempCache(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"gain"}, &stdout, &stderr); err == nil {
		t.Fatal("expected error when session is missing")
	}
}

func TestRunGainPlainEmpty(t *testing.T) {
	withTempCache(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"gain", "empty-sess"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "no compaction events") {
		t.Errorf("expected empty-state, got %s", stdout.String())
	}
}

func TestRunGainCompactIntegration(t *testing.T) {
	withTempCache(t)
	// cria sessão + roda compact para gerar 1 entry
	s, _ := Load("gain-int")
	s.AddTurn(Turn{Content: "We decided to use sliding window with K=5."})
	s.AddTurn(Turn{Content: "I will test heuristic on tools/cmd/ctx-window/main.go."})
	s.AddTurn(Turn{Content: "Error: hook failed with exit 127."})
	s.AddTurn(Turn{Content: "Next step: calibrate with mini-projects."})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := run([]string{"compact", "gain-int"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if err := run([]string{"gain", "gain-int"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "heuristic_compact") {
		t.Errorf("expected heuristic_compact in output:\n%s", out)
	}
	if !strings.Contains(out, "events: 1") {
		t.Errorf("expected 1 event, got:\n%s", out)
	}
}

func TestRunGainJSON(t *testing.T) {
	withTempCache(t)
	if err := AppendGainEntry("json-gain", GainEntry{Kind: "summarize", Version: 1, SavedChars: 500}); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := run([]string{"gain", "--json", "json-gain"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"entries"`) {
		t.Errorf("expected entries in JSON output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"summarize"`) {
		t.Errorf("expected kind=summarize: %s", stdout.String())
	}
}

// helper para teste de corrupção
func osOpenFileAppend(path string) interface {
	WriteString(string) (int, error)
	Close() error
} {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		panic(err)
	}
	return f
}
