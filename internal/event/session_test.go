package event

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()
	fnErr := fn()
	w.Close()
	os.Stdout = old
	out := <-outC
	return out, fnErr
}

func sampleEvent(kind, ref, title string) SessionEvent {
	return SessionEvent{
		SchemaVersion: SessionEventSchemaVersion,
		TS:            time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC),
		Kind:          kind,
		Ref:           ref,
		Title:         title,
		Actor:         "agent-sync",
		SessionID:     "sess-test",
	}
}

func setupEventFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, pathutil.SessionStateDirName), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return root
}

func TestAppendEventCreatesFile(t *testing.T) {
	root := setupEventFixture(t)
	if err := AppendEvent(root, sampleEvent("decision", "D-1", "primeira")); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	data, err := os.ReadFile(eventPath(root))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var e SessionEvent
	if err := json.Unmarshal(data, &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if e.Ref != "D-1" || e.Kind != "decision" {
		t.Errorf("dados incorretos: %+v", e)
	}
}

func TestAppendEventMultipleAppends(t *testing.T) {
	root := setupEventFixture(t)
	for i := 1; i <= 3; i++ {
		e := sampleEvent("action", "A-"+string(rune('0'+i)), "action")
		if err := AppendEvent(root, e); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	events, err := ReadEvents(root, EventReadOptions{})
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("esperava 3 eventos, veio %d", len(events))
	}
}

func TestAppendEventRejectsInvalidSchema(t *testing.T) {
	root := setupEventFixture(t)
	e := SessionEvent{
		Kind: "decision",
		// Ref faltando -> invalido no schema
	}
	if err := AppendEvent(root, e); err == nil {
		t.Errorf("esperava erro de validacao de schema")
	}
}

func TestReadEventsFilterKind(t *testing.T) {
	root := setupEventFixture(t)
	AppendEvent(root, sampleEvent("decision", "D-1", "d1"))
	AppendEvent(root, sampleEvent("action", "A-1", "a1"))
	AppendEvent(root, sampleEvent("decision", "D-2", "d2"))

	events, err := ReadEvents(root, EventReadOptions{Kind: "decision"})
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("esperava 2 decisions, veio %d", len(events))
	}
	for _, e := range events {
		if e.Kind != "decision" {
			t.Errorf("kind incorreto: %s", e.Kind)
		}
	}
}

func TestReadEventsFilterLast(t *testing.T) {
	root := setupEventFixture(t)
	for i := 1; i <= 5; i++ {
		AppendEvent(root, sampleEvent("action", "A-"+string(rune('0'+i)), "a"))
	}
	events, err := ReadEvents(root, EventReadOptions{Last: 2})
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("esperava 2 eventos, veio %d", len(events))
	}
	if events[0].Ref != "A-4" || events[1].Ref != "A-5" {
		t.Errorf("last nao pegou os ultimos: %+v", events)
	}
}

func TestReadEventsFilterSince(t *testing.T) {
	root := setupEventFixture(t)
	t1 := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 9, 19, 14, 0, 0, 0, time.UTC)

	e1 := sampleEvent("decision", "D-1", "d1")
	e1.TS = t1
	e2 := sampleEvent("decision", "D-2", "d2")
	e2.TS = t2
	e3 := sampleEvent("decision", "D-3", "d3")
	e3.TS = t3

	AppendEvent(root, e1)
	AppendEvent(root, e2)
	AppendEvent(root, e3)

	events, err := ReadEvents(root, EventReadOptions{Since: t2})
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("esperava 2 eventos (>= t2), veio %d", len(events))
	}
	if events[0].Ref != "D-2" || events[1].Ref != "D-3" {
		t.Errorf("since nao filtrou correto: %+v", events)
	}
}

func TestStatsEvents(t *testing.T) {
	root := setupEventFixture(t)
	AppendEvent(root, sampleEvent("decision", "D-1", "d1"))
	AppendEvent(root, sampleEvent("decision", "D-2", "d2"))
	AppendEvent(root, sampleEvent("action", "A-1", "a1"))

	stats, err := StatsEvents(root, EventReadOptions{})
	if err != nil {
		t.Fatalf("StatsEvents: %v", err)
	}
	if stats.Total != 3 {
		t.Errorf("Total: got %d, want 3", stats.Total)
	}
	if stats.ByKind["decision"] != 2 || stats.ByKind["action"] != 1 {
		t.Errorf("ByKind: %+v", stats.ByKind)
	}
	if stats.ByActor["agent-sync"] != 3 {
		t.Errorf("ByActor: %+v", stats.ByActor)
	}
}

func TestFilteredEvents(t *testing.T) {
	events := []SessionEvent{
		sampleEvent("decision", "D-1", "d1"),
		sampleEvent("action", "A-1", "a1"),
		sampleEvent("decision", "D-2", "d2"),
	}
	filtered := FilteredEvents(events, EventReadOptions{Kind: "decision"})
	if len(filtered) != 2 {
		t.Errorf("esperava 2 decisions, veio %d", len(filtered))
	}
}
