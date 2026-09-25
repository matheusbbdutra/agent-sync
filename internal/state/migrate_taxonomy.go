package state

// state_migrate_taxonomy.go: migracao da taxonomia 4-categorias (D/A/C/B)
// para 3-categorias (decisions/tasks/issues) proposta pela ADR D-46
// (commit 0b8cc28, 2026-09-21).
//
// Mapping:
//
//	decisions     -> decisions (mantem; adiciona legacy_kind)
//	next_actions  -> tasks     (converte; legacy_kind: "todo"|"delivery")
//	blockers      -> issues    (converte; legacy_kind: "bug"|"gap"|"blocker")
//	C-N (trilhas) -> tags em tasks/issues (sem migracao automatica — nao
//	                 existem no session-state.json deste projeto)
//
// Nao destrutivo: o JSON de entrada e' lido, a migracao gera um novo
// struct, e WriteSessionState persiste o resultado apos validacao contra
// schema 1.1. Backup do original e' gravado em
// .agent-sync/session-state.json.bak antes do write.
//
// Idempotente: rodar 2x produz mesmo resultado (campos legacy_kind ja
// presentes nao sao duplicados; tasks/issues duplicados sao detectados
// por ID).
//
// Nao destruir dados (reversibilidade via git revert).
import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// runStateMigrateTaxonomy implementa o subcommand `state migrate-taxonomy`
// da ADR D-46. Le session-state.json atual, classifica items conforme
// taxonomia nova (3 tipos), adiciona legacy_kind, e persiste via
// WriteSessionState (que valida contra schema 1.1 antes do write).
// Backup do original fica em .agent-sync/session-state.json.bak.
func runStateMigrateTaxonomy(args []string) error {
	f := newStateFlags("agent-sync state migrate-taxonomy")
	if err := f.parse(args, os.Stderr); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}

	src := statePath(root)
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("ler session-state.json: %w", err)
	}

	// Backup antes do write (D-48.x).
	bakPath := src + ".bak"
	if err := os.WriteFile(bakPath, data, 0o644); err != nil {
		return fmt.Errorf("backup para %s: %w", bakPath, err)
	}

	var s SessionState
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("parse session-state.json: %w", err)
	}

	s.Decisions = classifyDecisions(s.Decisions)

	// Bump schema_version: aceita 1.0 -> 1.2 (taxonomia + legacy_kind)
	// ou 1.1 -> 1.2 (so legacy_kind). Se ja for 1.2, mantem.
	if s.SchemaVersion == "1.0" || s.SchemaVersion == "1.1" {
		s.SchemaVersion = SessionStateSchemaVersion
	}

	// Mantem next_actions/blockers como historico (nao destrutivo -
	// reversibilidade via git revert). Schema 1.1 aceita ambos.

	s.normalize()
	if err := s.validate(); err != nil {
		return fmt.Errorf("validacao pos-migracao: %w", err)
	}

	if err := WriteSessionState(root, s); err != nil {
		return fmt.Errorf("escrita pos-migracao: %w", err)
	}
	fmt.Printf("ok (%d tasks, %d issues, %d decisions; backup em %s)\n",
		len(s.Tasks), len(s.Issues), len(s.Decisions), bakPath)
	return nil
}

// classifyDecisions adiciona legacy_kind heuristico a cada decision

// classifyDecisions adiciona legacy_kind heuristico a cada decision
// (D-48.x.x, schema 1.2). Heuristica:
//   - "architectural" se Title comeca com "ADR-" ou contem "Decisao"/"ADR"
//   - "retrospective" se Title comeca com "[cancelado" ou contem "log retroativo"
//   - "factual" senao (default - maioria dos D-N hoje sao observacoes factuais)
func classifyDecisions(decisions []SessionDecision) []SessionDecision {
	out := make([]SessionDecision, 0, len(decisions))
	for _, d := range decisions {
		title := strings.ToLower(d.Title)
		switch {
		case strings.HasPrefix(d.Title, "ADR-") || strings.Contains(title, "adr-"):
			d.LegacyKind = "architectural"
		case strings.HasPrefix(d.Title, "[cancelado") || strings.Contains(title, "log retroativo"):
			d.LegacyKind = "retrospective"
		default:
			d.LegacyKind = "factual"
		}
		out = append(out, d)
	}
	return out
}
