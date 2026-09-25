package memory

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/matheusdutra/agent-sync/internal/event"
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// helpers copiados de internal/event/session_test.go (test helpers
// package-private não podem ser importados; duplicação mínima aceitável
// até surgir um padrão de testutil compartilhado).

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
	return <-outC, fnErr
}

func sampleMemoryEvent(kind, ref, title string) event.SessionEvent {
	return event.SessionEvent{
		SchemaVersion: event.SessionEventSchemaVersion,
		TS:            time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC),
		Kind:          kind,
		Ref:           ref,
		Title:         title,
		Actor:         "agent-sync",
		SessionID:     "sess-test",
	}
}

func setupMemoryFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, pathutil.SessionStateDirName), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return root
}

func TestRunMemoryRecentListaEventosEmJSONL(t *testing.T) {
	root := setupMemoryFixture(t)
	for _, e := range []event.SessionEvent{
		sampleMemoryEvent("decision", "D-1", "primeira decisao"),
		sampleMemoryEvent("action", "A-1", "primeira acao"),
		sampleMemoryEvent("blocker", "B-1", "primeiro blocker"),
	} {
		if err := event.AppendEvent(root, e); err != nil {
			t.Fatalf("seed AppendEvent: %v", err)
		}
	}

	out, err := captureStdout(t, func() error {
		return runMemoryRecent([]string{"-root", root})
	})
	if err != nil {
		t.Fatalf("memory recent: %v", err)
	}

	// 3 linhas JSONL, cada uma parseável como SessionEvent.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("esperava 3 linhas, recebi %d: %q", len(lines), out)
	}
	for i, line := range lines {
		var got event.SessionEvent
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("linha %d nao parseou: %v (raw: %q)", i, err, line)
		}
		if got.SchemaVersion != event.SessionEventSchemaVersion {
			t.Fatalf("linha %d: schema_version=%q esperado %q", i, got.SchemaVersion, event.SessionEventSchemaVersion)
		}
	}
}

func TestRunMemoryRecentAplicaFiltroLast(t *testing.T) {
	root := setupMemoryFixture(t)
	for i := 0; i < 5; i++ {
		e := sampleMemoryEvent("decision", "D-1", "decisao")
		e.Ref = "D-" + string(rune('1'+i))
		if err := event.AppendEvent(root, e); err != nil {
			t.Fatalf("seed AppendEvent: %v", err)
		}
	}

	out, err := captureStdout(t, func() error {
		return runMemoryRecent([]string{"-root", root, "-last", "2"})
	})
	if err != nil {
		t.Fatalf("memory recent -last 2: %v", err)
	}

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("esperava 2 linhas (default -last 2), recebi %d", len(lines))
	}
}

// TestMemoryFeedbackGravaAppendOnly cobre A-58 (S-0.3 Camada 4, ses_f2b12742,
// 2026-09-24): runMemoryFeedback grava 1 entrada em JSONL append-only no
// diretorio apontado por -cache-dir. Valida 3 cenarios: helpful simples,
// stale com reason, e sinal invalido (recusado sem gravar nada).
//
// Isolamento: usa t.TempDir() via -cache-dir para NAO sujar
// ~/.cache/agent-sync/ real do usuario.
func TestMemoryFeedbackGravaAppendOnly(t *testing.T) {
	tmpDir := t.TempDir()

	// Cenario 1: helpful simples — grava 1 entrada com campos minimos
	out, err := captureStdout(t, func() error {
		return runMemoryFeedback([]string{"-cache-dir", tmpDir, "adr/A-58-test", "helpful"})
	})
	if err != nil {
		t.Fatalf("feedback helpful falhou: %v", err)
	}
	if !strings.Contains(out, "feedback registrado") {
		t.Errorf("esperava confirmacao 'feedback registrado', obteve: %s", out)
	}

	dst := filepath.Join(tmpDir, "memory_feedback.jsonl")
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ler %s: %v", dst, err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("esperava 1 linha, obteve %d: %q", len(lines), string(data))
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("linha nao parseou: %v (raw: %q)", err, lines[0])
	}
	for _, want := range []string{"ts", "path", "signal", "agent"} {
		if _, ok := entry[want]; !ok {
			t.Errorf("entrada sem campo %q: %v", want, entry)
		}
	}
	if entry["signal"] != "helpful" {
		t.Errorf("signal esperado 'helpful', obteve %v", entry["signal"])
	}
	if entry["path"] != "adr/A-58-test" {
		t.Errorf("path esperado 'adr/A-58-test', obteve %v", entry["path"])
	}
	if entry["agent"] != "agent-sync" {
		t.Errorf("agent default esperado 'agent-sync', obteve %v", entry["agent"])
	}

	// Cenario 2: stale com reason + agent custom — grava 2a entrada (append real)
	out, err = captureStdout(t, func() error {
		return runMemoryFeedback([]string{"-cache-dir", tmpDir, "-reason", "obsoleto", "-agent", "opencode", "adr/A-58-test", "stale"})
	})
	if err != nil {
		t.Fatalf("feedback stale falhou: %v", err)
	}
	data, _ = os.ReadFile(dst)
	lines = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("esperava 2 linhas (append), obteve %d: %q", len(lines), string(data))
	}
	var entry2 map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &entry2); err != nil {
		t.Fatalf("2a linha nao parseou: %v", err)
	}
	if entry2["signal"] != "stale" {
		t.Errorf("2a entrada: signal esperado 'stale', obteve %v", entry2["signal"])
	}
	if entry2["reason"] != "obsoleto" {
		t.Errorf("2a entrada: reason esperado 'obsoleto', obteve %v", entry2["reason"])
	}
	if entry2["agent"] != "opencode" {
		t.Errorf("2a entrada: agent esperado 'opencode', obteve %v", entry2["agent"])
	}

	// Cenario 3: signal invalido — recusado, NAO grava nada
	beforeCount := len(lines)
	_, err = captureStdout(t, func() error {
		return runMemoryFeedback([]string{"-cache-dir", tmpDir, "adr/A-58-test", "invalid_signal"})
	})
	if err == nil {
		t.Fatalf("signal invalido deveria retornar erro")
	}
	if !strings.Contains(err.Error(), "signal invalido") {
		t.Errorf("esperava msg 'signal invalido', obteve: %v", err)
	}
	afterData, _ := os.ReadFile(dst)
	afterLines := strings.Split(strings.TrimRight(string(afterData), "\n"), "\n")
	if len(afterLines) != beforeCount {
		t.Errorf("signal invalido nao deveria gravar: antes=%d depois=%d", beforeCount, len(afterLines))
	}
}

// TestMemoryWritePageGravaAppendOnly cobre A-59 (S-0.3 Camada 4,
// ses_f2ad21a27ffeasIkrIiVwQc53m, 2026-09-24): runMemoryWritePage grava
// 1 entrada em JSONL append-only no diretorio apontado por -cache-dir.
// Valida 3 cenarios: page simples (scope default=project), scope=global,
// e expires_at invalido (recusado sem gravar nada).
//
// Isolamento: usa t.TempDir() via -cache-dir para NAO sujar
// ~/.cache/agent-sync/ real do usuario. Padrao simetrico com
// TestMemoryFeedbackGravaAppendOnly (A-58).
func TestMemoryWritePageGravaAppendOnly(t *testing.T) {
	tmpDir := t.TempDir()

	// Cenario 1: page simples — body com frontmatter YAML, scope default=project
	body1 := "---\nscope: project\nexpires_at: 2026-12-31\n---\n# A-59 teste\nbody markdown simples\n"
	out, err := captureStdout(t, func() error {
		return runMemoryWritePage([]string{"-cache-dir", tmpDir, "-body", body1, "adr/A-59-teste"})
	})
	if err != nil {
		t.Fatalf("write-page simples falhou: %v", err)
	}
	if !strings.Contains(out, "page gravada") {
		t.Errorf("esperava confirmacao 'page gravada', obteve: %s", out)
	}

	dst := filepath.Join(tmpDir, "memory_pages.jsonl")
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ler %s: %v", dst, err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("esperava 1 linha, obteve %d: %q", len(lines), string(data))
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("linha nao parseou: %v (raw: %q)", err, lines[0])
	}
	for _, want := range []string{"ts", "path", "scope", "body", "agent"} {
		if _, ok := entry[want]; !ok {
			t.Errorf("entrada sem campo %q: %v", want, entry)
		}
	}
	if entry["scope"] != "project" {
		t.Errorf("scope esperado 'project' (default), obteve %v", entry["scope"])
	}
	if entry["path"] != "adr/A-59-teste" {
		t.Errorf("path esperado 'adr/A-59-teste', obteve %v", entry["path"])
	}
	if entry["agent"] != "agent-sync" {
		t.Errorf("agent default esperado 'agent-sync', obteve %v", entry["agent"])
	}

	// Cenario 2: scope=global — grava 2a entrada (append real)
	out, err = captureStdout(t, func() error {
		return runMemoryWritePage([]string{"-cache-dir", tmpDir, "-scope", "global", "-body", "# global page", "prefs/A-59-global"})
	})
	if err != nil {
		t.Fatalf("write-page scope=global falhou: %v", err)
	}
	data, _ = os.ReadFile(dst)
	lines = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("esperava 2 linhas (append), obteve %d: %q", len(lines), string(data))
	}
	var entry2 map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &entry2); err != nil {
		t.Fatalf("2a linha nao parseou: %v", err)
	}
	if entry2["scope"] != "global" {
		t.Errorf("2a entrada: scope esperado 'global', obteve %v", entry2["scope"])
	}

	// Cenario 3: expires_at invalido — recusado, NAO grava nada
	beforeCount := len(lines)
	_, err = captureStdout(t, func() error {
		return runMemoryWritePage([]string{"-cache-dir", tmpDir, "-expires-at", "ontem", "-body", "x", "adr/A-59-expires-bad"})
	})
	if err == nil {
		t.Fatalf("expires-at invalido deveria retornar erro")
	}
	if !strings.Contains(err.Error(), "expires-at invalido") {
		t.Errorf("esperava msg 'expires-at invalido', obteve: %v", err)
	}
	afterData, _ := os.ReadFile(dst)
	afterLines := strings.Split(strings.TrimRight(string(afterData), "\n"), "\n")
	if len(afterLines) != beforeCount {
		t.Errorf("expires-at invalido nao deveria gravar: antes=%d depois=%d", beforeCount, len(afterLines))
	}
}
