package event

import (
	"encoding/json"
	"strings"
	"testing"
)

func seedEventLog(t *testing.T, root string, events []SessionEvent) {
	t.Helper()
	for _, e := range events {
		if err := AppendEvent(root, e); err != nil {
			t.Fatalf("seed AppendEvent: %v", err)
		}
	}
}

func TestRunEventReadListaTodosEventos(t *testing.T) {
	root := setupEventFixture(t)
	seedEventLog(t, root, []SessionEvent{
		sampleEvent("decision", "D-1", "d1"),
		sampleEvent("action", "A-1", "a1"),
		sampleEvent("blocker", "B-1", "b1"),
	})
	out, err := captureStdout(t, func() error {
		return runEventRead([]string{"-root", root})
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Errorf("esperava 3 linhas JSONL, veio %d", len(lines))
	}
	for _, line := range lines {
		var e SessionEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Errorf("linha invalida: %v (%s)", err, line)
		}
	}
}

func TestRunEventReadFiltraPorKind(t *testing.T) {
	root := setupEventFixture(t)
	seedEventLog(t, root, []SessionEvent{
		sampleEvent("decision", "D-1", "d1"),
		sampleEvent("action", "A-1", "a1"),
		sampleEvent("decision", "D-2", "d2"),
	})
	out, err := captureStdout(t, func() error {
		return runEventRead([]string{"-root", root, "-kind", "decision"})
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Errorf("esperava 2 decisions, veio %d", len(lines))
	}
}

func TestRunEventReadFiltraPorLast(t *testing.T) {
	root := setupEventFixture(t)
	seedEventLog(t, root, []SessionEvent{
		sampleEvent("action", "A-1", "a1"),
		sampleEvent("action", "A-2", "a2"),
		sampleEvent("action", "A-3", "a3"),
	})
	out, err := captureStdout(t, func() error {
		return runEventRead([]string{"-root", root, "-last", "1"})
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Fatalf("esperava 1 linha, veio %d", len(lines))
	}
	var e SessionEvent
	json.Unmarshal([]byte(lines[0]), &e)
	if e.Ref != "A-3" {
		t.Errorf("esperava A-3, veio %s", e.Ref)
	}
}

func TestRunEventStatsRetornaJSONEstruturado(t *testing.T) {
	root := setupEventFixture(t)
	seedEventLog(t, root, []SessionEvent{
		sampleEvent("decision", "D-1", "d1"),
		sampleEvent("decision", "D-2", "d2"),
		sampleEvent("action", "A-1", "a1"),
	})
	out, err := captureStdout(t, func() error {
		return runEventStats([]string{"-root", root})
	})
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	var s EventStats
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		t.Fatalf("unmarshal stats: %v (%s)", err, out)
	}
	if s.Total != 3 {
		t.Errorf("Total: got %d, want 3", s.Total)
	}
	if s.ByKind["decision"] != 2 || s.ByKind["action"] != 1 {
		t.Errorf("ByKind: %+v", s.ByKind)
	}
}

func TestRunEventCommandDespachaSubcommands(t *testing.T) {
	root := setupEventFixture(t)
	seedEventLog(t, root, []SessionEvent{sampleEvent("decision", "D-1", "d1")})

	out, err := captureStdout(t, func() error {
		return RunCommand([]string{"read", "-root", root})
	})
	if err != nil {
		t.Fatalf("RunCommand read: %v", err)
	}
	if !strings.Contains(out, "D-1") {
		t.Errorf("saida nao contem D-1: %s", out)
	}

	outStats, err := captureStdout(t, func() error {
		return RunCommand([]string{"stats", "-root", root})
	})
	if err != nil {
		t.Fatalf("RunCommand stats: %v", err)
	}
	if !strings.Contains(outStats, `"total": 1`) {
		t.Errorf("stats nao contem total 1: %s", outStats)
	}
}

func TestRunEventCommandSubcommandDesconhecido(t *testing.T) {
	err := RunCommand([]string{"unknown"})
	if err == nil {
		t.Fatalf("esperava erro para subcommand desconhecido")
	}
}

func TestEventUsageExibeSubcommandsEFlags(t *testing.T) {
	out, err := captureStdout(t, func() error {
		return RunCommand([]string{"help"})
	})
	if err != nil {
		t.Fatalf("help: %v", err)
	}
	for _, expected := range []string{"read", "stats", "tail", "-last", "-kind", "-since", "-root"} {
		if !strings.Contains(out, expected) {
			t.Errorf("help nao contem %q:\n%s", expected, out)
		}
	}
}
