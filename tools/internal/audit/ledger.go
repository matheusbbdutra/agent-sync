package audit

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// LedgerSchemaVersion é a versão do schema JSON emitido por BuildRemovalAudit.
// Não segue o padrão `cartographer.audit-ledger.v1` (decisão consciente do
// ADR A-73 — divergence documentada).
const LedgerSchemaVersion = "agent-sync.audit-ledger.v1"

// Evidence é um único hit textual classificado.
type Evidence struct {
	Path        string        `json:"path"`
	LineStart   int           `json:"line_start"`
	LineEnd     int           `json:"line_end"`
	Match       string        `json:"match"`
	EvidenceKind EvidenceClass `json:"evidence_kind"`
}

// EvidenceClassEntry agrega os hits de uma classe + metadados.
type EvidenceClassEntry struct {
	Class      EvidenceClass `json:"class"`
	Status     string        `json:"status"` // "not-found" | "found" | "unknown"
	Summary    string        `json:"summary"`
	Active     []Evidence    `json:"active"`
	Count      int           `json:"count"`
	Omitted    int           `json:"omitted,omitempty"`
}

// Ledger é a estrutura serializada em JSON.
type Ledger struct {
	SchemaVersion  string                       `json:"schema_version"`
	Kind           string                       `json:"kind"` // "removal"
	ID             string                       `json:"id"`
	Target         AuditTarget                  `json:"target"`
	CreatedAt      string                       `json:"created_at"`
	UpdatedAt      string                       `json:"updated_at"`
	Snapshot       Snapshot                     `json:"snapshot"`
	Verdict        Verdict                      `json:"verdict"`
	Classes        []EvidenceClassEntry         `json:"classes"`
	SQLTables      []string                     `json:"sql_tables,omitempty"`
	ByFile         map[string]int               `json:"by_file"`
	TotalHits      int                          `json:"total_hits"`
}

type Snapshot struct {
	Root        string `json:"root"`
	FilesScanned int   `json:"files_scanned"`
	GeneratedAt string `json:"generated_at"`
}

type Verdict struct {
	Status   string   `json:"status"` // "passed" | "needs-review"
	Blockers []string `json:"blockers"`
}

// BuildRemovalAuditOptions agrupa opções para BuildRemovalAudit.
type BuildRemovalAuditOptions struct {
	Root       string
	Paths      []string                 // se vazio, Walk(Root) é chamado
	Target     string                   // obrigatório
	SQLTables  []string                 // opcional; se vazio, faz parse DDL inline
	SchemaGlobs []string                // caminhos para ExtractTablesFromFiles
	Now        time.Time                // injetável para testes
}

// BuildRemovalAudit é o orquestrador: walk → grep → classify → aggregate.
// Idempotente e determinístico (exceto Now, que é injetável).
func BuildRemovalAudit(opts BuildRemovalAuditOptions) (*Ledger, error) {
	if opts.Target == "" {
		return nil, fmt.Errorf("audit: target obrigatório")
	}
	if opts.Root == "" {
		return nil, fmt.Errorf("audit: root obrigatório")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	target, err := NewTarget(opts.Target)
	if err != nil {
		return nil, err
	}

	paths := opts.Paths
	if len(paths) == 0 {
		paths, err = Walk(opts.Root)
		if err != nil {
			return nil, fmt.Errorf("walk: %w", err)
		}
	}

	sqlTables := opts.SQLTables
	if sqlTables == nil {
		sqlTables = ExtractTablesFromFiles(opts.Root, opts.SchemaGlobs)
	}

	hits, err := LiteralHits(opts.Root, paths, target, DefaultMaxFileBytes)
	if err != nil {
		return nil, fmt.Errorf("literal-hits: %w", err)
	}

	classes := classifyHits(hits, target, sqlTables)
	verdict := computeVerdict(classes)
	byFile := countByFile(hits)

	return &Ledger{
		SchemaVersion: LedgerSchemaVersion,
		Kind:          "removal",
		ID:            slug(target.Raw) + "-removal",
		Target:        target,
		CreatedAt:     now.Format(time.RFC3339),
		UpdatedAt:     now.Format(time.RFC3339),
		Snapshot: Snapshot{
			Root:         opts.Root,
			FilesScanned: len(paths),
			GeneratedAt:  now.Format(time.RFC3339),
		},
		Verdict:   verdict,
		Classes:   classes,
		SQLTables: sqlTables,
		ByFile:    byFile,
		TotalHits: len(hits),
	}, nil
}

// classifyHits distribui cada hit pela sua classe e monta o agregado final.
func classifyHits(hits []FileHit, target AuditTarget, sqlTables []string) []EvidenceClassEntry {
	bucket := make(map[EvidenceClass][]Evidence, len(AllClasses()))
	for _, h := range hits {
		cls := ClassifyHit(h, target, sqlTables)
		bucket[cls] = append(bucket[cls], Evidence{
			Path:        h.Path,
			LineStart:   h.Line,
			LineEnd:     h.Line,
			Match:       h.Match,
			EvidenceKind: cls,
		})
	}

	out := make([]EvidenceClassEntry, 0, len(AllClasses()))
	for _, cls := range AllClasses() {
		active := bucket[cls]
		entry := EvidenceClassEntry{
			Class:   cls,
			Active:  active,
			Count:   len(active),
			Status:  classStatus(cls, active),
			Summary: classSummary(cls, len(active)),
		}
		out = append(out, entry)
	}
	return out
}

func classStatus(cls EvidenceClass, active []Evidence) string {
	switch {
	case cls == ClassUnknownLiteralHit && len(active) > 0:
		return "unknown"
	case len(active) > 0:
		return "found"
	default:
		return "not-found"
	}
}

func classSummary(cls EvidenceClass, count int) string {
	if count == 0 {
		return fmt.Sprintf("No %s hits found.", cls)
	}
	return fmt.Sprintf("Found %d %s hit(s).", count, cls)
}

func computeVerdict(classes []EvidenceClassEntry) Verdict {
	var blockers []string
	for _, c := range classes {
		if c.Count > 0 {
			blockers = append(blockers, fmt.Sprintf("%d active %s hit(s) remain", c.Count, c.Class))
		}
	}
	status := "passed"
	if len(blockers) > 0 {
		status = "needs-review"
	}
	return Verdict{Status: status, Blockers: blockers}
}

func countByFile(hits []FileHit) map[string]int {
	m := make(map[string]int, len(hits))
	for _, h := range hits {
		m[h.Path]++
	}
	// ordenação determinística via serialização de chaves ordenadas
	// (json.Marshal não ordena maps; mas para diff humano, ordenamos
	// aqui via struct separada).
	return m
}

// MarshalOrderedByFile serializa o ledger JSON com `by_file` ordenado
// alfabeticamente para diffs estáveis. Implementação manual para evitar
// dependência de terceiros.
func (l *Ledger) MarshalOrdered() ([]byte, error) {
	type orderedMap struct {
		K string `json:"k"`
		V int    `json:"v"`
	}
	keys := make([]string, 0, len(l.ByFile))
	for k := range l.ByFile {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := make([]orderedMap, 0, len(keys))
	for _, k := range keys {
		ordered = append(ordered, orderedMap{K: k, V: l.ByFile[k]})
	}

	// monta um wrapper com by_file como slice ordenado
	type auditTargetJSON struct {
		Raw      string   `json:"raw"`
		Matchers []string `json:"matchers"`
	}
	wrapper := struct {
		SchemaVersion string             `json:"schema_version"`
		Kind          string             `json:"kind"`
		ID            string             `json:"id"`
		Target        auditTargetJSON    `json:"target"`
		CreatedAt     string             `json:"created_at"`
		UpdatedAt     string             `json:"updated_at"`
		Snapshot      Snapshot           `json:"snapshot"`
		Verdict       Verdict            `json:"verdict"`
		Classes       []EvidenceClassEntry `json:"classes"`
		SQLTables     []string           `json:"sql_tables,omitempty"`
		ByFile        []orderedMap       `json:"by_file"`
		TotalHits     int                `json:"total_hits"`
	}{
		SchemaVersion: l.SchemaVersion,
		Kind:          l.Kind,
		ID:            l.ID,
		Target: auditTargetJSON{
			Raw:      l.Target.Raw,
			Matchers: matchersToLabels(l.Target.Matchers),
		},
		CreatedAt: l.CreatedAt,
		UpdatedAt: l.UpdatedAt,
		Snapshot:  l.Snapshot,
		Verdict:   l.Verdict,
		Classes:   l.Classes,
		SQLTables: l.SQLTables,
		ByFile:    ordered,
		TotalHits: l.TotalHits,
	}
	return json.MarshalIndent(wrapper, "", "  ")
}

func matchersToLabels(ms []TargetMatcher) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Label
	}
	return out
}

// slug espelha cartographer audit.ts:689-691 (kebab-case).
func slug(s string) string {
	var b []rune
	prevDash := true
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b = append(b, r)
			prevDash = false
		default:
			if !prevDash {
				b = append(b, '-')
				prevDash = true
			}
		}
	}
	out := strings.TrimRight(string(b), "-")
	if out == "" {
		return "target"
	}
	return out
}
