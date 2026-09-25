package budget

// budget_apply_test.go: testes para budget_apply.go.
// Migrado de budget_cli_test.go em 2026-09-21 (Fase 5).
import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// withStdin redireciona os.Stdin para um pipe com payload pre-escrito
// durante a execucao de fn. Retorna o que fn imprimiu em stdout.
func withStdin(t *testing.T, payload []byte, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdin: %v", err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	go func() {
		defer w.Close()
		_, _ = w.Write(payload)
	}()
	// Captura stdout tambem para funcoes que escrevem la.
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = wOut
	fnErr := fn()
	wOut.Close()
	os.Stdout = oldStdout
	os.Stdin = oldStdin
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, rOut); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	return buf.String(), fnErr
}

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

func seedTaskLog(t *testing.T, root string, tasks []AgentTask) {
	t.Helper()
	for _, tk := range tasks {
		if err := AppendAgentTask(root, tk); err != nil {
			t.Fatalf("seed AppendAgentTask: %v", err)
		}
	}
}

func TestRunBudgetReadListaTodasTasks(t *testing.T) {
	dir := t.TempDir()
	seedTaskLog(t, dir, []AgentTask{
		sampleTask("t-a"),
		sampleTask("t-b"),
		sampleTask("t-c"),
	})
	out, err := captureStdout(t, func() error {
		return runBudgetRead([]string{"-root", dir})
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("esperava 3 linhas, got %d: %s", len(lines), out)
	}
	var first AgentTask
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil || first.TaskID != "t-a" {
		t.Fatalf("linha 0 invalida: %v %+v", err, first)
	}
}

func TestRunBudgetReadFiltraPorCLI(t *testing.T) {
	dir := t.TempDir()
	a := sampleTask("t-1")
	a.CLI = "claude"
	b := sampleTask("t-2")
	b.CLI = "cursor"
	seedTaskLog(t, dir, []AgentTask{a, b})
	out, err := captureStdout(t, func() error {
		return runBudgetRead([]string{"-root", dir, "-cli", "claude"})
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Fatalf("esperava 1 linha com filtro CLI, got %d", len(lines))
	}
	var got AgentTask
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatal(err)
	}
	if got.CLI != "claude" {
		t.Fatalf("filtro CLI nao aplicado: %+v", got)
	}
}

func TestRunBudgetReadFiltraPorStatus(t *testing.T) {
	dir := t.TempDir()
	a := sampleTask("t-1")
	a.Status = "completed"
	b := sampleTask("t-2")
	b.Status = "failed"
	seedTaskLog(t, dir, []AgentTask{a, b})
	out, err := captureStdout(t, func() error {
		return runBudgetRead([]string{"-root", dir, "-status", "completed"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"status":"completed"`) {
		t.Fatalf("filtro status falhou: %s", out)
	}
	if strings.Contains(out, `"status":"failed"`) {
		t.Fatalf("filtro status nao excluiu failed: %s", out)
	}
}

func TestRunBudgetReadFiltraPorSince(t *testing.T) {
	dir := t.TempDir()
	old := sampleTask("t-old")
	old.TS = time.Now().UTC().Add(-2 * time.Hour)
	seedTaskLog(t, dir, []AgentTask{old, sampleTask("t-new")})
	cutoff := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	out, err := captureStdout(t, func() error {
		return runBudgetRead([]string{"-root", dir, "-since", cutoff})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "t-new") {
		t.Fatalf("filtro since nao retornou t-new: %s", out)
	}
	if strings.Contains(out, "t-old") {
		t.Fatalf("filtro since nao excluiu t-old: %s", out)
	}
}

func TestRunBudgetReadSinceInvalidoFalha(t *testing.T) {
	dir := t.TempDir()
	if err := runBudgetRead([]string{"-root", dir, "-since", "ontem"}); err == nil {
		t.Fatal("esperava erro para since fora do RFC3339")
	}
}

func TestRunBudgetStatsRetornaJSONEstruturado(t *testing.T) {
	dir := t.TempDir()
	a := sampleTask("t-1")
	a.CLI = "claude"
	seedTaskLog(t, dir, []AgentTask{a})
	out, err := captureStdout(t, func() error {
		return runBudgetStats([]string{"-root", dir})
	})
	if err != nil {
		t.Fatal(err)
	}
	var s AgentTaskStats
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		t.Fatalf("stats nao retornou JSON: %v\n%s", err, out)
	}
	if s.Total != 1 || s.ByCLI["claude"] != 1 {
		t.Fatalf("stats errados: %+v", s)
	}
}

func TestRunBudgetStatsLogVazioRetornaZeroValido(t *testing.T) {
	dir := t.TempDir()
	out, err := captureStdout(t, func() error {
		return runBudgetStats([]string{"-root", dir})
	})
	if err != nil {
		t.Fatal(err)
	}
	var s AgentTaskStats
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		t.Fatalf("stats nao retornou JSON: %v", err)
	}
	if s.Total != 0 || s.ByCLI == nil || s.ByStatus == nil {
		t.Fatalf("stats zero invalido: %+v", s)
	}
}

func TestRunBudgetCommandDespachaSubcommands(t *testing.T) {
	dir := t.TempDir()
	if err := AppendAgentTask(dir, sampleTask("t-1")); err != nil {
		t.Fatal(err)
	}
	if err := RunCommand([]string{"read", "-root", dir}); err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := RunCommand([]string{"stats", "-root", dir}); err != nil {
		t.Fatalf("stats: %v", err)
	}
	if err := RunCommand([]string{"help"}); err != nil {
		t.Fatalf("help: %v", err)
	}
}

func TestRunBudgetSubcommandDesconhecido(t *testing.T) {
	dir := t.TempDir()
	if err := RunCommand([]string{"invalido", "-root", dir}); err == nil {
		t.Fatal("esperava erro para subcommand desconhecido")
	}
}

func TestRunBudgetUsageListaFlagsESubcommands(t *testing.T) {
	var buf strings.Builder
	if err := budgetUsage(&buf); err != nil {
		t.Fatal(err)
	}
	text := buf.String()
	for _, want := range []string{"read", "stats", "write", "-last", "-cli", "-status", "-since", "-dry-run"} {
		if !strings.Contains(text, want) {
			t.Errorf("usage nao cita %q: %s", want, text)
		}
	}
}

func TestRunBudgetWriteAnexaAgentTaskValida(t *testing.T) {
	dir := t.TempDir()
	task := sampleTask("t-write-1")
	raw, err := json.Marshal(&task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out, fnErr := withStdin(t, raw, func() error {
		return runBudgetWrite([]string{"-root", dir})
	})
	if fnErr != nil {
		t.Fatalf("runBudgetWrite: %v (stdout=%q)", fnErr, out)
	}
	// Verifica que gravou no JSONL.
	read, err := ReadAgentTasks(dir, AgentTaskReadOptions{})
	if err != nil {
		t.Fatalf("ReadAgentTasks: %v", err)
	}
	if len(read) != 1 || read[0].TaskID != "t-write-1" {
		t.Errorf("esperava 1 task t-write-1, veio %d: %+v", len(read), read)
	}
}

func TestRunBudgetWriteDryRunNaoGrava(t *testing.T) {
	dir := t.TempDir()
	task := sampleTask("t-dry")
	raw, _ := json.Marshal(&task)
	out, fnErr := withStdin(t, raw, func() error {
		return runBudgetWrite([]string{"-root", dir, "-dry-run"})
	})
	if fnErr != nil {
		t.Fatalf("runBudgetWrite dry-run: %v", fnErr)
	}
	if !strings.Contains(out, `"task_id": "t-dry"`) && !strings.Contains(out, `"task_id":"t-dry"`) {
		t.Errorf("dry-run deveria imprimir o payload, veio: %s", out)
	}
	// Verifica que NAO gravou.
	read, _ := ReadAgentTasks(dir, AgentTaskReadOptions{})
	if len(read) != 0 {
		t.Errorf("dry-run nao deveria gravar, veio %d tasks", len(read))
	}
}

func TestRunBudgetWriteRejeitaJSONInvalido(t *testing.T) {
	dir := t.TempDir()
	_, fnErr := withStdin(t, []byte("{invalido"), func() error {
		return runBudgetWrite([]string{"-root", dir})
	})
	if fnErr == nil {
		t.Fatal("esperava erro de JSON invalido")
	}
}

func TestRunBudgetWriteRejeitaStdinVazio(t *testing.T) {
	dir := t.TempDir()
	_, fnErr := withStdin(t, []byte(""), func() error {
		return runBudgetWrite([]string{"-root", dir})
	})
	if fnErr == nil {
		t.Fatal("esperava erro de stdin vazio")
	}
}
