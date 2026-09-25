package state

// state_migrate_test.go: testes para state_migrate.go (stub hoje).
// Migrado de state_migrate_test.go em 2026-09-21 (Fase 3).
import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sampleMD = `# STATE — demo-project

> Header.

## Estado do repositório

Some text.

## Sessão atual

More text.

## Decisões

- **D-1** ADR-001 schema versionado — consistencia cross-CLI
- **D-2** ADR-002 lockless default — atomic rename é seguro

## Próximas ações (em ordem de prioridade)

1. ✅ **Wire fix commitado** — fechado em 2026-09-19
2. ❌ Cross-process handoff — pendente
3. Implementar event log (ADR-003)

## Bloqueios / perguntas abertas

- Sem git no CI

## Perguntas em aberto

- Open: heurística cobre todos os formatos?
- Open: migration best-effort é aceitável?
`

// TestParseStateMDSimplesExtraiTudo valida o parse de decisions + open_questions
// a partir de STATE.md. A-37 Etapa 2 (D-69) removeu NextActions/Blockers do
// struct e da chamada de parseStateMD: Tasks/Issues nao sao mais populados
// por este caminho (migracao agora vive em runStateMigrateTaxonomy separado).
// Teste checa o subset que ainda e populado aqui.
func TestParseStateMDSimplesExtraiTudo(t *testing.T) {
	s, warnings := parseStateMD([]byte(sampleMD), "/tmp/demo-project")
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
	if s.Project.Name != "demo-project" {
		t.Errorf("Project.Name=%q, esperava demo-project", s.Project.Name)
	}
	if len(s.Decisions) != 2 {
		t.Errorf("Decisions=%d, esperava 2", len(s.Decisions))
	}
	// A-37 Etapa 2: Tasks/Issues nao populados por parseStateMD (removido).
	// Migracao NextActions/Blockers -> Tasks/Issues vive em runStateMigrateTaxonomy.
	if len(s.Tasks) != 0 {
		t.Errorf("Tasks=%d, esperava 0 (A-37 Etapa 2: parseStateMD nao popula Tasks)", len(s.Tasks))
	}
	if len(s.Issues) != 0 {
		t.Errorf("Issues=%d, esperava 0 (A-37 Etapa 2: parseStateMD nao popula Issues)", len(s.Issues))
	}
	if len(s.OpenQuestions) != 2 {
		t.Errorf("OpenQuestions=%d, esperava 2", len(s.OpenQuestions))
	}
}

func TestParseStateMDFallbackParaFilepathBaseQuandoSemH1(t *testing.T) {
	md := "# titulo generico sem prefixo STATE —\n\n## Decisões\n\n- x\n"
	s, _ := parseStateMD([]byte(md), "/tmp/myproj")
	if s.Project.Name != "myproj" {
		t.Errorf("Project.Name=%q, esperava myproj (filepath.Base do root)", s.Project.Name)
	}
}

func TestParseStateMDSemSecoesEmiteWarnings(t *testing.T) {
	md := "# STATE — vazio\n\nApenas titulo, sem secoes.\n"
	_, warnings := parseStateMD([]byte(md), "/tmp")
	if len(warnings) < 4 {
		t.Errorf("esperava >=4 warnings (decisões/próximas/bloqueios/perguntas), veio %d: %v",
			len(warnings), warnings)
	}
}

func TestSplitMDSectionsIgnoraH3(t *testing.T) {
	md := "## A\n### A.1\ntexto do h3\n## B\ntexto de b\n"
	sections := splitMDSections(md)
	if _, ok := sections["a"]; !ok {
		t.Error("section 'a' nao encontrada")
	}
	if !strings.Contains(sections["a"], "### A.1") {
		t.Error("h3 deveria aparecer dentro do texto da section 'a'")
	}
	if sections["b"] == "" {
		t.Error("section 'b' deveria ter texto")
	}
}

// TestExtractActionsReconheceEmojis removido em A-37 Etapa 2 (D-69):
// parseStateMD deixou de popular s.Tasks (NextActions/Blockers foram
// removidos do struct). Os regexes reDoneMarker/reBlockedMarker que esse
// teste exercia estao em extractActionsFromSection, que perdeu caller
// (migrate_taxonomy nao chama; migracao nova vive em runStateMigrateTaxonomy
// separado). Coverage desse regex sera restaurada quando extractActionsFromSection
// for reativada em entrega futura.

func TestRunStateMigrateFromMDGeraJSONValido(t *testing.T) {
	root := t.TempDir()
	mdPath := filepath.Join(root, "STATE.md")
	if err := os.WriteFile(mdPath, []byte(sampleMD), 0o644); err != nil {
		t.Fatalf("write STATE.md: %v", err)
	}

	err := runStateMigrateFromMD([]string{"-root", root})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	loaded, err := ReadSessionState(root)
	if err != nil {
		t.Fatalf("read apos migrate: %v", err)
	}
	if loaded.Project.Name != "demo-project" {
		t.Errorf("Project.Name=%q apos migrate", loaded.Project.Name)
	}
	// A-37 Etapa 2: Tasks nao populado por runStateMigrateFromMD (migrate_taxonomy
	// separado). Migracao live em entrega futura (Etapas 3-4 do A-37).
	if len(loaded.Tasks) != 0 {
		t.Errorf("Tasks apos migrate=%d, esperava 0 (A-37 Etapa 2: runStateMigrateFromMD nao popula Tasks)", len(loaded.Tasks))
	}

	stateBytes, err := os.ReadFile(filepath.Join(root, SessionStateDirName, SessionStateFileName))
	if err != nil {
		t.Fatalf("session-state.json missing: %v", err)
	}
	if !json.Valid(stateBytes) {
		t.Errorf("session-state.json nao parseia como JSON: %s", stateBytes)
	}
}

func TestRunStateMigrateFromMDFalhaSemStateMD(t *testing.T) {
	root := t.TempDir()
	err := runStateMigrateFromMD([]string{"-root", root})
	if err == nil {
		t.Fatal("esperava erro com STATE.md ausente")
	}
	if !strings.Contains(err.Error(), "STATE.md") {
		t.Errorf("erro nao cita STATE.md: %v", err)
	}
}

func TestRunStateMigrateFromMDContraStateMDReal(t *testing.T) {
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for parent := repoRoot; parent != "/" && parent != "."; parent = filepath.Dir(parent) {
		candidate := filepath.Join(parent, "STATE.md")
		if _, err := os.Stat(candidate); err == nil {
			data, err := os.ReadFile(candidate)
			if err != nil {
				t.Fatalf("read %s: %v", candidate, err)
			}
			s, warnings := parseStateMD(data, parent)
			// A-37 Etapa 2: Tasks/Issues nao populados por parseStateMD (migracao live
			// em runStateMigrateTaxonomy separado). O log abaixo reporta 0 para ambos,
			// que e o estado atual esperado.
			t.Logf("STATE.md real parseado: project=%s, decisions=%d, tasks=%d, issues=%d, open_questions=%d, warnings=%d",
				s.Project.Name, len(s.Decisions), len(s.Tasks), len(s.Issues), len(s.OpenQuestions), len(warnings))
			if len(s.Decisions) < 1 {
				t.Errorf("STATE.md real tem '## Decisões'; extraido %d (heurística falhou)", len(s.Decisions))
			}
			if s.Session.ID == "" || s.Session.StartedAt.IsZero() {
				t.Error("session metadata não foi populada")
			}
			if time.Since(s.Session.StartedAt) > time.Hour {
				t.Errorf("session.started_at deveria ser now; veio %v", s.Session.StartedAt)
			}
			return
		}
	}
	t.Skip("STATE.md não encontrado em ancestrais — não estamos no repo?")
}
