package audit

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildRemovalAudit_EndToEnd(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "README.md"), "# tools/cmd/memory-mcp\nreference here\n")
	mustWrite(t, filepath.Join(root, "go.mod"), "module x\n// requires tools/cmd/memory-mcp/store\n")
	mustWrite(t, filepath.Join(root, "package.json"), `{"name": "foo", "deps": ["tools/cmd/memory-mcp"]}`)
	mustWrite(t, filepath.Join(root, "schema.go"), `
const ddl = "CREATE TABLE IF NOT EXISTS memories (id INTEGER);"
`)

	ledger, err := BuildRemovalAudit(BuildRemovalAuditOptions{
		Root:        root,
		Target:      "tools/cmd/memory-mcp",
		SchemaGlobs: []string{filepath.Join(root, "schema.go")},
		Now:         time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildRemovalAudit: %v", err)
	}

	if ledger.SchemaVersion != LedgerSchemaVersion {
		t.Errorf("SchemaVersion = %q; want %q", ledger.SchemaVersion, LedgerSchemaVersion)
	}
	if ledger.ID != "tools-cmd-memory-mcp-removal" {
		t.Errorf("ID = %q; want tools-cmd-memory-mcp-removal", ledger.ID)
	}
	if ledger.Target.Raw != "tools/cmd/memory-mcp" {
		t.Errorf("Target.Raw = %q", ledger.Target.Raw)
	}
	if ledger.Snapshot.FilesScanned != 4 {
		t.Errorf("FilesScanned = %d; want 4", ledger.Snapshot.FilesScanned)
	}
	if ledger.TotalHits == 0 {
		t.Error("TotalHits should be > 0")
	}

	// README.md → docs-active
	// go.mod → lockfile-reference (go.mod é lockfile Go)
	// package.json → package-dependency
	classes := map[EvidenceClass]int{}
	for _, c := range ledger.Classes {
		classes[c.Class] = c.Count
	}
	if classes[ClassDocsActive] == 0 {
		t.Error("expected docs-active hits")
	}
	if classes[ClassPackageDependency] == 0 {
		t.Error("expected package-dependency hits")
	}

	if ledger.Verdict.Status != "needs-review" {
		t.Errorf("Verdict.Status = %q; want needs-review", ledger.Verdict.Status)
	}
	if len(ledger.Verdict.Blockers) == 0 {
		t.Error("expected at least 1 blocker")
	}

	if len(ledger.SQLTables) == 0 || ledger.SQLTables[0] != "memories" {
		t.Errorf("SQLTables = %v; want memories", ledger.SQLTables)
	}
}

func TestBuildRemovalAudit_EmptyTarget(t *testing.T) {
	_, err := BuildRemovalAudit(BuildRemovalAuditOptions{Root: "/tmp", Target: ""})
	if err == nil {
		t.Error("expected error for empty target")
	}
}

func TestBuildRemovalAudit_NoHits(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.md"), "# no refs here\n")

	ledger, err := BuildRemovalAudit(BuildRemovalAuditOptions{
		Root:   root,
		Target: "completely-unrelated-target",
	})
	if err != nil {
		t.Fatalf("BuildRemovalAudit: %v", err)
	}
	if ledger.TotalHits != 0 {
		t.Errorf("TotalHits = %d; want 0", ledger.TotalHits)
	}
	if ledger.Verdict.Status != "passed" {
		t.Errorf("Verdict.Status = %q; want passed", ledger.Verdict.Status)
	}
}

func TestBuildRemovalAudit_DefaultWalkUsesRoot(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "x.md"), "memory-mcp hit here\n")
	// não passa Paths → Walk(root) é chamado; como root não é git repo,
	// cai em walkFallback. Vai encontrar x.md.
	ledger, err := BuildRemovalAudit(BuildRemovalAuditOptions{
		Root:   root,
		Target: "memory-mcp",
	})
	if err != nil {
		t.Fatalf("BuildRemovalAudit: %v", err)
	}
	if ledger.TotalHits == 0 {
		t.Error("expected hits via walkFallback")
	}
}

func TestLedgerMarshalOrdered_StableJSON(t *testing.T) {
	ledger := &Ledger{
		SchemaVersion: LedgerSchemaVersion,
		Kind:          "removal",
		ID:            "x",
		CreatedAt:     "2026-01-01T00:00:00Z",
		UpdatedAt:     "2026-01-01T00:00:00Z",
		ByFile: map[string]int{
			"z.go": 3,
			"a.go": 1,
			"m.go": 2,
		},
		TotalHits: 6,
	}
	data1, err := ledger.MarshalOrdered()
	if err != nil {
		t.Fatalf("MarshalOrdered: %v", err)
	}
	data2, err := ledger.MarshalOrdered()
	if err != nil {
		t.Fatalf("MarshalOrdered 2: %v", err)
	}
	if string(data1) != string(data2) {
		t.Errorf("MarshalOrdered não-determinístico")
	}
	// confere que contém "a.go" antes de "m.go" e "m.go" antes de "z.go"
	s := string(data1)
	iA, iM, iZ := indexOf(s, `"a.go"`), indexOf(s, `"m.go"`), indexOf(s, `"z.go"`)
	if !(iA < iM && iM < iZ) {
		t.Errorf("ordem esperada a < m < z; got %d %d %d", iA, iM, iZ)
	}

	// também deve ser JSON válido
	var generic any
	if err := json.Unmarshal(data1, &generic); err != nil {
		t.Errorf("output não é JSON válido: %v", err)
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"tools/cmd/memory-mcp": "tools-cmd-memory-mcp",
		"helloWorld":           "helloworld",
		"":                     "target",
		"---":                  "target",
		"a__b":                 "a-b",
	}
	for in, want := range cases {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q; want %q", in, got, want)
		}
	}
}
