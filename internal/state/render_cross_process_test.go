package state

// state_render_cross_process_test.go: testes cross-process do CLI state.
// Migrado de state_cli_cross_process_test.go em 2026-09-21 (Fase 3).
// state_apply_cross_process_test.go: testes cross-process do CLI state.
// Migrado de state_cli_cross_process_test.go em 2026-09-21 (Fase 3).
import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// crossProcessBin é o binário agent-sync construído uma única vez por
// `go test` para uso dos testes cross-process. Skip se go não disponível
// ou se build falhar (registrado em testBinErr).
var (
	crossProcessBin  string
	crossProcessErr  error
	crossProcessOnce sync.Once
)

// buildAgentSyncBin compila cmd/agent-sync para um arquivo temporário e
// retorna o path. Idempotente via sync.Once — primeira chamada compila,
// demais reutilizam.
func buildAgentSyncBin(t *testing.T) string {
	t.Helper()
	crossProcessOnce.Do(func() {
		bin, err := os.MkdirTemp("", "agent-sync-test-")
		if err != nil {
			crossProcessErr = err
			return
		}
		crossProcessBin = filepath.Join(bin, "agent-sync")
		cmd := exec.Command("go", "build", "-o", crossProcessBin, "./cmd/agent-sync")
		cmd.Dir = repoRoot(t)
		if out, err := cmd.CombinedOutput(); err != nil {
			crossProcessErr = &execExitErr{err: err, output: string(out)}
		}
	})
	if crossProcessErr != nil {
		t.Skipf("build do binário falhou: %v", crossProcessErr)
	}
	return crossProcessBin
}

type execExitErr struct {
	err    error
	output string
}

func (e *execExitErr) Error() string { return e.err.Error() + ": " + e.output }
func (e *execExitErr) Unwrap() error { return e.err }

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for d := wd; d != "/" && d != "."; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			data, err := os.ReadFile(filepath.Join(d, "go.mod"))
			if err != nil {
				t.Fatal(err)
			}
			if contains(string(data), "module github.com/matheusdutra/agent-sync") {
				return d
			}
		}
	}
	t.Fatal("repo root (go.mod com module agent-sync) não encontrado")
	return ""
}

// TestCrossProcessHandoffWriteReadNextAction valida o critério de aceite
// da ADR-002 Estágio B: CLI A escreve session-state.json, CLI B lê via
// next-action no turno seguinte. Processos distintos (subprocessos reais),
// mesmo .agent-sync/ root, portabilidade cross-process confirmada.
func TestCrossProcessHandoffWriteReadNextAction(t *testing.T) {
	bin := buildAgentSyncBin(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, SessionStateDirName), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	payload := sampleState()
	payload.Session.ID = "sess-handoff-A"
	// A-37 Etapa 2: NextActions -> Tasks; subcommand 'state next-action' deriva
	// SessionAction de Tasks (shim em model.go:430) via filtro legacy_kind=todo|delivery.
	payload.Tasks = []SessionTask{
		{ID: "T-1", Title: "acao distinta para handoff A->B", Status: "pending", LegacyKind: "todo", StartedAt: payload.Session.StartedAt},
	}
	payloadBytes, _ := json.MarshalIndent(&payload, "", "  ")
	payloadPath := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(payloadPath, payloadBytes, 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	// Processo A: write
	cmdA := exec.Command(bin, "state", "write", "-root", root, "-from", payloadPath)
	if out, err := cmdA.CombinedOutput(); err != nil {
		t.Fatalf("process A write falhou: %v\n%s", err, out)
	}

	// Processo B: next-action (turno seguinte, processo distinto)
	cmdB := exec.Command(bin, "state", "next-action", "-root", root)
	outB, err := cmdB.Output()
	if err != nil {
		t.Fatalf("process B next-action falhou: %v", err)
	}

	var got SessionAction
	if err := json.Unmarshal(outB, &got); err != nil {
		t.Fatalf("next-action stdout nao parseia: %v\nsaida=%s", err, outB)
	}
	if got.ID != "T-1" {
		t.Errorf("next-action ID=%q, esperava T-1", got.ID)
	}
	if got.Title != "acao distinta para handoff A->B" {
		t.Errorf("next-action title=%q", got.Title)
	}
}

// TestCrossProcessHandoffValidateEmOutroProcesso verifica que o validate
// roda em processo distinto do write — confirma que o JSON escrito pelo A
// é portável (mesmo JSON Schema, mesma lib, sem dependência de estado
// in-memory).
func TestCrossProcessHandoffValidateEmOutroProcesso(t *testing.T) {
	bin := buildAgentSyncBin(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, SessionStateDirName), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := WriteSessionState(root, sampleState()); err != nil {
		t.Fatalf("write in-process: %v", err)
	}

	cmd := exec.Command(bin, "state", "validate", "-root", root)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("validate em subprocesso: %v", err)
	}
	if !contains(string(out), "ok") {
		t.Errorf("esperava 'ok' em stdout, recebi %q", out)
	}
}

// TestCrossProcessWritesConcorrentesSemLock verifica que 2 subprocessos
// escrevendo no mesmo root (lockless, default) completam sem corromper
// o arquivo. Um deles vence o rename; o JSON final é de um dos dois.
func TestCrossProcessWritesConcorrentesSemLock(t *testing.T) {
	bin := buildAgentSyncBin(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, SessionStateDirName), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	makePayload := func(id string, actionTitle string) string {
		s := sampleState()
		s.Session.ID = id
		// A-37 Etapa 2: NextActions -> Tasks. Subcommand 'state next-action'
		// continua funcionando porque NextAction() e shim sobre Tasks.
		s.Tasks = []SessionTask{
			{ID: "T-1", Title: actionTitle, Status: "pending", LegacyKind: "todo", StartedAt: s.Session.StartedAt},
		}
		b, _ := json.MarshalIndent(&s, "", "  ")
		return string(b)
	}

	payloadA := filepath.Join(t.TempDir(), "A.json")
	payloadB := filepath.Join(t.TempDir(), "B.json")
	if err := os.WriteFile(payloadA, []byte(makePayload("sess-A", "acao de A")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(payloadB, []byte(makePayload("sess-B", "acao de B")), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv(SessionStateLockEnvVar, "")

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, p := range []string{payloadA, payloadB} {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(bin, "state", "write", "-root", root, "-from", p)
			if out, err := cmd.CombinedOutput(); err != nil {
				errs <- &execExitErr{err: err, output: string(out)}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("writer falhou: %v", err)
	}

	// Verifica JSON final é de um dos dois (não corrompido)
	loaded, err := ReadSessionState(root)
	if err != nil {
		t.Fatalf("read apos writes concorrentes: %v", err)
	}
	if loaded.Session.ID != "sess-A" && loaded.Session.ID != "sess-B" {
		t.Errorf("session.id=%q, esperava sess-A ou sess-B", loaded.Session.ID)
	}

	// Verifica que não há tmpfile leftover
	entries, err := os.ReadDir(filepath.Join(root, SessionStateDirName))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if contains(e.Name(), ".tmp.") {
			t.Errorf("tmpfile leftover: %s", e.Name())
		}
	}
}

// TestCrossProcessHandoffReadEmRootInvalidoFalha verifica que subprocesso
// reclamando root inexistente falha com erro estruturado (não silencioso).
func TestCrossProcessHandoffReadEmRootInvalidoFalha(t *testing.T) {
	bin := buildAgentSyncBin(t)
	cmd := exec.Command(bin, "state", "next-action", "-root", "/nonexistent-"+time.Now().Format("150405.000"))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("esperava erro em root inexistente, recebi saida=%s", out)
	}
}
