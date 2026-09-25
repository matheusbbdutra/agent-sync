package budget

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func sampleTask(taskID string) AgentTask {
	return AgentTask{
		SchemaVersion: AgentTaskSchemaVersion,
		TaskID:        taskID,
		TS:            time.Now().UTC(),
		CLI:           "claude",
		Model:         "claude-sonnet-4.5",
		Status:        "completed",
	}
}

func TestAppendAgentTaskCreatesFile(t *testing.T) {
	dir := t.TempDir()
	task := sampleTask("t-1")
	if err := AppendAgentTask(dir, task); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".agent-sync", AgentTaskFileName))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("arquivo deveria terminar com \\n: %q", string(data))
	}
}

func TestAppendAgentTaskRejectsInvalidSchema(t *testing.T) {
	dir := t.TempDir()
	task := sampleTask("t-bad")
	task.CLI = "invalido" // fora do enum
	if err := AppendAgentTask(dir, task); err == nil {
		t.Fatal("esperava erro de schema, recebi nil")
	}
	if _, err := os.Stat(filepath.Join(dir, ".agent-sync", AgentTaskFileName)); !os.IsNotExist(err) {
		t.Fatal("arquivo nao deveria ter sido criado apos falha de schema")
	}
}

func TestAppendAgentTaskMultiplosPreservaAppendOnly(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		if err := AppendAgentTask(dir, sampleTask("t-"+string(rune('a'+i)))); err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := ReadAgentTasks(dir, AgentTaskReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 5 {
		t.Fatalf("esperava 5 tasks, got %d", len(tasks))
	}
	for i, tk := range tasks {
		if tk.TaskID != "t-"+string(rune('a'+i)) {
			t.Fatalf("ordem quebrada em %d: %+v", i, tk)
		}
	}
}

func TestReadAgentTasksFiltraPorCLI(t *testing.T) {
	dir := t.TempDir()
	a := sampleTask("t-1")
	a.CLI = "claude"
	b := sampleTask("t-2")
	b.CLI = "cursor"
	if err := AppendAgentTask(dir, a); err != nil {
		t.Fatal(err)
	}
	if err := AppendAgentTask(dir, b); err != nil {
		t.Fatal(err)
	}
	got, err := ReadAgentTasks(dir, AgentTaskReadOptions{CLI: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].CLI != "claude" {
		t.Fatalf("filtro CLI falhou: %+v", got)
	}
}

func TestReadAgentTasksFiltraPorLast(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		if err := AppendAgentTask(dir, sampleTask("t-"+string(rune('a'+i)))); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ReadAgentTasks(dir, AgentTaskReadOptions{Last: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("esperava 3 tasks, got %d", len(got))
	}
	if got[2].TaskID != "t-e" {
		t.Fatalf("ultimo deveria ser t-e, got %s", got[2].TaskID)
	}
}

func TestReadAgentTasksFiltraPorSince(t *testing.T) {
	dir := t.TempDir()
	old := sampleTask("t-old")
	old.TS = time.Now().UTC().Add(-2 * time.Hour)
	if err := AppendAgentTask(dir, old); err != nil {
		t.Fatal(err)
	}
	recent := sampleTask("t-new")
	if err := AppendAgentTask(dir, recent); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().UTC().Add(-1 * time.Hour)
	got, err := ReadAgentTasks(dir, AgentTaskReadOptions{SinceTS: cutoff})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].TaskID != "t-new" {
		t.Fatalf("filtro Since falhou: %+v", got)
	}
}

func TestStatsAgentTasksContaPorCLIStatusETokens(t *testing.T) {
	dir := t.TempDir()
	a := sampleTask("t-1")
	a.CLI = "claude"
	a.Status = "completed"
	tokensIn := int64(100)
	tokensOut := int64(50)
	cost := 0.01
	a.TokensIn = &tokensIn
	a.TokensOut = &tokensOut
	a.CostEstimate = &cost
	if err := AppendAgentTask(dir, a); err != nil {
		t.Fatal(err)
	}
	b := sampleTask("t-2")
	b.CLI = "cursor"
	b.Status = "failed"
	if err := AppendAgentTask(dir, b); err != nil {
		t.Fatal(err)
	}
	s, err := StatsAgentTasks(dir, AgentTaskReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Total != 2 {
		t.Fatalf("Total esperado 2, got %d", s.Total)
	}
	if s.ByCLI["claude"] != 1 || s.ByCLI["cursor"] != 1 {
		t.Fatalf("ByCLI errado: %+v", s.ByCLI)
	}
	if s.ByStatus["completed"] != 1 || s.ByStatus["failed"] != 1 {
		t.Fatalf("ByStatus errado: %+v", s.ByStatus)
	}
	if s.TokensInTotal != 100 || s.TokensOutTotal != 50 {
		t.Fatalf("tokens somados errados: in=%d out=%d", s.TokensInTotal, s.TokensOutTotal)
	}
	if s.CostEstimateTotal != 0.01 {
		t.Fatalf("cost total errado: %f", s.CostEstimateTotal)
	}
}

func TestStatsAgentTasksLogVazioRetornaZero(t *testing.T) {
	dir := t.TempDir()
	s, err := StatsAgentTasks(dir, AgentTaskReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Total != 0 || s.ByCLI == nil || s.ByStatus == nil {
		t.Fatalf("stats zero invalido: %+v", s)
	}
}

func TestReadAgentTasksLogInexistenteRetornaVazio(t *testing.T) {
	dir := t.TempDir()
	got, err := ReadAgentTasks(dir, AgentTaskReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("log ausente deveria retornar nil, got %v", got)
	}
}

func TestAppendAgentTaskRotacionaEm10MB(t *testing.T) {
	dir := t.TempDir()
	// Padding para forcar rotacao: cria um arquivo pre-existente quase 10MB.
	preExisting := make([]byte, AgentTaskRotateBytes-100)
	for i := range preExisting {
		preExisting[i] = 'a'
	}
	if err := os.MkdirAll(filepath.Join(dir, ".agent-sync"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".agent-sync", AgentTaskFileName), preExisting, 0o644); err != nil {
		t.Fatal(err)
	}
	// Append de 1 task pequena forca rotacao.
	task := sampleTask("t-after-rotate")
	if err := AppendAgentTask(dir, task); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".agent-sync", AgentTaskRotationName)); err != nil {
		t.Fatalf("rotacao nao criou .1: %v", err)
	}
}
