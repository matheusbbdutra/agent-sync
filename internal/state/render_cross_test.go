package state

// state_render_cross_test.go: testes cross-CLI do CLI state render.
// Migrado de state_cli_cross_test.go em 2026-09-21 (Fase 3).
// state_apply_cross_test.go: testes cross-CLI do CLI state apply.
// Migrado de state_cli_cross_test.go em 2026-09-21 (Fase 3).
import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// crossCLIFixture e uma fixture de session-state por CLI. Cada CLI simula
// um "turno" diferente com proxima task distinta, exercitando o parser e o
// roteamento de next-action quando binarios diferentes (representados pelo
// mesmo codepath neste smoke) leem o mesmo diretorio .agent-sync/.
//
// A-37 Etapa 2: NextActions -> Tasks. O subcommand 'state next-action'
// continua funcionando via shim NextAction() que le de Tasks com
// legacy_kind=todo|delivery.
func crossCLIFixture(cliName string, actionID string, actionTitle string) SessionState {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	return SessionState{
		SchemaVersion: SessionStateSchemaVersion,
		Project: SessionProject{
			Name: "agent-sync",
			Root: "/srv/repos/agent-sync",
		},
		Git: SessionGit{
			Branch: "feat/docs-session-2026-09-19",
			Head:   "b03e1a1",
		},
		Session: SessionMeta{
			ID:        "sess-" + cliName,
			StartedAt: now,
			UpdatedAt: now,
		},
		Tasks: []SessionTask{
			{ID: actionID, Title: actionTitle, Status: "pending", LegacyKind: "todo", StartedAt: now},
		},
		Decisions:     []SessionDecision{},
		Issues:        []SessionIssue{},
		OpenQuestions: []string{},
	}
}

func crossCLIDirs(t *testing.T) map[string]string {
	t.Helper()
	dirs := map[string]string{}
	for _, cli := range []string{"claude", "opencode", "agy", "codex", "cursor-agent"} {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, SessionStateDirName), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", cli, err)
		}
		dirs[cli] = root
	}
	return dirs
}

func writeFixture(t *testing.T, root string, s SessionState) {
	t.Helper()
	if err := WriteSessionState(root, s); err != nil {
		t.Fatalf("WriteSessionState: %v", err)
	}
}

// TestRunStateCrossCLINextActionImplementa5RotasDoEstagioA.
//
// A ADR-002 Estagio A exige smoke nas 5 CLIs na ordem Claude -> OpenCode ->
// Antigravity -> Codex -> Cursor. Como o dispatch e o mesmo (binario unico
// chama jsonschema.Validate + ReadSessionState + NextAction), este teste
// verifica que cada CLIrouted-roots produz o contrato esperado:
//
//   - JSON valido contra schema v1
//   - next-action devolve a unica action pendente do fixture
//   - render contem o branch atual e a primeira acao
//
// Falha em qualquer ponto reverte a fila (regra da ADR-002).
func TestRunStateCrossCLINextActionImplementa5RotasDoEstagioA(t *testing.T) {
	dirs := crossCLIDirs(t)

	actionByCLI := map[string]struct{ id, title string }{
		"claude":       {"act-claude-handoff", "Claude leu snapshot anterior e propaga contexto"},
		"opencode":     {"act-opencode-hook", "OpenCode injeta next-action em session.idle"},
		"agy":          {"act-agy-preinvoke", "Antigravity PreInvocation consulta STATE.md"},
		"codex":        {"act-codex-adapter", "Codex adapter ja emulado (ja implementa)"},
		"cursor-agent": {"act-cursor-stop", "Cursor stop hook consulta next-action (Trilha A)"},
	}

	for _, cliName := range []string{"claude", "opencode", "agy", "codex", "cursor-agent"} {
		cliName := cliName
		t.Run(cliName, func(t *testing.T) {
			root := dirs[cliName]
			a := actionByCLI[cliName]
			writeFixture(t, root, crossCLIFixture(cliName, a.id, a.title))

			if err := runStateValidate([]string{"-root", root}); err != nil {
				t.Fatalf("[%s] validate: %v", cliName, err)
			}

			out, err := captureStdout(t, func() error {
				return runStateNextAction([]string{"-root", root})
			})
			if err != nil {
				t.Fatalf("[%s] next-action: %v", cliName, err)
			}
			var got SessionAction
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("[%s] next-action stdout nao parseia: %v\nsaida=%q", cliName, err, out)
			}
			if got.ID != a.id {
				t.Errorf("[%s] next-action id=%q, esperava %q", cliName, got.ID, a.id)
			}
			if got.Title != a.title {
				t.Errorf("[%s] next-action title=%q, esperava %q", cliName, got.Title, a.title)
			}
			if got.Status != "pending" {
				t.Errorf("[%s] next-action status=%q, esperava pending", cliName, got.Status)
			}
		})
	}
}

// TestRunStateCrossCLIRenderEstavelEntre5CLIs verifica que render produz
// o mesmo header + secoes para fixtures com mesmos project/branch. As
// diferencas esperadas sao apenas no session.id e action.id (que sao
// especificos de cada CLI). Falha representa regressao no template.
func TestRunStateCrossCLIRenderEstavelEntre5CLIs(t *testing.T) {
	dirs := crossCLIDirs(t)
	for cliName, root := range dirs {
		cliName, root := cliName, root
		writeFixture(t, root, crossCLIFixture(cliName, "act-x", "acao comum"))
		t.Run(cliName, func(t *testing.T) {
			out, err := captureStdout(t, func() error {
				return runStateRender([]string{"-root", root})
			})
			if err != nil {
				t.Fatalf("[%s] render: %v", cliName, err)
			}
			for _, want := range []string{"# STATE", "agent-sync", "feat/docs-session-2026-09-19", "## Tarefas"} {
				if !contains(out, want) {
					t.Errorf("[%s] render nao contem %q", cliName, want)
				}
			}
		})
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
