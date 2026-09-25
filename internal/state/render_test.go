package state

// state_render_test.go: testes para state_render.go (CLI render).
// Migrado de state_cli_test.go em 2026-09-21 (Fase 3).
// state_apply_test.go: testes para state_apply.go (CLI apply).
// Migrado de state_cli_test.go em 2026-09-21 (Fase 3).
import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupStateFixture(t *testing.T, s SessionState) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, SessionStateDirName), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if s.SchemaVersion == "" {
		s.SchemaVersion = SessionStateSchemaVersion
	}
	if s.Session.StartedAt.IsZero() {
		s.Session.StartedAt = time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	}
	s.Session.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	if err := WriteSessionState(root, s); err != nil {
		t.Fatalf("WriteSessionState: %v", err)
	}
	return root
}

// sampleState é a fixture usada por todos os testes de render/snapshot/event.
// A-37 Etapa 2: NextActions/Blockers removidos do struct (D-69); fonte canonica
// é Tasks/Issues. Subcommand 'state next-action' deriva a primeira SessionAction
// de Tasks com LegacyKind=todo|delivery via shim em model.go:430.
func sampleState() SessionState {
	return SessionState{
		SchemaVersion: SessionStateSchemaVersion,
		Project: SessionProject{
			Name: "agent-sync",
			Root: "/srv/repos/agent-sync",
		},
		Git: SessionGit{
			Branch:             "feat/docs-session-2026-09-19",
			Head:               "b03e1a1",
			WorkingTreeSummary: "limpo exceto untracked .agent-sync/ + tools/ctx-window",
		},
		Session: SessionMeta{
			ID:        "sess-2026-09-19",
			StartedAt: time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC),
		},
		Decisions: []SessionDecision{
			{ID: "D-1", Title: "schema-output versionado", Rationale: "consistencia cross-CLI", MadeAt: time.Date(2026, 9, 19, 11, 0, 0, 0, time.UTC)},
		},
		Tasks: []SessionTask{
			{ID: "T-1", Title: "implementar lib jsonschema", Status: "done", LegacyKind: "delivery", StartedAt: time.Date(2026, 9, 19, 10, 30, 0, 0, time.UTC)},
			{ID: "T-2", Title: "wirar subcommand state", Status: "in_progress", LegacyKind: "todo", StartedAt: time.Date(2026, 9, 19, 11, 0, 0, 0, time.UTC)},
			{ID: "T-3", Title: "smoke 5 CLIs", Status: "pending", LegacyKind: "todo", StartedAt: time.Date(2026, 9, 19, 11, 30, 0, 0, time.UTC)},
		},
		Issues:        []SessionIssue{},
		OpenQuestions: []string{"smoke real atras de cada commit?"},
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	err = fn()
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy: %v", err)
	}
	return buf.String(), err
}

func TestRunStateRead(t *testing.T) {
	root := setupStateFixture(t, sampleState())
	out, err := captureStdout(t, func() error {
		return runStateRead([]string{"-root", root})
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout nao parseia como JSON: %v\nsaida=%q", err, out)
	}
	if got["schema_version"] != SessionStateSchemaVersion {
		t.Errorf("schema_version=%v, esperava %q", got["schema_version"], SessionStateSchemaVersion)
	}
}

func TestRunStateRenderContemProjetoEBranch(t *testing.T) {
	root := setupStateFixture(t, sampleState())
	out, err := captureStdout(t, func() error {
		return runStateRender([]string{"-root", root})
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{"# STATE", "agent-sync", "feat/docs-session-2026-09-19", "## Tarefas"} {
		if !strings.Contains(out, want) {
			t.Errorf("render nao contem %q\nsaida=%q", want, out)
		}
	}
}

func TestRunStateValidateAceitaPayloadValido(t *testing.T) {
	root := setupStateFixture(t, sampleState())
	out, err := captureStdout(t, func() error {
		return runStateValidate([]string{"-root", root})
	})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !strings.Contains(out, "ok") {
		t.Errorf("esperava 'ok' em stdout, recebi %q", out)
	}
}

// TestRunStateNextActionDevolvePrimeiraPendente valida que o shim NextAction
// (model.go:430) deriva a primeira SessionAction de Tasks (LegacyKind=todo|
// delivery + Status=pending). sampleState() cria T-3 com essas duas condicoes.
func TestRunStateNextActionDevolvePrimeiraPendente(t *testing.T) {
	root := setupStateFixture(t, sampleState())
	out, err := captureStdout(t, func() error {
		return runStateNextAction([]string{"-root", root})
	})
	if err != nil {
		t.Fatalf("next-action: %v", err)
	}
	var a SessionAction
	if err := json.Unmarshal([]byte(out), &a); err != nil {
		t.Fatalf("stdout nao parseia como action: %v\nsaida=%q", err, out)
	}
	if a.ID != "T-3" {
		t.Errorf("ID=%q, esperava T-3 (unica task com status=pending)", a.ID)
	}
	if a.Status != "pending" {
		t.Errorf("status=%q, esperava pending", a.Status)
	}
}

func TestRunStateNextActionVazioQuandoNenhumaPendente(t *testing.T) {
	state := sampleState()
	// A-37 Etapa 2: Tasks substituted for NextActions. Forcamos todas as tasks
	// (com LegacyKind=todo|delivery) para status=done para esvaziar o resultado
	// do shim NextAction.
	for i := range state.Tasks {
		if state.Tasks[i].LegacyKind == "todo" || state.Tasks[i].LegacyKind == "delivery" {
			state.Tasks[i].Status = "done"
		}
	}
	root := setupStateFixture(t, state)
	out, err := captureStdout(t, func() error {
		return runStateNextAction([]string{"-root", root})
	})
	if err != nil {
		t.Fatalf("next-action sem pendentes: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("esperava stdout vazio, recebi %q", out)
	}
}

func TestRunStateWriteRejeitaSchemaInvalido(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, SessionStateDirName), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	bad := `{"schema_version":"v1","project":{"name":"x","root":"/tmp"},"git":{"branch":"main","head":"abcdef0"},"session":{"id":"s","started_at":"2026-09-19T10:00:00Z","updated_at":"2026-09-19T10:00:00Z"}}`
	badPath := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(badPath, []byte(bad), 0o644); err != nil {
		t.Fatalf("write bad.json: %v", err)
	}
	err := runStateWrite([]string{"-root", root, "-from", badPath})
	if err == nil {
		t.Fatal("esperava erro de schema, recebi nil")
	}
	if !strings.Contains(err.Error(), "schema") {
		t.Errorf("erro nao cita schema: %v", err)
	}
}

func TestRunStateWriteAceitaJSONValido(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, SessionStateDirName), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	good := sampleState()
	goodBytes, _ := json.MarshalIndent(&good, "", "  ")
	goodPath := filepath.Join(t.TempDir(), "good.json")
	if err := os.WriteFile(goodPath, goodBytes, 0o644); err != nil {
		t.Fatalf("write good.json: %v", err)
	}
	out, err := captureStdout(t, func() error {
		return runStateWrite([]string{"-root", root, "-from", goodPath})
	})
	if err != nil {
		t.Fatalf("write valido: %v", err)
	}
	if !strings.Contains(out, "ok") {
		t.Errorf("esperava 'ok', recebi %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, SessionStateDirName, SessionStateFileName)); err != nil {
		t.Errorf("arquivo nao foi escrito: %v", err)
	}
}

// TestRunStateWriteSmokeCriterio6Taxonomia satisfaz o criterio 6 do
// docs/ADR-taxonomia-estado.md (Proposto → Aceito): "Smoke real: adicionar
// 1 task, 1 issue, 1 decision novos; verificar render". Usa o subcommand
// equivalente a 'agent-sync task add'/'issue add' (que nao existe ainda —
// ADR anti-over-engineering explicito) via 'state write -from <JSON>'.
// Verifica end-to-end: serializa fixture com 3 itens novos, escreve, valida
// schema, renderiza, confirma que os 3 IDs aparecem no STATE.md gerado.
func TestRunStateWriteSmokeCriterio6Taxonomia(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, SessionStateDirName), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	now := time.Date(2026, 9, 24, 0, 30, 0, 0, time.UTC)
	s := sampleState()
	s.Decisions = append(s.Decisions, SessionDecision{
		ID:         "D-100",
		Title:      "Smoke criterio 6 taxonomia",
		Rationale:  "item sintetico para satisfazer ADR-taxonomia-estado.md criterio 6",
		MadeAt:     now,
		LegacyKind: "retrospective",
	})
	s.Tasks = append(s.Tasks, SessionTask{
		ID:         "T-100",
		Title:      "Smoke criterio 6 taxonomia (task sintetica)",
		Status:     "pending",
		LegacyKind: "todo",
		StartedAt:  now,
	})
	s.Issues = append(s.Issues, SessionIssue{
		ID:          "I-100",
		Title:       "Smoke criterio 6 taxonomia (issue sintetica)",
		Severity:    "low",
		LegacyKind:  "gap",
		Description: "item sintetico para satisfazer ADR-taxonomia-estado.md criterio 6",
		DetectedAt:  now,
		Resolved:    false,
	})

	goodBytes, err := json.MarshalIndent(&s, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	goodPath := filepath.Join(t.TempDir(), "smoke-criterio6.json")
	if err := os.WriteFile(goodPath, goodBytes, 0o644); err != nil {
		t.Fatalf("write smoke-criterio6.json: %v", err)
	}

	if _, err := captureStdout(t, func() error {
		return runStateWrite([]string{"-root", root, "-from", goodPath})
	}); err != nil {
		t.Fatalf("write smoke: %v", err)
	}

	if _, err := captureStdout(t, func() error {
		return runStateValidate([]string{"-root", root})
	}); err != nil {
		t.Fatalf("validate smoke: %v", err)
	}

	rendered, err := captureStdout(t, func() error {
		return runStateRender([]string{"-root", root})
	})
	if err != nil {
		t.Fatalf("render smoke: %v", err)
	}
	for _, want := range []string{"D-100", "T-100", "I-100"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("render nao contem %q (criterio 6 do ADR-taxonomia-estado falha)", want)
		}
	}
}

func TestRunStateUsageListaSubcommands(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := stateUsage(buf); err != nil {
		t.Errorf("stateUsage: %v", err)
	}
	for _, want := range []string{"read", "render", "validate", "next-action", "write", "migrate-from-md"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("usage nao lista %q", want)
		}
	}
}

func TestRunStateCommandDespachaSubcommand(t *testing.T) {
	cases := []string{"read", "render", "validate", "next-action", "write", "help", "migrate-from-md"}
	for _, sub := range cases {
		t.Run(sub, func(t *testing.T) {
			err := RunCommand([]string{sub, "-root", t.TempDir()})
			switch sub {
			case "migrate-from-md":
				if err == nil {
					t.Fatal("migrate-from-md devia falhar (Estágio B), passou")
				}
			default:
				if err != nil && !strings.Contains(err.Error(), "session-state") && sub != "write" {
					t.Logf("subcommand %s retornou: %v", sub, err)
				}
			}
		})
	}
}

func TestRunStateCommandSubcommandDesconhecido(t *testing.T) {
	err := RunCommand([]string{"explodir"})
	if err == nil {
		t.Fatal("esperava erro para subcommand desconhecido")
	}
	if !strings.Contains(err.Error(), "desconhecido") {
		t.Errorf("mensagem nao cita 'desconhecido': %v", err)
	}
}

// TestRunStateValidateAceitaCamposExtrasStripaDos (D-69, A-37): apos auto-
// migration em normalizeRawJSON (render.go:287), campos top-level nao
// declarados no schema sao strip-ados antes da validacao jsonschema. Esse
// teste verifica o novo comportamento: 'extra':true e silenciosamente removido
// in-memory e o validate passa. Substitui o teste antigo que esperava erro
// (quebrava porque o strip-a ja tinha acontecido antes do schema validate).
func TestRunStateValidateAceitaCamposExtrasStripaDos(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, SessionStateDirName), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	withExtra := `{"schema_version":"` + SessionStateSchemaVersion + `","project":{"name":"x","root":"/tmp"},"git":{"branch":"main","head":"abcdef0"},"session":{"id":"s","started_at":"2026-09-19T10:00:00Z","updated_at":"2026-09-19T10:00:00Z"},"extra":true,"outro_exemplo":{"nested":"x"}}`
	statePath := filepath.Join(root, SessionStateDirName, SessionStateFileName)
	if err := os.WriteFile(statePath, []byte(withExtra), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := runStateValidate([]string{"-root", root}); err != nil {
		t.Fatalf("validate agora aceita campos extras (strip-a via normalizeRawJSON): %v", err)
	}
}
